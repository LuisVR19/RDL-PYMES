package http_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	httpadapter "rdl/portal-gateway/internal/adapters/http"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/platform/config"
	"rdl/portal-gateway/internal/platform/health"
	"rdl/portal-gateway/internal/wiring"
	"rdl/portal-gateway/pkg/identity"
)

// Las pruebas van contra el router REAL armado por wiring, con las cuatro APIs simuladas con httptest:
// lo que se prueba es lo que corre en producción, no una copia del cableado.

const (
	// #nosec G101 -- no es una credencial: es el valor que las APIs falsas esperan ver llegar.
	userToken   = "token.del.usuario.firmado"
	userSubject = "8f1c4b2e-0000-4000-8000-000000000001"
	orgID       = "3f2a7c10-0000-4000-8000-0000000000aa"
)

// recorded es lo que una API falsa vio llegar.
type recorded struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// upstream es una API falsa que registra cada petición y responde lo que le diga el caso de prueba.
type upstream struct {
	server *httptest.Server
	mu     sync.Mutex
	got    []recorded
	// respond puede cambiar entre llamadas (para probar reintentos).
	respond func(w http.ResponseWriter, r *http.Request, attempt int)
}

func newUpstream(t *testing.T) *upstream {
	t.Helper()
	u := &upstream{}
	u.respond = func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		u.mu.Lock()
		u.got = append(u.got, recorded{
			Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
			Header: r.Header.Clone(), Body: string(body),
		})
		attempt := len(u.got)
		respond := u.respond
		u.mu.Unlock()
		respond(w, r, attempt)
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *upstream) requests() []recorded {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]recorded(nil), u.got...)
}

func (u *upstream) calls() int { return len(u.requests()) }

func (u *upstream) last(t *testing.T) recorded {
	t.Helper()
	got := u.requests()
	if len(got) == 0 {
		t.Fatal("la API no recibió ninguna llamada")
	}
	return got[len(got)-1]
}

func (u *upstream) setResponse(fn func(w http.ResponseWriter, r *http.Request, attempt int)) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.respond = fn
}

// status responde siempre ese código, con Problem Details como lo haría una API de verdad.
func status(code int, problemType string) func(http.ResponseWriter, *http.Request, int) {
	return func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(code)
		_, _ = io.WriteString(w, `{"type":"`+problemType+`","title":"probando","status":`+itoa(code)+`}`)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

type fakeVerifier struct{ err error }

func (f fakeVerifier) Verify(_ context.Context, raw string) (identity.Identity, error) {
	if f.err != nil {
		return identity.Identity{}, f.err
	}
	return identity.Identity{
		Subject:        userSubject,
		OrganizationID: uuid.MustParse(orgID),
		RawToken:       raw,
	}, nil
}

// gateway es el sistema bajo prueba: el router real más las APIs falsas que lo rodean.
type gateway struct {
	handler   http.Handler
	upstreams map[routes.Service]*upstream
}

type options struct {
	// unconfigured deja esos servicios sin URL, como E-Invoice y Receivables hoy.
	unconfigured []routes.Service
	verifier     httpadapter.Verifier
	timeouts     map[routes.Service]time.Duration
	budget       time.Duration
}

func newGateway(t *testing.T, opt options) *gateway {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	g := &gateway{upstreams: map[routes.Service]*upstream{}}
	cfg := config.Config{
		ServiceName: "portal-gateway-test",
		HTTP:        config.HTTP{MaxRequestBody: 1 << 20},
		CORS:        config.CORS{AllowedOrigins: []string{"http://localhost:5173"}},
		Upstreams: config.Upstreams{
			Services:             map[routes.Service]config.Upstream{},
			Budget:               orDefault(opt.budget, 5*time.Second),
			BillingSummarySource: config.SummarySourcePublic,
		},
	}
	for _, s := range routes.Services() {
		timeout := orDefault(opt.timeouts[s], 2*time.Second)
		if contains(opt.unconfigured, s) {
			cfg.Upstreams.Services[s] = config.Upstream{Timeout: timeout}
			continue
		}
		u := newUpstream(t)
		g.upstreams[s] = u
		cfg.Upstreams.Services[s] = config.Upstream{URL: u.server.URL, Timeout: timeout}
	}

	verifier := opt.verifier
	if verifier == nil {
		verifier = fakeVerifier{}
	}
	deps := wiring.Deps(cfg, log, verifier, wiring.Clients(cfg, log), health.New(log, time.Second))
	handler, err := httpadapter.NewRouter(deps)
	if err != nil {
		t.Fatalf("armando el router: %v", err)
	}
	g.handler = handler
	return g
}

func (g *gateway) up(t *testing.T, s routes.Service) *upstream {
	t.Helper()
	u, ok := g.upstreams[s]
	if !ok {
		t.Fatalf("el servicio %s no está configurado en esta prueba", s)
	}
	return u
}

// do manda una petición al gateway con un token válido, salvo que la prueba cambie los headers.
func (g *gateway) do(req *http.Request) *httptest.ResponseRecorder {
	if req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+userToken)
	}
	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, req)
	return rec
}

func (g *gateway) get(path string) *httptest.ResponseRecorder {
	return g.do(httptest.NewRequest(http.MethodGet, path, nil))
}

// totalCalls cuenta las llamadas recibidas por todas las APIs.
func (g *gateway) totalCalls() int {
	n := 0
	for _, u := range g.upstreams {
		n += u.calls()
	}
	return n
}

func contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func orDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

var errInvalidToken = errors.New("token inválido")
