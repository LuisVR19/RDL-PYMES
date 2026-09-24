package events

import (
	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events/money"
)

var (
	InvoiceCancelledSpec = Spec{Type: "InvoiceCancelled", Version: 1, Producer: ServiceBilling, SchemaFile: "schemas/events/invoice-cancelled.v1.json"}
	CreditNoteIssuedSpec = Spec{Type: "CreditNoteIssued", Version: 1, Producer: ServiceBilling, SchemaFile: "schemas/events/credit-note-issued.v1.json"}
	DebitNoteIssuedSpec  = Spec{Type: "DebitNoteIssued", Version: 1, Producer: ServiceBilling, SchemaFile: "schemas/events/debit-note-issued.v1.json"}
)

// InvoiceCancelledV1: schemas/events/invoice-cancelled.v1.json. Anulación con motivo; sin líneas.
type InvoiceCancelledV1 struct {
	Envelope

	InvoiceID         uuid.UUID      `json:"invoiceId"`
	InvoiceNumber     string         `json:"invoiceNumber"`
	CancelledAt       Instant        `json:"cancelledAt"`
	CancelledByUserID uuid.UUID      `json:"cancelledByUserId"`
	Reason            string         `json:"reason"`
	Currency          money.Currency `json:"currency"`
	Total             money.Amount   `json:"total"`
}

func (InvoiceCancelledV1) Spec() Spec                       { return InvoiceCancelledSpec }
func (e InvoiceCancelledV1) Aggregate() (string, uuid.UUID) { return "invoice", e.InvoiceID }

// IssuedNote: schemas/events/parts/issued-note.v1.json. Cuerpo común de las notas de crédito y débito.
type IssuedNote struct {
	DocumentID              uuid.UUID          `json:"documentId"`
	BranchID                *uuid.UUID         `json:"branchId,omitempty"`
	DocumentNumber          string             `json:"documentNumber"`
	ReferencedInvoiceID     uuid.UUID          `json:"referencedInvoiceId"`
	ReferencedInvoiceNumber string             `json:"referencedInvoiceNumber"`
	ReferenceReason         string             `json:"referenceReason"`
	IssuedAt                Instant            `json:"issuedAt"`
	IssueDate               Date               `json:"issueDate"`
	IssuedByUserID          uuid.UUID          `json:"issuedByUserId"`
	SaleConditionCode       string             `json:"saleConditionCode"`
	Currency                money.Currency     `json:"currency"`
	ExchangeRate            money.ExchangeRate `json:"exchangeRate"`
	CustomerSnapshot        CustomerSnapshot   `json:"customerSnapshot"`
	Lines                   []DocumentLine     `json:"lines"`
	Totals
	Notes string `json:"notes,omitempty"`
}

func (n IssuedNote) DocumentLines() []DocumentLine { return n.Lines }
func (n IssuedNote) DocumentTotals() Totals        { return n.Totals }

// CreditNoteIssuedV1: schemas/events/credit-note-issued.v1.json. Disminuye el saldo de la factura referenciada.
type CreditNoteIssuedV1 struct {
	Envelope
	IssuedNote
}

func (CreditNoteIssuedV1) Spec() Spec                       { return CreditNoteIssuedSpec }
func (e CreditNoteIssuedV1) Aggregate() (string, uuid.UUID) { return "credit_note", e.DocumentID }

// DebitNoteIssuedV1: schemas/events/debit-note-issued.v1.json. Aumenta el saldo; lleva vencimiento propio.
type DebitNoteIssuedV1 struct {
	Envelope
	IssuedNote
	DueDate Date `json:"dueDate"`
}

func (DebitNoteIssuedV1) Spec() Spec                       { return DebitNoteIssuedSpec }
func (e DebitNoteIssuedV1) Aggregate() (string, uuid.UUID) { return "debit_note", e.DocumentID }
