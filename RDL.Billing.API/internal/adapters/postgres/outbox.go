package postgres

import (
	"context"
	"fmt"

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
	cid, ok := correlation.FromContext(ctx)
	if !ok {
		cid = uuid.New()
	}
	e, err := events.InvoiceIssued(inv, issueDate, cid)
	if err != nil {
		return err
	}
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
