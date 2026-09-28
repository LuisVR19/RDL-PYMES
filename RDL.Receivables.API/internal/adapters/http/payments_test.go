package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/tenancy"
)

// fakeCommands registra lo que llega a cada caso de uso y devuelve el error configurado.
type fakeCommands struct {
	err      error
	key      string
	tenant   tenancy.Context
	payment  app.PaymentInput
	apply    app.ApplyInput
	id       uuid.UUID
	reason   string
	followUp app.FollowUpInput
	promise  app.PromiseInput
	status   collection.PromiseStatus
	calls    int
}

func (f *fakeCommands) record(t tenancy.Context, key string) { f.calls++; f.tenant, f.key = t, key }

type createPaymentFn struct{ *fakeCommands }

func (f createPaymentFn) Execute(_ context.Context, t tenancy.Context, key string, in app.PaymentInput) (app.CommandResult[app.PaymentView], error) {
	f.record(t, key)
	f.payment = in
	return app.CommandResult[app.PaymentView]{Value: app.PaymentView{ID: uuid.New(), Status: payment.StatusPosted}}, f.err
}

type applyFn struct{ *fakeCommands }

func (f applyFn) Execute(_ context.Context, t tenancy.Context, key string, in app.ApplyInput) (app.CommandResult[app.ApplicationView], error) {
	f.record(t, key)
	f.apply = in
	return app.CommandResult[app.ApplicationView]{Value: app.ApplicationView{ID: uuid.New()}}, f.err
}

type reasonFn struct{ *fakeCommands }

func (f reasonFn) Execute(_ context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (app.CommandResult[app.ApplicationView], error) {
	f.record(t, key)
	f.id, f.reason = id, reason
	return app.CommandResult[app.ApplicationView]{Value: app.ApplicationView{ID: id}}, f.err
}

type voidFn struct{ *fakeCommands }

func (f voidFn) Execute(_ context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (app.CommandResult[app.PaymentView], error) {
	f.record(t, key)
	f.id, f.reason = id, reason
	return app.CommandResult[app.PaymentView]{Value: app.PaymentView{ID: id, Status: payment.StatusVoided}}, f.err
}

type followUpFn struct{ *fakeCommands }

func (f followUpFn) Execute(_ context.Context, t tenancy.Context, key string, id uuid.UUID, in app.FollowUpInput) (app.CommandResult[app.FollowUpView], error) {
	f.record(t, key)
	f.id, f.followUp = id, in
	return app.CommandResult[app.FollowUpView]{Value: app.FollowUpView{ID: uuid.New(), ReceivableID: id, Type: in.Type}}, f.err
}

type promiseFn struct{ *fakeCommands }

func (f promiseFn) Execute(_ context.Context, t tenancy.Context, key string, id uuid.UUID, in app.PromiseInput) (app.CommandResult[app.PromiseView], error) {
	f.record(t, key)
	f.id, f.promise = id, in
	return app.CommandResult[app.PromiseView]{Value: app.PromiseView{ID: uuid.New(), ReceivableID: id}}, f.err
}

type closeFn struct{ *fakeCommands }

func (f closeFn) Execute(_ context.Context, t tenancy.Context, key string, id uuid.UUID, to collection.PromiseStatus) (app.CommandResult[app.PromiseView], error) {
	f.record(t, key)
	f.id, f.status = id, to
	return app.CommandResult[app.PromiseView]{Value: app.PromiseView{ID: id, Status: to}}, f.err
}

func commandRouter(f *fakeCommands) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    fakeVerifier{id: tenancy.Identity{Subject: "sub-1", OrganizationID: tokenOrg}},
		Memberships: fakeResolver{m: tenancy.Membership{UserID: uuid.New(), Roles: []string{"owner"}}},
		Receivables: &ReceivableHandlers{List: &fakeList{}},
		Payments: &PaymentHandlers{
			Create: createPaymentFn{f}, Apply: applyFn{f}, Reverse: reasonFn{f}, Void: voidFn{f},
		},
		Collection: &CollectionHandlers{
			CreateFollowUp: followUpFn{f}, CreatePromise: promiseFn{f}, ClosePromise: closeFn{f},
		},
	})
}

func post(h http.Handler, target, body string, headers ...string) (*httptest.ResponseRecorder, problemBody) {
	req := httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var p problemBody
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec, p
}

const key = "Idempotency-Key"

