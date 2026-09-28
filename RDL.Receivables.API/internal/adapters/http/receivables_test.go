package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/tenancy"
)

type fakeVerifier struct{ id tenancy.Identity }

func (f fakeVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	if raw != "valid" {
		return tenancy.Identity{}, errors.New("firma inválida")
	}
	return f.id, nil
}

type fakeResolver struct {
	m   tenancy.Membership
	err error
}

func (f fakeResolver) ActiveMembership(context.Context, string, uuid.UUID) (tenancy.Membership, error) {
	return f.m, f.err
}

type fakeList struct {
	page   app.ReceivablePage
	err    error
	tenant tenancy.Context
	query  app.ReceivableQuery
	calls  int
}

func (f *fakeList) Execute(_ context.Context, t tenancy.Context, q app.ReceivableQuery) (app.ReceivablePage, error) {
	f.calls++
	f.tenant, f.query = t, q
	return f.page, f.err
}

var tokenOrg = uuid.MustParse("7a1c3d2e-0b4f-4e6a-9c8d-1f2e3a4b5c6d")

func newProtectedRouter(org uuid.UUID, membershipErr error, list *fakeList) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    fakeVerifier{id: tenancy.Identity{Subject: "sub-1", OrganizationID: org}},
		Memberships: fakeResolver{m: tenancy.Membership{UserID: uuid.New(), Roles: []string{"collector"}}, err: membershipErr},
		Receivables: &ReceivableHandlers{List: list},
	})
}

type problemBody struct {
	Type   string `json:"type"`
	Errors []struct {
		Field string `json:"field"`
	} `json:"errors"`
}

func do(h http.Handler, method, target, token string) (*httptest.ResponseRecorder, problemBody) {
	req := httptest.NewRequest(method, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var p problemBody
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec, p
}

func TestReceivablesRequiresTenant(t *testing.T) {
	cases := []struct {
		name    string
		org     uuid.UUID
		memErr  error
		token   string
		status  int
		problem string
	}{
		{"sin token", tokenOrg, nil, "", http.StatusUnauthorized, "unauthenticated"},
		{"token inválido", tokenOrg, nil, "otro", http.StatusUnauthorized, "unauthenticated"},
		{"sin org_id", uuid.Nil, nil, "valid", http.StatusForbidden, "no-active-organization"},
		{"membresía suspendida", tokenOrg, tenancy.ErrNoMembership, "valid", http.StatusForbidden, "membership-inactive"},
	}
	for _, c := range cases {
		list := &fakeList{}
		rec, p := do(newProtectedRouter(c.org, c.memErr, list), http.MethodGet, "/v1/receivables", c.token)
		if rec.Code != c.status || p.Type != "urn:rdl:receivables:problem:"+c.problem {
			t.Errorf("%s: code=%d type=%q", c.name, rec.Code, p.Type)
		}
		if list.calls != 0 {
			t.Errorf("%s: el caso de uso no debe ejecutarse", c.name)
		}
	}
}

func TestListReceivablesResponse(t *testing.T) {
	settled := time.Date(2026, 9, 20, 15, 4, 5, 0, time.FixedZone("CR", -6*3600))
	issued, _ := civil.Parse("2026-09-01")
	due, _ := civil.Parse("2026-10-01")
	next := app.PageCursor{At: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC), ID: uuid.New()}
	list := &fakeList{page: app.ReceivablePage{
		Items: []app.ReceivableView{{
			ID: uuid.New(), SourceInvoiceID: uuid.New(), CustomerID: uuid.New(), CustomerLegalName: "Cliente S.A.",
			DocumentNumber: "00100001010000000001", Currency: "CRC", OriginalAmount: "11300", BalanceAmount: "0",
			IssuedOn: issued, DueOn: due, Status: "paid", SettledAt: &settled,
		}},
		Next: &next,
	}}
	rec, _ := do(newProtectedRouter(tokenOrg, nil, list), http.MethodGet,
		"/v1/receivables?limit=1&status=paid&overdue=false&organization_id="+uuid.NewString(), "valid")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	// Criterio de aislamiento 6: el organization_id de la query no cambia el tenant.
	if list.tenant.OrganizationID() != tokenOrg {
		t.Fatalf("tenant=%s, se esperaba el del token", list.tenant.OrganizationID())
	}
	if list.query.Limit != 1 || list.query.Status != "paid" || list.query.Overdue == nil || *list.query.Overdue {
		t.Fatalf("query=%+v", list.query)
	}

	var body struct {
		Items      []map[string]any `json:"items"`
		NextCursor *string          `json:"nextCursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	it := body.Items[0]
	if it["issuedOn"] != "2026-09-01" || it["dueOn"] != "2026-10-01" || it["balanceAmount"] != "0" ||
		it["settledAt"] != "2026-09-20T21:04:05Z" || it["currency"] != "CRC" {
		t.Fatalf("item=%v", it)
	}
	if body.NextCursor == nil {
		t.Fatal("falta nextCursor")
	}
	if c, err := decodeCursor(*body.NextCursor); err != nil || c.ID != next.ID || !c.At.Equal(next.At) {
		t.Fatalf("cursor=%+v err=%v", c, err)
	}
}

func TestListReceivablesEmptyPage(t *testing.T) {
	rec, _ := do(newProtectedRouter(tokenOrg, nil, &fakeList{}), http.MethodGet, "/v1/receivables", "valid")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if got := rec.Body.String(); got != "{\"items\":[],\"nextCursor\":null}\n" {
		t.Fatalf("body=%s", got)
	}
}

func TestListReceivablesValidation(t *testing.T) {
	for query, field := range map[string]string{
		"limit=0":             "limit",
		"limit=101":           "limit",
		"limit=x":             "limit",
		"cursor=no.es.base64": "cursor",
		"cursor=e30":          "cursor", // {} en base64url
		"status=pagada":       "status",
		"customerId=123":      "customerId",
		"overdue=1":           "overdue",
		"overdue=si":          "overdue",
	} {
		list := &fakeList{}
		rec, p := do(newProtectedRouter(tokenOrg, nil, list), http.MethodGet, "/v1/receivables?"+query, "valid")
		if rec.Code != http.StatusUnprocessableEntity || p.Type != "urn:rdl:receivables:problem:validation" ||
			len(p.Errors) != 1 || p.Errors[0].Field != field {
			t.Errorf("%s: code=%d problem=%+v", query, rec.Code, p)
		}
		if list.calls != 0 {
			t.Errorf("%s: no debe llegar al caso de uso", query)
		}
	}
}

func TestListReceivablesErrors(t *testing.T) {
	for err, status := range map[error]int{
		app.ErrForbidden:             http.StatusForbidden,
		errors.New("conexión caída"): http.StatusInternalServerError,
	} {
		rec, _ := do(newProtectedRouter(tokenOrg, nil, &fakeList{err: err}), http.MethodGet, "/v1/receivables", "valid")
		if rec.Code != status {
			t.Errorf("%v: code=%d", err, rec.Code)
		}
		if status == http.StatusInternalServerError && json.Valid(rec.Body.Bytes()) {
			var p map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &p)
			if _, leaked := p["detail"]; leaked {
				t.Error("un 500 no debe exponer el error interno")
			}
		}
	}
}

func TestReceivablesMethodNotAllowed(t *testing.T) {
	rec, p := do(newProtectedRouter(tokenOrg, nil, &fakeList{}), http.MethodPost, "/v1/receivables", "valid")
	if rec.Code != http.StatusMethodNotAllowed || p.Type != "urn:rdl:receivables:problem:method-not-allowed" {
		t.Fatalf("code=%d type=%q", rec.Code, p.Type)
	}
}

type fakeBalances struct {
	views []app.ReceivableView
	ids   []uuid.UUID
	calls int
}

func (f *fakeBalances) Execute(_ context.Context, _ tenancy.Context, ids []uuid.UUID) ([]app.ReceivableView, error) {
	f.calls++
	f.ids = ids
	return f.views, nil
}

func postBatch(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/receivables/by-invoice", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func newBatchRouter(b *fakeBalances) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    fakeVerifier{id: tenancy.Identity{Subject: "sub-1", OrganizationID: tokenOrg}},
		Memberships: fakeResolver{m: tenancy.Membership{UserID: uuid.New(), Roles: []string{"biller"}}},
		Receivables: &ReceivableHandlers{ByInvoices: b},
	})
}

func TestBalancesByInvoiceResponse(t *testing.T) {
	inv, rec := uuid.New(), uuid.New()
	b := &fakeBalances{views: []app.ReceivableView{{
		ID: rec, SourceInvoiceID: inv, Currency: "CRC", BalanceAmount: "1300.5", Status: "partially_paid",
		DueOn: civil.Date{Year: 2026, Month: 10, Day: 15},
	}}}
	res := postBatch(newBatchRouter(b), `{"ids":["`+inv.String()+`","`+uuid.NewString()+`"]}`)
	if res.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", res.Code, res.Body)
	}
	if len(b.ids) != 2 || b.ids[0] != inv {
		t.Errorf("ids que llegan al caso de uso: %v", b.ids)
	}
	var out struct {
		Items []map[string]string `json:"items"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"invoiceId": inv.String(), "receivableId": rec.String(), "status": "partially_paid", "currency": "CRC",
		"balanceAmount": "1300.5", "dueOn": "2026-10-15",
	}
	if len(out.Items) != 1 || !maps.Equal(out.Items[0], want) {
		t.Errorf("items: %+v", out.Items)
	}
}

