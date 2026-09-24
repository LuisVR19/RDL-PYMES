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
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

type fakeProductCommands struct {
	gotKey   string
	gotInput product.NewInput
	gotID    uuid.UUID
	gotPatch product.Patch
	called   bool
	result   product.Product
	err      error
}

func (f *fakeProductCommands) Create() createProduct {
	return productCreateFn(func(_ context.Context, _ tenancy.Context, k string, in product.NewInput) (app.CreateProductResult, error) {
		f.called, f.gotKey, f.gotInput = true, k, in
		return app.CreateProductResult{Product: f.result}, f.err
	})
}

func (f *fakeProductCommands) Update() updateProduct {
	return productUpdateFn(func(_ context.Context, _ tenancy.Context, id uuid.UUID, p product.Patch) (product.Product, error) {
		f.called, f.gotID, f.gotPatch = true, id, p
		return f.result, f.err
	})
}

type productCreateFn func(context.Context, tenancy.Context, string, product.NewInput) (app.CreateProductResult, error)

func (fn productCreateFn) Execute(ctx context.Context, t tenancy.Context, k string, in product.NewInput) (app.CreateProductResult, error) {
	return fn(ctx, t, k, in)
}

type productUpdateFn func(context.Context, tenancy.Context, uuid.UUID, product.Patch) (product.Product, error)

func (fn productUpdateFn) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p product.Patch) (product.Product, error) {
	return fn(ctx, t, id, p)
}

func newProductRouter(f *fakeProductCommands) http.Handler {
	return newRouterWith(Deps{Products: &ProductHandlers{Create: f.Create(), Update: f.Update()}})
}

func sampleProduct() product.Product {
	now := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	return product.Product{
		ID: uuid.New(), OrganizationID: orgA, Code: "SERV-01", Description: "Consultoría", CabysCode: "8314100000100",
		UnitOfMeasureCode: "Sp", UnitPrice: money.MustAmountForTest("25000.5"), Currency: money.MustCurrencyForTest("CRC"),
		IsService: true, IsActive: true, Taxes: []product.Tax{{TypeCode: "01", RateCode: "08"}}, CreatedAt: now, UpdatedAt: now,
	}
}

const validProductBody = `{"code":"SERV-01","description":"Consultoría","cabysCode":"8314100000100","unitOfMeasureCode":"Sp",
"unitPrice":"25000.50","currency":"CRC","isService":true,"taxes":[{"taxTypeCode":"01","taxRateCode":"08"}]}`

func TestCreateProductHTTP(t *testing.T) {
	f := &fakeProductCommands{result: sampleProduct()}
	rec := doBody(newProductRouter(f), http.MethodPost, "/v1/products", "tok-a", validProductBody, "Idempotency-Key", "k-1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	in := f.gotInput
	if f.gotKey != "k-1" || in.UnitPrice.String() != "25000.50" || in.Currency.String() != "CRC" || !in.IsService ||
		in.IsActive != nil || len(in.Taxes) != 1 || in.Taxes[0].RateCode != "08" {
		t.Fatalf("input = %+v", in)
	}
	var body map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body["unitPrice"] != "25000.5" || body["isActive"] != true || body["id"] != f.result.ID.String() {
		t.Fatalf("respuesta = %v", body)
	}
	if taxes, _ := body["taxes"].([]any); len(taxes) != 1 {
		t.Fatalf("taxes = %v", body["taxes"])
	}
}

func TestCreateProductRejectsBadMoney(t *testing.T) {
	cases := []struct {
		name, body  string
		status      int
		problemType string
		field       string
	}{
		{"precio como number", `{"code":"A","description":"d","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":25000,"currency":"CRC","isService":true}`,
			http.StatusBadRequest, "malformed-request", ""},
		{"precio negativo", `{"code":"A","description":"d","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":"-1","currency":"CRC","isService":true}`,
			http.StatusUnprocessableEntity, "validation", "unitPrice"},
		{"precio con 6 decimales", `{"code":"A","description":"d","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":"1.123456","currency":"CRC","isService":true}`,
			http.StatusUnprocessableEntity, "validation", "unitPrice"},
		{"moneda en minúsculas", `{"code":"A","description":"d","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":"1","currency":"crc","isService":true}`,
			http.StatusUnprocessableEntity, "validation", "currency"},
		{"sin isService", `{"code":"A","description":"d","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":"1","currency":"CRC"}`,
			http.StatusUnprocessableEntity, "validation", "isService"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeProductCommands{}
			rec := doBody(newProductRouter(f), http.MethodPost, "/v1/products", "tok-a", tc.body, "Idempotency-Key", "k")
			p := problemOf(t, rec)
			if rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s detail=%s", rec.Code, p.Type, p.Detail)
			}
			if tc.field != "" && (len(p.Errors) != 1 || p.Errors[0].Field != tc.field) {
				t.Fatalf("errors = %+v", p.Errors)
			}
			if f.called {
				t.Fatal("un monto inválido no llega al caso de uso")
			}
		})
	}
}

