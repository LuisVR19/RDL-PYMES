package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"
	"pgregory.net/rapid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
)

// issuedInvoice arma un documento emitido con las líneas dadas, calculadas con las funciones del dominio.
// fataler es lo que usa issuedInvoice de *testing.T y *rapid.T.
type fataler interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

func issuedInvoice(t fataler, lines []invoice.LineInput, credit *int) invoice.Invoice {
	t.Helper()
	inv := invoice.Invoice{
		ID: uuid.New(), OrganizationID: uuid.New(), Status: invoice.StatusIssued, Number: "FAC-00000042",
		Header: invoice.Header{
			DocumentType: invoice.TypeInvoice, CustomerID: uuid.New(), SaleConditionCode: "02", CreditTermDays: credit,
			Currency: money.MustCurrencyForTest("USD"), ExchangeRate: money.MustExchangeRateForTest("512.34"),
			Notes: "Entrega en bodega",
		},
		Customer: invoice.CustomerSnapshot{IdentificationTypeCode: "02", IdentificationNumber: "3101123456",
			LegalName: "Cliente S.A.", Email: "facturas@cliente.example", Phone: "2222-2222", Address: "San José"},
		DueDate: "2026-10-25",
	}
	at, by := time.Date(2026, 9, 25, 5, 30, 0, 0, time.UTC), uuid.New()
	inv.IssuedAt, inv.IssuedByUserID = &at, &by
	var amounts []invoice.LineAmounts
	for i, in := range lines {
		a, err := invoice.CalculateLine(in)
		if err != nil {
			t.Fatalf("línea %d: %v", i, err)
		}
		pid := uuid.New()
		l := invoice.Line{Number: i + 1, ProductID: &pid, ProductCode: fmt.Sprintf("P-%d", i), CabysCode: "8314100000100",
			Description: "Servicio", UnitOfMeasureCode: "Sp", IsService: true, Quantity: in.Quantity,
			UnitPrice: money.CanonicalAmount(in.UnitPrice), Discount: a.Discount, Subtotal: a.Subtotal, Tax: a.Tax, Total: a.Total}
		if !a.Discount.IsZero() {
			l.DiscountReason = "Cliente frecuente"
		}
		for j, tx := range in.Taxes {
			lt := invoice.LineTax{TypeCode: tx.TypeCode, RateCode: "08", Rate: tx.Rate, TaxableBase: a.Taxes[j].TaxableBase, Amount: a.Taxes[j].Amount}
			if tx.ExoneratedRate != nil {
				lt.Exoneration = &invoice.Exoneration{DocumentTypeCode: "03", DocumentNumber: "AL-1-2026",
					Institution: "Ministerio de Hacienda", IssuedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
					ExoneratedRate: *tx.ExoneratedRate, Amount: a.Taxes[j].Exoneration}
			}
			l.Taxes = append(l.Taxes, lt)
		}
		inv.Lines = append(inv.Lines, l)
		amounts = append(amounts, a)
	}
	tot, err := invoice.CalculateTotals(amounts)
	if err != nil {
		t.Fatal(err)
	}
	inv.Totals = tot
	return inv
}

func pct(s string) *money.TaxRate { p := money.MustTaxRateForTest(s); return &p }

