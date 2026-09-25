// Package http traduce HTTP ↔ casos de uso. No contiene reglas de negocio.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/correlation"
	"rdl/receivables-api/pkg/requestinfo"
	"rdl/receivables-api/pkg/tenancy"
)

type Deps struct {
	Log         *slog.Logger
	Health      *health.Handler
	Verifier    tenancy.TokenVerifier
	Memberships tenancy.MembershipResolver
	Receivables *ReceivableHandlers
	Payments    *PaymentHandlers
	Collection  *CollectionHandlers
}

// NewRouter arma el mux con el router estándar de Go 1.22+, igual que Platform.
//
//	/healthz, /readyz    públicas
//	/v1/...              JWT válido + TenantContext (org_id del token + membresía activa revalidada en core)
//	/internal/v1/...     lo mismo: el BFF reenvía el JWT del usuario (R10, bff-internal.yaml)
//
// Todo /v1 e /internal/v1 operan dentro de una organización, así que el TenantContext se exige para todo el prefijo.
func NewRouter(d Deps) http.Handler {
	errs := errorResponder{log: d.Log}
	tenancyFail := tenancyErrorWriter(d.Log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)
	mux.HandleFunc("/", notFound)

	v1 := newRoutes()
	internal := newRoutes()
	if d.Receivables != nil {
		d.Receivables.register(v1, errs.write)
		d.Receivables.registerInternal(internal, errs.write)
	}
	if d.Payments != nil {
		d.Payments.register(v1, errs.write)
	}
	if d.Collection != nil {
		d.Collection.register(v1, errs.write)
	}
	if d.Verifier != nil && d.Memberships != nil {
		guard := func(h http.Handler) http.Handler {
			return tenancy.Authenticate(d.Verifier, tenancyFail)(tenancy.RequireOrganization(d.Memberships, tenancyFail)(h))
		}
		mux.Handle("/v1/", guard(v1.finish().mux))
		mux.Handle("/internal/v1/", guard(internal.finish().mux))
	}

	var h http.Handler = mux
	h = accessLog(d.Log, h)
	h = recoverer(d.Log, h)
	h = requestinfo.Middleware(h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, "receivables-api")
}

func notFound(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound) }

// routes envuelve un ServeMux para que cada ruta con método tenga su 405 en Problem Details (el mux estándar
// lo respondería en texto plano) y para anotar el patrón en trazas y logs aunque el mux esté anidado.
// Origen: RDL.Platform.API. Cambio: el 405 se registra al final (finish) y por método, no con la ruta sin método:
// "/v1/receivables/aging" sin método chocaría con "GET /v1/receivables/{id}" en el mux de Go.
type routes struct {
	mux     *http.ServeMux
	methods map[string]map[string]bool // ruta → métodos con handler
	order   []string
}

func newRoutes() *routes {
	rt := &routes{mux: http.NewServeMux(), methods: map[string]map[string]bool{}}
	rt.mux.HandleFunc("/", notFound)
	return rt
}

func (rt *routes) handle(pattern string, h http.HandlerFunc) {
	rt.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		setRoute(r.Context(), pattern)
		h(w, r)
	})
	method, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod {
		return
	}
	if rt.methods[path] == nil {
		rt.methods[path] = map[string]bool{}
		rt.order = append(rt.order, path)
	}
	rt.methods[path][method] = true
}

// finish registra el 405 para cada método sin handler de cada ruta. Se llama una vez, después de todas las rutas.
func (rt *routes) finish() *routes {
	notAllowed := func(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.MethodNotAllowed) }
	for _, path := range rt.order {
		for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			if !rt.methods[path][m] {
				rt.mux.HandleFunc(m+" "+path, notAllowed)
			}
		}
	}
	return rt
}

type routeKey struct{}

type routeHolder struct{ pattern string }

func setRoute(ctx context.Context, pattern string) {
	if h, ok := ctx.Value(routeKey{}).(*routeHolder); ok {
		h.pattern = pattern
	}
	// Nombre de ruta y no URL: evita alta cardinalidad en las trazas.
	trace.SpanFromContext(ctx).SetName(pattern)
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				log.ErrorContext(r.Context(), "panic en handler", slog.Any("panic", rec))
				problem.Write(w, r, problem.Internal)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		route := &routeHolder{}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), routeKey{}, route)))
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		log.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("route", route.pattern),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
