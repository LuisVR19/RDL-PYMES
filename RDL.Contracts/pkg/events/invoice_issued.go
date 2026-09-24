package events

import (
	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events/money"
)

const (
	InvoiceIssuedType     = "InvoiceIssued"
	InvoiceIssuedSchemaV1 = "schemas/events/invoice-issued.v1.json"
)

var InvoiceIssuedSpec = Spec{Type: InvoiceIssuedType, Version: 1, Producer: ServiceBilling, SchemaFile: InvoiceIssuedSchemaV1}

// InvoiceIssuedV1: schemas/events/invoice-issued.v1.json. Lo produce Billing; lo consumen fiscal y Receivables.
// Fórmulas de los montos en docs/eventos/invoice-issued.md.
type InvoiceIssuedV1 struct {
	Envelope

	InvoiceID         uuid.UUID          `json:"invoiceId"`
	BranchID          *uuid.UUID         `json:"branchId,omitempty"`
	InvoiceNumber     string             `json:"invoiceNumber"`
	IssuedAt          Instant            `json:"issuedAt"`
	IssueDate         Date               `json:"issueDate"`
	DueDate           Date               `json:"dueDate"`
	CreditTermDays    *int               `json:"creditTermDays,omitempty"`
	IssuedByUserID    uuid.UUID          `json:"issuedByUserId"`
	SaleConditionCode string             `json:"saleConditionCode"`
	Currency          money.Currency     `json:"currency"`
	ExchangeRate      money.ExchangeRate `json:"exchangeRate"`
	CustomerSnapshot  CustomerSnapshot   `json:"customerSnapshot"`
	Lines             []DocumentLine     `json:"lines"`
	PaymentMethods    []PaymentMethod    `json:"paymentMethods,omitempty"`
	Totals
	Notes string `json:"notes,omitempty"`
}

func (InvoiceIssuedV1) Spec() Spec                       { return InvoiceIssuedSpec }
func (e InvoiceIssuedV1) Aggregate() (string, uuid.UUID) { return "invoice", e.InvoiceID }
func (e InvoiceIssuedV1) DocumentLines() []DocumentLine  { return e.Lines }
func (e InvoiceIssuedV1) DocumentTotals() Totals         { return e.Totals }

// OutboxRow es un atajo de ToOutboxRow(e).
func (e InvoiceIssuedV1) OutboxRow() (OutboxRow, error) { return ToOutboxRow(e) }

// Totals son los totales de una factura o nota (fórmulas en docs/eventos/invoice-issued.md).
type Totals struct {
	Subtotal    money.Amount `json:"subtotal"`
	Discount    money.Amount `json:"discount"`
	Tax         money.Amount `json:"tax"`
	Exoneration money.Amount `json:"exoneration"`
	Total       money.Amount `json:"total"`
}
