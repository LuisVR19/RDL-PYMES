// Package money es el único lugar del dominio que toca valores decimales. Los tipos y formatos son los del repo de
// contratos (pkg/events/money: strings decimales, nunca float); la aritmética es github.com/shopspring/decimal
// (ADR 0005) y el redondeo de D2 vive solo en Round.
package money

import (
	"fmt"
	"strings"

	cmoney "bitbucket.org/rdl/contracts/pkg/events/money"
	"github.com/shopspring/decimal"
)

type (
	Amount       = cmoney.Amount
	Currency     = cmoney.Currency
	Quantity     = cmoney.Quantity
	Percentage   = cmoney.Percentage
	ExchangeRate = cmoney.ExchangeRate
	// TaxRate es una tarifa en puntos, formato 4,2 del Anexo 1 v4.4 (tarifa exonerada): "6.5" de un IVA de "13".
	TaxRate = cmoney.TaxRate
)

var (
	ErrInvalid        = cmoney.ErrInvalid
	ParseAmount       = cmoney.ParseAmount
	ParseCurrency     = cmoney.ParseCurrency
	ParseQuantity     = cmoney.ParseQuantity
	ParsePercentage   = cmoney.ParsePercentage
	ParseExchangeRate = cmoney.ParseExchangeRate
	ParseTaxRate      = cmoney.ParseTaxRate
)

// Zero es el monto cero ("0").
var Zero = cmoney.MustAmount("0")

// Scale son los decimales de shared.money_amount y del XML de Hacienda.
const Scale = 5

// Round aplica D2 (ADR 0007 del repo de contratos, reglas del borrador de los Anexos v4.4): 5 decimales, mitad hacia arriba mirando el sexto decimal.
// shopspring redondea "mitad lejos de cero", que para montos (nunca negativos) es exactamente mitad hacia arriba.
// Es el ÚNICO redondeo del sistema: cualquier otro cálculo pasa por aquí.
func Round(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		// Un monto negativo es un error de cálculo aguas arriba, no algo que redondear.
		panic(fmt.Sprintf("money.Round: monto negativo %s", d))
	}
	return d.Round(Scale)
}

// Percent calcula round(base × rate / 100). Dividir entre 100 es correr la coma: exacto, sin división decimal.
func Percent(base, rate decimal.Decimal) decimal.Decimal {
	return Round(base.Mul(rate).Shift(-2))
}

// Dec convierte un valor del contrato al tipo de cálculo. Los strings ya están validados: el error es imposible.
func Dec(v fmt.Stringer) decimal.Decimal {
	return decimal.RequireFromString(v.String())
}

// ToAmount convierte un resultado de cálculo en Amount. Falla si no cabe en shared.money_amount (13 enteros,
// 5 decimales, ≥ 0): ahí el cálculo produjo algo que la base y el contrato no pueden guardar.
func ToAmount(d decimal.Decimal) (Amount, error) {
	if d.Exponent() < -Scale && !d.Equal(d.Round(Scale)) {
		return Amount{}, fmt.Errorf("%w: %s tiene más de %d decimales (falta redondear)", ErrInvalid, d, Scale)
	}
	a, err := ParseAmount(d.String())
	if err != nil {
		return Amount{}, fmt.Errorf("monto fuera de rango: %w", err)
	}
	return a, nil
}

// CanonicalAmount quita los ceros decimales a la derecha ("1300.50000" → "1300.5", "12.000" → "12"): la base
// devuelve numeric(18,5) con 5 decimales y el cliente puede enviar cualquier cantidad. Con una sola forma, comparar
// dos montos iguales da igual y la API responde siempre lo mismo.
func CanonicalAmount(a Amount) Amount {
	return cmoney.MustAmount(trimZeros(a.String()))
}

func trimZeros(s string) string {
	whole, frac, ok := strings.Cut(s, ".")
	if !ok {
		return s
	}
	if frac = strings.TrimRight(frac, "0"); frac == "" {
		return whole
	}
	return whole + "." + frac
}

// MustAmountForTest y compañía son para tests y constantes: entran en pánico si el valor no es válido.
func MustAmountForTest(s string) Amount         { return cmoney.MustAmount(s) }
func MustCurrencyForTest(s string) Currency     { return cmoney.MustCurrency(s) }
func MustQuantityForTest(s string) Quantity     { return cmoney.MustQuantity(s) }
func MustPercentageForTest(s string) Percentage { return cmoney.MustPercentage(s) }
func MustTaxRateForTest(s string) TaxRate       { return cmoney.MustTaxRate(s) }
func MustExchangeRateForTest(s string) ExchangeRate {
	return cmoney.MustExchangeRate(s)
}
