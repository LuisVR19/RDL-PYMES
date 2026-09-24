// Package money tiene los tipos decimales exactos de los contratos: montos, tipos de cambio, cantidades, porcentajes y
// monedas. Se guardan como el string decimal que viaja en JSON, así que nunca pasan por float y el round trip no
// pierde nada ("1300.50" sigue siendo "1300.50").
//
// No hacen aritmética: cada API calcula con su librería decimal y convierte con Parse*/String. Los patrones son los
// mismos de schemas/common/*.json; un test lo garantiza.
package money

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	AmountPattern     = `^(0|[1-9][0-9]{0,12})(\.[0-9]{1,5})?$`
	QuantityPattern   = `^(0|[1-9][0-9]{0,12})(\.[0-9]{1,3})?$`
	PercentagePattern = `^(100(\.0{1,4})?|[1-9]?[0-9](\.[0-9]{1,4})?)$`
	CurrencyPattern   = `^[A-Z]{3}$`
	TaxRatePattern    = `^(0|[1-9][0-9]?)(\.[0-9]{1,2})?$`
)

var (
	amountRe     = regexp.MustCompile(AmountPattern)
	quantityRe   = regexp.MustCompile(QuantityPattern)
	percentageRe = regexp.MustCompile(PercentagePattern)
	currencyRe   = regexp.MustCompile(CurrencyPattern)
	taxRateRe    = regexp.MustCompile(TaxRatePattern)
	zeroRe       = regexp.MustCompile(`^0(\.0+)?$`)
)

// ErrInvalid envuelve todo valor que no cumple su formato.
var ErrInvalid = errors.New("valor decimal inválido")

// Amount es un monto >= 0 con hasta 13 enteros y 5 decimales (shared.money_amount). El valor cero es "0".
type Amount struct{ v string }

// ExchangeRate es un tipo de cambio > 0 con hasta 5 decimales (shared.exchange_rate).
type ExchangeRate struct{ v string }

// Quantity es una cantidad > 0 con hasta 3 decimales (shared.quantity).
type Quantity struct{ v string }

// Percentage es un porcentaje entre 0 y 100 con hasta 4 decimales (shared.percentage). El valor cero es "0".
type Percentage struct{ v string }

// Currency es un código ISO 4217 (shared.currency_code).
type Currency struct{ v string }

// TaxRate es una tarifa en puntos porcentuales con formato 4,2 del Anexo 1 v4.4 ("13" es 13%, "0.5" es 0,5%).
// Se usa en la tarifa exonerada. El valor cero es "0".
type TaxRate struct{ v string }

func ParseAmount(s string) (Amount, error) {
	if err := check("monto", amountRe, s, false); err != nil {
		return Amount{}, err
	}
	return Amount{v: s}, nil
}

func ParseExchangeRate(s string) (ExchangeRate, error) {
	if err := check("tipo de cambio", amountRe, s, true); err != nil {
		return ExchangeRate{}, err
	}
	return ExchangeRate{v: s}, nil
}

func ParseQuantity(s string) (Quantity, error) {
	if err := check("cantidad", quantityRe, s, true); err != nil {
		return Quantity{}, err
	}
	return Quantity{v: s}, nil
}

func ParsePercentage(s string) (Percentage, error) {
	if err := check("porcentaje", percentageRe, s, false); err != nil {
		return Percentage{}, err
	}
	return Percentage{v: s}, nil
}

func ParseTaxRate(s string) (TaxRate, error) {
	if err := check("tarifa", taxRateRe, s, false); err != nil {
		return TaxRate{}, err
	}
	return TaxRate{v: s}, nil
}

func ParseCurrency(s string) (Currency, error) {
	if !currencyRe.MatchString(s) {
		return Currency{}, fmt.Errorf("%w: moneda %q (ISO 4217 en mayúsculas)", ErrInvalid, s)
	}
	return Currency{v: s}, nil
}

// MustAmount y compañía son para constantes y tests: entran en pánico si el valor no es válido.
func MustAmount(s string) Amount             { return must(ParseAmount(s)) }
func MustExchangeRate(s string) ExchangeRate { return must(ParseExchangeRate(s)) }
func MustQuantity(s string) Quantity         { return must(ParseQuantity(s)) }
func MustPercentage(s string) Percentage     { return must(ParsePercentage(s)) }
func MustCurrency(s string) Currency         { return must(ParseCurrency(s)) }
func MustTaxRate(s string) TaxRate           { return must(ParseTaxRate(s)) }

func (a Amount) String() string       { return orZero(a.v) }
func (a Amount) IsZero() bool         { return zeroRe.MatchString(a.String()) }
func (r ExchangeRate) String() string { return r.v }
func (q Quantity) String() string     { return q.v }
func (p Percentage) String() string   { return orZero(p.v) }
func (c Currency) String() string     { return c.v }
func (t TaxRate) String() string      { return orZero(t.v) }

func (a Amount) MarshalJSON() ([]byte, error) { return json.Marshal(a.String()) }
func (r ExchangeRate) MarshalJSON() ([]byte, error) {
	return marshalRequired("tipo de cambio", r.v)
}
func (q Quantity) MarshalJSON() ([]byte, error)   { return marshalRequired("cantidad", q.v) }
func (p Percentage) MarshalJSON() ([]byte, error) { return json.Marshal(p.String()) }
func (c Currency) MarshalJSON() ([]byte, error)   { return marshalRequired("moneda", c.v) }
func (t TaxRate) MarshalJSON() ([]byte, error)    { return json.Marshal(t.String()) }

func (a *Amount) UnmarshalJSON(b []byte) error       { return unmarshal(b, ParseAmount, a) }
func (r *ExchangeRate) UnmarshalJSON(b []byte) error { return unmarshal(b, ParseExchangeRate, r) }
func (q *Quantity) UnmarshalJSON(b []byte) error     { return unmarshal(b, ParseQuantity, q) }
func (p *Percentage) UnmarshalJSON(b []byte) error   { return unmarshal(b, ParsePercentage, p) }
func (c *Currency) UnmarshalJSON(b []byte) error     { return unmarshal(b, ParseCurrency, c) }
func (t *TaxRate) UnmarshalJSON(b []byte) error      { return unmarshal(b, ParseTaxRate, t) }

func check(kind string, re *regexp.Regexp, s string, nonZero bool) error {
	if !re.MatchString(s) {
		return fmt.Errorf("%w: %s %q (string decimal sin signo, sin exponente ni separador de miles)", ErrInvalid, kind, s)
	}
	if nonZero && zeroRe.MatchString(s) {
		return fmt.Errorf("%w: %s debe ser mayor que cero", ErrInvalid, kind)
	}
	return nil
}

// unmarshal rechaza un JSON number: justo el error que estos tipos existen para impedir.
func unmarshal[T any](b []byte, parse func(string) (T, error), dst *T) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: se esperaba un string decimal, llegó %s", ErrInvalid, b)
	}
	v, err := parse(s)
	if err != nil {
		return err
	}
	*dst = v
	return nil
}

func marshalRequired(kind, v string) ([]byte, error) {
	if v == "" {
		return nil, fmt.Errorf("%w: %s sin valor", ErrInvalid, kind)
	}
	return json.Marshal(v)
}

func orZero(v string) string {
	if v == "" {
		return "0"
	}
	return v
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
