// Package http es el adapter HTTP de la E-Invoice API: traduce HTTP ↔ casos de uso. No contiene reglas de negocio.
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

	"rdl/einvoice-api/internal/adapters/http/problem"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/pkg/correlation"
	"rdl/einvoice-api/pkg/requestinfo"
	"rdl/einvoice-api/pkg/tenancy"
)

// Deps son las dependencias del router. Los grupos de handlers en nil no registran rutas (lo usan los tests).
type Deps struct {
	Log           *slog.Logger
	Health        *health.Handler
	Verifier      tenancy.TokenVerifier
	Memberships   tenancy.MembershipResolver
	FiscalProfile *FiscalProfileHandlers
	FiscalStatus  *FiscalStatusHandlers
	FiscalConfig  *FiscalConfigHandlers
	Catalogs      *CatalogHandlers
}

// NewRouter arma el mux con el router estándar de Go 1.22+, igual que Platform y Receivables.
//
//	/healthz, /readyz    públicas
//	/v1/...              JWT válido + TenantContext (org_id del token + membresía activa revalidada en core)
//	/internal/v1/...     lo mismo: el BFF reenvía el JWT del usuario (bff-internal.yaml)
func NewRouter(d Deps) http.Handler {
	log := d.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	errs := errorResponder{log: log}
	tenancyFail := tenancyErrorWriter(log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)
	mux.HandleFunc("/", notFound)

	v1, internal := newRoutes(), newRoutes()
	if d.FiscalProfile != nil {
		d.FiscalProfile.register(v1, errs.write)
	}
	if d.FiscalConfig != nil {
		d.FiscalConfig.register(v1, errs.write)
	}
	if d.Catalogs != nil {
		d.Catalogs.register(v1, errs.write)
	}
	if d.FiscalStatus != nil {
		d.FiscalStatus.register(internal, errs.write)
	}
	if d.Verifier != nil && d.Memberships != nil {
		guard := func(h http.Handler) http.Handler {
			return tenancy.Authenticate(d.Verifier, tenancyFail)(tenancy.RequireOrganization(d.Memberships, tenancyFail)(h))
		}
		mux.Handle("/v1/", guard(v1.finish().mux))
		mux.Handle("/internal/v1/", guard(internal.finish().mux))
	}

	var h http.Handler = mux
	h = accessLog(log, h)
	h = recoverer(log, h)
	h = requestinfo.Middleware(h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, "einvoice-api")
}

func notFound(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound) }

// routes envuelve un ServeMux para que cada ruta con método tenga su 405 en Problem Details (el mux estándar
// lo respondería en texto plano) y para anotar el patrón en trazas y logs aunque el mux esté anidado.
// Origen: RDL.Receivables.API.
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
