package document

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SourceType es el tipo del documento comercial de Billing (CHECK electronic_documents_source_type_ck).
type SourceType string

const (
	SourceInvoice    SourceType = "invoice"
	SourceCreditNote SourceType = "credit_note"
	SourceDebitNote  SourceType = "debit_note"
)

// Códigos de tipo de comprobante de V1 (decisión 1). Anexo 1, Nota 3 inciso c), pág. 66.
const (
	TypeInvoice    = "01" // Factura electrónica
	TypeDebitNote  = "02" // Nota de débito electrónica
	TypeCreditNote = "03" // Nota de crédito electrónica
)

// TypeCode es el tipo de comprobante de la Nota 3 que corresponde al documento comercial.
func (s SourceType) TypeCode() string {
	switch s {
	case SourceCreditNote:
		return TypeCreditNote
	case SourceDebitNote:
		return TypeDebitNote
	default:
		return TypeInvoice
	}
}

// Límites de la estructura del Anexo 1 que se pueden verificar al recibir el evento. El resto (catálogos, formatos,
// cuadre de totales) se valida antes de firmar, en el incremento 7 (informe 0003 §6).
const (
	MaxLines          = 1000 // LineaDetalle {1,1000}, Anexo 1 pág. 26
	MaxPaymentMethods = 4    // MedioPago {1,4}, Anexo 1 pág. 55
)

// ErrStructure: el documento comercial no cabe en la estructura del comprobante. Permanente: no se reintenta.
var ErrStructure = errors.New("el documento no cumple la estructura del comprobante 4.4")

// Party es el snapshot de emisor o receptor. Del emisor sale del perfil fiscal; del receptor, del customerSnapshot.
type Party struct {
	IdentificationTypeCode string
	IdentificationNumber   string
	LegalName              string
	TradeName              string
	EconomicActivityCode   string
	Email                  string
	Phone                  string
	ProvinceCode           string
	CantonCode             string
	DistrictCode           string
	Neighborhood           string
	Address                string
}

// Exoneration de un impuesto de una línea. Rate son PUNTOS de tarifa (TarifaExonerada, Anexo 1 págs. 45–46).
type Exoneration struct {
	DocumentTypeCode string
	DocumentNumber   string
	Institution      string
	IssuedAt         time.Time
	Rate             decimal.Decimal
	Amount           decimal.Decimal
}

type LineTax struct {
	TaxTypeCode string
	TaxRateCode string
	Rate        decimal.Decimal
	TaxableBase decimal.Decimal
	Amount      decimal.Decimal
	Exoneration *Exoneration
}

type Line struct {
	Number            int
	CabysCode         string
	ProductCode       string
	Description       string
	UnitOfMeasureCode string
	Quantity          decimal.Decimal
	UnitPrice         decimal.Decimal
	GrossAmount       decimal.Decimal
	Discount          decimal.Decimal
	DiscountReason    string
	Subtotal          decimal.Decimal
	Tax               decimal.Decimal
	Total             decimal.Decimal
	Taxes             []LineTax
}

type PaymentMethod struct {
	Code   string
	Amount *decimal.Decimal
}

// Document es el comprobante recién recibido: el snapshot del documento comercial y del emisor, en processing y
// todavía sin terminal, consecutivo, clave ni XML (incrementos 6 a 8).
type Document struct {
	ID                   uuid.UUID
	OrganizationID       uuid.UUID
	SourceType           SourceType
	SourceDocumentID     uuid.UUID
	SourceDocumentNumber string
	SourceEventID        uuid.UUID
	IssuedAt             time.Time
	Issuer               Party
	Receiver             Party
	SaleConditionCode    string
	CreditTermDays       *int
	CurrencyCode         string
	ExchangeRate         decimal.Decimal
	Subtotal             decimal.Decimal
	Discount             decimal.Decimal
	Tax                  decimal.Decimal
	Exoneration          decimal.Decimal
	Total                decimal.Decimal
	Lines                []Line
	PaymentMethods       []PaymentMethod
	Status               Status
}

// New arma el documento en processing y verifica los límites de estructura conocidos al recibirlo.
func New(d Document) (Document, error) {
	if len(d.Lines) == 0 || len(d.Lines) > MaxLines {
		return Document{}, fmt.Errorf("%w: %d líneas (de 1 a %d, Anexo 1 pág. 26)", ErrStructure, len(d.Lines), MaxLines)
	}
	if len(d.PaymentMethods) > MaxPaymentMethods {
		return Document{}, fmt.Errorf("%w: %d medios de pago (máximo %d, Anexo 1 pág. 55)", ErrStructure,
			len(d.PaymentMethods), MaxPaymentMethods)
	}
	seen := map[int]bool{}
	for _, l := range d.Lines {
		if seen[l.Number] {
			return Document{}, fmt.Errorf("%w: número de línea %d repetido", ErrStructure, l.Number)
		}
		seen[l.Number] = true
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.Status = StatusProcessing
	return d, nil
}

// TypeCode es el tipo de comprobante (Nota 3) del documento.
func (d Document) TypeCode() string { return d.SourceType.TypeCode() }
