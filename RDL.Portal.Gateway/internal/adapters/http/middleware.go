package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"rdl/portal-gateway/internal/adapters/http/problem"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/pkg/correlation"
	"rdl/portal-gateway/pkg/identity"
)

// Verifier valida el JWT del usuario. Lo implementa `internal/adapters/auth`.
type Verifier interface {
	Verify(ctx context.Context, rawToken string) (identity.Identity, error)
}

// authenticate corta en el Portal Gateway todo token inválido o vencido: ninguna API recibe la llamada.
// No revalida la membresía ni decide la organización: eso es de cada API dueña, contra su base.
func authenticate(v Verifier, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				problem.Write(w, r, problem.Unauthorized.WithDetail("Falta el encabezado Authorization: Bearer."))
				return
			}
			id, err := v.Verify(r.Context(), raw)
			if err != nil {
				// El motivo exacto va al log: decirle al cliente por qué no sirve su token ayuda a adivinar.
				log.InfoContext(r.Context(), "token rechazado", slog.Any("error", err))
				problem.Write(w, r, problem.Unauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(identity.With(r.Context(), id)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// withBudget acota lo que puede durar una petición completa, composiciones incluidas. Sin esto, una API lenta
// mantendría ocupada una conexión del portal indefinidamente.
func withBudget(budget time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), budget)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// corsHeaders son los que el portal puede mandar. Coinciden con la lista blanca de propagación: pedir permiso
// para un header que después se descarta solo confunde.
var corsHeaders = strings.Join(routes.RequestHeaders, ", ")

// corsExposed son los que el navegador deja leer al portal.
var corsExposed = strings.Join(routes.ResponseHeaders, ", ")

// cors responde el preflight y marca las respuestas. Solo para los orígenes declarados y sin cookies:
// el token viaja en el header, así que no hace falta `Allow-Credentials` y no hay riesgo de CSRF por cookie.
func cors(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Expose-Headers", corsExposed)
			}
			// El preflight va antes de autenticar: el navegador no manda el token en un OPTIONS.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if origin != "" && slices.Contains(allowed, origin) {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE")
					w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
					w.Header().Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func notFound(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound) }

func methodNotAllowed(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.MethodNot) }

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

// Unwrap deja que http.ResponseController llegue al writer real (necesario para el flush de los listados largos
// y, más adelante, para SSE).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func accessLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		attrs := []any{
			slog.String("method", r.Method),
			slog.String("route", r.Pattern),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		}
		// El log dice quién y en qué organización actuaba, pero la organización sale del token verificado,
		// nunca de la petición.
		if id, ok := identity.From(r.Context()); ok {
			attrs = append(attrs, slog.String("subject", id.Subject))
			if id.OrganizationID.String() != "00000000-0000-0000-0000-000000000000" {
				attrs = append(attrs, slog.String("organizationId", id.OrganizationID.String()))
			}
		}
		log.InfoContext(r.Context(), "http request", attrs...)
	})
}

// correlationOf devuelve el id que el gateway fijó para esta petición, para propagarlo a las APIs.
func correlationOf(r *http.Request) string {
	if id, ok := correlation.FromContext(r.Context()); ok {
		return id.String()
	}
	return ""
}
