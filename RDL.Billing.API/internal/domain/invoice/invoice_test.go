package invoice

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/money"
)

func header() Header {
	return Header{
		DocumentType: TypeInvoice, CustomerID: uuid.New(), SaleConditionCode: "01",
		Currency: money.MustCurrencyForTest("CRC"), ExchangeRate: money.MustExchangeRateForTest("1"),
	}
}

func draftLine(q, price, discount, reason string, rates ...string) LineDraft {
	d := LineDraft{
		CabysCode: "8314100000100", Description: "Consultoría", UnitOfMeasureCode: "Sp", IsService: true,
		Quantity: money.MustQuantityForTest(q), UnitPrice: money.MustAmountForTest(price),
		Discount: money.MustAmountForTest(discount), DiscountReason: reason,
	}
	for i, r := range rates {
		d.Taxes = append(d.Taxes, TaxDraft{TypeCode: []string{"01", "02"}[i], RateCode: "08", Rate: money.MustPercentageForTest(r)})
	}
	return d
}

func fieldsOf(err error) map[string]bool {
	out := map[string]bool{}
	var walk func(error)
	walk = func(e error) {
		if j, ok := e.(interface{ Unwrap() []error }); ok {
			for _, x := range j.Unwrap() {
				walk(x)
			}
			return
		}
		var fe FieldError
		if errors.As(e, &fe) {
			out[fe.Field] = true
		}
	}
	walk(err)
	return out
}

func TestNewDraft(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	inv, err := NewDraft(org, user, header())
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != StatusDraft || inv.Number != "" || inv.OrganizationID != org || inv.CreatedByUserID != user ||
		len(inv.Lines) != 0 || inv.Totals.Total.String() != "0" {
		t.Fatalf("borrador = %+v", inv)
	}
}

func TestNewDraftValidation(t *testing.T) {
	neg, far := -1, 5000
	cases := map[string]struct {
		mut   func(*Header)
		field string
	}{
		"nota de crédito (F5)": {func(h *Header) { h.DocumentType = TypeCreditNote }, "documentType"},
		"tipo desconocido":     {func(h *Header) { h.DocumentType = "receipt" }, "documentType"},
		"sin cliente":          {func(h *Header) { h.CustomerID = uuid.Nil }, "customerId"},
		"condición inválida":   {func(h *Header) { h.SaleConditionCode = "contado 30" }, "saleConditionCode"},
		"plazo negativo":       {func(h *Header) { h.CreditTermDays = &neg }, "creditTermDays"},
		"plazo absurdo":        {func(h *Header) { h.CreditTermDays = &far }, "creditTermDays"},
		"sin moneda":           {func(h *Header) { h.Currency = money.Currency{} }, "currency"},
		"notas largas":         {func(h *Header) { h.Notes = strings.Repeat("n", 2001) }, "notes"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := header()
			c.mut(&h)
			if _, err := NewDraft(uuid.New(), uuid.New(), h); !errors.Is(err, ErrInvalid) || !fieldsOf(err)[c.field] {
				t.Fatalf("err = %v, se esperaba error en %s", err, c.field)
			}
		})
	}
}

func TestReplaceLinesCalculates(t *testing.T) {
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	next, err := inv.ReplaceLines([]LineDraft{
		draftLine("2", "1500", "250", "Cliente frecuente", "13"),
		draftLine("3", "0.33333", "0", "", "13"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Lines) != 2 || next.Lines[0].Number != 1 || next.Lines[1].Number != 2 {
		t.Fatalf("líneas = %+v", next.Lines)
	}
	l := next.Lines[0]
	if l.Subtotal.String() != "2750" || l.Tax.String() != "357.5" || l.Total.String() != "3107.5" ||
		l.Taxes[0].TaxableBase.String() != "2750" || l.Taxes[0].Amount.String() != "357.5" || l.Taxes[0].RateCode != "08" {
		t.Fatalf("línea 1 = %+v", l)
	}
	tot := next.Totals
	if tot.Subtotal.String() != "2750.99999" || tot.Discount.String() != "250" || tot.Tax.String() != "357.63" ||
		tot.Total.String() != "3108.62999" {
		t.Fatalf("totales = %+v", tot)
	}
	if len(inv.Lines) != 0 {
		t.Fatal("ReplaceLines no modifica el original")
	}
	// Reemplazar por una lista vacía deja el borrador en cero.
	empty, err := next.ReplaceLines(nil)
	if err != nil || len(empty.Lines) != 0 || empty.Totals.Total.String() != "0" {
		t.Fatalf("vacío: %+v %v", empty.Totals, err)
	}
}

func TestReplaceLinesValidation(t *testing.T) {
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	_, err := inv.ReplaceLines([]LineDraft{
		draftLine("1", "100", "10", ""),                     // descuento sin motivo
		draftLine("1", "100", "0", ""),                      // bien
		draftLine("1", "100", "100.00001", "demasiado"),     // descuento mayor que el monto
		draftLine("1", "100", "1", strings.Repeat("m", 81)), // motivo largo
		draftLine("1", "100", "0", "", "13", "13"),          // dos impuestos (tipos distintos: bien)
	})
	f := fieldsOf(err)
	if !errors.Is(err, ErrInvalid) || !f["lines[0].discountReason"] || !f["lines[2].discount"] || !f["lines[3].discountReason"] ||
		f["lines[1].discount"] || len(f) != 3 {
		t.Fatalf("campos = %v (err %v)", f, err)
	}
}

func TestDiscountReasonIsDroppedWithoutDiscount(t *testing.T) {
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	next, err := inv.ReplaceLines([]LineDraft{draftLine("1", "100", "0", "sin descuento")})
	if err != nil || next.Lines[0].DiscountReason != "" {
		t.Fatalf("motivo = %q err=%v", next.Lines[0].DiscountReason, err)
	}
}

func TestOnlyDraftsChange(t *testing.T) {
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	for _, st := range []Status{StatusIssued, StatusCancelled} {
		inv.Status = st
		notes := "x"
		if _, err := inv.ApplyHeader(HeaderPatch{Notes: &notes}); !errors.Is(err, ErrNotDraft) {
			t.Fatalf("%s: editar encabezado: %v", st, err)
		}
		if _, err := inv.ReplaceLines(nil); !errors.Is(err, ErrNotDraft) {
			t.Fatalf("%s: reemplazar líneas: %v", st, err)
		}
		if err := inv.CanDiscard(); !errors.Is(err, ErrNotDraft) {
			t.Fatalf("%s: descartar: %v", st, err)
		}
	}
}

func TestApplyHeader(t *testing.T) {
	branch := uuid.New()
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	inv.BranchID = &branch
	days := 30
	daysPtr := &days
	var noBranch *uuid.UUID
	notes := "  Entregar en bodega  "
	next, err := inv.ApplyHeader(HeaderPatch{CreditTermDays: &daysPtr, BranchID: &noBranch, Notes: &notes})
	if err != nil || next.BranchID != nil || *next.CreditTermDays != 30 || next.Notes != "Entregar en bodega" ||
		next.CustomerID != inv.CustomerID {
		t.Fatalf("resultado = %+v err=%v", next.Header, err)
	}
}
