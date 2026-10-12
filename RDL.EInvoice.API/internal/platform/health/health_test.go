package health

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func ok(name string) Checker {
	return CheckFunc{CheckName: name, Fn: func(context.Context) error { return nil }}
}

func failing(name string) Checker {
	return CheckFunc{CheckName: name, Fn: func(context.Context) error { return errors.New("password authentication failed for user x") }}
}

func ready(t *testing.T, h *Handler) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	return rec.Code, body
}

func TestReadyOKWhenAllChecksPass(t *testing.T) {
	code, body := ready(t, New(slog.New(slog.DiscardHandler), time.Second, ok("db"), ok("jwks")))
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("code=%d body=%v", code, body)
	}
}

func TestReadyUnavailableWithoutLeakingErrors(t *testing.T) {
	h := New(slog.New(slog.DiscardHandler), time.Second, ok("jwks"), failing("db"))
	rec := httptest.NewRecorder()
	h.Ready(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
	if got := rec.Body.String(); strings.Contains(got, "password") {
		t.Fatalf("la respuesta expone el error interno: %s", got)
	}
}

func TestReadyRespectsTimeout(t *testing.T) {
	slow := CheckFunc{CheckName: "db", Fn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	start := time.Now()
	code, _ := ready(t, New(slog.New(slog.DiscardHandler), 50*time.Millisecond, slow))
	if code != http.StatusServiceUnavailable || time.Since(start) > time.Second {
		t.Fatalf("code=%d elapsed=%v", code, time.Since(start))
	}
}
