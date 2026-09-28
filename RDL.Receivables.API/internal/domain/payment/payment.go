// Package payment es el agregado pago: monto, aplicaciones vigentes, reversos y anulación.
// Invariante: la suma de las aplicaciones vigentes nunca supera el monto del pago.
package payment

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
)

// Status sigue RDL.Contracts/state-machines/payment.yaml.
type Status string

const (
	StatusPosted Status = "posted"
	StatusVoided Status = "voided"
)

var (
	// ErrVoided: el pago está anulado (payment-voided, 409).
	ErrVoided = errors.New("el pago está anulado")
	// ErrExceedsPayment: la suma aplicada superaría el monto del pago (application-exceeds-payment, 422).
	ErrExceedsPayment = errors.New("la aplicación supera el disponible del pago")
	// ErrDuplicateApplication: el pago ya tiene una aplicación vigente a esa cuenta (409).
	ErrDuplicateApplication = errors.New("el pago ya tiene una aplicación vigente a esta cuenta")
	// ErrApplicationNotFound: la aplicación no es de este pago.
	ErrApplicationNotFound = errors.New("la aplicación no existe en este pago")
	// ErrApplicationReversed: la aplicación ya estaba revertida (application-reversed, 409).
	ErrApplicationReversed = errors.New("la aplicación ya fue revertida")
	// ErrReasonRequired: anular o revertir exige un motivo.
	ErrReasonRequired = errors.New("el motivo es obligatorio")
	// ErrActiveApplications: se intentó anular con aplicaciones vigentes (la base lo exige igual: payments_guard).
	ErrActiveApplications = errors.New("revierta las aplicaciones vigentes antes de anular el pago")
	// ErrInconsistent: datos incompletos o cargados de la base sin cumplir las invariantes.
	ErrInconsistent = errors.New("el pago no es consistente")
)

// Data son los datos del pago que no cambian una vez registrado.
type Data struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	Currency   string
	Amount     decimal.Decimal
}

// Application es la parte del pago aplicada a una cuenta. Se revierte con motivo; nunca se borra ni se edita.
type Application struct {
	ID             uuid.UUID
	ReceivableID   uuid.UUID
	Amount         decimal.Decimal
	AppliedAt      time.Time
	ReversedAt     time.Time
	ReversalReason string
}

func (a Application) Active() bool { return a.ReversedAt.IsZero() }

// Payment es el agregado pago.
type Payment struct {
	data         Data
	status       Status
	voidReason   string
	voidedAt     time.Time
	applications []Application
}

// New registra un pago en posted.
func New(d Data) (*Payment, error) {
	return Rehydrate(d, StatusPosted, "", time.Time{}, nil)
}

// Rehydrate reconstruye el pago desde la base y verifica sus invariantes.
func Rehydrate(d Data, status Status, voidReason string, voidedAt time.Time, apps []Application) (*Payment, error) {
	if d.ID == uuid.Nil || d.CustomerID == uuid.Nil || d.Currency == "" {
		return nil, fmt.Errorf("%w: faltan datos del pago", ErrInconsistent)
	}
	if err := amount.Positive(d.Amount); err != nil {
		return nil, err
	}
	p := &Payment{data: d, status: status, voidReason: voidReason, voidedAt: voidedAt, applications: slices.Clone(apps)}
	switch {
	case status != StatusPosted && status != StatusVoided:
		return nil, fmt.Errorf("%w: estado %q", ErrInconsistent, status)
	case (status == StatusVoided) != (voidReason != "" && !voidedAt.IsZero()):
		return nil, fmt.Errorf("%w: anulación sin motivo o instante", ErrInconsistent)
	case p.Applied().GreaterThan(d.Amount):
		return nil, fmt.Errorf("%w: aplicado mayor que el monto", ErrInconsistent)
	case status == StatusVoided && p.Applied().IsPositive():
		return nil, fmt.Errorf("%w: pago anulado con aplicaciones vigentes", ErrInconsistent)
	}
	return p, nil
}