func TestCreatePaymentParsesMoneyWithoutFloat(t *testing.T) {
	f := &fakeCommands{}
	h := commandRouter(f)
	rid := uuid.New()
	body := fmt.Sprintf(`{"customerId":%q,"receivedOn":"2026-09-20","amount":"1300.50","currency":"CRC",
		"paymentMethodCode":"04","applications":[{"receivableId":%q,"amount":"0.00001"}]}`, uuid.NewString(), rid)
	rec, _ := post(h, "/v1/payments", body, key, "k-1")
	if rec.Code != http.StatusCreated || f.key != "k-1" || f.tenant.OrganizationID() != tokenOrg {
		t.Fatalf("%d, key %q, org %s", rec.Code, f.key, f.tenant.OrganizationID())
	}
	if f.payment.Amount.String() != "1300.5" || f.payment.Applications[0].Amount.String() != "0.00001" ||
		f.payment.Applications[0].ReceivableID != rid || f.payment.ReceivedOn.String() != "2026-09-20" {
		t.Errorf("entrada: %+v", f.payment)
	}
}

func TestCommandValidation(t *testing.T) {
	cases := []struct {
		name, target, body string
		headers            []string
		status             int
		problem            string
		field              string
	}{
		{"sin Idempotency-Key", "/v1/payments", `{}`, nil, 400, "idempotency-key-required", ""},
		{"JSON roto", "/v1/payments", `{"amount":`, []string{key, "k"}, 400, "malformed-request", ""},
		{"campo desconocido (organizationId)", "/v1/payments", `{"organizationId":"x"}`, []string{key, "k"}, 400, "malformed-request", ""},
		{"monto como número", "/v1/payments", `{"amount":1300.5}`, []string{key, "k"}, 400, "malformed-request", ""},
		{"monto con 6 decimales", "/v1/payments", `{"customerId":"` + uuid.NewString() + `","receivedOn":"2026-09-20","amount":"1.000001","currency":"CRC","paymentMethodCode":"01"}`, []string{key, "k"}, 422, "validation", "amount"},
		{"monto cero", "/v1/payments", `{"customerId":"` + uuid.NewString() + `","receivedOn":"2026-09-20","amount":"0","currency":"CRC","paymentMethodCode":"01"}`, []string{key, "k"}, 422, "validation", "amount"},
		{"moneda en minúsculas", "/v1/payments", `{"customerId":"` + uuid.NewString() + `","receivedOn":"2026-09-20","amount":"1","currency":"crc","paymentMethodCode":"01"}`, []string{key, "k"}, 422, "validation", "currency"},
		{"fecha inválida", "/v1/payments", `{"customerId":"` + uuid.NewString() + `","receivedOn":"20/09/2026","amount":"1","currency":"CRC","paymentMethodCode":"01"}`, []string{key, "k"}, 422, "validation", "receivedOn"},
		{"aplicación sin cuenta", "/v1/payment-applications", `{"paymentId":"` + uuid.NewString() + `","amount":"1"}`, []string{key, "k"}, 422, "validation", "receivableId"},
		{"reverso sin motivo", "/v1/payment-applications/" + uuid.NewString() + "/reverse", `{"reason":"  "}`, []string{key, "k"}, 422, "validation", "reason"},
		{"anulación con motivo largo", "/v1/payments/" + uuid.NewString() + "/void", `{"reason":"` + strings.Repeat("x", 501) + `"}`, []string{key, "k"}, 422, "validation", "reason"},
		{"id de ruta inválido", "/v1/payments/no-es-uuid/void", `{"reason":"x"}`, []string{key, "k"}, 404, "not-found", ""},
		{"seguimiento de tipo desconocido", "/v1/receivables/" + uuid.NewString() + "/follow-ups", `{"followupType":"fax","notes":"x"}`, []string{key, "k"}, 422, "validation", "followupType"},
		{"promesa sin fecha", "/v1/receivables/" + uuid.NewString() + "/promises", `{"promisedAmount":"1"}`, []string{key, "k"}, 422, "validation", "promisedOn"},
		{"cierre a pending", "/v1/payment-promises/" + uuid.NewString() + "/status", `{"status":"pending"}`, []string{key, "k"}, 422, "validation", "status"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeCommands{}
			rec, p := post(commandRouter(f), c.target, c.body, c.headers...)
			if rec.Code != c.status || p.Type != "urn:rdl:receivables:problem:"+c.problem {
				t.Fatalf("%d %s: %s", rec.Code, p.Type, rec.Body)
			}
			if c.field != "" && (len(p.Errors) == 0 || p.Errors[0].Field != c.field) {
				t.Errorf("campo: %+v", p.Errors)
			}
			if f.calls != 0 {
				t.Error("una petición inválida no debe llegar al caso de uso")
			}
		})
	}
}

