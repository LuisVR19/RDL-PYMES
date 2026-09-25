package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
)

// Ajustes de saldo por documentos de Billing (incremento 8): notas de crédito (R3), notas de débito (R4) y anulación
// de la factura (R2). El tenant es el organizationId del evento; la idempotencia la dan el inbox y, además, la
// unicidad del documento de origen en el agregado (el mismo documento en otro evento no cambia nada).

var (
	// ErrInvoiceNotYetReceived: la nota o la anulación llegó antes que su InvoiceIssued (sin orden garantizado).
	// Es transitorio: se reintenta y, si nunca llega, termina en dead letter.
	ErrInvoiceNotYetReceived = errors.New("la factura todavía no tiene cuenta por cobrar")
	// ErrDocumentMismatch: moneda o total del evento distintos de los de la cuenta.
	ErrDocumentMismatch = errors.New("el documento no corresponde con la cuenta por cobrar")
	// errPaymentsChanged: entre leer los pagos vigentes y bloquear la cuenta, otra transacción aplicó o revirtió.
	// Transitorio: el siguiente intento vuelve a leer.
	errPaymentsChanged = errors.New("cambiaron los pagos de la cuenta durante el bloqueo")
)

type CreditNoteIssued struct {
	DocumentID     uuid.UUID
	DocumentNumber string
	InvoiceID      uuid.UUID
	Reason         string
	Currency       string
	Total          decimal.Decimal
}

type DebitNoteIssued struct {
	DocumentID     uuid.UUID
	DocumentNumber string
	InvoiceID      uuid.UUID
	Reason         string
	Currency       string
	Total          decimal.Decimal
	// DueDate de la nota: la cuenta conserva el vencimiento de la factura (R4); queda en el audit.
	DueDate civil.Date
}

type InvoiceCancelled struct {
	InvoiceID     uuid.UUID
	InvoiceNumber string
	Reason        string
	Currency      string
	Total         decimal.Decimal
}

// lockForAdjustment bloquea, en orden estable, los pagos con aplicaciones vigentes en la cuenta de la factura y
// después la cuenta.
func lockForAdjustment(ctx context.Context, tx EventTx, org, invoiceID uuid.UUID) (locked, *receivable.Receivable, error) {
	rec, found, err := tx.ReceivableStore().FindByInvoice(ctx, org, invoiceID)
	if err != nil {
		return locked{}, nil, err
	}
	if !found {
		return locked{}, nil, fmt.Errorf("%w: factura %s", ErrInvoiceNotYetReceived, invoiceID)
	}
	l := tx.Ledger()
	paymentIDs, err := l.ActivePaymentIDs(ctx, org, rec.ID)
	if err != nil {
		return locked{}, nil, err
	}
	lk, err := lockAll(ctx, l, org, paymentIDs, []uuid.UUID{rec.ID})
	if err != nil {
		return locked{}, nil, err
	}
	r := lk.receivables[rec.ID]
	if !slices.Equal(sortedUnique(paymentIDs), r.ActivePaymentIDs()) {
		return locked{}, nil, errPaymentsChanged
	}
	return lk, r, nil
}

// adjustmentEffect persiste un ajuste ya aplicado al agregado: primero las reversiones (así el saldo nunca pasa por
// negativo en la base), después el ajuste; luego verifica y publica ReceivableSettled si corresponde.
func adjustmentEffect(ctx context.Context, tx EventTx, ev IncomingEvent, lk locked, r *receivable.Receivable,
	res receivable.AdjustmentResult, reason string, now time.Time,
) error {
	org := ev.Ref.OrganizationID
	l := tx.Ledger()
	for _, rev := range res.Reversed {
		if err := l.ReverseApplication(ctx, org, rev.ApplicationID, rev.Reason, now, uuid.Nil); err != nil {
			return err
		}
	}
	stored := r.Adjustments()[len(r.Adjustments())-1] // la cancelación trae el monto que calculó el agregado
	if err := l.InsertAdjustment(ctx, org, r.ID(), stored, reason); err != nil {
		return err
	}
	return settle(ctx, l, tx.Outbox(), org, r, lk.meta[r.ID()], res.Change, ev.Ref.CorrelationID, now)
}

