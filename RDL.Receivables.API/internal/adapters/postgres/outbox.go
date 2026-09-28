package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	contracts "bitbucket.org/rdl/contracts/pkg/events"
	"bitbucket.org/rdl/contracts/pkg/events/money"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
)

// outbox serializa con los DTOs de RDL.Contracts y valida contra el schema antes de insertar: un evento que no
// cumple el contrato hace rollback del cambio entero en lugar de publicarse mal.
type outbox struct {
	q         *db.Queries
	validator *contracts.Validator
}

var errNoValidator = errors.New("postgres: outbox sin validador de contratos")

func (o outbox) PaymentReceived(ctx context.Context, e app.PaymentReceivedEvent) error {
	amount, err := money.ParseAmount(e.Amount.String())
	if err != nil {
		return err
	}
	rate, err := money.ParseExchangeRate(e.ExchangeRate.String())
	if err != nil {
		return err
	}
	currency, err := money.ParseCurrency(e.Currency)
	if err != nil {
		return err
	}
	ev := contracts.PaymentReceivedV1{
		Envelope:  header(contracts.PaymentReceivedSpec, e.EventMeta),
		PaymentID: e.PaymentID, CustomerID: e.CustomerID, ReceivedOn: date(e.ReceivedOn), Amount: amount,
		Currency: currency, ExchangeRate: rate, PaymentMethodCode: e.PaymentMethodCode, Reference: e.Reference,
		ReceivedByUserID: e.ReceivedByUserID, Applications: []contracts.PaymentApplication{},
	}
	for _, a := range e.Applications {
		amt, err := money.ParseAmount(a.Amount.String())
		if err != nil {
			return err
		}
		ev.Applications = append(ev.Applications, contracts.PaymentApplication{
			ApplicationID: a.ApplicationID, ReceivableID: a.ReceivableID, SourceInvoiceID: a.SourceInvoiceID, Amount: amt,
		})
	}
	return o.write(ctx, ev)
}

func (o outbox) ReceivableSettled(ctx context.Context, e app.ReceivableSettledEvent) error {
	original, err := money.ParseAmount(e.OriginalAmount.String())
	if err != nil {
		return err
	}
	currency, err := money.ParseCurrency(e.Currency)
	if err != nil {
		return err
	}
	return o.write(ctx, contracts.ReceivableSettledV1{
		Envelope:     header(contracts.ReceivableSettledSpec, e.EventMeta),
		ReceivableID: e.ReceivableID, SourceInvoiceID: e.SourceInvoiceID, CustomerID: e.CustomerID,
		DocumentNumber: e.DocumentNumber, Currency: currency, OriginalAmount: original,
		SettledAt: contracts.NewInstant(e.SettledAt),
	})
}

func (o outbox) write(ctx context.Context, e contracts.Event) error {
	if o.validator == nil {
		return errNoValidator
	}
	row, err := contracts.ToOutboxRow(e)
	if err != nil {
		return err
	}
	if err := o.validator.Validate(e.Spec().SchemaFile, row.Payload); err != nil {
		return fmt.Errorf("%s no cumple su schema: %w", row.EventType, err)
	}
	err = o.q.InsertOutboxMessage(ctx, db.InsertOutboxMessageParams{
		ID: row.ID, OrganizationID: row.OrganizationID, EventType: row.EventType,
		EventVersion:  int32(row.EventVersion), // #nosec G115 -- versión de catálogo (1, 2...)
		AggregateType: row.AggregateType, AggregateID: row.AggregateID, CorrelationID: row.CorrelationID,
		Payload: string(row.Payload), OccurredAt: row.OccurredAt.Time(),
	})
	if err != nil {
		return fmt.Errorf("escribiendo %s en el outbox: %w", row.EventType, err)
	}
	return nil
}

// header: organización del cambio, correlationId del request o del evento de origen y un eventId nuevo.
func header(s contracts.Spec, m app.EventMeta) contracts.Envelope {
	cid := m.CorrelationID
	if cid == uuid.Nil {
		cid = uuid.New()
	}
	return contracts.NewHeader(s, m.OrganizationID, cid, contracts.NewInstant(m.OccurredAt))
}

func date(d civil.Date) contracts.Date {
	return contracts.Date{Year: d.Year, Month: d.Month, Day: d.Day}
}
