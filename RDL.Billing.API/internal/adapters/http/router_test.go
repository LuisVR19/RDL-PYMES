package http

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

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/platform/health"
	"rdl/billing-api/pkg/correlation"
	"rdl/billing-api/pkg/tenancy"
)

// --- dobles de tenancy: el token es opaco y la membresía se decide por tabla ---

type fakeVerifier map[string]tenancy.Identity

func (v fakeVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	id, ok := v[raw]
	if !ok {
		return tenancy.Identity{}, errors.New("token desconocido")
	}
	return id, nil
}

type fakeMemberships map[uuid.UUID]tenancy.Membership // por organización

func (m fakeMemberships) ActiveMembership(_ context.Context, _ string, org uuid.UUID) (tenancy.Membership, error) {
	mem, ok := m[org]
	if !ok {
		return tenancy.Membership{}, tenancy.ErrNoMembership
	}
	return mem, nil
}

// fakeListCustomers registra con qué TenantContext y consulta se llamó al caso de uso.
type fakeListCustomers struct {
	gotTenant tenancy.Context
	gotQuery  app.CustomerQuery
	page      app.CustomerPage
	err       error
}

func (f *fakeListCustomers) Execute(_ context.Context, t tenancy.Context, q app.CustomerQuery) (app.CustomerPage, error) {
	f.gotTenant, f.gotQuery = t, q
	return f.page, f.err
}

var (
	orgA   = uuid.New()
	orgB   = uuid.New()
	userID = uuid.New()
)

func newTestRouter(list listCustomers) http.Handler {
	return newCustomerRouter(&CustomerHandlers{List: list})
}

func newCustomerRouter(h *CustomerHandlers) http.Handler {
	return newRouterWith(Deps{Customers: h})
}

// newRouterWith completa d con los dobles de tenancy: tok-a es miembro biller de orgA; tok-b trae el org_id de orgB,
// donde no es miembro; tok-no-org no trae org_id.
func newRouterWith(d Deps) http.Handler {
	d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	d.Health = health.New(d.Log, time.Second)
	d.Verifier = fakeVerifier{
		"tok-a":      {Subject: "sub-a", OrganizationID: orgA},
		"tok-no-org": {Subject: "sub-a"},
		"tok-b":      {Subject: "sub-a", OrganizationID: orgB},
	}
	d.Memberships = fakeMemberships{orgA: {UserID: userID, Roles: []string{"biller"}}}
	return NewRouter(d)
}

func do(h http.Handler, method, path, token string, headers ...string) *httptest.ResponseRecorder {
	return doBody(h, method, path, token, "", headers...)
}

