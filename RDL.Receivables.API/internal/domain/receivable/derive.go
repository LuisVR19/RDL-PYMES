package receivable

import "github.com/shopspring/decimal"

// Totals son las sumas de las que se deriva el saldo. Créditos incluye notas de crédito y castigos (write_off);
// la anulación va aparte porque además decide el estado.
type Totals struct {
	Original     decimal.Decimal
	Debits       decimal.Decimal
	Credits      decimal.Decimal
	Cancellation decimal.Decimal
	Cancelled    bool
	Applied      decimal.Decimal
}

// Due es el total adeudado: el original más las notas de débito.
func (t Totals) Due() decimal.Decimal { return t.Original.Add(t.Debits) }

// Derive es la MISMA fórmula que receivables.recalculate_receivable (migración 00004, decisión R1): si cambia una,
// cambia la otra. Devuelve ErrNegativeBalance en vez de un saldo negativo.
//
//	saldo = original + débitos − créditos − anulación − aplicaciones vigentes
//	cancelled si hay anulación; paid si saldo = 0; open si saldo = total adeudado; si no, partially_paid.
func Derive(t Totals) (decimal.Decimal, Status, error) {
	balance := t.Due().Sub(t.Credits).Sub(t.Cancellation).Sub(t.Applied)
	if balance.IsNegative() {
		return decimal.Zero, "", ErrNegativeBalance
	}
	switch {
	case t.Cancelled:
		return balance, StatusCancelled, nil
	case balance.IsZero():
		return balance, StatusPaid, nil
	case balance.Equal(t.Due()):
		return balance, StatusOpen, nil
	default:
		return balance, StatusPartiallyPaid, nil
	}
}
