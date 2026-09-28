// Package settlement es el servicio de dominio que opera sobre pagos y cuentas a la vez: aplicar, revertir, anular
// un pago y los ajustes que revierten aplicaciones (R2, R3). Las reglas que cruzan los dos agregados (cliente,
// moneda, disponible del pago, saldo de la cuenta) viven aquí; cada agregado sigue protegiendo las suyas.
//
// Cada función valida todo antes de cambiar algo. Quien la llama ya bloqueó las filas en orden estable (pagos por id,
// después cuentas por id) y hace rollback de la transacción si algo falla.
package settlement

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

var (
	// ErrCustomerMismatch: el pago y la cuenta son de clientes distintos (customer-mismatch, 422, R11).
	ErrCustomerMismatch = errors.New("el pago y la cuenta pertenecen a clientes distintos")
	// ErrCurrencyMismatch: en V1 un pago solo se aplica a cuentas de su misma moneda (currency-mismatch, 422).
	ErrCurrencyMismatch = errors.New("el pago y la cuenta están en monedas distintas")
	// ErrNotLoaded: falta un agregado que la operación necesita (error de programación del caso de uso).
	ErrNotLoaded = errors.New("falta cargar un pago o una cuenta de la operación")
	// ErrMismatch: la aplicación no une este pago con esta cuenta.
	ErrMismatch = errors.New("la aplicación no corresponde a este pago y esta cuenta")
)

// Effect es el cambio de estado de una cuenta dentro de una operación que toca varias.
type Effect struct {
	ReceivableID uuid.UUID
	Change       receivable.Change
}

// Apply aplica (parte de) un pago a una cuenta.
func Apply(p *payment.Payment, r *receivable.Receivable, applicationID uuid.UUID, amt decimal.Decimal, at time.Time,
) (receivable.Change, error) {
	switch {
	case p.Status() == payment.StatusVoided:
		return receivable.Change{}, payment.ErrVoided
	case r.Status() == receivable.StatusCancelled:
		return receivable.Change{}, receivable.ErrCancelled
	case p.CustomerID() != r.CustomerID():
		return receivable.Change{}, ErrCustomerMismatch
	case p.Currency() != r.Currency():
		return receivable.Change{}, ErrCurrencyMismatch
	}
	if err := p.CanApply(r.ID(), amt); err != nil {
		return receivable.Change{}, err
	}
	if err := r.CanApply(p.ID(), amt); err != nil {
		return receivable.Change{}, err
	}
	if err := p.Apply(payment.Application{ID: applicationID, ReceivableID: r.ID(), Amount: amt, AppliedAt: at}); err != nil {
		return receivable.Change{}, err
	}
	return r.Apply(receivable.Application{ID: applicationID, PaymentID: p.ID(), Amount: amt, AppliedAt: at})
}

// Reverse revierte una aplicación con motivo: el pago y la cuenta vuelven exactamente a como estaban antes.
func Reverse(p *payment.Payment, r *receivable.Receivable, applicationID uuid.UUID, reason string, at time.Time,
) (receivable.Change, error) {
	if err := canReverse(p, r, applicationID, reason); err != nil {
		return receivable.Change{}, err
	}
	if err := p.ReverseApplication(applicationID, reason, at); err != nil {
		return receivable.Change{}, err
	}
	return r.ReverseApplication(applicationID, reason, at)
}

// VoidPayment anula un pago: revierte todas sus aplicaciones vigentes (con el mismo motivo) y lo pasa a voided.
// receivables debe traer todas las cuentas con aplicaciones vigentes del pago.
func VoidPayment(p *payment.Payment, receivables map[uuid.UUID]*receivable.Receivable, reason string, at time.Time,
) ([]Effect, error) {
	if err := p.CanVoid(reason); err != nil {
		return nil, err
	}
	active := slices.DeleteFunc(p.Applications(), func(a payment.Application) bool { return !a.Active() })
	slices.SortFunc(active, func(a, b payment.Application) int {
		return cmp.Compare(a.ReceivableID.String(), b.ReceivableID.String())
	})
	for _, a := range active {
		r, ok := receivables[a.ReceivableID]
		if !ok {
			return nil, fmt.Errorf("%w: cuenta %s", ErrNotLoaded, a.ReceivableID)
		}
		if err := canReverse(p, r, a.ID, reason); err != nil {
			return nil, err
		}
	}
	effects := make([]Effect, 0, len(active))
	for _, a := range active {
		change, err := Reverse(p, receivables[a.ReceivableID], a.ID, reason, at)
		if err != nil {
			return nil, err
		}
		effects = append(effects, Effect{ReceivableID: a.ReceivableID, Change: change})
	}
	return effects, p.Void(reason, at)
}

// DebitNote registra una nota de débito (R4). No toca pagos.
func DebitNote(r *receivable.Receivable, adj receivable.Adjustment) (receivable.AdjustmentResult, error) {
	return r.AddDebitNote(adj)
}

// CreditNote registra una nota de crédito; si supera el saldo, las aplicaciones que la cuenta revierte (R3) se
// registran también en sus pagos. payments debe traer los pagos de todas las aplicaciones vigentes de la cuenta.
func CreditNote(r *receivable.Receivable, payments map[uuid.UUID]*payment.Payment, adj receivable.Adjustment,
	at time.Time,
) (receivable.AdjustmentResult, error) {
	return withReleases(r, payments, at, func() (receivable.AdjustmentResult, error) { return r.AddCreditNote(adj, at) })
}

// CancelInvoice anula la cuenta (R2): lo aplicado vuelve a estar disponible en cada pago.
func CancelInvoice(r *receivable.Receivable, payments map[uuid.UUID]*payment.Payment, adj receivable.Adjustment,
	at time.Time,
) (receivable.AdjustmentResult, error) {
	return withReleases(r, payments, at, func() (receivable.AdjustmentResult, error) { return r.Cancel(adj, at) })
}

func withReleases(r *receivable.Receivable, payments map[uuid.UUID]*payment.Payment, at time.Time,
	add func() (receivable.AdjustmentResult, error),
) (receivable.AdjustmentResult, error) {
	for _, id := range r.ActivePaymentIDs() {
		p, ok := payments[id]
		if !ok {
			return receivable.AdjustmentResult{}, fmt.Errorf("%w: pago %s", ErrNotLoaded, id)
		}
		for _, a := range r.Applications() {
			if a.Active() && a.PaymentID == id {
				if _, err := p.CanReverse(a.ID, receivable.ReasonCreditNote); err != nil {
					return receivable.AdjustmentResult{}, fmt.Errorf("%w: %w", ErrMismatch, err)
				}
			}
		}
	}
	res, err := add()
	if err != nil {
		return receivable.AdjustmentResult{}, err
	}
	for _, rev := range res.Reversed {
		if err := payments[rev.PaymentID].ReverseApplication(rev.ApplicationID, rev.Reason, at); err != nil {
			return receivable.AdjustmentResult{}, err
		}
	}
	return res, nil
}

func canReverse(p *payment.Payment, r *receivable.Receivable, applicationID uuid.UUID, reason string) error {
	pa, err := p.CanReverse(applicationID, reason)
	if err != nil {
		return err
	}
	ra, err := r.CanReverse(applicationID, reason)
	if err != nil {
		return err
	}
	if pa.ReceivableID != r.ID() || ra.PaymentID != p.ID() || !pa.Amount.Equal(ra.Amount) {
		return ErrMismatch
	}
	return nil
}
