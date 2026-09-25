package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/pkg/tenancy"
)

// fakeInvoiceCommands registra lo que llega a cada caso de uso de facturas.
type fakeInvoiceCommands struct {
	called    string
	gotHeader app.HeaderInput
	gotLines  []app.LineRequest
	gotPatch  invoice.HeaderPatch
	gotLinesP *[]app.LineRequest
	gotQuery  app.InvoiceQuery
	gotID     uuid.UUID
	result    invoice.Invoice
	history   []app.StatusChange
	gotKey    string
	replayed  bool
	err       error
}

type invCreateFn func(context.Context, tenancy.Context, string, app.HeaderInput, []app.LineRequest) (app.CreateInvoiceResult, error)

func (fn invCreateFn) Execute(ctx context.Context, t tenancy.Context, k string, h app.HeaderInput, l []app.LineRequest) (app.CreateInvoiceResult, error) {
	return fn(ctx, t, k, h, l)
}

type invUpdateFn func(context.Context, tenancy.Context, uuid.UUID, invoice.HeaderPatch, *[]app.LineRequest) (invoice.Invoice, error)

func (fn invUpdateFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p invoice.HeaderPatch, l *[]app.LineRequest) (invoice.Invoice, error) {
	return fn(ctx, t, id, p, l)
}

type invLinesFn func(context.Context, tenancy.Context, uuid.UUID, []app.LineRequest) (invoice.Invoice, error)

func (fn invLinesFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, l []app.LineRequest) (invoice.Invoice, error) {
	return fn(ctx, t, id, l)
}

type invDiscardFn func(context.Context, tenancy.Context, uuid.UUID) error

func (fn invDiscardFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) error {
	return fn(ctx, t, id)
}

type invListFn func(context.Context, tenancy.Context, app.InvoiceQuery) (app.InvoicePage, error)

func (fn invListFn) Execute(ctx context.Context, t tenancy.Context, q app.InvoiceQuery) (app.InvoicePage, error) {
	return fn(ctx, t, q)
}

type invHistoryFn func(context.Context, tenancy.Context, uuid.UUID) ([]app.StatusChange, error)

func (fn invHistoryFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) ([]app.StatusChange, error) {
	return fn(ctx, t, id)
}

type invIssueFn func(context.Context, tenancy.Context, string, uuid.UUID) (app.IssueResult, error)

func (fn invIssueFn) Execute(ctx context.Context, t tenancy.Context, k string, id uuid.UUID) (app.IssueResult, error) {
	return fn(ctx, t, k, id)
}

func newInvoiceRouter(f *fakeInvoiceCommands) http.Handler {
	return newRouterWith(Deps{Invoices: &InvoiceHandlers{
		Issue: invIssueFn(func(_ context.Context, _ tenancy.Context, k string, id uuid.UUID) (app.IssueResult, error) {
			f.called, f.gotID, f.gotKey = "issue", id, k
			return app.IssueResult{Invoice: f.result, Replayed: f.replayed}, f.err
		}),
		Create: invCreateFn(func(_ context.Context, _ tenancy.Context, _ string, h app.HeaderInput, l []app.LineRequest) (app.CreateInvoiceResult, error) {
			f.called, f.gotHeader, f.gotLines = "create", h, l
			return app.CreateInvoiceResult{Invoice: f.result}, f.err
		}),
		List: invListFn(func(_ context.Context, _ tenancy.Context, q app.InvoiceQuery) (app.InvoicePage, error) {
			f.called, f.gotQuery = "list", q
			return app.InvoicePage{Items: []invoice.Invoice{f.result}}, f.err
		}),
		Get: getInvoiceFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID) (invoice.Invoice, error) {
			f.called, f.gotID = "get", id
			return f.result, f.err
		}),
		Update: invUpdateFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID, p invoice.HeaderPatch, l *[]app.LineRequest) (invoice.Invoice, error) {
			f.called, f.gotID, f.gotPatch, f.gotLinesP = "update", id, p, l
			return f.result, f.err
		}),
		ReplaceLines: invLinesFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID, l []app.LineRequest) (invoice.Invoice, error) {
			f.called, f.gotID, f.gotLines = "lines", id, l
			return f.result, f.err
		}),
		Discard: invDiscardFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID) error {
			f.called, f.gotID = "discard", id
			return f.err
		}),
		History: invHistoryFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID) ([]app.StatusChange, error) {
			f.called, f.gotID = "history", id
			return f.history, f.err
		}),
		Summary: getInvoiceFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID) (invoice.Invoice, error) {
			f.called, f.gotID = "summary", id
			return f.result, f.err
		}),
	}})
}

