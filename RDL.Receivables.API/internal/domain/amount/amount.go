// Package amount valida los montos del dominio: decimal exacto (shopspring/decimal, ADR 0004) con la misma forma que
// shared.money_amount (numeric(18,5)). El dominio solo suma, resta y compara; nunca divide ni redondea.
package amount

import (
	"errors"

	"github.com/shopspring/decimal"
)

// Scale es la cantidad de decimales de shared.money_amount.
const Scale = 5

// ErrInvalid: el monto no es mayor que cero o no cabe en numeric(18,5).
var ErrInvalid = errors.New("monto inválido: debe ser mayor que cero, con hasta 13 enteros y 5 decimales")

var limit = decimal.New(1, 13)

// Positive valida un monto de una operación (pago, aplicación, ajuste): > 0 y dentro de numeric(18,5).
// Un valor con más de 5 decimales se rechaza en vez de redondearse: redondear un monto es una decisión de negocio.
func Positive(d decimal.Decimal) error {
	if !d.IsPositive() || d.GreaterThanOrEqual(limit) || !d.Equal(d.Truncate(Scale)) {
		return ErrInvalid
	}
	return nil
}