func TestBalancesByInvoiceValidation(t *testing.T) {
	id := uuid.NewString()
	many := make([]string, app.MaxBalanceBatch+1)
	for i := range many {
		many[i] = `"` + uuid.NewString() + `"`
	}
	cases := map[string]struct {
		body   string
		status int
		field  string
	}{
		"vacío":            {`{"ids":[]}`, http.StatusUnprocessableEntity, "ids"},
		"sin ids":          {`{}`, http.StatusUnprocessableEntity, "ids"},
		"más de 100":       {`{"ids":[` + strings.Join(many, ",") + `]}`, http.StatusUnprocessableEntity, "ids"},
		"id inválido":      {`{"ids":["x"]}`, http.StatusUnprocessableEntity, "ids[0]"},
		"id repetido":      {`{"ids":["` + id + `","` + id + `"]}`, http.StatusUnprocessableEntity, "ids[1]"},
		"campo extra":      {`{"ids":["` + id + `"],"organizationId":"` + id + `"}`, http.StatusBadRequest, ""},
		"JSON mal formado": {`{"ids":`, http.StatusBadRequest, ""},
	}
	for name, c := range cases {
		b := &fakeBalances{}
		res := postBatch(newBatchRouter(b), c.body)
		var p problemBody
		_ = json.Unmarshal(res.Body.Bytes(), &p)
		if res.Code != c.status {
			t.Errorf("%s: code=%d body=%s", name, res.Code, res.Body)
		}
		if c.field != "" && (len(p.Errors) == 0 || p.Errors[0].Field != c.field) {
			t.Errorf("%s: errores %+v", name, p.Errors)
		}
		if b.calls != 0 {
			t.Errorf("%s: el caso de uso no debe ejecutarse", name)
		}
	}
}