type getInvoiceFn func(context.Context, tenancy.Context, uuid.UUID) (invoice.Invoice, error)

func (fn getInvoiceFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (invoice.Invoice, error) {
	return fn(ctx, t, id)
}

func sampleDraft() invoice.Invoice {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	pid := uuid.New()
	inv, _ := invoice.NewDraft(orgA, userID, invoice.Header{
		DocumentType: invoice.TypeInvoice, CustomerID: uuid.New(), SaleConditionCode: "01",
		Currency: money.MustCurrencyForTest("CRC"), ExchangeRate: money.MustExchangeRateForTest("1"),
	})
	inv, _ = inv.ReplaceLines([]invoice.LineDraft{{
		ProductID: &pid, ProductCode: "SERV-01", CabysCode: "8314100000100", Description: "Consultoría",
		UnitOfMeasureCode: "Sp", IsService: true, Quantity: money.MustQuantityForTest("3"),
		UnitPrice: money.MustAmountForTest("0.33333"), Discount: money.Zero,
		Taxes: []invoice.TaxDraft{{TypeCode: "01", RateCode: "08", Rate: money.MustPercentageForTest("13")}},
	}})
	inv.ID, inv.CreatedAt, inv.UpdatedAt = uuid.New(), now, now
	return inv
}

func TestCreateInvoiceHTTP(t *testing.T) {
	f := &fakeInvoiceCommands{result: sampleDraft()}
	body := `{"documentType":"invoice","customerId":"` + uuid.NewString() + `","saleConditionCode":"01","currency":"CRC",
	  "lines":[{"productId":"` + uuid.NewString() + `","quantity":"3","discount":"0.5","discountReason":"Promo"}]}`
	rec := doBody(newInvoiceRouter(f), http.MethodPost, "/v1/invoices", "tok-a", body, "Idempotency-Key", "k")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	if f.gotHeader.ExchangeRate != nil || f.gotHeader.Currency.String() != "CRC" || len(f.gotLines) != 1 ||
		f.gotLines[0].Quantity.String() != "3" || f.gotLines[0].Discount.String() != "0.5" || f.gotLines[0].UnitPrice != nil {
		t.Fatalf("header=%+v lines=%+v", f.gotHeader, f.gotLines)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	lines, _ := out["lines"].([]any)
	line, _ := lines[0].(map[string]any)
	taxes, _ := line["taxes"].([]any)
	tax, _ := taxes[0].(map[string]any)
	// Formato DocumentLine v1: montos como string, número null en borrador, sin snapshot de cliente.
	if out["number"] != nil || out["status"] != "draft" || out["total"] != "1.12999" || out["customerSnapshot"] != nil ||
		line["subtotal"] != "0.99999" || line["quantity"] != "3" || tax["rate"] != "13" || tax["amount"] != "0.13" {
		t.Fatalf("respuesta = %v", out)
	}
}

func TestCreateInvoiceValidationHTTP(t *testing.T) {
	cust, prod := uuid.NewString(), uuid.NewString()
	base := `"documentType":"invoice","customerId":"` + cust + `","saleConditionCode":"01"`
	cases := []struct {
		name, body  string
		status      int
		problemType string
		field       string
	}{
		{"cantidad como number", `{` + base + `,"currency":"CRC","lines":[{"productId":"` + prod + `","quantity":3}]}`,
			http.StatusBadRequest, "malformed-request", ""},
		{"cantidad cero", `{` + base + `,"currency":"CRC","lines":[{"productId":"` + prod + `","quantity":"0"}]}`,
			http.StatusUnprocessableEntity, "validation", "lines[0].quantity"},
		{"línea sin producto", `{` + base + `,"currency":"CRC","lines":[{"quantity":"1"}]}`,
			http.StatusUnprocessableEntity, "validation", "lines[0].productId"},
		{"sin moneda", `{` + base + `}`, http.StatusUnprocessableEntity, "validation", "currency"},
		{"tipo de cambio inválido", `{` + base + `,"currency":"USD","exchangeRate":"0"}`,
			http.StatusUnprocessableEntity, "validation", "exchangeRate"},
		{"nota (F5)", `{` + base + `,"currency":"CRC","referencedInvoiceId":"` + uuid.NewString() + `"}`,
			http.StatusUnprocessableEntity, "validation", "referencedInvoiceId"},
		{"organizationId en el cuerpo", `{` + base + `,"currency":"CRC","organizationId":"` + orgB.String() + `"}`,
			http.StatusBadRequest, "malformed-request", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeInvoiceCommands{}
			rec := doBody(newInvoiceRouter(f), http.MethodPost, "/v1/invoices", "tok-a", tc.body, "Idempotency-Key", "k")
			p := problemOf(t, rec)
			if rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s detail=%s errors=%+v", rec.Code, p.Type, p.Detail, p.Errors)
			}
			if tc.field != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tc.field) {
				t.Fatalf("errors = %+v", p.Errors)
			}
			if f.called != "" {
				t.Fatal("no llega al caso de uso")
			}
		})
	}
}