func doBody(h http.Handler, method, path, token, body string, headers ...string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) problem.Details {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type = %q, cuerpo %s", ct, rec.Body)
	}
	var p problem.Details
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHealthz(t *testing.T) {
	rec := do(newTestRouter(&fakeListCustomers{}), http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUnknownRouteIsProblemDetails(t *testing.T) {
	rec := do(newTestRouter(&fakeListCustomers{}), http.MethodGet, "/nada", "")
	p := problemOf(t, rec)
	if rec.Code != http.StatusNotFound || p.Type != "urn:rdl:billing:problem:not-found" || p.CorrelationID == "" {
		t.Fatalf("status=%d problem=%+v", rec.Code, p)
	}
}

func TestCorrelationIDIsEchoedOrGenerated(t *testing.T) {
	h := newTestRouter(&fakeListCustomers{})
	sent := uuid.NewString()
	if got := do(h, http.MethodGet, "/healthz", "", correlation.Header, sent).Header().Get(correlation.Header); got != sent {
		t.Fatalf("correlation id = %q, se esperaba %q", got, sent)
	}
	got := do(h, http.MethodGet, "/healthz", "", correlation.Header, "no-es-uuid").Header().Get(correlation.Header)
	if _, err := uuid.Parse(got); err != nil || strings.Contains(got, "no-es-uuid") {
		t.Fatalf("un valor que no es UUID debe reemplazarse, llegó %q", got)
	}
}

func TestV1RequiresTenant(t *testing.T) {
	cases := []struct {
		name, token string
		status      int
		problemType string
	}{
		{"sin token", "", http.StatusUnauthorized, "unauthenticated"},
		{"token inválido", "tok-falso", http.StatusUnauthorized, "unauthenticated"},
		{"token sin org_id", "tok-no-org", http.StatusForbidden, "no-active-organization"},
		{"sin membresía activa en la organización del token", "tok-b", http.StatusForbidden, "membership-inactive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			list := &fakeListCustomers{}
			rec := do(newTestRouter(list), http.MethodGet, "/v1/customers", tc.token)
			p := problemOf(t, rec)
			if rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
			if list.gotTenant.OrganizationID() != uuid.Nil {
				t.Fatal("el caso de uso no debe ejecutarse sin TenantContext")
			}
		})
	}
}

func TestListCustomersUsesTokenOrganizationOnly(t *testing.T) {
	list := &fakeListCustomers{}
	h := newTestRouter(list)
	// Ni la query, ni un header, ni el body pueden cambiar la organización activa.
	rec := do(h, http.MethodGet, "/v1/customers?organizationId="+orgB.String(), "tok-a",
		"X-Organization-Id", orgB.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	if list.gotTenant.OrganizationID() != orgA || list.gotTenant.UserID() != userID {
		t.Fatalf("tenant = %v/%v, se esperaba %v/%v", list.gotTenant.OrganizationID(), list.gotTenant.UserID(), orgA, userID)
	}
	if roles := list.gotTenant.Roles(); len(roles) != 1 || roles[0] != "biller" {
		t.Fatalf("roles = %v: deben salir de la membresía revalidada", roles)
	}
}

func TestListCustomersResponse(t *testing.T) {
	c := customer.Customer{
		ID: uuid.New(), OrganizationID: orgA,
		Identification: customer.Identification{TypeCode: "01", Number: "112340567"},
		LegalName:      "Ana Pérez", IsActive: true,
		CreatedAt: time.Date(2026, 9, 24, 12, 0, 0, 0, time.FixedZone("CR", -6*3600)),
	}
	next := &app.PageCursor{At: c.CreatedAt, ID: c.ID}
	list := &fakeListCustomers{page: app.CustomerPage{Items: []customer.Customer{c}, Next: next}}
	rec := do(newTestRouter(list), http.MethodGet, "/v1/customers?limit=1&q=ana&active=true", "tok-a")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	var body struct {
		Items []struct {
			ID             string `json:"id"`
			Identification struct {
				TypeCode, Number string
			} `json:"identification"`
			LegalName string `json:"legalName"`
			IsActive  bool   `json:"isActive"`
			CreatedAt string `json:"createdAt"`
		} `json:"items"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != c.ID.String() || body.Items[0].Identification.Number != "112340567" {
		t.Fatalf("items = %+v", body.Items)
	}
	if !strings.HasSuffix(body.Items[0].CreatedAt, "Z") {
		t.Fatalf("createdAt debe ir en UTC con Z, llegó %q", body.Items[0].CreatedAt)
	}
	if body.NextCursor == nil {
		t.Fatal("falta nextCursor")
	}
	if q := list.gotQuery; q.Limit != 1 || q.Search != "ana" || q.Active == nil || !*q.Active {
		t.Fatalf("consulta = %+v", q)
	}

	// El cursor devuelto se acepta en la página siguiente.
	rec = do(newTestRouter(list), http.MethodGet, "/v1/customers?cursor="+*body.NextCursor, "tok-a")
	if rec.Code != http.StatusOK || list.gotQuery.After == nil || list.gotQuery.After.ID != c.ID {
		t.Fatalf("status=%d after=%+v", rec.Code, list.gotQuery.After)
	}
}

func TestListCustomersEmptyPageHasNullCursor(t *testing.T) {
	rec := do(newTestRouter(&fakeListCustomers{}), http.MethodGet, "/v1/customers", "tok-a")
	if got := strings.TrimSpace(rec.Body.String()); got != `{"items":[],"nextCursor":null}` {
		t.Fatalf("cuerpo = %s", got)
	}
}

func TestListCustomersValidation(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=101", "limit=x", "active=quizas", "cursor=no-base64!", "cursor=bm9wZQ", "q=" + strings.Repeat("a", 101)} {
		t.Run(query, func(t *testing.T) {
			list := &fakeListCustomers{}
			rec := do(newTestRouter(list), http.MethodGet, "/v1/customers?"+query, "tok-a")
			p := problemOf(t, rec)
			if rec.Code != http.StatusUnprocessableEntity || p.Type != problem.TypeBase+"validation" || len(p.Errors) == 0 {
				t.Fatalf("status=%d problem=%+v", rec.Code, p)
			}
		})
	}
}

func TestListCustomersForbidden(t *testing.T) {
	rec := do(newTestRouter(&fakeListCustomers{err: app.ErrForbidden}), http.MethodGet, "/v1/customers", "tok-a")
	if p := problemOf(t, rec); rec.Code != http.StatusForbidden || p.Type != problem.TypeBase+"forbidden" {
		t.Fatalf("status=%d type=%s", rec.Code, p.Type)
	}
}

func TestInternalErrorsAreNotLeaked(t *testing.T) {
	list := &fakeListCustomers{err: errors.New(`pq: relation "billing.customers" does not exist`)}
	rec := do(newTestRouter(list), http.MethodGet, "/v1/customers", "tok-a")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "billing.customers") {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
}

func TestMethodNotAllowedIsProblemDetails(t *testing.T) {
	rec := do(newTestRouter(&fakeListCustomers{}), http.MethodDelete, "/v1/customers", "tok-a")
	if p := problemOf(t, rec); rec.Code != http.StatusMethodNotAllowed || p.Type != problem.TypeBase+"method-not-allowed" {
		t.Fatalf("status=%d type=%s", rec.Code, p.Type)
	}
}
