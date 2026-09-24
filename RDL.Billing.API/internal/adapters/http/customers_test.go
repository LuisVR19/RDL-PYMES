package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/pkg/tenancy"
)

// fakeCustomerCommands registra lo que llega a los casos de uso y responde lo configurado.
type fakeCustomerCommands struct {
	gotTenant tenancy.Context
	gotKey    string
	gotInput  customer.NewInput
	gotID     uuid.UUID
	gotPatch  customer.Patch
	result    customer.Customer
	replayed  bool
	err       error
}

func (f *fakeCustomerCommands) create(_ context.Context, t tenancy.Context, key string, in customer.NewInput) (app.CreateCustomerResult, error) {
	f.gotTenant, f.gotKey, f.gotInput = t, key, in
	return app.CreateCustomerResult{Customer: f.result, Replayed: f.replayed}, f.err
}

func (f *fakeCustomerCommands) get(_ context.Context, t tenancy.Context, id uuid.UUID) (customer.Customer, error) {
	f.gotTenant, f.gotID = t, id
	return f.result, f.err
}

func (f *fakeCustomerCommands) update(_ context.Context, t tenancy.Context, id uuid.UUID, p customer.Patch) (customer.Customer, error) {
	f.gotTenant, f.gotID, f.gotPatch = t, id, p
	return f.result, f.err
}

type createFn func(context.Context, tenancy.Context, string, customer.NewInput) (app.CreateCustomerResult, error)

func (fn createFn) Execute(ctx context.Context, t tenancy.Context, k string, in customer.NewInput) (app.CreateCustomerResult, error) {
	return fn(ctx, t, k, in)
}

type getFn func(context.Context, tenancy.Context, uuid.UUID) (customer.Customer, error)

func (fn getFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (customer.Customer, error) {
	return fn(ctx, t, id)
}

type updateFn func(context.Context, tenancy.Context, uuid.UUID, customer.Patch) (customer.Customer, error)

func (fn updateFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p customer.Patch) (customer.Customer, error) {
	return fn(ctx, t, id, p)
}

func newCommandsRouter(f *fakeCustomerCommands) http.Handler {
	return newCustomerRouter(&CustomerHandlers{
		List:   &fakeListCustomers{},
		Create: createFn(f.create),
		Get:    getFn(f.get),
		Update: updateFn(f.update),
	})
}

func sampleCustomer() customer.Customer {
	now := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	return customer.Customer{
		ID: uuid.New(), OrganizationID: orgA,
		Identification: customer.Identification{TypeCode: "01", Number: "112340567"},
		LegalName:      "Ana Pérez", Email: "ana@example.com", IsActive: true, CreatedAt: now, UpdatedAt: now,
	}
}

const validCreateBody = `{"identification":{"typeCode":"01","number":"112340567"},"legalName":"Ana Pérez","email":"ana@example.com"}`

func TestCreateCustomerHTTP(t *testing.T) {
	f := &fakeCustomerCommands{result: sampleCustomer()}
	rec := doBody(newCommandsRouter(f), http.MethodPost, "/v1/customers", "tok-a", validCreateBody, "Idempotency-Key", "k-1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	if f.gotKey != "k-1" || f.gotTenant.OrganizationID() != orgA || f.gotInput.IdentificationNumber != "112340567" ||
		f.gotInput.LegalName != "Ana Pérez" || f.gotInput.Email != "ana@example.com" {
		t.Fatalf("llegó al caso de uso: key=%q input=%+v", f.gotKey, f.gotInput)
	}
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["id"] != f.result.ID.String() || body["isActive"] != true || rec.Header().Get("Idempotent-Replayed") != "" {
		t.Fatalf("respuesta = %v", body)
	}
}

func TestCreateCustomerReplayHeader(t *testing.T) {
	f := &fakeCustomerCommands{result: sampleCustomer(), replayed: true}
	rec := doBody(newCommandsRouter(f), http.MethodPost, "/v1/customers", "tok-a", validCreateBody, "Idempotency-Key", "k-1")
	if rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("status=%d replayed=%q", rec.Code, rec.Header().Get("Idempotent-Replayed"))
	}
}

func TestCreateCustomerErrors(t *testing.T) {
	cases := []struct {
		name, body, key string
		err             error
		status          int
		problemType     string
	}{
		{"sin Idempotency-Key", validCreateBody, "", nil, http.StatusBadRequest, "idempotency-key-required"},
		{"Idempotency-Key con espacios internos", validCreateBody, "k 1", nil, http.StatusBadRequest, "idempotency-key-required"},
		{"JSON mal formado", `{"legalName":`, "k", nil, http.StatusBadRequest, "malformed-request"},
		{"campo desconocido", `{"organizationId":"` + orgB.String() + `"}`, "k", nil, http.StatusBadRequest, "malformed-request"},
		{"sin identificación", `{"legalName":"Ana"}`, "k", nil, http.StatusUnprocessableEntity, "validation"},
		{"dominio inválido", validCreateBody, "k", customer.FieldError{Field: "legalName", Message: "x"}, http.StatusUnprocessableEntity, "validation"},
		{"identificación repetida", validCreateBody, "k", app.ErrCustomerIdentificationTaken, http.StatusConflict, "customer-identification-taken"},
		{"clave reutilizada", validCreateBody, "k", app.ErrIdempotencyKeyReused, http.StatusUnprocessableEntity, "idempotency-key-reused"},
		{"rol sin permiso", validCreateBody, "k", app.ErrForbidden, http.StatusForbidden, "forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeCustomerCommands{err: tc.err}
			var headers []string
			if tc.key != "" {
				headers = []string{"Idempotency-Key", tc.key}
			}
			rec := doBody(newCommandsRouter(f), http.MethodPost, "/v1/customers", "tok-a", tc.body, headers...)
			p := problemOf(t, rec)
			if rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s detail=%s", rec.Code, p.Type, p.Detail)
			}
		})
	}
}

