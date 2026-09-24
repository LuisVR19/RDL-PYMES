// Package invoice modela la factura comercial y sus notas: el agregado, su máquina de estados, los snapshots y el
// cálculo de montos. El cálculo vive SOLO aquí, en funciones puras y deterministas, sin tocar la base.
package invoice

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"

	"rdl/billing-api/internal/domain/money"
)

// Fórmulas del Anexo de Hacienda 4.4 y del contrato (docs/eventos/invoice-issued.md, ADR 0007 del repo de contratos):
//
//	gross              = round(quantity × unitPrice)                (MontoTotal)
//	subtotal           = gross − discount                            (SubTotal)
//	tax.amount         = round(tax.taxableBase × tax.rate / 100)     (Monto)
//	exoneration.amount = round(subtotal × exoneratedRate / 100)      (MontoExoneracion)
//	line.tax           = Σ tax.amount − Σ exoneration.amount         (ImpuestoNeto)
//	line.total         = subtotal + line.tax                         (MontoTotalLinea)
//
// Solo se redondea donde dice round (money.Round, D2); lo demás son restas y sumas de valores de 5 decimales, exactas.
// TODO(fiscal): la base imponible es el subtotal para todo impuesto; los que se calculan sobre otra base (o sobre
// otro impuesto) y el impuesto asumido por el emisor no están en el modelo v1.

var (
	// ErrDiscountExceedsGross: el descuento supera cantidad × precio (el subtotal sería negativo).
	ErrDiscountExceedsGross = errors.New("invoice: el descuento supera el monto de la línea")
	// ErrExoneratedRateExceedsRate: la tarifa exonerada supera la tarifa del impuesto.
	ErrExoneratedRateExceedsRate = errors.New("invoice: la tarifa exonerada supera la tarifa del impuesto")
	// ErrDuplicateTaxType: dos impuestos del mismo tipo en una línea (invoice_line_taxes_line_tax_uk).
	ErrDuplicateTaxType = errors.New("invoice: tipo de impuesto repetido en la línea")
	// ErrAmountOutOfRange: un monto calculado no cabe en shared.money_amount (13 enteros).
	ErrAmountOutOfRange = errors.New("invoice: un monto calculado excede 13 dígitos enteros")
)

// LineInput son los datos de una línea que determinan sus montos.
type LineInput struct {
	Quantity  money.Quantity
	UnitPrice money.Amount
	Discount  money.Amount
	Taxes     []TaxInput
}

type TaxInput struct {
	TypeCode string
	Rate     money.Percentage
	// ExoneratedRate nil = sin exoneración. Es la tarifa exonerada en puntos (6.5 de un IVA de 13), no un % del impuesto.
	ExoneratedRate *money.TaxRate
}

// LineAmounts es el resultado del cálculo de una línea. Taxes va en el mismo orden que LineInput.Taxes.
type LineAmounts struct {
	Gross    money.Amount // quantity × unitPrice, redondeado; no se guarda, se usa para validar el descuento
	Discount money.Amount
	Subtotal money.Amount
	Tax      money.Amount // neto: impuestos − exoneraciones
	Total    money.Amount
	Taxes    []TaxAmounts
}

type TaxAmounts struct {
	TaxableBase money.Amount
	Amount      money.Amount // antes de la exoneración
	Exoneration money.Amount // cero si no hay exoneración
}

// Totals son los totales del documento: sumas exactas de las líneas ya calculadas.
type Totals struct {
	Subtotal    money.Amount // Σ subtotal (después de descuentos)
	Discount    money.Amount // Σ descuento (informativo)
	Tax         money.Amount // Σ impuestos antes de exoneración
	Exoneration money.Amount // Σ exoneraciones
	Total       money.Amount // subtotal + tax − exoneration = Σ total de línea
}

// CalculateLine aplica las fórmulas a una línea. Es pura: mismo input, mismo resultado, sin efectos.
func CalculateLine(in LineInput) (LineAmounts, error) {
	gross := money.Round(money.Dec(in.Quantity).Mul(money.Dec(in.UnitPrice)))
	discount := money.Dec(in.Discount)
	if discount.GreaterThan(gross) {
		return LineAmounts{}, ErrDiscountExceedsGross
	}
	subtotal := gross.Sub(discount)

	taxes := make([]TaxAmounts, 0, len(in.Taxes))
	seen := make(map[string]bool, len(in.Taxes))
	taxSum, exoSum := decimal.Zero, decimal.Zero
	for _, t := range in.Taxes {
		if seen[t.TypeCode] {
			return LineAmounts{}, fmt.Errorf("%w: %s", ErrDuplicateTaxType, t.TypeCode)
		}
		seen[t.TypeCode] = true

		rate := money.Dec(t.Rate)
		amount := money.Percent(subtotal, rate)
		exo := decimal.Zero
		if t.ExoneratedRate != nil {
			exRate := money.Dec(t.ExoneratedRate)
			if exRate.GreaterThan(rate) {
				return LineAmounts{}, ErrExoneratedRateExceedsRate
			}
			// Con exRate ≤ rate sobre la misma base, round(...) es monótono: exo ≤ amount y el neto nunca es negativo.
			exo = money.Percent(subtotal, exRate)
		}
		taxSum, exoSum = taxSum.Add(amount), exoSum.Add(exo)
		ta, err := amounts(subtotal, amount, exo)
		if err != nil {
			return LineAmounts{}, err
		}
		taxes = append(taxes, TaxAmounts{TaxableBase: ta[0], Amount: ta[1], Exoneration: ta[2]})
	}

	net := taxSum.Sub(exoSum)
	a, err := amounts(gross, discount, subtotal, net, subtotal.Add(net))
	if err != nil {
		return LineAmounts{}, err
	}
	return LineAmounts{Gross: a[0], Discount: a[1], Subtotal: a[2], Tax: a[3], Total: a[4], Taxes: taxes}, nil
}

// CalculateTotals suma las líneas. No redondea: todas las líneas ya tienen 5 decimales.
func CalculateTotals(lines []LineAmounts) (Totals, error) {
	subtotal, discount, tax, exo, total := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	for _, l := range lines {
		subtotal = subtotal.Add(money.Dec(l.Subtotal))
		discount = discount.Add(money.Dec(l.Discount))
		total = total.Add(money.Dec(l.Total))
		for _, t := range l.Taxes {
			tax = tax.Add(money.Dec(t.Amount))
			exo = exo.Add(money.Dec(t.Exoneration))
		}
	}
	a, err := amounts(subtotal, discount, tax, exo, total)
	if err != nil {
		return Totals{}, err
	}
	return Totals{Subtotal: a[0], Discount: a[1], Tax: a[2], Exoneration: a[3], Total: a[4]}, nil
}

func amounts(ds ...decimal.Decimal) ([]money.Amount, error) {
	out := make([]money.Amount, len(ds))
	for i, d := range ds {
		a, err := money.ToAmount(d)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrAmountOutOfRange, d)
		}
		out[i] = money.CanonicalAmount(a)
	}
	return out, nil
}
