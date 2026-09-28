package receivable

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
)

// Motivos de las reversiones que hace el propio agregado (decisiones R2 y R3).
const (
	ReasonInvoiceCancelled = "Factura anulada"
	ReasonCreditNote       = "Nota de crédito mayor que el saldo" //nolint:gosec // G101: es un motivo, no una credencial
)

// errNoInstant: una reversión sin instante quedaría vigente (Active mira ReversedAt).
var errNoInstant = fmt.Errorf("%w: reversión sin instante", ErrInconsistent)

// Invoice son los datos de la factura que originan la cuenta (InvoiceIssued) y que después son inmutables.
// ID es el de la factura en Billing (source_invoice_id), no el de la cuenta.
type Invoice struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	Currency   string
	Original   decimal.Decimal
	IssuedOn   civil.Date
	DueOn      civil.Date
}

type AdjustmentType string

const (
	AdjustmentCreditNote   AdjustmentType = "credit_note"
	AdjustmentDebitNote    AdjustmentType = "debit_note"
	AdjustmentCancellation AdjustmentType = "cancellation"
	// AdjustmentWriteOff existe en la base pero no tiene operación en V1 (R5); se cuenta como crédito al cargar.
	AdjustmentWriteOff AdjustmentType = "write_off"
)

// Adjustment es un ajuste de saldo por un documento de Billing. Es inmutable: nunca se edita ni se borra.
type Adjustment struct {
	ID               uuid.UUID
	Type             AdjustmentType
	Amount           decimal.Decimal
	SourceDocumentID uuid.UUID
	SourceEventID    uuid.UUID
}

// Application es la parte de un pago aplicada a la cuenta. Se revierte con motivo; nunca se borra ni se edita.
type Application struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	Amount         decimal.Decimal
	AppliedAt      time.Time
	ReversedAt     time.Time
	ReversalReason string
}

func (a Application) Active() bool { return a.ReversedAt.IsZero() }

// Change es el cambio de estado que produjo una operación. From == To si el estado no cambió.
type Change struct{ From, To Status }

// Settled indica que la cuenta pasó a paid con esta operación: hay que emitir ReceivableSettled.
func (c Change) Settled() bool { return c.To == StatusPaid && c.From != StatusPaid }

// Reversal es una aplicación que el agregado revirtió por su cuenta (R2, R3). El pago dueño debe registrarla.
type Reversal struct {
	ApplicationID uuid.UUID
	PaymentID     uuid.UUID
	Amount        decimal.Decimal
	Reason        string
}

// AdjustmentResult es el efecto de registrar un ajuste. Duplicate: el documento ya estaba registrado con el mismo
// monto y no se cambió nada (reprocesar un evento es inocuo, invariante 4).
type AdjustmentResult struct {
	Change
	Duplicate bool
	Reversed  []Reversal
}

// Receivable es el agregado cuenta por cobrar. El saldo y el estado solo cambian aquí, derivados con Derive.
type Receivable struct {
	id           uuid.UUID
	invoice      Invoice
	adjustments  []Adjustment
	applications []Application
	balance      decimal.Decimal
	status       Status
}

// New crea la cuenta id de una factura recién emitida: open, con saldo igual al total.
func New(id uuid.UUID, inv Invoice) (*Receivable, error) {
	return Rehydrate(id, inv, nil, nil)
}

// Rehydrate reconstruye la cuenta desde la base y recalcula saldo y estado con la misma fórmula que la base.
func Rehydrate(id uuid.UUID, inv Invoice, adjustments []Adjustment, applications []Application) (*Receivable, error) {
	if id == uuid.Nil || inv.ID == uuid.Nil || inv.CustomerID == uuid.Nil || inv.Currency == "" {
		return nil, fmt.Errorf("%w: faltan datos de la factura", ErrInconsistent)
	}
	if err := amount.Positive(inv.Original); err != nil {
		return nil, err
	}
	if inv.IssuedOn.IsZero() || inv.DueOn.IsZero() || inv.DueOn.Before(inv.IssuedOn) {
		return nil, ErrInvalidDates
	}
	r := &Receivable{id: id, invoice: inv}
	if _, err := r.commit(slices.Clone(adjustments), slices.Clone(applications)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInconsistent, err)
	}
	return r, nil
}

func (r *Receivable) ID() uuid.UUID               { return r.id }
func (r *Receivable) Invoice() Invoice            { return r.invoice }
func (r *Receivable) CustomerID() uuid.UUID       { return r.invoice.CustomerID }
func (r *Receivable) Currency() string            { return r.invoice.Currency }
func (r *Receivable) Balance() decimal.Decimal    { return r.balance }
func (r *Receivable) Status() Status              { return r.status }
func (r *Receivable) Adjustments() []Adjustment   { return slices.Clone(r.adjustments) }
func (r *Receivable) Applications() []Application { return slices.Clone(r.applications) }

