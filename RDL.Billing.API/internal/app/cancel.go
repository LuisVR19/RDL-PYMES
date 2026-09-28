package app

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/pkg/tenancy"
)

// CancelInvoice es issued → cancelled (POST /v1/invoices/{id}/cancel), owner y admin. En UNA transacción: guarda la
// anulación con su motivo, el historial, el audit e InvoiceCancelled v1 en el outbox. No llama a ningún servicio:
// Receivables ajusta la cuenta por cobrar y fiscal decide la corrección ante Hacienda al consumir el evento.
// Idempotente por Idempotency-Key.
type CancelInvoice struct {
	tx  TxManager
	now func() time.Time
}

func NewCancelInvoice(tx TxManager) *CancelInvoice {
	return &CancelInvoice{tx: tx, now: time.Now}
}

type CancelResult struct {
	Invoice  invoice.Invoice
	Replayed bool
}

func (uc *CancelInvoice) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, id uuid.UUID, reason string) (CancelResult, error) {
	if err := authorize(t, permission.InvoicesCancel); err != nil {
		return CancelResult{}, err
	}
	hash, err := requestHash(map[string]string{"cancel": id.String(), "reason": reason})
	if err != nil {
		return CancelResult{}, err
	}
	var res CancelResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		// Primero la clave: el reintento de una anulación confirmada responde lo mismo, no 409.
		prevID, err := replayed(ctx, tx, t.OrganizationID(), idempotencyKey, hash, "invoiceId")
		if err != nil {
			return err
		}
		if prevID != uuid.Nil {
			inv, err := tx.Invoices().Get(ctx, t.OrganizationID(), prevID)
			res = CancelResult{Invoice: inv, Replayed: true}
			return err
		}

		current, err := tx.Invoices().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		// Precisión de microsegundos, la de timestamptz: el evento y la base guardan el mismo instante.
		at := uc.now().UTC().Truncate(time.Microsecond)
		cancelled, err := current.Cancel(invoice.CancelInput{Reason: reason, At: at, By: t.UserID()})
		if err != nil {
			return err
		}
		if cancelled, err = tx.Invoices().MarkCancelled(ctx, cancelled); err != nil {
			return err
		}
		userID := t.UserID()
		if err := tx.Invoices().AddStatusChange(ctx, t.OrganizationID(), id, StatusChange{
			From: invoice.StatusIssued, To: invoice.StatusCancelled, Reason: cancelled.CancellationReason,
			ChangedByUserID: &userID, ChangedAt: at,
		}); err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invoice.cancelled", EntityType: "invoice", EntityID: id, Reason: cancelled.CancellationReason,
			Before: map[string]any{"status": string(invoice.StatusIssued)},
			After: map[string]any{
				"status": string(invoice.StatusCancelled), "number": cancelled.Number,
				"cancelledAt": at.Format(time.RFC3339Nano), "total": cancelled.Totals.Total.String(),
				"currency": cancelled.Currency.String(),
			},
		}); err != nil {
			return err
		}
		if err := tx.Outbox().InvoiceCancelled(ctx, cancelled); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusOK, Result: map[string]string{"invoiceId": id.String()},
		}); err != nil {
			return err
		}
		res = CancelResult{Invoice: cancelled}
		return nil
	})
	return res, err
}
