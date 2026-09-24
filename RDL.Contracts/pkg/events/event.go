package events

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Event es lo que tienen en común todos los DTOs del catálogo: su sobre, su schema y el agregado que los origina.
type Event interface {
	Header() Envelope
	// Spec es el contrato fijo del evento: tipo, versión, productor y schema.
	Spec() Spec
	// Aggregate es la entidad del productor que cambió (aggregate_type, aggregate_id del outbox).
	Aggregate() (kind string, id uuid.UUID)
}

type Spec struct {
	Type       string
	Version    int
	Producer   Service
	SchemaFile string
}

func (e Envelope) Header() Envelope { return e }

// ToOutboxRow serializa el evento para integration.outbox_messages. Verifica que el sobre coincida con el Spec
// del evento, pero no valida el payload: llame a Validator.Validate antes de escribir.
func ToOutboxRow(e Event) (OutboxRow, error) {
	h, s := e.Header(), e.Spec()
	if h.EventType != s.Type || h.Version != s.Version || h.SourceService != s.Producer {
		return OutboxRow{}, fmt.Errorf("%w: el sobre (%s v%d desde %s) no corresponde a %s v%d desde %s",
			ErrInvalid, h.EventType, h.Version, h.SourceService, s.Type, s.Version, s.Producer)
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return OutboxRow{}, err
	}
	kind, id := e.Aggregate()
	return OutboxRow{
		ID: h.EventID, SourceService: h.SourceService, OrganizationID: h.OrganizationID,
		EventType: h.EventType, EventVersion: h.Version, AggregateType: kind, AggregateID: id,
		CorrelationID: h.CorrelationID, OccurredAt: h.OccurredAt, Payload: payload,
	}, nil
}

// NewHeader arma el sobre de un evento a partir de su Spec, con un eventId nuevo.
func NewHeader(s Spec, organizationID, correlationID uuid.UUID, occurredAt Instant) Envelope {
	return NewEnvelope(s.Type, s.Version, s.Producer, organizationID, correlationID, occurredAt)
}

// Catalog son los eventos v1 del catálogo 6.2 de la arquitectura. No se agregan eventos que no estén ahí (D8).
var Catalog = []Spec{
	InvoiceIssuedSpec,
	InvoiceCancelledSpec,
	CreditNoteIssuedSpec,
	DebitNoteIssuedSpec,
	ElectronicDocumentAcceptedSpec,
	ElectronicDocumentRejectedSpec,
	PaymentReceivedSpec,
	ReceivableSettledSpec,
}
