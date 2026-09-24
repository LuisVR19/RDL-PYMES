// Package events tiene los DTOs Go de los eventos del catálogo (arquitectura 6.2) y un validador contra sus JSON
// Schema. Los structs se escriben a mano; un test garantiza que coinciden campo por campo con los schemas.
//
// Regla de versionado: un cambio incompatible no toca InvoiceIssuedV1, agrega InvoiceIssuedV2.
package events

import (
	"errors"

	"github.com/google/uuid"
)

// ErrInvalid envuelve todo valor que no cumple el contrato.
var ErrInvalid = errors.New("evento inválido")

// Envelope es el sobre común (schemas/events/envelope.v1.json). Va embebido en cada evento: en JSON sus campos
// quedan en el mismo objeto que los del cuerpo.
type Envelope struct {
	EventID        uuid.UUID `json:"eventId"`
	EventType      string    `json:"eventType"`
	Version        int       `json:"version"`
	OccurredAt     Instant   `json:"occurredAt"`
	CorrelationID  uuid.UUID `json:"correlationId"`
	OrganizationID uuid.UUID `json:"organizationId"`
	SourceService  Service   `json:"sourceService"`
}

// NewEnvelope arma el sobre con un eventId nuevo. occurredAt es el momento del hecho de negocio (por ejemplo, la
// emisión), no el de la publicación.
func NewEnvelope(eventType string, version int, source Service, organizationID, correlationID uuid.UUID, occurredAt Instant) Envelope {
	return Envelope{
		EventID: uuid.New(), EventType: eventType, Version: version, OccurredAt: occurredAt,
		CorrelationID: correlationID, OrganizationID: organizationID, SourceService: source,
	}
}

// OutboxRow es cómo se guarda un evento en integration.outbox_messages (convenciones §10).
type OutboxRow struct {
	ID             uuid.UUID
	SourceService  Service
	OrganizationID uuid.UUID
	EventType      string
	EventVersion   int
	AggregateType  string
	AggregateID    uuid.UUID
	CorrelationID  uuid.UUID
	OccurredAt     Instant
	// Payload es el evento completo serializado (sobre + cuerpo).
	Payload []byte
}