func (p *Payment) ID() uuid.UUID               { return p.data.ID }
func (p *Payment) Data() Data                  { return p.data }
func (p *Payment) CustomerID() uuid.UUID       { return p.data.CustomerID }
func (p *Payment) Currency() string            { return p.data.Currency }
func (p *Payment) Amount() decimal.Decimal     { return p.data.Amount }
func (p *Payment) Status() Status              { return p.status }
func (p *Payment) VoidReason() string          { return p.voidReason }
func (p *Payment) Applications() []Application { return slices.Clone(p.applications) }

// Applied es la suma de las aplicaciones vigentes.
func (p *Payment) Applied() decimal.Decimal {
	sum := decimal.Zero
	for _, a := range p.applications {
		if a.Active() {
			sum = sum.Add(a.Amount)
		}
	}
	return sum
}

// Available es lo que todavía se puede aplicar (incluye lo liberado por reversos: el "saldo a favor" de R2).
func (p *Payment) Available() decimal.Decimal { return p.data.Amount.Sub(p.Applied()) }

// ActiveReceivableIDs son las cuentas con aplicaciones vigentes de este pago.
func (p *Payment) ActiveReceivableIDs() []uuid.UUID {
	var ids []uuid.UUID
	for _, a := range p.applications {
		if a.Active() {
			ids = append(ids, a.ReceivableID)
		}
	}
	return ids
}

// CanApply valida una aplicación sin cambiar nada.
func (p *Payment) CanApply(receivableID uuid.UUID, amt decimal.Decimal) error {
	if p.status == StatusVoided {
		return ErrVoided
	}
	if err := amount.Positive(amt); err != nil {
		return err
	}
	if slices.ContainsFunc(p.applications, func(a Application) bool { return a.Active() && a.ReceivableID == receivableID }) {
		return ErrDuplicateApplication
	}
	if amt.GreaterThan(p.Available()) {
		return ErrExceedsPayment
	}
	return nil
}

// Apply registra una aplicación del pago.
func (p *Payment) Apply(app Application) error {
	if app.ID == uuid.Nil || app.ReceivableID == uuid.Nil || !app.Active() {
		return fmt.Errorf("%w: aplicación incompleta", ErrInconsistent)
	}
	if err := p.CanApply(app.ReceivableID, app.Amount); err != nil {
		return err
	}
	p.applications = append(p.applications, app)
	return nil
}

// CanReverse valida la reversión sin cambiar nada. Un pago anulado ya no tiene aplicaciones vigentes.
func (p *Payment) CanReverse(applicationID uuid.UUID, reason string) (Application, error) {
	if reason == "" {
		return Application{}, ErrReasonRequired
	}
	i := slices.IndexFunc(p.applications, func(a Application) bool { return a.ID == applicationID })
	if i < 0 {
		return Application{}, ErrApplicationNotFound
	}
	if !p.applications[i].Active() {
		return Application{}, ErrApplicationReversed
	}
	return p.applications[i], nil
}

// ReverseApplication revierte una aplicación: su monto vuelve a estar disponible.
func (p *Payment) ReverseApplication(applicationID uuid.UUID, reason string, at time.Time) error {
	if at.IsZero() {
		return fmt.Errorf("%w: reversión sin instante", ErrInconsistent)
	}
	if _, err := p.CanReverse(applicationID, reason); err != nil {
		return err
	}
	i := slices.IndexFunc(p.applications, func(a Application) bool { return a.ID == applicationID })
	p.applications[i].ReversedAt, p.applications[i].ReversalReason = at, reason
	return nil
}

// CanVoid valida la anulación sin mirar las aplicaciones (el servicio las revierte antes de llamar a Void).
func (p *Payment) CanVoid(reason string) error {
	if p.status == StatusVoided {
		return ErrVoided
	}
	if reason == "" {
		return ErrReasonRequired
	}
	return nil
}

// Void pasa el pago a voided. Igual que la base, exige que no queden aplicaciones vigentes.
func (p *Payment) Void(reason string, at time.Time) error {
	if at.IsZero() {
		return fmt.Errorf("%w: anulación sin instante", ErrInconsistent)
	}
	if err := p.CanVoid(reason); err != nil {
		return err
	}
	if p.Applied().IsPositive() {
		return ErrActiveApplications
	}
	p.status, p.voidReason, p.voidedAt = StatusVoided, reason, at
	return nil
}