func TestInvoiceIssuedValidatesAgainstSchema(t *testing.T) {
	thirty := 30
	inv := issuedInvoice(t, []invoice.LineInput{
		{Quantity: money.MustQuantityForTest("2"), UnitPrice: money.MustAmountForTest("1500"), Discount: money.MustAmountForTest("250"),
			Taxes: []invoice.TaxInput{{TypeCode: "01", Rate: money.MustPercentageForTest("13")}, {TypeCode: "02", Rate: money.MustPercentageForTest("1")}}},
		{Quantity: money.MustQuantityForTest("3"), UnitPrice: money.MustAmountForTest("0.33333"), Discount: money.Zero,
			Taxes: []invoice.TaxInput{{TypeCode: "01", Rate: money.MustPercentageForTest("13"), ExoneratedRate: pct("6.5")}}},
		{Quantity: money.MustQuantityForTest("1"), UnitPrice: money.MustAmountForTest("100"), Discount: money.Zero},
	}, &thirty)
	corr := uuid.New()
	e, err := InvoiceIssued(inv, "2026-09-24", corr)
	if err != nil {
		t.Fatal(err)
	}
	row, err := OutboxRow(e)
	if err != nil {
		t.Fatalf("el evento no valida contra invoice-issued.v1.json: %v", err)
	}
	// El sobre y la fila del outbox (convenciones §10).
	if row.ID != e.EventID || row.EventType != "InvoiceIssued" || row.EventVersion != 1 || row.SourceService != "billing" ||
		row.AggregateType != "invoice" || row.AggregateID != inv.ID || row.OrganizationID != inv.OrganizationID ||
		row.CorrelationID != corr || !row.OccurredAt.Time().Equal(*inv.IssuedAt) {
		t.Fatalf("fila = %+v", row)
	}
	var payload map[string]any
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["issueDate"] != "2026-09-24" || payload["dueDate"] != "2026-10-25" || payload["invoiceNumber"] != "FAC-00000042" ||
		payload["total"] != inv.Totals.Total.String() || payload["occurredAt"] != "2026-09-25T05:30:00Z" {
		t.Fatalf("payload = %v", payload)
	}
	lines := payload["lines"].([]any)
	exo := lines[1].(map[string]any)["taxes"].([]any)[0].(map[string]any)["exoneration"].(map[string]any)
	if exo["exoneratedRate"] != "6.5" || exo["amount"] != "0.065" {
		t.Fatalf("exoneración = %v", exo)
	}
	if taxes := lines[2].(map[string]any)["taxes"].([]any); len(taxes) != 0 {
		t.Fatal("una línea sin impuestos lleva taxes: []")
	}
}

func TestInvoiceIssuedRejectsDrafts(t *testing.T) {
	inv := issuedInvoice(t, []invoice.LineInput{{Quantity: money.MustQuantityForTest("1"), UnitPrice: money.MustAmountForTest("1"), Discount: money.Zero}}, nil)
	inv.Status = invoice.StatusDraft
	if _, err := InvoiceIssued(inv, "2026-09-24", uuid.New()); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("err = %v", err)
	}
}

// TestInvoiceIssuedBrokenEventIsNotWritten: un evento que no cumple el schema no produce fila de outbox.
func TestInvoiceIssuedBrokenEventIsNotWritten(t *testing.T) {
	inv := issuedInvoice(t, []invoice.LineInput{{Quantity: money.MustQuantityForTest("1"), UnitPrice: money.MustAmountForTest("1"), Discount: money.Zero}}, nil)
	e, err := InvoiceIssued(inv, "2026-09-24", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	e.Lines = nil // minItems: 1
	if _, err := OutboxRow(e); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("err = %v", err)
	}
	e2, _ := InvoiceIssued(inv, "2026-09-24", uuid.New())
	e2.SaleConditionCode = "con espacios" // FiscalCode
	if _, err := OutboxRow(e2); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("err = %v", err)
	}
}

// Cualquier factura que el dominio puede calcular produce un evento que valida.
func TestPropertyEveryIssuedInvoiceValidates(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 6).Draw(rt, "lines")
		var lines []invoice.LineInput
		for i := range n {
			q := rapid.Int64Range(1, 9_999_999).Draw(rt, fmt.Sprintf("q%d", i))
			p := rapid.Int64Range(0, 99_999_999_999).Draw(rt, fmt.Sprintf("p%d", i))
			in := invoice.LineInput{
				Quantity:  money.MustQuantityForTest(trim(fmt.Sprintf("%d.%03d", q/1000, q%1000))),
				UnitPrice: money.MustAmountForTest(trim(fmt.Sprintf("%d.%05d", p/100000, p%100000))),
				Discount:  money.Zero,
			}
			if rapid.Bool().Draw(rt, fmt.Sprintf("iva%d", i)) {
				tax := invoice.TaxInput{TypeCode: "01", Rate: money.MustPercentageForTest("13")}
				if rapid.Bool().Draw(rt, fmt.Sprintf("ex%d", i)) {
					tax.ExoneratedRate = pct(rapid.SampledFrom([]string{"0", "1", "6.5", "12.99", "13"}).Draw(rt, "exr"))
				}
				in.Taxes = append(in.Taxes, tax)
			}
			lines = append(lines, in)
		}
		inv := issuedInvoice(rt, lines, nil)
		e, err := InvoiceIssued(inv, "2026-09-24", uuid.New())
		if err != nil {
			rt.Fatal(err)
		}
		if _, err := OutboxRow(e); err != nil {
			rt.Fatalf("no valida: %v", err)
		}
	})
}

func trim(s string) string { return money.CanonicalAmount(money.MustAmountForTest(s)).String() }

var _ cevents.Event = cevents.InvoiceIssuedV1{}
