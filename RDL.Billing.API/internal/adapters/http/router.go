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

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/platform/health"
	"rdl/billing-api/pkg/correlation"
	"rdl/billing-api/pkg/requestinfo"
	"rdl/billing-api/pkg/tenancy"
)

type Deps struct {
	Log         *slog.Logger
	Health      *health.Handler
	Verifier    tenancy.TokenVerifier
	Memberships tenancy.MembershipResolver
	Customers   *CustomerHandlers
	Products    *ProductHandlers
	Invoices    *InvoiceHandlers
	Sequences   *SequenceHandlers
}

// NewRouter arma el mux con el router estándar de Go 1.22+, igual que Platform (su ADR 0005).
//
//	/healthz, /readyz   públicas
//	/v1/...             JWT válido + TenantContext (org_id del token + membresía activa revalidada en core).
//	                    Billing no tiene rutas sin organización: todas operan sobre la organización activa.
//	/internal/v1/...    las que compone el Portal Gateway (openapi/bff-internal.yaml). Misma cadena que /v1:
//	                    el gateway reenvía el token del usuario, no uno de servicio. Que solo se alcancen por la
//	                    red interna es trabajo del despliegue; el gateway además nunca las expone al portal.
func NewRouter(d Deps) http.Handler {
	errs := errorResponder{log: d.Log}
	tenancyFail := tenancyErrorWriter(d.Log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)
	mux.HandleFunc("/", notFound)

	v1 := newRoutes()
	if d.Customers != nil {
		d.Customers.register(v1, errs.write)
	}
	if d.Products != nil {
		d.Products.register(v1, errs.write)
	}
	if d.Invoices != nil {
		d.Invoices.register(v1, errs.write)
	}
	if d.Sequences != nil {
		d.Sequences.register(v1, errs.write)
	}
	withTenant := tenancy.RequireOrganization(d.Memberships, tenancyFail)(v1.mux)
	authenticated := tenancy.Authenticate(d.Verifier, tenancyFail)(withTenant)
	mux.Handle("/v1/", authenticated)
	mux.Handle("/internal/v1/", authenticated)

	var h http.Handler = mux
	h = accessLog(d.Log, h)
	h = recoverer(d.Log, h)
	h = requestinfo.Middleware(h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, "billing-api")
}

func notFound(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound) }

// routes envuelve un ServeMux para que cada ruta con método tenga su 405 en Problem Details (el mux estándar
// lo respondería en texto plano) y para anotar el patrón en trazas y logs aunque el mux esté anidado.
type routes struct {
	mux   *http.ServeMux
	paths map[string]bool
}

func newRoutes() *routes {
	rt := &routes{mux: http.NewServeMux(), paths: map[string]bool{}}
	rt.mux.HandleFunc("/", notFound)
	return rt
}

func (rt *routes) handle(pattern string, h http.HandlerFunc) {
	rt.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		setRoute(r.Context(), pattern)
		h(w, r)
	})
	_, path, hasMethod := strings.Cut(pattern, " ")
	if !hasMethod || rt.paths[path] {
		return
	}
	rt.paths[path] = true
	rt.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		problem.Write(w, r, problem.MethodNotAllowed)
	})
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
