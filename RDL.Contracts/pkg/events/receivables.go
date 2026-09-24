package events

import (
	"encoding/json"

	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events/money"
)

var (
	PaymentReceivedSpec   = Spec{Type: "PaymentReceived", Version: 1, Producer: ServiceReceivables, SchemaFile: "schemas/events/payment-received.v1.json"}
	ReceivableSettledSpec = Spec{Type: "ReceivableSettled", Version: 1, Producer: ServiceReceivables, SchemaFile: "schemas/events/receivable-settled.v1.json"}
)

// PaymentApplication: parte de un pago aplicada a una cuenta por cobrar.
type PaymentApplication struct {
	ApplicationID   uuid.UUID    `json:"applicationId"`
	ReceivableID    uuid.UUID    `json:"receivableId"`
	SourceInvoiceID uuid.UUID    `json:"sourceInvoiceId"`
	Amount          money.Amount `json:"amount"`
}

// PaymentReceivedV1: schemas/events/payment-received.v1.json.
type PaymentReceivedV1 struct {
	Envelope

	PaymentID         uuid.UUID            `json:"paymentId"`
	CustomerID        uuid.UUID            `json:"customerId"`
	ReceivedOn        Date                 `json:"receivedOn"`
	Amount            money.Amount         `json:"amount"`
	Currency          money.Currency       `json:"currency"`
	ExchangeRate      money.ExchangeRate   `json:"exchangeRate"`
	PaymentMethodCode string               `json:"paymentMethodCode"`
	Reference         string               `json:"reference,omitempty"`
	ReceivedByUserID  uuid.UUID            `json:"receivedByUserId"`
	Applications      []PaymentApplication `json:"applications"`
}

func (PaymentReceivedV1) Spec() Spec                       { return PaymentReceivedSpec }
func (e PaymentReceivedV1) Aggregate() (string, uuid.UUID) { return "payment", e.PaymentID }

// MarshalJSON escribe `applications: []` aunque el slice sea nil: el schema exige un arreglo.
func (e PaymentReceivedV1) MarshalJSON() ([]byte, error) {
	type plain PaymentReceivedV1
	if e.Applications == nil {
		e.Applications = []PaymentApplication{}
	}
	return json.Marshal(plain(e))
}

// ReceivableSettledV1: schemas/events/receivable-settled.v1.json.
type ReceivableSettledV1 struct {
	Envelope

	ReceivableID    uuid.UUID      `json:"receivableId"`
	SourceInvoiceID uuid.UUID      `json:"sourceInvoiceId"`
	CustomerID      uuid.UUID      `json:"customerId"`
	DocumentNumber  string         `json:"documentNumber"`
	Currency        money.Currency `json:"currency"`
	OriginalAmount  money.Amount   `json:"originalAmount"`
	SettledAt       Instant        `json:"settledAt"`
}

func (ReceivableSettledV1) Spec() Spec                       { return ReceivableSettledSpec }
func (e ReceivableSettledV1) Aggregate() (string, uuid.UUID) { return "receivable", e.ReceivableID }
