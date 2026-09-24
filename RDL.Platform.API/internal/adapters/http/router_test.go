package http

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

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/correlation"
	"rdl/platform-api/pkg/tenancy"
)

type stubVerifier struct{ id tenancy.Identity }

func (s stubVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	if raw != "valid" {
		return tenancy.Identity{}, errors.New("firma inválida para kid xyz")
	}
	return s.id, nil
}

type stubMemberships struct{ err error }

func (s stubMemberships) ActiveMembership(context.Context, string, uuid.UUID) (tenancy.Membership, error) {
	if s.err != nil {
		return tenancy.Membership{}, s.err
	}
	return tenancy.Membership{UserID: uuid.New(), Roles: []string{"owner"}}, nil
}

func newRouterWith(id tenancy.Identity, membershipErr error) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    stubVerifier{id: id},
		Memberships: stubMemberships{err: membershipErr},
	})
}

func newTestRouter() http.Handler { return newRouterWith(tenancy.Identity{Subject: "s"}, nil) }

func do(h http.Handler, path, token string) (*httptest.ResponseRecorder, problem.Details) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var p problem.Details
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec, p
}

func TestV1RequiresAuthentication(t *testing.T) {
	for _, token := range []string{"", "forjado"} {
		rec, p := do(newTestRouter(), "/v1/me", token)
		if rec.Code != http.StatusUnauthorized || p.Type != problem.TypeBase+"unauthenticated" {
			t.Fatalf("token %q: code=%d problem=%+v", token, rec.Code, p)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatal("falta WWW-Authenticate")
		}
		if strings.Contains(rec.Body.String(), "kid") {
			t.Fatalf("expone el detalle del error de verificación: %s", rec.Body.String())
		}
	}
}

func TestCurrentRoutesRequireActiveOrganization(t *testing.T) {
	rec, p := do(newRouterWith(tenancy.Identity{Subject: "s"}, nil), "/v1/organizations/current", "valid")
	if rec.Code != http.StatusForbidden || p.Type != problem.TypeBase+"no-active-organization" {
		t.Fatalf("sin org_id: code=%d problem=%+v", rec.Code, p)
	}

	withOrg := tenancy.Identity{Subject: "s", OrganizationID: uuid.New()}
	rec, p = do(newRouterWith(withOrg, tenancy.ErrNoMembership), "/v1/organizations/current/users", "valid")
	if rec.Code != http.StatusForbidden || p.Type != problem.TypeBase+"membership-inactive" {
		t.Fatalf("membresía revocada: code=%d problem=%+v", rec.Code, p)
	}

	// Con org_id y membresía activa pasa la barrera (404 porque aún no hay handlers en este incremento).
	rec, _ = do(newRouterWith(withOrg, nil), "/v1/organizations/current", "valid")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("con tenant válido: code=%d", rec.Code)
	}
}

func TestResolverFailureIsInternalWithoutDetails(t *testing.T) {
	withOrg := tenancy.Identity{Subject: "s", OrganizationID: uuid.New()}
	rec, p := do(newRouterWith(withOrg, errors.New("dial tcp 10.0.0.5:6543: timeout")), "/v1/organizations/current", "valid")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.0.0.5") || p.Detail != "" {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealthzAlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if rec.Header().Get(correlation.Header) == "" {
		t.Fatal("falta X-Correlation-Id en la respuesta")
	}
}

func TestUnknownRouteReturnsProblemDetails(t *testing.T) {
	cid := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/no-existe", nil)
	req.Header.Set(correlation.Header, cid.String())
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("content-type=%q", ct)
	}
	var p problem.Details
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Type != problem.TypeBase+"not-found" || p.Status != 404 || p.CorrelationID != cid.String() || p.Instance != "/no-existe" {
		t.Fatalf("problem=%+v", p)
	}
}

func TestPanicBecomesInternalProblemWithoutDetails(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	h := correlation.Middleware(recoverer(log, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("pgx: conexión rota a 10.0.0.5")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d", rec.Code)
	}
	var p problem.Details
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Detail != "" || p.Type != problem.TypeBase+"internal" {
		t.Fatalf("expone detalle interno: %+v", p)
	}
}
