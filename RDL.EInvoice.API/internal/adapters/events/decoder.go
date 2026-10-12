// Package events traduce los mensajes del bus a comandos de app: valida cada payload contra el JSON Schema de
// RDL.Contracts (pkg/events.Validator) y lo decodifica con los DTOs del mismo módulo. También tiene el runner del
// consumidor y el puerto de la fuente de mensajes (el transporte real es TODO(P2)). Origen: RDL.Receivables.API.
package events

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/document"
)

// translator decodifica un tipo de evento ya validado y lo convierte en el comando de app.
type translator func(payload []byte) (any, error)

// Decoder implementa app.EventDecoder para los eventos que consume fiscal.
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

// NewDecoder registra los eventos consumidos. Hoy solo InvoiceIssued (F3); CreditNoteIssued, DebitNoteIssued e
// InvoiceCancelled llegan con el incremento 11 (notas y anulación, decisión 2). Uno nuevo se agrega aquí y con su
// handler en wiring, sin tocar los existentes.
func NewDecoder(v *contracts.Validator) *Decoder {
	d := &Decoder{validator: v, translators: map[key]entry{}}
	d.register(contracts.InvoiceIssuedSpec, invoiceIssued)
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

// invoiceIssued traduce InvoiceIssued v1 al snapshot del comprobante (informe 0003, secciones 1 a 4). Los montos
// pasan de string decimal a decimal exacto sin redondear: el cuadre con la especificación se valida antes de firmar.
func invoiceIssued(payload []byte) (any, error) {
	var e contracts.InvoiceIssuedV1
	if err := json.Unmarshal(payload, &e); err != nil {
		return nil, err
	}
	p := parser{}
	d := document.Document{
		SourceDocumentID:     e.InvoiceID,
		SourceDocumentNumber: e.InvoiceNumber,
		IssuedAt:             e.IssuedAt.Time(),
		Receiver: document.Party{
			IdentificationTypeCode: e.CustomerSnapshot.Identification.TypeCode,
			IdentificationNumber:   e.CustomerSnapshot.Identification.Number,
			LegalName:              e.CustomerSnapshot.LegalName,
			Email:                  e.CustomerSnapshot.Email,
			Phone:                  e.CustomerSnapshot.Phone,
			Address:                e.CustomerSnapshot.Address,
		},
		SaleConditionCode: e.SaleConditionCode,
		CreditTermDays:    e.CreditTermDays,
		CurrencyCode:      e.Currency.String(),
		ExchangeRate:      p.dec("exchangeRate", e.ExchangeRate.String()),
		Subtotal:          p.dec("subtotal", e.Subtotal.String()),
		Discount:          p.dec("discount", e.Discount.String()),
		Tax:               p.dec("tax", e.Tax.String()),
		Exoneration:       p.dec("exoneration", e.Exoneration.String()),
		Total:             p.dec("total", e.Total.String()),
	}
	for i, l := range e.Lines {
		field := fmt.Sprintf("lines[%d].", i)
		line := document.Line{
			Number: l.LineNumber, CabysCode: l.CabysCode, ProductCode: l.ProductCode, Description: l.Description,
			UnitOfMeasureCode: l.UnitOfMeasureCode,
			Quantity:          p.dec(field+"quantity", l.Quantity.String()),
			UnitPrice:         p.dec(field+"unitPrice", l.UnitPrice.String()),
			GrossAmount:       p.dec(field+"grossAmount", l.GrossAmount.String()),
			Discount:          p.dec(field+"discount", l.Discount.String()),
			DiscountReason:    l.DiscountReason,
			Subtotal:          p.dec(field+"subtotal", l.Subtotal.String()),
			Tax:               p.dec(field+"tax", l.Tax.String()),
			Total:             p.dec(field+"total", l.Total.String()),
		}
		for j, t := range l.Taxes {
			tf := fmt.Sprintf("%staxes[%d].", field, j)
			tax := document.LineTax{
				TaxTypeCode: t.TaxTypeCode, TaxRateCode: t.TaxRateCode,
				Rate:        p.dec(tf+"rate", t.Rate.String()),
				TaxableBase: p.dec(tf+"taxableBase", t.TaxableBase.String()),
				Amount:      p.dec(tf+"amount", t.Amount.String()),
			}
			if x := t.Exoneration; x != nil {
				tax.Exoneration = &document.Exoneration{
					DocumentTypeCode: x.DocumentTypeCode, DocumentNumber: x.DocumentNumber, Institution: x.Institution,
					IssuedAt: x.IssuedAt.Time(),
					Rate:     p.dec(tf+"exoneration.exoneratedRate", x.ExoneratedRate.String()),
					Amount:   p.dec(tf+"exoneration.amount", x.Amount.String()),
				}
			}
			line.Taxes = append(line.Taxes, tax)
		}
		d.Lines = append(d.Lines, line)
	}
	for i, pm := range e.PaymentMethods {
		m := document.PaymentMethod{Code: pm.PaymentMethodCode}
		if pm.Amount != nil {
			v := p.dec(fmt.Sprintf("paymentMethods[%d].amount", i), pm.Amount.String())
			m.Amount = &v
		}
		d.PaymentMethods = append(d.PaymentMethods, m)
	}
	if p.err != nil {
		return nil, p.err
	}
	return app.InvoiceIssued{Document: d}, nil
}

// parser junta el primer error de conversión; el schema ya validó el formato, así que no debería ocurrir.
type parser struct{ err error }

func (p *parser) dec(field, s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%s %q: %w", field, s, err)
	}
	return v
}