// Cada error de dominio o de aplicación tiene su status y su problem type, en un único lugar.
func TestErrorMapping(t *testing.T) {
	cases := []struct {
		err     error
		status  int
		problem string
	}{
		{payment.ErrExceedsPayment, 422, "application-exceeds-payment"},
		{receivable.ErrExceedsBalance, 422, "application-exceeds-balance"},
		{settlement.ErrCurrencyMismatch, 422, "currency-mismatch"},
		{settlement.ErrCustomerMismatch, 422, "customer-mismatch"},
		{payment.ErrVoided, 409, "payment-voided"},
		{payment.ErrApplicationReversed, 409, "application-reversed"},
		{receivable.ErrApplicationReversed, 409, "application-reversed"},
		{receivable.ErrCancelled, 409, "conflict"},
		{payment.ErrDuplicateApplication, 409, "conflict"},
		{collection.ErrPromiseClosed, 409, "conflict"},
		{collection.ErrNotCollectible, 409, "conflict"},
		{collection.ErrPromiseExceedsBalance, 422, "validation"},
		{amount.ErrInvalid, 422, "validation"},
		{app.ValidationError{Field: "receivedOn", Message: "futura"}, 422, "validation"},
		{app.ErrIdempotencyKeyReused, 422, "idempotency-key-reused"},
		{app.ErrNotFound, 404, "not-found"},
		{app.ErrForbidden, 403, "forbidden"},
		{fmt.Errorf("envuelto: %w", app.ErrInconsistentState), 500, "internal"},
		{errors.New("pgx: conexión cerrada con detalle interno"), 500, "internal"},
	}
	for _, c := range cases {
		f := &fakeCommands{err: c.err}
		rec, p := post(commandRouter(f), "/v1/payment-applications",
			`{"paymentId":"`+uuid.NewString()+`","receivableId":"`+uuid.NewString()+`","amount":"1"}`, key, "k")
		if rec.Code != c.status || p.Type != "urn:rdl:receivables:problem:"+c.problem {
			t.Errorf("%v: %d %s", c.err, rec.Code, p.Type)
		}
		if c.status == 500 && strings.Contains(rec.Body.String(), "pgx") {
			t.Errorf("un error interno se filtró a la respuesta: %s", rec.Body)
		}
	}
}

// Métodos no permitidos en Problem Details, sin chocar /v1/receivables/aging con /v1/receivables/{id}.
func TestMethodNotAllowed(t *testing.T) {
	h := commandRouter(&fakeCommands{})
	for _, c := range []struct{ method, path string }{
		{http.MethodDelete, "/v1/payments/" + uuid.NewString()},
		{http.MethodGet, "/v1/payments/" + uuid.NewString() + "/void"},
		{http.MethodPost, "/v1/receivables/aging"},
		{http.MethodPut, "/internal/v1/receivables/by-invoice/" + uuid.NewString()},
	} {
		rec, p := do(h, c.method, c.path, "valid")
		if rec.Code != http.StatusMethodNotAllowed || p.Type != "urn:rdl:receivables:problem:method-not-allowed" {
			t.Errorf("%s %s: %d %s", c.method, c.path, rec.Code, p.Type)
		}
	}
}

func TestReasonAndStatusReachUseCase(t *testing.T) {
	f := &fakeCommands{}
	h := commandRouter(f)
	id := uuid.New()
	if rec, _ := post(h, "/v1/payment-applications/"+id.String()+"/reverse", `{"reason":"  Monto equivocado "}`, key, "k"); rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if f.id != id || f.reason != "Monto equivocado" {
		t.Errorf("reverso: %s %q", f.id, f.reason)
	}
	if rec, _ := post(h, "/v1/payment-promises/"+id.String()+"/status", `{"status":"broken"}`, key, "k"); rec.Code != 200 || f.status != collection.PromiseBroken {
		t.Errorf("cierre: %d %s", rec.Code, f.status)
	}
	if rec, _ := post(h, "/v1/receivables/"+id.String()+"/follow-ups", `{"followupType":"visit","notes":"x","nextActionOn":"2026-10-01"}`, key, "k"); rec.Code != 201 ||
		f.followUp.NextActionOn == nil || f.followUp.NextActionOn.String() != "2026-10-01" {
		t.Errorf("seguimiento: %d %+v", rec.Code, f.followUp)
	}
}