func TestCreateCustomerValidationNamesJSONFields(t *testing.T) {
	// Error real del dominio: sin número de identificación ni nombre legal.
	_, domainErr := customer.New(uuid.New(), uuid.New(), customer.NewInput{IdentificationTypeCode: "01"})
	f := &fakeCustomerCommands{err: domainErr}
	rec := doBody(newCommandsRouter(f), http.MethodPost, "/v1/customers", "tok-a", validCreateBody, "Idempotency-Key", "k")
	p := problemOf(t, rec)
	if len(p.Errors) != 2 || p.Errors[0].Field != "identification.number" || p.Errors[1].Field != "legalName" {
		t.Fatalf("errors = %+v", p.Errors)
	}
}

func TestGetCustomerHTTP(t *testing.T) {
	f := &fakeCustomerCommands{result: sampleCustomer()}
	rec := do(newCommandsRouter(f), http.MethodGet, "/v1/customers/"+f.result.ID.String(), "tok-a")
	if rec.Code != http.StatusOK || f.gotID != f.result.ID || f.gotTenant.OrganizationID() != orgA {
		t.Fatalf("status=%d id=%v", rec.Code, f.gotID)
	}
}

func TestGetCustomerNotFoundCases(t *testing.T) {
	for name, tc := range map[string]struct {
		path string
		err  error
	}{
		"id mal formado":            {"/v1/customers/no-es-uuid", nil},
		"inexistente o de otra org": {"/v1/customers/" + uuid.NewString(), app.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := do(newCommandsRouter(&fakeCustomerCommands{err: tc.err}), http.MethodGet, tc.path, "tok-a")
			if p := problemOf(t, rec); rec.Code != http.StatusNotFound || p.Type != problem.TypeBase+"not-found" {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
		})
	}
}

func TestUpdateCustomerHTTP(t *testing.T) {
	f := &fakeCustomerCommands{result: sampleCustomer()}
	id := f.result.ID
	rec := doBody(newCommandsRouter(f), http.MethodPatch, "/v1/customers/"+id.String(), "tok-a",
		`{"legalName":"Ana P.","email":"","isActive":false}`)
	if rec.Code != http.StatusOK || f.gotID != id {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	p := f.gotPatch
	if p.LegalName == nil || *p.LegalName != "Ana P." || p.Email == nil || *p.Email != "" ||
		p.IsActive == nil || *p.IsActive || p.Phone != nil || p.TradeName != nil || p.Address != nil {
		t.Fatalf("patch = %+v (ausente = nil, \"\" = borrar)", p)
	}
}

func TestUpdateCustomerCannotChangeIdentification(t *testing.T) {
	f := &fakeCustomerCommands{result: sampleCustomer()}
	rec := doBody(newCommandsRouter(f), http.MethodPatch, "/v1/customers/"+f.result.ID.String(), "tok-a",
		`{"identification":{"typeCode":"02","number":"3101123456"}}`)
	if p := problemOf(t, rec); rec.Code != http.StatusBadRequest || !strings.Contains(p.Detail, "identification") {
		t.Fatalf("status=%d detail=%s", rec.Code, p.Detail)
	}
	if f.gotID != uuid.Nil {
		t.Fatal("no debe llegar al caso de uso")
	}
}
