package app

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// scenario: una organización con un cliente activo, uno inactivo y dos productos (uno en USD).
type scenario struct {
	f                  *fakeTx
	org                uuid.UUID
	tn                 tenancy.Context
	customer, inactive uuid.UUID
	crcProduct         product.Product
	usdProduct         product.Product
}

func newScenario(t *testing.T) scenario {
	t.Helper()
	org := uuid.New()
	f := newFakeTx()
	owner := tenant(org, "owner")
	c1, err := NewCreateCustomer(f).Execute(ctx, owner, "c1", newCustomerInput("1"))
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := NewCreateCustomer(f).Execute(ctx, owner, "c2", newCustomerInput("2"))
	off := false
	if _, err := NewUpdateCustomer(f).Execute(ctx, owner, c2.Customer.ID, customer.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	p1, err := NewCreateProduct(f).Execute(ctx, owner, "p1", newProductInput("SERV-01"))
	if err != nil {
		t.Fatal(err)
	}
	usd := newProductInput("SERV-USD")
	usd.Currency, usd.UnitPrice = money.MustCurrencyForTest("USD"), money.MustAmountForTest("100")
	p2, _ := NewCreateProduct(f).Execute(ctx, owner, "p2", usd)
	f.state.audit = nil
	return scenario{f: f, org: org, tn: tenant(org, "biller"), customer: c1.Customer.ID, inactive: c2.Customer.ID,
		crcProduct: p1.Product, usdProduct: p2.Product}
}

func (s scenario) header() HeaderInput {
	return HeaderInput{DocumentType: invoice.TypeInvoice, CustomerID: s.customer, SaleConditionCode: "01",
		Currency: money.MustCurrencyForTest("CRC")}
}

func (s scenario) line(p product.Product, qty string) LineRequest {
	return LineRequest{ProductID: p.ID, Quantity: money.MustQuantityForTest(qty)}
}

func TestCreateDraftWithLines(t *testing.T) {
	s := newScenario(t)
	res, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k1", s.header(), []LineRequest{s.line(s.crcProduct, "2")})
	if err != nil {
		t.Fatal(err)
	}
	inv := res.Invoice
	if inv.Status != invoice.StatusDraft || inv.Number != "" || inv.ExchangeRate.String() != "1" || len(inv.Lines) != 1 {
		t.Fatalf("borrador = %+v", inv)
	}
	l := inv.Lines[0]
	// El snapshot y la tasa salen del producto y del catálogo, no del request.
	if *l.ProductID != s.crcProduct.ID || l.CabysCode != s.crcProduct.CabysCode || l.Description != "Consultoría" ||
		l.UnitPrice.String() != "25000.5" || l.Taxes[0].Rate.String() != "13" || l.Taxes[0].RateCode != "08" {
		t.Fatalf("línea = %+v", l)
	}
	if inv.Totals.Subtotal.String() != "50001" || inv.Totals.Tax.String() != "6500.13" || inv.Totals.Total.String() != "56501.13" {
		t.Fatalf("totales = %+v", inv.Totals)
	}
	if len(s.f.state.audit) != 1 || s.f.state.audit[0].Action != "invoice.created" || len(s.f.state.history[inv.ID]) != 0 {
		t.Fatalf("audit=%+v historial=%v", s.f.state.audit, s.f.state.history[inv.ID])
	}
}

func TestCreateDraftIdempotent(t *testing.T) {
	s := newScenario(t)
	uc := NewCreateInvoiceDraft(s.f)
	lines := []LineRequest{s.line(s.crcProduct, "1")}
	first, err := uc.Execute(ctx, s.tn, "k1", s.header(), lines)
	if err != nil {
		t.Fatal(err)
	}
	again, err := uc.Execute(ctx, s.tn, "k1", s.header(), lines)
	if err != nil || !again.Replayed || again.Invoice.ID != first.Invoice.ID || len(s.f.state.invoices) != 1 {
		t.Fatalf("reintento: %+v err=%v", again, err)
	}
	if _, err := uc.Execute(ctx, s.tn, "k1", s.header(), []LineRequest{s.line(s.crcProduct, "3")}); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("otro contenido: err = %v", err)
	}
}

