package postgres

import (
	"context"
	"fmt"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/events"
	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/pkg/correlation"
)

type outbox struct{ q *db.Queries }

// InvoiceIssued arma InvoiceIssued v1, lo valida contra su JSON Schema y lo escribe en el outbox. Si el evento no
// valida, devuelve error y la transacción de la emisión se revierte entera: no queda factura emitida sin evento.
func (o outbox) InvoiceIssued(ctx context.Context, inv invoice.Invoice, issueDate string) error {
	e, err := events.InvoiceIssued(inv, issueDate, correlationID(ctx))
	if err != nil {
		return err
	}
	return o.write(ctx, e)
}

// NoteIssued escribe CreditNoteIssued o DebitNoteIssued v1, con la misma garantía que InvoiceIssued.
func (o outbox) NoteIssued(ctx context.Context, note invoice.Invoice, referencedNumber, issueDate string) error {
	e, err := events.NoteIssued(note, referencedNumber, issueDate, correlationID(ctx))
	if err != nil {
		return err
	}
	return o.write(ctx, e)
}

// InvoiceCancelled escribe InvoiceCancelled v1 en la transacción de la anulación.
func (o outbox) InvoiceCancelled(ctx context.Context, inv invoice.Invoice) error {
	e, err := events.InvoiceCancelled(inv, correlationID(ctx))
	if err != nil {
		return err
	}
	return o.write(ctx, e)
}

// write valida el evento contra su schema y solo entonces lo inserta en integration.outbox_messages.
func (o outbox) write(ctx context.Context, e cevents.Event) error {
	row, err := events.OutboxRow(e)
	if err != nil {
		return err
	}
	if err := o.q.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
		ID: row.ID, SourceService: string(row.SourceService), OrganizationID: row.OrganizationID, EventType: row.EventType,
		EventVersion:  int32(row.EventVersion), // #nosec G115 -- versión de evento (1, 2...)
		AggregateType: row.AggregateType, AggregateID: row.AggregateID, CorrelationID: row.CorrelationID,
		Payload: string(row.Payload), OccurredAt: row.OccurredAt.Time(),
	}); err != nil {
		return fmt.Errorf("escribiendo outbox: %w", err)
	}
	return nil
}

// correlationID es el del request; sin él (un proceso interno), uno nuevo.
func correlationID(ctx context.Context) uuid.UUID {
	if cid, ok := correlation.FromContext(ctx); ok {
		return cid
	}
	return uuid.New()
}
