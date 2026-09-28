package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/correlation"
)

func newTestRouter() http.Handler {
	log := slog.New(slog.DiscardHandler)
	ok := health.CheckFunc{CheckName: "database", Fn: func(context.Context) error { return nil }}
	return NewRouter(Deps{Log: log, Health: health.New(log, time.Second, ok)})
}

func TestHealthEndpoints(t *testing.T) {
	h := newTestRouter()
	for _, path := range []string{"/healthz", "/readyz"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: code=%d", path, rec.Code)
		}
	}
}

func TestUnknownRouteIsProblemDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/nada", nil)
	req.Header.Set(correlation.Header, "0b8e7c6d-5a4f-4e3d-9c2b-1a0f9e8d7c6b")
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type=%q", ct)
	}
	var body struct {
		Type          string `json:"type"`
		CorrelationID string `json:"correlationId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Type != "urn:rdl:receivables:problem:not-found" {
		t.Errorf("type=%q", body.Type)
	}
	if body.CorrelationID != "0b8e7c6d-5a4f-4e3d-9c2b-1a0f9e8d7c6b" {
		t.Errorf("correlationId=%q", body.CorrelationID)
	}
}

func TestCorrelationIDIsGeneratedWhenInvalid(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set(correlation.Header, "no-es-uuid")
	newTestRouter().ServeHTTP(rec, req)
	if _, err := uuid.Parse(rec.Header().Get(correlation.Header)); err != nil {
		t.Fatalf("X-Correlation-Id=%q", rec.Header().Get(correlation.Header))
	}
}
