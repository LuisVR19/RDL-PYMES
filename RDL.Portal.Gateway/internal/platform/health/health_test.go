package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ok(name string, critical bool) Checker {
	return Check{CheckName: name, IsCritical: critical, Fn: func(context.Context) error { return nil }}
}

func failing(name string, critical bool) Checker {
	return Check{CheckName: name, IsCritical: critical, Fn: func(context.Context) error {
		return errors.New("dial tcp 127.0.0.1:8082: connection refused")
	}}
}

func ready(t *testing.T, checkers ...Checker) (int, map[string]any, string) {
	t.Helper()
	var logs strings.Builder
	h := New(slog.New(slog.NewTextHandler(&logs, nil)), time.Second, checkers...)
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	return rec.Code, body, logs.String()
}

func TestReadyWhenEverythingResponds(t *testing.T) {
	code, body, _ := ready(t, ok("jwks", true), ok("billing", false))
	if code != http.StatusOK || body["status"] != "ok" {
		t.Errorf("code=%d status=%v", code, body["status"])
	}
}

// Condición del incremento: una API destino caída no saca al gateway del balanceador.
func TestDegradedWhenAnUpstreamIsDown(t *testing.T) {
	code, body, _ := ready(t, ok("jwks", true), ok("platform", false), failing("fiscal", false))
	if code != http.StatusOK {
		t.Errorf("code=%d: una API secundaria caída no deja al gateway no listo", code)
	}
	if body["status"] != "degraded" {
		t.Errorf("status=%v, want degraded", body["status"])
	}
	checks, _ := body["checks"].(map[string]any)
	if checks["fiscal"] != "unavailable" {
		t.Errorf("checks=%v", checks)
	}
}

// Sin JWKS no se puede autenticar a nadie: eso sí es no estar listo.
func TestNotReadyWhenCriticalCheckFails(t *testing.T) {
	code, body, _ := ready(t, failing("jwks", true), ok("billing", false))
	if code != http.StatusServiceUnavailable || body["status"] != "unavailable" {
		t.Errorf("code=%d status=%v", code, body["status"])
	}
}

// El detalle de la falla no puede salir en la respuesta: /readyz es público.
func TestFailureDetailStaysInTheLog(t *testing.T) {
	_, body, logs := ready(t, failing("fiscal", false))
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "connection refused") {
		t.Errorf("la respuesta filtra el detalle: %s", raw)
	}
	if !strings.Contains(logs, "connection refused") {
		t.Errorf("el log debería tener el detalle: %s", logs)
	}
}

func TestLiveIsAlwaysOK(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second, failing("jwks", true))
	rec := httptest.NewRecorder()
	h.Live(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("code=%d: /healthz solo dice que el proceso vive", rec.Code)
	}
}
