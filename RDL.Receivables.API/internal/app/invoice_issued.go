package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
)

// ErrInvoiceConflict: ya existe la cuenta de esa factura, creada por otro evento con datos distintos.
var ErrInvoiceConflict = errors.New("la factura ya tiene una cuenta por cobrar con otros datos")

// InvoiceIssued es el comando que sale de InvoiceIssued v1: lo que Receivables necesita de la factura.
// IssueDate y DueDate ya son fechas de negocio en la zona de la organización (las calcula Billing).
type InvoiceIssued struct {
	InvoiceID              uuid.UUID
	InvoiceNumber          string
	CustomerID             uuid.UUID
	CustomerIdentification string
	CustomerLegalName      string
	SaleConditionCode      string
	Currency               string
	Total                  decimal.Decimal
	IssueDate              civil.Date
	DueDate                civil.Date
}

// ReceivableStore persiste cuentas por cobrar. Nunca escribe saldo ni estado: los calcula la base (triggers).
type ReceivableStore interface {
	// FindByInvoice busca la cuenta de una factura de la organización de la sesión.
	FindByInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (ReceivableRecord, bool, error)
	Create(ctx context.Context, organizationID uuid.UUID, r NewReceivable) error
}

// ReceivableRecord son los datos de origen de una cuenta existente, para reconocer un reenvío.
type ReceivableRecord struct {
	ID         uuid.UUID
	CustomerID uuid.UUID
	Currency   string
	Original   decimal.Decimal
}

// NewReceivable es la cuenta recién creada más el snapshot de la factura que se guarda con ella.
type NewReceivable struct {
	Receivable             *receivable.Receivable
	SourceEventID          uuid.UUID
	InvoiceNumber          string
	CustomerIdentification string
	CustomerLegalName      string
	SaleConditionCode      string
}

// CreateReceivableFromInvoice crea la cuenta por cobrar de una factura emitida: open, saldo = total.
type CreateReceivableFromInvoice struct {
	newID func() uuid.UUID
}

func NewCreateReceivableFromInvoice() *CreateReceivableFromInvoice {
	return &CreateReceivableFromInvoice{newID: uuid.New}
}

func (uc *CreateReceivableFromInvoice) Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error {
	cmd, ok := ev.Body.(InvoiceIssued)
	if !ok {
		return fmt.Errorf("%w: %s no trae un InvoiceIssued", ErrInvalidEvent, ev.Ref.Type)
	}
	org := ev.Ref.OrganizationID

	// Otro eventId con la misma factura (Billing la republicó): si es la misma, no cambia nada (invariante 4).
	existing, found, err := tx.ReceivableStore().FindByInvoice(ctx, org, cmd.InvoiceID)
	if err != nil {
		return err
	}
	if found {
		if existing.CustomerID == cmd.CustomerID && existing.Currency == cmd.Currency && existing.Original.Equal(cmd.Total) {
			return nil
		}
		return reject(ErrInvoiceConflict)
	}

	r, err := receivable.New(uc.newID(), receivable.Invoice{
		ID: cmd.InvoiceID, CustomerID: cmd.CustomerID, Currency: cmd.Currency, Original: cmd.Total,
		IssuedOn: cmd.IssueDate, DueOn: cmd.DueDate,
	})
	if err != nil {
		// Un total 0 (factura exonerada por completo) no se puede registrar: original_amount > 0 en la base.
		// TODO(contratos): ¿una factura de total 0 debe llegar a Receivables? Hoy va a dead letter.
		return reject(err)
	}
	if err := tx.ReceivableStore().Create(ctx, org, NewReceivable{
		Receivable: r, SourceEventID: ev.Ref.ID, InvoiceNumber: cmd.InvoiceNumber,
		CustomerIdentification: cmd.CustomerIdentification, CustomerLegalName: cmd.CustomerLegalName,
		SaleConditionCode: cmd.SaleConditionCode,
	}); err != nil {
		return err
	}
	return tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: org,
		ActorType:      ActorService,
		Action:         "receivable.created",
		EntityType:     "receivable",
		EntityID:       r.ID(),
		CorrelationID:  ev.Ref.CorrelationID,
		Payload: map[string]any{
			"sourceEventId": ev.Ref.ID,
			"after": map[string]any{
				"invoiceId":     cmd.InvoiceID,
				"invoiceNumber": cmd.InvoiceNumber,
				"customerId":    cmd.CustomerID,
				"currency":      cmd.Currency,
				"original":      r.Invoice().Original.String(),
				"balance":       r.Balance().String(),
				"status":        r.Status(),
				"issuedOn":      cmd.IssueDate.String(),
				"dueOn":         cmd.DueDate.String(),
			},
		},
	})
}
