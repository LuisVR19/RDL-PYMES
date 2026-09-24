package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/tenancy"
)

type fakeGetMe struct {
	u   user.User
	err error
	got tenancy.Identity
}

func (f *fakeGetMe) Execute(_ context.Context, id tenancy.Identity) (user.User, error) {
	f.got = id
	return f.u, f.err
}

type fakeListMemberships struct{ res app.MyMemberships }

func (f fakeListMemberships) Execute(context.Context, tenancy.Identity) (app.MyMemberships, error) {
	return f.res, nil
}

type fakeSelect struct {
	err    error
	called uuid.UUID
}

func (f *fakeSelect) Execute(_ context.Context, _ tenancy.Identity, org uuid.UUID) error {
	f.called = org
	return f.err
}

func meRouter(me *MeHandlers) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    stubVerifier{id: tenancy.Identity{Subject: "sub-ana", Email: "ana@example.test"}},
		Memberships: stubMemberships{},
		Me:          me,
	})
}

// call hace una petición autenticada. headers: "Nombre: valor".
func call(h http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, h := range headers {
		name, value, _ := strings.Cut(h, ":")
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) problem.Details {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("content-type=%q body=%s", ct, rec.Body.String())
	}
	var p problem.Details
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGetMeReturnsProfileFromVerifiedIdentity(t *testing.T) {
	org := uuid.New()
	getMe := &fakeGetMe{u: user.User{ID: uuid.New(), Email: "ana@example.test", FullName: "Ana", Status: user.StatusActive, ActiveOrganizationID: org, CreatedAt: time.Now()}}
	rec := call(meRouter(&MeHandlers{GetMe: getMe}), http.MethodGet, "/v1/me", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["email"] != "ana@example.test" || body["activeOrganizationId"] != org.String() {
		t.Fatalf("body=%v", body)
	}
	if getMe.got.Subject != "sub-ana" {
		t.Fatalf("el caso de uso debe recibir la identidad del token, got %+v", getMe.got)
	}
}

func TestGetMeMapsDomainErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		slug   string
	}{
		{user.ErrEmailRequired, http.StatusUnprocessableEntity, "email-required"},
		{user.ErrDisabled, http.StatusForbidden, "user-disabled"},
		{fmt.Errorf("%w: email usado", app.ErrConflict), http.StatusConflict, "conflict"},
		{fmt.Errorf("conexión rota a 10.1.2.3"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		rec := call(meRouter(&MeHandlers{GetMe: &fakeGetMe{err: c.err}}), http.MethodGet, "/v1/me", "")
		p := problemOf(t, rec)
		if rec.Code != c.status || p.Type != problem.TypeBase+c.slug || strings.Contains(rec.Body.String(), "10.1.2.3") {
			t.Errorf("err=%v: code=%d problem=%+v", c.err, rec.Code, p)
		}
	}
}

func TestMethodNotAllowedIsProblemDetails(t *testing.T) {
	rec := call(meRouter(&MeHandlers{GetMe: &fakeGetMe{}}), http.MethodDelete, "/v1/me", "")
	if p := problemOf(t, rec); rec.Code != http.StatusMethodNotAllowed || p.Type != problem.TypeBase+"method-not-allowed" {
		t.Fatalf("code=%d problem=%+v", rec.Code, p)
	}
}

func TestListMembershipsMarksActiveOrganization(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	list := fakeListMemberships{res: app.MyMemberships{
		ActiveOrganizationID: orgB,
		Items: []membership.Summary{
			{OrganizationID: orgA, LegalName: "A", Status: membership.StatusActive, Roles: []membership.Role{membership.RoleAccountant}},
			{OrganizationID: orgB, LegalName: "B", Status: membership.StatusActive, Roles: []membership.Role{membership.RoleOwner}},
		},
	}}
	rec := call(meRouter(&MeHandlers{ListMyMemberships: list}), http.MethodGet, "/v1/me/memberships", "")
	var body membershipsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("code=%d err=%v", rec.Code, err)
	}
	if len(body.Items) != 2 || body.Items[0].IsActive || !body.Items[1].IsActive || body.Items[1].Roles[0] != "owner" {
		t.Fatalf("body=%+v", body)
	}
}

func TestSelectActiveOrganizationValidation(t *testing.T) {
	cases := map[string]struct {
		body   string
		status int
		slug   string
	}{
		"vacío":           {"", http.StatusBadRequest, "malformed-request"},
		"no es json":      {"{", http.StatusBadRequest, "malformed-request"},
		"campo extra":     {`{"organizationId":"` + uuid.NewString() + `","role":"owner"}`, http.StatusBadRequest, "malformed-request"},
		"dos objetos":     {`{"organizationId":"` + uuid.NewString() + `"}{}`, http.StatusBadRequest, "malformed-request"},
		"falta el campo":  {`{}`, http.StatusUnprocessableEntity, "validation"},
		"uuid inválido":   {`{"organizationId":"abc"}`, http.StatusUnprocessableEntity, "validation"},
		"tipo equivocado": {`{"organizationId":5}`, http.StatusBadRequest, "malformed-request"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			sel := &fakeSelect{}
			rec := call(meRouter(&MeHandlers{SelectActiveOrganization: sel}), http.MethodPut, "/v1/me/active-organization", c.body)
			p := problemOf(t, rec)
			if rec.Code != c.status || p.Type != problem.TypeBase+c.slug || sel.called != uuid.Nil {
				t.Fatalf("code=%d problem=%+v called=%s", rec.Code, p, sel.called)
			}
		})
	}
}

func TestSelectActiveOrganizationValidationNamesJSONField(t *testing.T) {
	rec := call(meRouter(&MeHandlers{SelectActiveOrganization: &fakeSelect{}}), http.MethodPut, "/v1/me/active-organization", `{}`)
	p := problemOf(t, rec)
	if len(p.Errors) != 1 || p.Errors[0].Field != "organizationId" {
		t.Fatalf("errors=%+v", p.Errors)
	}
}

func TestSelectActiveOrganizationNotMemberIs404(t *testing.T) {
	rec := call(meRouter(&MeHandlers{SelectActiveOrganization: &fakeSelect{err: app.ErrNotFound}}), http.MethodPut,
		"/v1/me/active-organization", `{"organizationId":"`+uuid.NewString()+`"}`)
	if p := problemOf(t, rec); rec.Code != http.StatusNotFound || p.Type != problem.TypeBase+"not-found" {
		t.Fatalf("code=%d problem=%+v", rec.Code, p)
	}
}

func TestSelectActiveOrganizationSuccess(t *testing.T) {
	org := uuid.New()
	sel := &fakeSelect{}
	rec := call(meRouter(&MeHandlers{SelectActiveOrganization: sel}), http.MethodPut,
		"/v1/me/active-organization", `{"organizationId":"`+org.String()+`"}`)
	var body selectActiveOrganizationResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusOK || sel.called != org || body.ActiveOrganizationID != org || !body.TokenRefreshRequired {
		t.Fatalf("code=%d body=%+v called=%s", rec.Code, body, sel.called)
	}
}
