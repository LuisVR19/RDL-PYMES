package events

import (
	"encoding/json"

	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events/money"
)

// Identification: schemas/common/identification.json. TypeCode: TODO(fiscal), catálogo de tipos.
type Identification struct {
	TypeCode string `json:"typeCode"`
	Number   string `json:"number"`
}

// CustomerSnapshot: schemas/common/customer-snapshot.json. Datos del cliente copiados al emitir.
type CustomerSnapshot struct {
	CustomerID     uuid.UUID      `json:"customerId"`
	Identification Identification `json:"identification"`
	LegalName      string         `json:"legalName"`
	Email          string         `json:"email,omitempty"`
	Phone          string         `json:"phone,omitempty"`
	Address        string         `json:"address,omitempty"`
}

// Exoneration: schemas/events/parts/exoneration.v1.json. Todos los campos o ninguno (por eso es un puntero en LineTax).
type Exoneration struct {
	DocumentTypeCode string           `json:"documentTypeCode"`
	DocumentNumber   string           `json:"documentNumber"`
	Institution      string           `json:"institution"`
	IssuedAt         Instant          `json:"issuedAt"`
	Percentage       money.Percentage `json:"percentage"`
	Amount           money.Amount     `json:"amount"`
}

// LineTax: schemas/events/parts/line-tax.v1.json.
type LineTax struct {
	TaxTypeCode string           `json:"taxTypeCode"`
	TaxRateCode string           `json:"taxRateCode,omitempty"`
	Rate        money.Percentage `json:"rate"`
	TaxableBase money.Amount     `json:"taxableBase"`
	Amount      money.Amount     `json:"amount"`
	Exoneration *Exoneration     `json:"exoneration,omitempty"`
}

// DocumentLine: schemas/events/parts/document-line.v1.json. Snapshot del producto al emitir.
type DocumentLine struct {
	LineNumber        int            `json:"lineNumber"`
	ProductID         *uuid.UUID     `json:"productId,omitempty"`
	ProductCode       string         `json:"productCode,omitempty"`
	CabysCode         string         `json:"cabysCode"`
	Description       string         `json:"description"`
	UnitOfMeasureCode string         `json:"unitOfMeasureCode"`
	IsService         bool           `json:"isService"`
	Quantity          money.Quantity `json:"quantity"`
	UnitPrice         money.Amount   `json:"unitPrice"`
	Discount          money.Amount   `json:"discount"`
	DiscountReason    string         `json:"discountReason,omitempty"`
	Subtotal          money.Amount   `json:"subtotal"`
	Tax               money.Amount   `json:"tax"`
	Total             money.Amount   `json:"total"`
	Taxes             []LineTax      `json:"taxes"`
}

// MarshalJSON escribe `taxes: []` aunque el slice sea nil: el schema exige un arreglo, no null.
func (l DocumentLine) MarshalJSON() ([]byte, error) {
	type plain DocumentLine
	if l.Taxes == nil {
		l.Taxes = []LineTax{}
	}
	return json.Marshal(plain(l))
}

// PaymentMethod: medio de pago declarado al emitir. Amount es opcional.
type PaymentMethod struct {
	PaymentMethodCode string        `json:"paymentMethodCode"`
	Amount            *money.Amount `json:"amount,omitempty"`
}