func reversalsAudit(rs []receivable.Reversal) []map[string]any {
	out := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		out = append(out, map[string]any{
			"applicationId": r.ApplicationID, "paymentId": r.PaymentID, "amount": r.Amount.String(), "reason": r.Reason,
		})
	}
	return out
}

func checkCurrency(r *receivable.Receivable, currency string) error {
	if r.Currency() != currency {
		return reject(fmt.Errorf("%w: moneda %s, la cuenta está en %s", ErrDocumentMismatch, currency, r.Currency()))
	}
	return nil
}

// domainReject marca como permanente un error de regla de negocio; un error de infraestructura sigue transitorio.
func domainReject(err error) error {
	for _, e := range []error{
		receivable.ErrCancelled, receivable.ErrCreditExceedsDebt, receivable.ErrNothingToCancel,
		receivable.ErrConflictingAdjustment, receivable.ErrNegativeBalance, receivable.ErrInconsistent,
		amount.ErrInvalid, settlement.ErrNotLoaded, settlement.ErrMismatch,
	} {
		if errors.Is(err, e) {
			return reject(err)
		}
	}
	return err
}

// ApplyCreditNote: CreditNoteIssued disminuye el saldo; si supera el saldo, revierte aplicaciones de la más reciente
// a la más antigua solo hasta donde haga falta (R3). Si llega a cero, ReceivableSettled.
type ApplyCreditNote struct {
	now   func() time.Time
	newID func() uuid.UUID
}

func NewApplyCreditNote() *ApplyCreditNote { return &ApplyCreditNote{now: utcNow, newID: uuid.New} }

func (uc *ApplyCreditNote) Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error {
	cmd, ok := ev.Body.(CreditNoteIssued)
	if !ok {
		return fmt.Errorf("%w: %s no trae un CreditNoteIssued", ErrInvalidEvent, ev.Ref.Type)
	}
	now := uc.now()
	lk, r, err := lockForAdjustment(ctx, tx, ev.Ref.OrganizationID, cmd.InvoiceID)
	if err != nil {
		return err
	}
	if err := checkCurrency(r, cmd.Currency); err != nil {
		return err
	}
	before := r.Balance()
	adj := receivable.Adjustment{ID: uc.newID(), Amount: cmd.Total, SourceDocumentID: cmd.DocumentID, SourceEventID: ev.Ref.ID}
	res, err := settlement.CreditNote(r, lk.payments, adj, now)
	if err != nil {
		return domainReject(err)
	}
	if res.Duplicate {
		return nil
	}
	if err := adjustmentEffect(ctx, tx, ev, lk, r, res, noteReason(cmd.Reason, "Nota de crédito", cmd.DocumentNumber), now); err != nil {
		return err
	}
	return tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: ev.Ref.OrganizationID, ActorType: ActorService, CorrelationID: ev.Ref.CorrelationID,
		Action: "receivable.credit_note_applied", EntityType: "receivable", EntityID: r.ID(),
		Payload: map[string]any{
			"sourceEventId": ev.Ref.ID, "documentId": cmd.DocumentID, "documentNumber": cmd.DocumentNumber,
			"amount": cmd.Total.String(), "reversedApplications": reversalsAudit(res.Reversed),
			"before": map[string]any{"balance": before.String(), "status": res.From},
			"after":  map[string]any{"balance": r.Balance().String(), "status": res.To},
		},
	})
}

// ApplyDebitNote: DebitNoteIssued suma a la cuenta de la factura y conserva su vencimiento (R4). Una cuenta pagada
// se reabre. Sobre una cuenta anulada: dead letter.
type ApplyDebitNote struct {
	now   func() time.Time
	newID func() uuid.UUID
}

func NewApplyDebitNote() *ApplyDebitNote { return &ApplyDebitNote{now: utcNow, newID: uuid.New} }