func TestInvoiceErrorsHTTP(t *testing.T) {
	id := uuid.NewString()
	for name, tc := range map[string]struct {
		method, path, body string
		err                error
		status             int
		problemType        string
	}{
		"editar emitido":       {http.MethodPatch, "/v1/invoices/" + id, `{"notes":"x"}`, invoice.ErrNotDraft, http.StatusConflict, "invoice-not-draft"},
		"líneas de emitido":    {http.MethodPut, "/v1/invoices/" + id + "/lines", `[{"productId":"` + id + `","quantity":"1"}]`, invoice.ErrNotDraft, http.StatusConflict, "invoice-not-draft"},
		"descartar emitido":    {http.MethodDelete, "/v1/invoices/" + id, "", invoice.ErrNotDraft, http.StatusConflict, "invoice-not-draft"},
		"cliente inactivo":     {http.MethodPatch, "/v1/invoices/" + id, `{"customerId":"` + id + `"}`, app.ErrCustomerInactive, http.StatusUnprocessableEntity, "customer-inactive"},
		"de otra organización": {http.MethodGet, "/v1/invoices/" + id, "", app.ErrNotFound, http.StatusNotFound, "not-found"},
		"historial ajeno":      {http.MethodGet, "/v1/invoices/" + id + "/history", "", app.ErrNotFound, http.StatusNotFound, "not-found"},
		"resumen ajeno":        {http.MethodGet, "/internal/v1/invoices/" + id + "/summary", "", app.ErrNotFound, http.StatusNotFound, "not-found"},
		"resumen sin permiso":  {http.MethodGet, "/internal/v1/invoices/" + id + "/summary", "", app.ErrForbidden, http.StatusForbidden, "forbidden"},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInvoiceCommands{err: tc.err}
			rec := doBody(newInvoiceRouter(f), tc.method, tc.path, "tok-a", tc.body)
			if p := problemOf(t, rec); rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
		})
	}
}

// La forma exacta de getInvoiceSummary en openapi/bff-internal.yaml: sin líneas, montos como string, número
// null y sin customerLegalName en borrador.
func TestInvoiceSummaryHTTP(t *testing.T) {
	draft := sampleDraft()
	issued := sampleDraft()
	issued.Status, issued.Number = invoice.StatusIssued, "00100001010000000001"
	issued.Customer = invoice.CustomerSnapshot{IdentificationTypeCode: "02", IdentificationNumber: "3101123456", LegalName: "Cliente S.A."}

	for name, tc := range map[string]struct {
		inv  invoice.Invoice
		want map[string]any
	}{
		"borrador": {draft, map[string]any{"id": draft.ID.String(), "documentType": "invoice", "number": nil, "status": "draft",
			"requiresCorrection": false, "currency": "CRC", "total": "1.12999"}},
		"emitida": {issued, map[string]any{"id": issued.ID.String(), "documentType": "invoice", "number": "00100001010000000001",
			"status": "issued", "requiresCorrection": false, "customerLegalName": "Cliente S.A.", "currency": "CRC", "total": "1.12999"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInvoiceCommands{result: tc.inv}
			rec := do(newInvoiceRouter(f), http.MethodGet, "/internal/v1/invoices/"+tc.inv.ID.String()+"/summary", "tok-a")
			if rec.Code != http.StatusOK || f.called != "summary" || f.gotID != tc.inv.ID {
				t.Fatalf("status=%d called=%s cuerpo=%s", rec.Code, f.called, rec.Body)
			}
			var out map[string]any
			if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
				t.Fatal(err)
			}
			if len(out) != len(tc.want) {
				t.Fatalf("campos = %v, se esperaba %v", out, tc.want)
			}
			for k, v := range tc.want {
				if out[k] != v {
					t.Fatalf("%s = %v, se esperaba %v (%v)", k, out[k], v, out)
				}
			}
		})
	}
}

