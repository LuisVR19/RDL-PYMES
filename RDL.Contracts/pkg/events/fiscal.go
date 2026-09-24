package events

import "github.com/google/uuid"

var (
	ElectronicDocumentAcceptedSpec = Spec{Type: "ElectronicDocumentAccepted", Version: 1, Producer: ServiceFiscal, SchemaFile: "schemas/events/electronic-document-accepted.v1.json"}
	ElectronicDocumentRejectedSpec = Spec{Type: "ElectronicDocumentRejected", Version: 1, Producer: ServiceFiscal, SchemaFile: "schemas/events/electronic-document-rejected.v1.json"}
)

// DocumentType: schemas/common/document-type.json.
type DocumentType string

const (
	DocumentInvoice    DocumentType = "invoice"
	DocumentCreditNote DocumentType = "credit_note"
	DocumentDebitNote  DocumentType = "debit_note"
)

// ElectronicDocumentRef: schemas/events/parts/electronic-document.v1.json.
type ElectronicDocumentRef struct {
	ElectronicDocumentID  uuid.UUID    `json:"electronicDocumentId"`
	SourceType            DocumentType `json:"sourceType"`
	SourceDocumentID      uuid.UUID    `json:"sourceDocumentId"`
	SourceDocumentNumber  string       `json:"sourceDocumentNumber"`
	NumericKey            string       `json:"numericKey"`
	ConsecutiveNumber     string       `json:"consecutiveNumber"`
	HaciendaStatusMessage string       `json:"haciendaStatusMessage,omitempty"`
}

// ElectronicDocumentAcceptedV1: schemas/events/electronic-document-accepted.v1.json.
type ElectronicDocumentAcceptedV1 struct {
	Envelope
	ElectronicDocumentRef
	AcceptedAt Instant `json:"acceptedAt"`
}

func (ElectronicDocumentAcceptedV1) Spec() Spec { return ElectronicDocumentAcceptedSpec }
func (e ElectronicDocumentAcceptedV1) Aggregate() (string, uuid.UUID) {
	return "electronic_document", e.ElectronicDocumentID
}

// ElectronicDocumentRejectedV1: schemas/events/electronic-document-rejected.v1.json.
type ElectronicDocumentRejectedV1 struct {
	Envelope
	ElectronicDocumentRef
	RejectedAt      Instant `json:"rejectedAt"`
	RejectionReason string  `json:"rejectionReason"`
}

func (ElectronicDocumentRejectedV1) Spec() Spec { return ElectronicDocumentRejectedSpec }
func (e ElectronicDocumentRejectedV1) Aggregate() (string, uuid.UUID) {
	return "electronic_document", e.ElectronicDocumentID
}