// Totals expone las sumas de las que sale el saldo (los tests comparan con la base).
func (r *Receivable) Totals() Totals {
	return totals(r.invoice.Original, r.adjustments, r.applications)
}

// ActivePaymentIDs son los pagos con aplicaciones vigentes, ordenados por id: el orden en que se bloquean antes de
// anular la factura o registrar una nota de crédito (pagos primero, igual que los triggers).
func (r *Receivable) ActivePaymentIDs() []uuid.UUID {
	var ids []uuid.UUID
	for _, a := range r.applications {
		if a.Active() && !slices.Contains(ids, a.PaymentID) {
			ids = append(ids, a.PaymentID)
		}
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return cmp.Compare(a.String(), b.String()) })
	return ids
}

// CanApply valida una aplicación sin cambiar nada; el servicio de aplicación la usa antes de tocar el pago.
func (r *Receivable) CanApply(paymentID uuid.UUID, amt decimal.Decimal) error {
	if r.status == StatusCancelled {
		return ErrCancelled
	}
	if err := amount.Positive(amt); err != nil {
		return err
	}
	if slices.ContainsFunc(r.applications, func(a Application) bool { return a.Active() && a.PaymentID == paymentID }) {
		return ErrDuplicateApplication
	}
	if amt.GreaterThan(r.balance) {
		return ErrExceedsBalance
	}
	return nil
}

// Apply registra una aplicación de pago. La moneda y el cliente los valida el servicio, que ve los dos agregados.
func (r *Receivable) Apply(app Application) (Change, error) {
	if app.ID == uuid.Nil || app.PaymentID == uuid.Nil || !app.Active() {
		return Change{}, fmt.Errorf("%w: aplicación incompleta", ErrInconsistent)
	}
	if err := r.CanApply(app.PaymentID, app.Amount); err != nil {
		return Change{}, err
	}
	return r.commit(r.adjustments, append(slices.Clone(r.applications), app))
}

// CanReverse valida la reversión sin cambiar nada.
func (r *Receivable) CanReverse(applicationID uuid.UUID, reason string) (Application, error) {
	if reason == "" {
		return Application{}, ErrReasonRequired
	}
	i := slices.IndexFunc(r.applications, func(a Application) bool { return a.ID == applicationID })
	if i < 0 {
		return Application{}, ErrApplicationNotFound
	}
	if !r.applications[i].Active() {
		return Application{}, ErrApplicationReversed
	}
	return r.applications[i], nil
}

// ReverseApplication revierte una aplicación: el saldo vuelve exactamente a lo que era antes de aplicarla.
func (r *Receivable) ReverseApplication(applicationID uuid.UUID, reason string, at time.Time) (Change, error) {
	if at.IsZero() {
		return Change{}, errNoInstant
	}
	if _, err := r.CanReverse(applicationID, reason); err != nil {
		return Change{}, err
	}
	apps := slices.Clone(r.applications)
	reverse(apps, applicationID, reason, at)
	return r.commit(r.adjustments, apps)
}

// AddDebitNote suma una nota de débito a la cuenta de la factura (R4: se conserva el due_on de la factura). Una
// cuenta paid se reabre.
func (r *Receivable) AddDebitNote(adj Adjustment) (AdjustmentResult, error) {
	return r.addAdjustment(AdjustmentDebitNote, adj, func([]Application) ([]Reversal, error) { return nil, nil })
}

// AddCreditNote resta una nota de crédito. Si supera el saldo, revierte aplicaciones de la más reciente a la más
// antigua solo hasta donde haga falta (R3); lo liberado vuelve a estar disponible en cada pago.
func (r *Receivable) AddCreditNote(adj Adjustment, at time.Time) (AdjustmentResult, error) {
	if at.IsZero() {
		return AdjustmentResult{}, errNoInstant
	}
	return r.addAdjustment(AdjustmentCreditNote, adj, func(apps []Application) ([]Reversal, error) {
		missing := adj.Amount.Sub(r.balance)
		var reversed []Reversal
		for _, a := range newestFirst(apps) {
			if !missing.IsPositive() {
				break
			}
			reversed = append(reversed, reverse(apps, a.ID, ReasonCreditNote, at))
			missing = missing.Sub(a.Amount)
		}
		if missing.IsPositive() {
			return nil, ErrCreditExceedsDebt
		}
		return reversed, nil
	})
}

