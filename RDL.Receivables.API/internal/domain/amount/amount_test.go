package amount

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Los bordes de shared.money_amount (numeric(18,5)): 13 enteros y 5 decimales, estrictamente mayor que cero.
func TestPositive(t *testing.T) {
	valid := []string{"0.00001", "1", "1.10", "9999999999999.99999"}
	invalid := []string{"0", "-0.00001", "0.000001", "1.000001", "10000000000000"}
	for _, s := range valid {
		if err := Positive(decimal.RequireFromString(s)); err != nil {
			t.Errorf("%s debe ser válido: %v", s, err)
		}
	}
	for _, s := range invalid {
		if err := Positive(decimal.RequireFromString(s)); err == nil {
			t.Errorf("%s debe rechazarse", s)
		}
	}
}