func (uc *ApplyDebitNote) Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error {
	cmd, ok := ev.Body.(DebitNoteIssued)
	if !ok {
		return fmt.Errorf("%w: %s no trae un DebitNoteIssued", ErrInvalidEvent, ev.Ref.Type)
	}
	now := uc.now()
	lk, r, err := lockForAdjustment(ctx, tx, ev.Ref.OrganizationID, cmd.InvoiceID)
	if err != nil {
		return err
	}
	if err := checkCurrency(r, cmd.Currency); err != nil {
		return err
	}
	before := r.Balance()
	adj := receivable.Adjustment{ID: uc.newID(), Amount: cmd.Total, SourceDocumentID: cmd.DocumentID, SourceEventID: ev.Ref.ID}
	res, err := settlement.DebitNote(r, adj)
	if err != nil {
		return domainReject(err)
	}
	if res.Duplicate {
		return nil
	}
	if err := adjustmentEffect(ctx, tx, ev, lk, r, res, noteReason(cmd.Reason, "Nota de débito", cmd.DocumentNumber), now); err != nil {
		return err
	}
	return tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: ev.Ref.OrganizationID, ActorType: ActorService, CorrelationID: ev.Ref.CorrelationID,
		Action: "receivable.debit_note_applied", EntityType: "receivable", EntityID: r.ID(),
		Payload: map[string]any{
			"sourceEventId": ev.Ref.ID, "documentId": cmd.DocumentID, "documentNumber": cmd.DocumentNumber,
			"amount": cmd.Total.String(),
			// R4: el vencimiento de la nota no cambia el de la cuenta. TODO(contratos): ¿cuenta propia por nota?
			"noteDueDate": cmd.DueDate.String(), "receivableDueOn": r.Invoice().DueOn.String(),
			"before": map[string]any{"balance": before.String(), "status": res.From},
			"after":  map[string]any{"balance": r.Balance().String(), "status": res.To},
		},
	})
}

// CancelReceivable: InvoiceCancelled revierte todas las aplicaciones vigentes (lo aplicado vuelve a estar disponible
// en cada pago) y registra la cancelación por el saldo que queda (R2). Sin evento nuevo.
type CancelReceivable struct {
	now   func() time.Time
	newID func() uuid.UUID
}

func NewCancelReceivable() *CancelReceivable { return &CancelReceivable{now: utcNow, newID: uuid.New} }

func (uc *CancelReceivable) Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error {
	cmd, ok := ev.Body.(InvoiceCancelled)
	if !ok {
		return fmt.Errorf("%w: %s no trae un InvoiceCancelled", ErrInvalidEvent, ev.Ref.Type)
	}
	now := uc.now()
	lk, r, err := lockForAdjustment(ctx, tx, ev.Ref.OrganizationID, cmd.InvoiceID)
	if err != nil {
		return err
	}
	if err := checkCurrency(r, cmd.Currency); err != nil {
		return err
	}
	if !cmd.Total.Equal(r.Invoice().Original) {
		return reject(fmt.Errorf("%w: total %s, la cuenta es por %s", ErrDocumentMismatch, cmd.Total, r.Invoice().Original))
	}
	before := r.Balance()
	// La anulación usa el id de la factura como documento de origen: una factura se anula una sola vez.
	adj := receivable.Adjustment{ID: uc.newID(), SourceDocumentID: cmd.InvoiceID, SourceEventID: ev.Ref.ID}
	res, err := settlement.CancelInvoice(r, lk.payments, adj, now)
	if err != nil {
		// Una factura saldada solo con notas de crédito no tiene saldo que anular: la base no admite un ajuste de 0.
		// TODO(contratos): ¿qué pasa con una factura anulada después de acreditarla por completo?
		return domainReject(err)
	}
	if res.Duplicate {
		return nil
	}
	if err := adjustmentEffect(ctx, tx, ev, lk, r, res, noteReason(cmd.Reason, "Factura anulada", cmd.InvoiceNumber), now); err != nil {
		return err
	}
	stored := r.Adjustments()[len(r.Adjustments())-1]
	return tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: ev.Ref.OrganizationID, ActorType: ActorService, CorrelationID: ev.Ref.CorrelationID,
		Action: "receivable.cancelled", EntityType: "receivable", EntityID: r.ID(),
		Payload: map[string]any{
			"sourceEventId": ev.Ref.ID, "invoiceId": cmd.InvoiceID, "reason": cmd.Reason,
			"cancelledAmount": stored.Amount.String(), "reversedApplications": reversalsAudit(res.Reversed),
			"before": map[string]any{"balance": before.String(), "status": res.From},
			"after":  map[string]any{"balance": r.Balance().String(), "status": res.To},
		},
	})
}

// noteReason: receivable_adjustments.reason es obligatorio; si Billing no manda motivo, se usa el documento.
func noteReason(reason, kind, number string) string {
	if reason != "" {
		return reason
	}
	return kind + " " + number
}