// Cancel anula la cuenta (InvoiceCancelled, R2): revierte todas las aplicaciones vigentes y registra el ajuste
// cancellation por el saldo que queda. El monto de adj se ignora: lo calcula el agregado.
func (r *Receivable) Cancel(adj Adjustment, at time.Time) (AdjustmentResult, error) {
	if at.IsZero() {
		return AdjustmentResult{}, errNoInstant
	}
	return r.addAdjustment(AdjustmentCancellation, adj, func(apps []Application) ([]Reversal, error) {
		var reversed []Reversal
		for _, a := range newestFirst(apps) {
			reversed = append(reversed, reverse(apps, a.ID, ReasonInvoiceCancelled, at))
		}
		return reversed, nil
	})
}

// addAdjustment es el camino común de los ajustes por evento: idempotencia por documento, cuenta anulada,
// reversiones previas (si las hay) y recálculo. Nada cambia si algo falla.
func (r *Receivable) addAdjustment(
	typ AdjustmentType, adj Adjustment, release func(apps []Application) ([]Reversal, error),
) (AdjustmentResult, error) {
	adj.Type = typ
	if adj.ID == uuid.Nil || adj.SourceDocumentID == uuid.Nil || adj.SourceEventID == uuid.Nil {
		return AdjustmentResult{}, fmt.Errorf("%w: ajuste sin id, documento o evento de origen", ErrInconsistent)
	}
	if dup, err := r.duplicate(adj); dup || err != nil {
		return AdjustmentResult{Change: Change{From: r.status, To: r.status}, Duplicate: dup}, err
	}
	if r.status == StatusCancelled {
		return AdjustmentResult{}, ErrCancelled
	}
	if typ != AdjustmentCancellation {
		if err := amount.Positive(adj.Amount); err != nil {
			return AdjustmentResult{}, err
		}
	}

	apps := slices.Clone(r.applications)
	reversed, err := release(apps)
	if err != nil {
		return AdjustmentResult{}, err
	}
	if typ == AdjustmentCancellation {
		t := totals(r.invoice.Original, r.adjustments, apps)
		adj.Amount = t.Due().Sub(t.Credits).Sub(t.Applied)
		if !adj.Amount.IsPositive() {
			return AdjustmentResult{}, ErrNothingToCancel
		}
	}
	change, err := r.commit(append(slices.Clone(r.adjustments), adj), apps)
	if err != nil {
		return AdjustmentResult{}, err
	}
	return AdjustmentResult{Change: change, Reversed: reversed}, nil
}

// duplicate reconoce un documento ya registrado (UK (organization_id, adjustment_type, source_document_id) y
// (organization_id, source_event_id)). El mismo documento con otro monto no es un duplicado: es un error.
func (r *Receivable) duplicate(adj Adjustment) (bool, error) {
	for _, old := range r.adjustments {
		sameDocument := old.Type == adj.Type && old.SourceDocumentID == adj.SourceDocumentID
		switch {
		case sameDocument && (adj.Type == AdjustmentCancellation || old.Amount.Equal(adj.Amount)):
			return true, nil
		case sameDocument || old.SourceEventID == adj.SourceEventID:
			return false, ErrConflictingAdjustment
		}
	}
	return false, nil
}

// commit recalcula con las listas nuevas y solo las adopta si el resultado es válido.
func (r *Receivable) commit(adjustments []Adjustment, applications []Application) (Change, error) {
	balance, status, err := Derive(totals(r.invoice.Original, adjustments, applications))
	if err != nil {
		return Change{}, err
	}
	change := Change{From: r.status, To: status}
	r.adjustments, r.applications, r.balance, r.status = adjustments, applications, balance, status
	return change, nil
}

func totals(original decimal.Decimal, adjustments []Adjustment, applications []Application) Totals {
	t := Totals{Original: original}
	for _, a := range adjustments {
		switch a.Type {
		case AdjustmentDebitNote:
			t.Debits = t.Debits.Add(a.Amount)
		case AdjustmentCancellation:
			t.Cancellation = t.Cancellation.Add(a.Amount)
			t.Cancelled = true
		default: // credit_note y write_off
			t.Credits = t.Credits.Add(a.Amount)
		}
	}
	for _, a := range applications {
		if a.Active() {
			t.Applied = t.Applied.Add(a.Amount)
		}
	}
	return t
}

// newestFirst son las aplicaciones vigentes de la más reciente a la más antigua (a igual fecha, por id).
func newestFirst(apps []Application) []Application {
	active := slices.DeleteFunc(slices.Clone(apps), func(a Application) bool { return !a.Active() })
	slices.SortFunc(active, func(a, b Application) int {
		if c := b.AppliedAt.Compare(a.AppliedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID.String(), a.ID.String())
	})
	return active
}

func reverse(apps []Application, id uuid.UUID, reason string, at time.Time) Reversal {
	i := slices.IndexFunc(apps, func(a Application) bool { return a.ID == id })
	apps[i].ReversedAt, apps[i].ReversalReason = at, reason
	return Reversal{ApplicationID: id, PaymentID: apps[i].PaymentID, Amount: apps[i].Amount, Reason: reason}
}
