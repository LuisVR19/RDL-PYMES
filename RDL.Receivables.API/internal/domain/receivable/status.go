// Package receivable es el agregado cuenta por cobrar: saldo, estado derivado, ajustes y aplicaciones.
package receivable

// Status sigue RDL.Contracts/state-machines/receivable.yaml.
type Status string

const (
	StatusOpen          Status = "open"
	StatusPartiallyPaid Status = "partially_paid"
	StatusPaid          Status = "paid"
	StatusCancelled     Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusPartiallyPaid, StatusPaid, StatusCancelled:
		return true
	}
	return false
}

// Collectible indica si la cuenta todavía tiene saldo que cobrar: solo esas pueden estar vencidas.
func (s Status) Collectible() bool { return s == StatusOpen || s == StatusPartiallyPaid }
