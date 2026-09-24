// Package health expone /healthz (el proceso vive) y /readyz (sus dependencias responden).
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Checker verifica una dependencia. Debe respetar el deadline del contexto.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckFunc adapta una función a Checker.
type CheckFunc struct {
	CheckName string
	Fn        func(ctx context.Context) error
}

func (c CheckFunc) Name() string                    { return c.CheckName }
func (c CheckFunc) Check(ctx context.Context) error { return c.Fn(ctx) }

type Handler struct {
	log      *slog.Logger
	checkers []Checker
	timeout  time.Duration
}

func New(log *slog.Logger, timeout time.Duration, checkers ...Checker) *Handler {
	return &Handler{log: log, checkers: checkers, timeout: timeout}
}

func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	results := make(map[string]string, len(h.checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	healthy := true
	for _, c := range h.checkers {
		wg.Go(func() {
			status := "ok"
			if err := c.Check(ctx); err != nil {
				// El detalle va al log, no a la respuesta: /readyz es público.
				h.log.WarnContext(ctx, "readiness check failed", slog.String("check", c.Name()), slog.Any("error", err))
				status = "unavailable"
			}
			mu.Lock()
			defer mu.Unlock()
			results[c.Name()] = status
			if status != "ok" {
				healthy = false
			}
		})
	}
	wg.Wait()

	code, status := http.StatusOK, "ok"
	if !healthy {
		code, status = http.StatusServiceUnavailable, "unavailable"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
