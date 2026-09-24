package invoice

import (
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"bitbucket.org/rdl/contracts/pkg/events"

	"rdl/billing-api/internal/domain/money"
)

func pct(s string) *money.TaxRate {
	p := money.MustTaxRateForTest(s)
	return &p
}

func line(q, price, discount string, taxes ...TaxInput) LineInput {
	return LineInput{
		Quantity: money.MustQuantityForTest(q), UnitPrice: money.MustAmountForTest(price),
		Discount: money.MustAmountForTest(discount), Taxes: taxes,
	}
}

func iva(rate string) TaxInput {
	return TaxInput{TypeCode: "01", Rate: money.MustPercentageForTest(rate)}
}

func TestCalculateLineExamples(t *testing.T) {
	cases := []struct {
		name                             string
		in                               LineInput
		subtotal, tax, total, taxAmt, ex string
	}{
		{"arquitectura 6.3", line("1", "10000", "0", iva("13")), "10000", "1300", "11300", "1300", "0"},
		{"descuento", line("2", "1500", "250", iva("13")), "2750", "357.5", "3107.5", "357.5", "0"},
		{"redondeo del impuesto", line("3", "0.33333", "0", iva("13")), "0.99999", "0.13", "1.12999", "0.13", "0"},
		{"mitad exacta en el bruto", line("0.5", "0.00001", "0"), "0.00001", "0", "0.00001", "", ""},
		{"exoneración a la mitad", line("1", "1000", "0", TaxInput{TypeCode: "01", Rate: money.MustPercentageForTest("13"), ExoneratedRate: pct("6.5")}),
			"1000", "65", "1065", "130", "65"},
		{"exoneración total", line("1", "1000", "0", TaxInput{TypeCode: "01", Rate: money.MustPercentageForTest("13"), ExoneratedRate: pct("13")}),
			"1000", "0", "1000", "130", "130"},
		{"sin impuestos", line("1.5", "100", "0"), "150", "0", "150", "", ""},
		{"descuento total", line("1", "100", "100", iva("13")), "0", "0", "0", "0", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CalculateLine(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if got.Subtotal.String() != c.subtotal || got.Tax.String() != c.tax || got.Total.String() != c.total {
				t.Fatalf("subtotal=%s tax=%s total=%s", got.Subtotal, got.Tax, got.Total)
			}
			if c.taxAmt != "" && (got.Taxes[0].Amount.String() != c.taxAmt || got.Taxes[0].Exoneration.String() != c.ex ||
				got.Taxes[0].TaxableBase.String() != c.subtotal) {
				t.Fatalf("impuesto = %+v", got.Taxes[0])
			}
		})
	}
}

func TestCalculateLineErrors(t *testing.T) {
	cases := map[string]struct {
		in   LineInput
		want error
	}{
		"descuento mayor que el monto": {line("2", "10", "20.00001"), ErrDiscountExceedsGross},
		"tarifa exonerada mayor":       {line("1", "10", "0", TaxInput{TypeCode: "01", Rate: money.MustPercentageForTest("4"), ExoneratedRate: pct("13")}), ErrExoneratedRateExceedsRate},
		"impuesto repetido":            {line("1", "10", "0", iva("13"), iva("4")), ErrDuplicateTaxType},
		"no cabe en 13 enteros":        {line("999999999", "99999", "0"), ErrAmountOutOfRange},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := CalculateLine(c.in); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, se esperaba %v", err, c.want)
			}
		})
	}
}

// TestContractExamples recalcula cada InvoiceIssued válido del repo de contratos: Billing produce exactamente los
// montos que el contrato documenta, incluido el ejemplo que necesita redondeo.
func TestContractExamples(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "RDL.Contracts", "examples", "events", "invoice-issued.v1.json"))
	if err != nil {
		t.Skipf("sin el repo de contratos al lado: %v", err)
	}
	var suite struct {
		Valid []json.RawMessage `json:"valid"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	if len(suite.Valid) < 3 {
		t.Fatalf("se esperaban al menos 3 ejemplos válidos, hay %d", len(suite.Valid))
	}
	for i, rawEv := range suite.Valid {
		var ev events.InvoiceIssuedV1
		if err := json.Unmarshal(rawEv, &ev); err != nil {
			t.Fatalf("ejemplo %d: %v", i, err)
		}
		var lines []LineAmounts
		for _, l := range ev.Lines {
			in := LineInput{Quantity: l.Quantity, UnitPrice: l.UnitPrice, Discount: l.Discount}
			for _, tx := range l.Taxes {
				ti := TaxInput{TypeCode: tx.TaxTypeCode, Rate: tx.Rate}
				if tx.Exoneration != nil {
					r := tx.Exoneration.ExoneratedRate
					ti.ExoneratedRate = &r
				}
				in.Taxes = append(in.Taxes, ti)
			}
			got, err := CalculateLine(in)
			if err != nil {
				t.Fatalf("ejemplo %d línea %d: %v", i, l.LineNumber, err)
			}
			same := func(a, b money.Amount) bool { return money.Dec(a).Equal(money.Dec(b)) }
			if !same(got.Gross, l.GrossAmount) || !same(got.Subtotal, l.Subtotal) || !same(got.Tax, l.Tax) || !same(got.Total, l.Total) {
				t.Errorf("ejemplo %d línea %d: calculado %s/%s/%s, contrato %s/%s/%s", i, l.LineNumber,
					got.Subtotal, got.Tax, got.Total, l.Subtotal, l.Tax, l.Total)
			}
			for j, tx := range l.Taxes {
				if !same(got.Taxes[j].Amount, tx.Amount) {
					t.Errorf("ejemplo %d línea %d impuesto %d: %s ≠ %s", i, l.LineNumber, j, got.Taxes[j].Amount, tx.Amount)
				}
				if tx.Exoneration != nil && !same(got.Taxes[j].Exoneration, tx.Exoneration.Amount) {
					t.Errorf("ejemplo %d exoneración: %s ≠ %s", i, got.Taxes[j].Exoneration, tx.Exoneration.Amount)
				}
			}
			lines = append(lines, got)
		}
		tot, err := CalculateTotals(lines)
		if err != nil {
			t.Fatal(err)
		}
		same := func(a, b money.Amount) bool { return money.Dec(a).Equal(money.Dec(b)) }
		if !same(tot.Subtotal, ev.Subtotal) || !same(tot.Discount, ev.Discount) || !same(tot.Tax, ev.Tax) ||
			!same(tot.Exoneration, ev.Exoneration) || !same(tot.Total, ev.Total) {
			t.Errorf("ejemplo %d totales: calculado %+v; contrato %s/%s/%s/%s/%s", i, tot,
				ev.Subtotal, ev.Discount, ev.Tax, ev.Exoneration, ev.Total)
		}
	}
}

// rat es la referencia exacta de los tests de propiedades.
func rat(v interface{ String() string }) *big.Rat {
	r, ok := new(big.Rat).SetString(v.String())
	if !ok {
		panic(v.String())
	}
	return r
}