func TestProductErrorsHTTP(t *testing.T) {
	_, domainErr := product.New(uuid.New(), product.NewInput{Currency: money.MustCurrencyForTest("CRC")})
	for name, tc := range map[string]struct {
		err         error
		status      int
		problemType string
	}{
		"código repetido": {app.ErrProductCodeTaken, http.StatusConflict, "product-code-taken"},
		"dominio":         {domainErr, http.StatusUnprocessableEntity, "validation"},
	} {
		t.Run(name, func(t *testing.T) {
			f := &fakeProductCommands{err: tc.err}
			rec := doBody(newProductRouter(f), http.MethodPost, "/v1/products", "tok-a", validProductBody, "Idempotency-Key", "k")
			if p := problemOf(t, rec); rec.Code != tc.status || p.Type != problem.TypeBase+tc.problemType {
				t.Fatalf("status=%d type=%s", rec.Code, p.Type)
			}
		})
	}
}

func TestUpdateProductHTTP(t *testing.T) {
	f := &fakeProductCommands{result: sampleProduct()}
	id := f.result.ID
	rec := doBody(newProductRouter(f), http.MethodPatch, "/v1/products/"+id.String(), "tok-a",
		`{"unitPrice":"30000","isActive":false,"taxes":[]}`)
	if rec.Code != http.StatusOK || f.gotID != id {
		t.Fatalf("status=%d cuerpo=%s", rec.Code, rec.Body)
	}
	p := f.gotPatch
	if p.UnitPrice == nil || p.UnitPrice.String() != "30000" || p.IsActive == nil || *p.IsActive ||
		p.Taxes == nil || len(*p.Taxes) != 0 || p.Code != nil || p.Currency != nil {
		t.Fatalf("patch = %+v", p)
	}

	// Sin taxes en el cuerpo, los impuestos no cambian (nil, no lista vacía).
	f = &fakeProductCommands{result: sampleProduct()}
	doBody(newProductRouter(f), http.MethodPatch, "/v1/products/"+id.String(), "tok-a", `{"description":"Otra"}`)
	if f.gotPatch.Taxes != nil {
		t.Fatal("taxes ausente debe llegar como nil")
	}
}

func TestUpdateProductAcceptsFullProductInput(t *testing.T) {
	f := &fakeProductCommands{result: sampleProduct()}
	rec := doBody(newProductRouter(f), http.MethodPatch, "/v1/products/"+f.result.ID.String(), "tok-a", validProductBody)
	if rec.Code != http.StatusOK || f.gotPatch.Code == nil || f.gotPatch.Taxes == nil {
		t.Fatalf("el cuerpo ProductInput del contrato debe aceptarse: status=%d patch=%+v", rec.Code, f.gotPatch)
	}
}
