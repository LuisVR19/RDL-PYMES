// Package http traduce HTTP ↔ casos de uso. No contiene reglas de negocio.
package http

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/correlation"
	"rdl/receivables-api/pkg/requestinfo"
)

type Deps struct {
	Log    *slog.Logger
	Health *health.Handler
}

// NewRouter arma el mux con el router estándar de Go 1.22+, igual que Platform.
//
//	/healthz, /readyz    públicas
//	/v1/...              (incremento 2) JWT válido + TenantContext
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)
	mux.HandleFunc("/", notFound)

	var h http.Handler = mux
	h = accessLog(d.Log, h)
	h = recoverer(d.Log, h)
	h = requestinfo.Middleware(h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, "receivables-api")
}

func notFound(w http.ResponseWriter, r *http.Request) { problem.Write(w, r, problem.NotFound) }

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
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			return
		}
		log.InfoContext(r.Context(), "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
		)
	})
}