func TestCreateDraftReferencesAreValidated(t *testing.T) {
	s := newScenario(t)
	other := newScenario(t) // otra organización
	uc := NewCreateInvoiceDraft(s.f)
	foreignCustomer := s.header()
	foreignCustomer.CustomerID = other.customer
	if _, err := uc.Execute(ctx, s.tn, "a", foreignCustomer, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cliente de otra organización: err = %v", err)
	}
	inactive := s.header()
	inactive.CustomerID = s.inactive
	if _, err := uc.Execute(ctx, s.tn, "b", inactive, nil); !errors.Is(err, ErrCustomerInactive) {
		t.Fatalf("cliente inactivo: err = %v", err)
	}
	foreignLine := []LineRequest{{ProductID: other.crcProduct.ID, Quantity: money.MustQuantityForTest("1")}}
	if _, err := uc.Execute(ctx, s.tn, "c", s.header(), foreignLine); !errors.Is(err, ErrNotFound) {
		t.Fatalf("producto de otra organización: err = %v", err)
	}
	foreignBranch := s.header()
	b := s.f.addBranch(other.org, true)
	foreignBranch.BranchID = &b
	if _, err := uc.Execute(ctx, s.tn, "d", foreignBranch, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sucursal de otra organización: err = %v", err)
	}
	closed := s.header()
	cb := s.f.addBranch(s.org, false)
	closed.BranchID = &cb
	if _, err := uc.Execute(ctx, s.tn, "e", closed, nil); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("sucursal inactiva: err = %v", err)
	}
	ok := s.header()
	good := s.f.addBranch(s.org, true)
	ok.BranchID = &good
	if res, err := uc.Execute(ctx, s.tn, "f", ok, nil); err != nil || *res.Invoice.BranchID != good {
		t.Fatalf("sucursal propia y activa: err = %v", err)
	}
	if len(s.f.state.invoices) != 1 || len(s.f.state.audit) != 1 {
		t.Fatal("ningún intento rechazado deja algo escrito")
	}
}

func TestCreateDraftCurrencyRules(t *testing.T) {
	s := newScenario(t)
	uc := NewCreateInvoiceDraft(s.f)
	usd := s.header()
	usd.Currency = money.MustCurrencyForTest("USD")
	if _, err := uc.Execute(ctx, s.tn, "a", usd, nil); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("USD sin tipo de cambio: err = %v", err)
	}
	two := money.MustExchangeRateForTest("2")
	local := s.header()
	local.ExchangeRate = &two
	if _, err := uc.Execute(ctx, s.tn, "b", local, nil); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("CRC con tipo de cambio 2: err = %v", err)
	}
	// Producto en USD en un documento en CRC: hay que indicar el precio.
	if _, err := uc.Execute(ctx, s.tn, "c", s.header(), []LineRequest{s.line(s.usdProduct, "1")}); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("producto en otra moneda sin precio: err = %v", err)
	}
	price := money.MustAmountForTest("51234.5")
	l := s.line(s.usdProduct, "1")
	l.UnitPrice = &price
	res, err := uc.Execute(ctx, s.tn, "d", s.header(), []LineRequest{l})
	if err != nil || res.Invoice.Lines[0].UnitPrice.String() != "51234.5" {
		t.Fatalf("con precio explícito: %v", err)
	}
	rate := money.MustExchangeRateForTest("512.34")
	usd.ExchangeRate = &rate
	res, err = uc.Execute(ctx, s.tn, "e", usd, []LineRequest{s.line(s.usdProduct, "1")})
	if err != nil || res.Invoice.Lines[0].UnitPrice.String() != "100" || res.Invoice.ExchangeRate.String() != "512.34" {
		t.Fatalf("documento en USD: %+v %v", res.Invoice, err)
	}
}

func TestCreateDraftUnknownTaxRateIsRejected(t *testing.T) {
	s := newScenario(t)
	delete(s.f.cat.rates, "08") // catálogo fiscal sin la tarifa del producto
	_, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "a", s.header(), []LineRequest{s.line(s.crcProduct, "1")})
	if !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("err = %v: la tasa no se inventa", err)
	}
}

func TestInactiveProductIsRejected(t *testing.T) {
	s := newScenario(t)
	off := false
	if _, err := NewUpdateProduct(s.f).Execute(ctx, tenant(s.org, "owner"), s.crcProduct.ID, product.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	_, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "a", s.header(), []LineRequest{s.line(s.crcProduct, "1")})
	if !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestRolesForDrafts(t *testing.T) {
	s := newScenario(t)
	for _, role := range []string{"collector", "accountant", "read_only"} {
		if _, err := NewCreateInvoiceDraft(s.f).Execute(ctx, tenant(s.org, role), "k", s.header(), nil); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s crea: err = %v", role, err)
		}
	}
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	for _, role := range []string{"collector", "accountant", "read_only"} {
		if _, err := NewGetInvoice(s.f).Execute(ctx, tenant(s.org, role), res.Invoice.ID); err != nil {
			t.Fatalf("rol %s lee: %v", role, err)
		}
		if err := NewDiscardInvoiceDraft(s.f).Execute(ctx, tenant(s.org, role), res.Invoice.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s descarta: err = %v", role, err)
		}
	}
}

// El resumen trae el encabezado y los totales del documento, sin leer líneas, y lo pueden leer todos los roles.
func TestInvoiceSummary(t *testing.T) {
	s := newScenario(t)
	res, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"owner", "admin", "biller", "collector", "accountant", "read_only"} {
		got, err := NewGetInvoiceSummary(s.f).Execute(ctx, tenant(s.org, role), res.Invoice.ID)
		if err != nil {
			t.Fatalf("rol %s: %v", role, err)
		}
		if got.ID != res.Invoice.ID || got.Status != invoice.StatusDraft || got.Lines != nil ||
			got.Totals.Total.String() != res.Invoice.Totals.Total.String() {
			t.Fatalf("rol %s: resumen = %+v", role, got)
		}
	}
	if _, err := NewGetInvoiceSummary(s.f).Execute(ctx, s.tn, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inexistente: err = %v", err)
	}
}

