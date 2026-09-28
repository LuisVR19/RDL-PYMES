// Package events traduce los mensajes del bus a comandos de app: valida cada payload contra el JSON Schema de
// RDL.Contracts (pkg/events.Validator) y lo decodifica con los DTOs del mismo módulo. También tiene el runner del
// consumidor y el puerto de la fuente de mensajes (el transporte real es TODO(P2)).
package events

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
)

// translator decodifica un tipo de evento ya validado y lo convierte en el comando de app.
type translator func(payload []byte) (any, error)

// Decoder implementa app.EventDecoder para los eventos que consume Receivables.
type Decoder struct {
	validator   *contracts.Validator
	translators map[key]entry
}

type key struct {
	eventType string
	version   int
}

type entry struct {
	spec      contracts.Spec
	translate translator
}

// NewDecoder registra los eventos consumidos. Uno nuevo se agrega aquí y con su handler en wiring, sin tocar los
// existentes.
func NewDecoder(v *contracts.Validator) *Decoder {
	d := &Decoder{validator: v, translators: map[key]entry{}}
	d.register(contracts.InvoiceIssuedSpec, invoiceIssued)
	d.register(contracts.CreditNoteIssuedSpec, creditNoteIssued)
	d.register(contracts.DebitNoteIssuedSpec, debitNoteIssued)
	d.register(contracts.InvoiceCancelledSpec, invoiceCancelled)
	return d
}

func (d *Decoder) register(s contracts.Spec, t translator) {
	d.translators[key{s.Type, s.Version}] = entry{spec: s, translate: t}
}

// rawEnvelope lee el sobre sin validar, para ubicar el schema y para Peek.
type rawEnvelope struct {
	EventID        string `json:"eventId"`
	EventType      string `json:"eventType"`
	Version        int    `json:"version"`
	OccurredAt     string `json:"occurredAt"`
	CorrelationID  string `json:"correlationId"`
	OrganizationID string `json:"organizationId"`
}

func (d *Decoder) Peek(payload []byte) app.EventRef {
	var raw rawEnvelope
	_ = json.Unmarshal(payload, &raw)
	ref := app.EventRef{Type: raw.EventType, Version: raw.Version}
	ref.ID, _ = uuid.Parse(raw.EventID)
	ref.CorrelationID, _ = uuid.Parse(raw.CorrelationID)
	ref.OrganizationID, _ = uuid.Parse(raw.OrganizationID)
	if ref.Type == "" {
		ref.Type = "unknown"
	}
	return ref
}

func (d *Decoder) Decode(payload []byte) (app.IncomingEvent, error) {
	var raw rawEnvelope
	if err := json.Unmarshal(payload, &raw); err != nil {
		return app.IncomingEvent{}, fmt.Errorf("%w: JSON mal formado", app.ErrInvalidEvent)
	}
	e, ok := d.translators[key{raw.EventType, raw.Version}]
	if !ok {
		return app.IncomingEvent{}, fmt.Errorf("%w: %s v%d", app.ErrUnsupportedEvent, raw.EventType, raw.Version)
	}
	if err := d.validator.Validate(e.spec.SchemaFile, payload); err != nil {
		return app.IncomingEvent{}, fmt.Errorf("%w: %w", app.ErrInvalidEvent, err)
	}
	var env contracts.Envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return app.IncomingEvent{}, fmt.Errorf("%w: sobre: %w", app.ErrInvalidEvent, err)
	}
	if env.SourceService != e.spec.Producer {
		return app.IncomingEvent{}, fmt.Errorf("%w: %s lo produce %s, no %s", app.ErrInvalidEvent, env.EventType,
			e.spec.Producer, env.SourceService)
	}
	body, err := e.translate(payload)
	if err != nil {
		return app.IncomingEvent{}, fmt.Errorf("%w: %w", app.ErrInvalidEvent, err)
	}
	return app.IncomingEvent{
		Ref: app.EventRef{
			ID: env.EventID, Type: env.EventType, Version: env.Version, OrganizationID: env.OrganizationID,
			CorrelationID: env.CorrelationID, OccurredAt: env.OccurredAt.Time(),
		},
		Body: body,
	}, nil
}

func invoiceIssued(payload []byte) (any, error) {
	var e contracts.InvoiceIssuedV1
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(e.Total.String())
	if err != nil {
		return nil, fmt.Errorf("total %q: %w", e.Total, err)
	}
	return app.InvoiceIssued{
		InvoiceID:              e.InvoiceID,
		InvoiceNumber:          e.InvoiceNumber,
		CustomerID:             e.CustomerSnapshot.CustomerID,
		CustomerIdentification: e.CustomerSnapshot.Identification.Number,
		CustomerLegalName:      e.CustomerSnapshot.LegalName,
		SaleConditionCode:      e.SaleConditionCode,
		Currency:               e.Currency.String(),
		Total:                  total,
		IssueDate:              date(e.IssueDate),
		DueDate:                date(e.DueDate),
	}, nil
}

func creditNoteIssued(payload []byte) (any, error) {
	var e contracts.CreditNoteIssuedV1
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(e.Total.String())
	if err != nil {
		return nil, fmt.Errorf("total %q: %w", e.Total, err)
	}
	return app.CreditNoteIssued{
		DocumentID: e.DocumentID, DocumentNumber: e.DocumentNumber, InvoiceID: e.ReferencedInvoiceID,
		Reason: e.ReferenceReason, Currency: e.Currency.String(), Total: total,
	}, nil
}

func debitNoteIssued(payload []byte) (any, error) {
	var e contracts.DebitNoteIssuedV1
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(e.Total.String())
	if err != nil {
		return nil, fmt.Errorf("total %q: %w", e.Total, err)
	}
	return app.DebitNoteIssued{
		DocumentID: e.DocumentID, DocumentNumber: e.DocumentNumber, InvoiceID: e.ReferencedInvoiceID,
		Reason: e.ReferenceReason, Currency: e.Currency.String(), Total: total, DueDate: date(e.DueDate),
	}, nil
}

func invoiceCancelled(payload []byte) (any, error) {
	var e contracts.InvoiceCancelledV1
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	total, err := decimal.NewFromString(e.Total.String())
	if err != nil {
		return nil, fmt.Errorf("total %q: %w", e.Total, err)
	}
	return app.InvoiceCancelled{
		InvoiceID: e.InvoiceID, InvoiceNumber: e.InvoiceNumber, Reason: e.Reason, Currency: e.Currency.String(), Total: total,
	}, nil
}

func date(d contracts.Date) civil.Date { return civil.Date{Year: d.Year, Month: d.Month, Day: d.Day} }
