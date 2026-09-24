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

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/correlation"
	"rdl/platform-api/pkg/requestinfo"
	"rdl/platform-api/pkg/tenancy"
)

type Deps struct {
	Log           *slog.Logger
	Health        *health.Handler
	Verifier      tenancy.TokenVerifier
	Memberships   tenancy.MembershipResolver
	Me            *MeHandlers
	Organizations *OrganizationHandlers
	Members       *MemberHandlers
	Invitations   *InvitationHandlers
	Branches      *BranchHandlers
}

// NewRouter arma el mux con el router estándar de Go 1.22+ (docs/decisiones/0005-router-y-dependencias.md).
//
//	/healthz, /readyz                    públicas
//	/v1/...                              requieren JWT válido
//	/v1/organizations/current[/...]      requieren además TenantContext (org_id del token + membresía activa)
func NewRouter(d Deps) http.Handler {
	errs := errorResponder{log: d.Log}
	tenancyFail := tenancyErrorWriter(d.Log)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)
	mux.HandleFunc("/", notFound)

	current := newRoutes()
	if d.Organizations != nil {
		d.Organizations.registerCurrent(current, errs.write)
	}
	if d.Members != nil {
		d.Members.register(current, errs.write)
	}
	if d.Invitations != nil {
		d.Invitations.registerCurrent(current, errs.write)
	}
	if d.Branches != nil {
		d.Branches.register(current, errs.write)
	}
	withTenant := tenancy.RequireOrganization(d.Memberships, tenancyFail)(current.mux)

	v1 := newRoutes()
	v1.mux.Handle("/v1/organizations/current", withTenant)
	v1.mux.Handle("/v1/organizations/current/", withTenant)
	if d.Me != nil {
		d.Me.register(v1, errs.write)
	}
	if d.Organizations != nil {
		d.Organizations.registerAuthenticated(v1, errs.write)
	}
	if d.Invitations != nil {
		d.Invitations.registerAuthenticated(v1, errs.write)
	}

	mux.Handle("/v1/", tenancy.Authenticate(d.Verifier, tenancyFail)(v1.mux))

	var h http.Handler = mux
	h = accessLog(d.Log, h)
	h = recoverer(d.Log, h)
	h = requestinfo.Middleware(h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, "platform-api")
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