// La ruta interna pasa por la misma cadena que /v1: sin token no llega al caso de uso, y un id inválido es 404.
func TestInvoiceSummaryRequiresTheSameAuthentication(t *testing.T) {
	f := &fakeInvoiceCommands{result: sampleDraft()}
	h := newInvoiceRouter(f)
	if rec := do(h, http.MethodGet, "/internal/v1/invoices/"+uuid.NewString()+"/summary", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin token: status=%d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/internal/v1/invoices/"+uuid.NewString()+"/summary", "tok-no-org"); rec.Code != http.StatusForbidden {
		t.Fatalf("sin organización: status=%d", rec.Code)
	}
	if rec := do(h, http.MethodPost, "/internal/v1/invoices/"+uuid.NewString()+"/summary", "tok-a"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: status=%d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "/internal/v1/invoices/no-es-uuid/summary", "tok-a"); rec.Code != http.StatusNotFound {
		t.Fatalf("id inválido: status=%d", rec.Code)
	}
	if f.called != "" {
		t.Fatalf("llegó al caso de uso: %s", f.called)
	}
}

func TestUpdateInvoiceNullableFields(t *testing.T) {
	f := &fakeInvoiceCommands{result: sampleDraft()}
	h := newInvoiceRouter(f)
	id := uuid.NewString()

	rec := doBody(h, http.MethodPatch, "/v1/invoices/"+id, "tok-a", `{"branchId":null,"creditTermDays":30}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	p := f.gotPatch
	if p.BranchID == nil || *p.BranchID != nil || p.CreditTermDays == nil || **p.CreditTermDays != 30 || f.gotLinesP != nil {
		t.Fatalf("patch = %+v (branchId null = quitar, lines ausente = nil)", p)
	}

	doBody(h, http.MethodPatch, "/v1/invoices/"+id, "tok-a", `{"notes":"x"}`)
	if f.gotPatch.BranchID != nil || f.gotPatch.CreditTermDays != nil {
		t.Fatal("un campo ausente no cambia")
	}

	rec = doBody(h, http.MethodPatch, "/v1/invoices/"+id, "tok-a", `{"documentType":"credit_note"}`)
	if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || p.Errors[0].Field != "documentType" {
		t.Fatalf("cambiar el tipo: status=%d %+v", rec.Code, p)
	}
}

func TestReplaceLinesHTTP(t *testing.T) {
	f := &fakeInvoiceCommands{result: sampleDraft()}
	h := newInvoiceRouter(f)
	id := uuid.New()
	rec := doBody(h, http.MethodPut, "/v1/invoices/"+id.String()+"/lines", "tok-a",
		`[{"productId":"`+uuid.NewString()+`","quantity":"2","unitPrice":"10"}]`)
	if rec.Code != http.StatusOK || f.called != "lines" || f.gotID != id || f.gotLines[0].UnitPrice.String() != "10" {
		t.Fatalf("status=%d llamado=%s", rec.Code, f.called)
	}
	rec = doBody(h, http.MethodPut, "/v1/invoices/"+id.String()+"/lines", "tok-a", `[]`)
	if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || p.Errors[0].Field != "lines" {
		t.Fatalf("lista vacía: status=%d %+v", rec.Code, p)
	}
}

func TestDiscardAndHistoryHTTP(t *testing.T) {
	f := &fakeInvoiceCommands{}
	h := newInvoiceRouter(f)
	id := uuid.New()
	if rec := do(h, http.MethodDelete, "/v1/invoices/"+id.String(), "tok-a"); rec.Code != http.StatusNoContent || f.gotID != id {
		t.Fatalf("delete: status=%d", rec.Code)
	}
	f.history = []app.StatusChange{{From: invoice.StatusDraft, To: invoice.StatusIssued, ChangedAt: time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)}}
	rec := do(h, http.MethodGet, "/v1/invoices/"+id.String()+"/history", "tok-a")
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if rec.Code != http.StatusOK || len(out) != 1 || out[0]["fromStatus"] != "draft" || out[0]["toStatus"] != "issued" {
		t.Fatalf("history: status=%d %v", rec.Code, out)
	}
}

func TestListInvoicesQueryHTTP(t *testing.T) {
	f := &fakeInvoiceCommands{result: sampleDraft()}
	cust := uuid.New()
	rec := do(newInvoiceRouter(f), http.MethodGet,
		"/v1/invoices?status=draft&documentType=invoice&customerId="+cust.String()+"&requiresCorrection=false&issuedFrom=2026-09-01&issuedTo=2026-09-30", "tok-a")
	q := f.gotQuery
	if rec.Code != http.StatusOK || q.Status != invoice.StatusDraft || q.DocumentType != invoice.TypeInvoice ||
		*q.CustomerID != cust || q.RequiresCorrection == nil || *q.RequiresCorrection || q.IssuedFrom != "2026-09-01" || q.IssuedTo != "2026-09-30" {
		t.Fatalf("status=%d query=%+v", rec.Code, q)
	}
	for _, bad := range []string{"status=open", "documentType=receipt", "customerId=x", "issuedFrom=25/09/2026"} {
		rec := do(newInvoiceRouter(&fakeInvoiceCommands{}), http.MethodGet, "/v1/invoices?"+bad, "tok-a")
		if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || p.Type != problem.TypeBase+"validation" {
			t.Fatalf("%s: status=%d", bad, rec.Code)
		}
	}
}

func TestReplaceLinesValidationNamesFields(t *testing.T) {
	f := &fakeInvoiceCommands{}
	rec := doBody(newInvoiceRouter(f), http.MethodPut, "/v1/invoices/"+uuid.NewString()+"/lines", "tok-a",
		`[{"quantity":"1"}]`)
	if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field != "lines[0].productId" {
		t.Fatalf("status=%d errors=%+v", rec.Code, p.Errors)
	}
}

func TestIssueInvoiceHTTP(t *testing.T) {
	issued := sampleDraft()
	at := time.Date(2026, 9, 25, 5, 30, 0, 0, time.UTC)
	by := userID
	issued.Status, issued.Number, issued.IssuedAt, issued.IssuedByUserID, issued.DueDate = invoice.StatusIssued, "FAC-00000001", &at, &by, "2026-09-24"
	issued.Customer = invoice.CustomerSnapshot{IdentificationTypeCode: "02", IdentificationNumber: "3101123456", LegalName: "Cliente S.A."}
	f := &fakeInvoiceCommands{result: issued}
	id := uuid.New()
	rec := doBody(newInvoiceRouter(f), http.MethodPost, "/v1/invoices/"+id.String()+"/issue", "tok-a", "", "Idempotency-Key", "k-1")
	if rec.Code != http.StatusCreated || f.gotID != id || f.gotKey != "k-1" {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	snap, _ := out["customerSnapshot"].(map[string]any)
	if out["status"] != "issued" || out["number"] != "FAC-00000001" || out["issuedAt"] != "2026-09-25T05:30:00Z" ||
		out["dueDate"] != "2026-09-24" || snap["legalName"] != "Cliente S.A." {
		t.Fatalf("respuesta = %v", out)
	}

	f.replayed = true
	rec = doBody(newInvoiceRouter(f), http.MethodPost, "/v1/invoices/"+id.String()+"/issue", "tok-a", "", "Idempotency-Key", "k-1")
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("reintento: status=%d", rec.Code)
	}
}

func TestIssueInvoiceErrorsHTTP(t *testing.T) {
	path := "/v1/invoices/" + uuid.NewString() + "/issue"
	for name, tc := range map[string]struct {
		key         string
		err         error
		status      int
		problemType string
	}{
		"sin Idempotency-Key": {"", nil, http.StatusBadRequest, "idempotency-key-required"},
		"ya emitido":          {"k", invoice.ErrNotDraft, http.StatusConflict, "invoice-not-draft"},
		"sin líneas":          {"k", invoice.ErrWithoutLines, http.StatusUnprocessableEntity, "invoice-without-lines"},
		"cliente inactivo":    {"k", app.ErrCustomerInactive, http.StatusUnprocessableEntity, "customer-inactive"},
		"clave reutilizada":   {"k", app.ErrIdempotencyKeyReused, http.StatusUnprocessableEntity, "idempotency-key-reused"},
		"de otra org":         {"k", app.ErrNotFound, http.StatusNotFound, "not-found"},
		"rol sin permiso":     {"k", app.ErrForbidden, http.StatusForbidden, "forbidden"},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeInvoiceCommands{err: tc.err}
			var headers []string
			if tc.key != "" {
				headers = []string{"Idempotency-Key", tc.key}
			}
			rec := doBody(newInvoiceRouter(f), http.MethodPost, path, "tok-a", "", headers...)
			if p := problemOf(t, rec); rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
		})
	}
}