func TestReplaceLinesAndUpdateHeader(t *testing.T) {
	s := newScenario(t)
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	id := res.Invoice.ID
	inv, err := NewReplaceInvoiceLines(s.f).Execute(ctx, s.tn, id, []LineRequest{s.line(s.crcProduct, "1"), s.line(s.crcProduct, "3")})
	if err != nil || len(inv.Lines) != 2 || inv.Totals.Total.String() != "113002.26" {
		t.Fatalf("líneas: %+v %v", inv.Totals, err)
	}
	notes := "Entrega el lunes"
	inv, err = NewUpdateInvoiceDraft(s.f).Execute(ctx, s.tn, id, invoice.HeaderPatch{Notes: &notes}, nil)
	if err != nil || inv.Notes != notes || len(inv.Lines) != 2 {
		t.Fatalf("encabezado: %+v %v", inv, err)
	}
	usd := money.MustCurrencyForTest("USD")
	rate := money.MustExchangeRateForTest("512")
	if _, err := NewUpdateInvoiceDraft(s.f).Execute(ctx, s.tn, id, invoice.HeaderPatch{Currency: &usd, ExchangeRate: &rate}, nil); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("cambiar moneda sin reenviar líneas: err = %v", err)
	}
	actions := []string{}
	for _, a := range s.f.state.audit {
		actions = append(actions, a.Action)
	}
	if len(actions) != 3 || actions[1] != "invoice.updated" || actions[2] != "invoice.updated" {
		t.Fatalf("auditoría = %v", actions)
	}
}

func TestIssuedDocumentsCannotChange(t *testing.T) {
	s := newScenario(t)
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), []LineRequest{s.line(s.crcProduct, "1")})
	s.f.state.invoices[0].Status = invoice.StatusIssued // lo que hará el incremento 8
	id := res.Invoice.ID
	notes := "x"
	if _, err := NewUpdateInvoiceDraft(s.f).Execute(ctx, s.tn, id, invoice.HeaderPatch{Notes: &notes}, nil); !errors.Is(err, invoice.ErrNotDraft) {
		t.Fatalf("editar: err = %v", err)
	}
	if _, err := NewReplaceInvoiceLines(s.f).Execute(ctx, s.tn, id, []LineRequest{s.line(s.crcProduct, "9")}); !errors.Is(err, invoice.ErrNotDraft) {
		t.Fatalf("líneas: err = %v", err)
	}
	if err := NewDiscardInvoiceDraft(s.f).Execute(ctx, s.tn, id); !errors.Is(err, invoice.ErrNotDraft) {
		t.Fatalf("descartar: err = %v", err)
	}
	if len(s.f.state.invoices) != 1 || s.f.state.invoices[0].Notes != "" || s.f.state.invoices[0].Lines[0].Quantity.String() != "1" {
		t.Fatal("el documento emitido quedó intacto")
	}
}

func TestDiscardDraft(t *testing.T) {
	s := newScenario(t)
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	if err := NewDiscardInvoiceDraft(s.f).Execute(ctx, s.tn, res.Invoice.ID); err != nil {
		t.Fatal(err)
	}
	last := s.f.state.audit[len(s.f.state.audit)-1]
	if len(s.f.state.invoices) != 0 || last.Action != "invoice.discarded" || last.Before == nil {
		t.Fatalf("invoices=%d audit=%+v", len(s.f.state.invoices), last)
	}
	if _, err := NewGetInvoice(s.f).Execute(ctx, s.tn, res.Invoice.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("descartado: err = %v", err)
	}
}

func TestInvoiceOfAnotherOrganizationIsNotFound(t *testing.T) {
	s := newScenario(t)
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	intruder := tenant(uuid.New(), "owner")
	id := res.Invoice.ID
	if _, err := NewGetInvoice(s.f).Execute(ctx, intruder, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: %v", err)
	}
	if _, err := NewGetInvoiceHistory(s.f).Execute(ctx, intruder, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("history: %v", err)
	}
	if _, err := NewGetInvoiceSummary(s.f).Execute(ctx, intruder, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("summary: %v", err)
	}
	notes := "x"
	if _, err := NewUpdateInvoiceDraft(s.f).Execute(ctx, intruder, id, invoice.HeaderPatch{Notes: &notes}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("patch: %v", err)
	}
	if err := NewDiscardInvoiceDraft(s.f).Execute(ctx, intruder, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete: %v", err)
	}
	if len(s.f.state.invoices) != 1 {
		t.Fatal("el borrador de la otra organización quedó intacto")
	}
}

func TestHistoryOfDraftIsEmpty(t *testing.T) {
	s := newScenario(t)
	res, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "k", s.header(), nil)
	h, err := NewGetInvoiceHistory(s.f).Execute(ctx, tenant(s.org, "read_only"), res.Invoice.ID)
	if err != nil || len(h) != 0 {
		t.Fatalf("historial = %v err=%v: la creación no es una transición", h, err)
	}
}
