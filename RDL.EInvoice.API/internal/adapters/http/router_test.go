package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rdl/einvoice-api/internal/platform/health"
)

func newRouter(check error) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := health.New(log, time.Second, health.CheckFunc{CheckName: "database", Fn: func(context.Context) error { return check }})
	return NewRouter(Deps{Health: h})
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newRouter(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadyzReportsUnavailableWithoutLeakingDetail(t *testing.T) {
	rec := httptest.NewRecorder()
	newRouter(errors.New("password authentication failed for fiscal_api")).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "password") || !strings.Contains(body, `"database":"unavailable"`) {
		t.Fatalf("body = %s", body)
	}
}
