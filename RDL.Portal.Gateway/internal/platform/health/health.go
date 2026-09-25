// Package health expone /healthz (el proceso vive) y /readyz (sus dependencias responden).
//
// A diferencia de una API de dominio, el Portal Gateway depende de varias APIs y no de una base. Por eso
// distingue dos clases de dependencia (ADR 0002):
//   - crítica (el JWKS): sin ella no se puede autenticar a nadie y el gateway no sirve para nada → 503;
//   - degradable (cada API destino): si E-Invoice está caída, las pantallas de Hacienda fallan pero facturar
//     sigue funcionando. Sacar el gateway del balanceador por eso dejaría al portal sin nada.
//
// Una API que no está configurada en este ambiente no se chequea: no es una falla, es una fase del proyecto.
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
	// Critical: su falla deja el servicio no listo. Si no, solo lo marca degradado.
	Critical() bool
}

// Check adapta una función a Checker.
type Check struct {
	CheckName  string
	IsCritical bool
	Fn         func(ctx context.Context) error
}

func (c Check) Name() string                    { return c.CheckName }
func (c Check) Check(ctx context.Context) error { return c.Fn(ctx) }
func (c Check) Critical() bool                  { return c.IsCritical }

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

// Ready responde 200 "ok", 200 "degraded" (alguna API destino no contesta) o 503 (falla una dependencia crítica).
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	results := make(map[string]string, len(h.checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup
	ready, degraded := true, false

	for _, c := range h.checkers {
		wg.Go(func() {
			status := "ok"
			if err := c.Check(ctx); err != nil {
				// El detalle va al log, no a la respuesta: /readyz es público.
				h.log.WarnContext(ctx, "readiness check failed",
					slog.String("check", c.Name()), slog.Bool("critical", c.Critical()), slog.Any("error", err))
				status = "unavailable"
			}
			mu.Lock()
			defer mu.Unlock()
			results[c.Name()] = status
			if status == "ok" {
				return
			}
			if c.Critical() {
				ready = false
			} else {
				degraded = true
			}
		})
	}
	wg.Wait()

	code, status := http.StatusOK, "ok"
	switch {
	case !ready:
		code, status = http.StatusServiceUnavailable, "unavailable"
	case degraded:
		status = "degraded"
	}
	writeJSON(w, code, map[string]any{"status": status, "checks": results})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
