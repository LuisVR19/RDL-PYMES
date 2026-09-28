package invoice

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/money"
)

// Status son los estados de state-machines/invoice.yaml del contrato (= CHECK invoices_status_ck).
type Status string

const (
	StatusDraft     Status = "draft"
	StatusIssued    Status = "issued"
	StatusCancelled Status = "cancelled"
)

// DocumentType: factura y notas comparten tabla (billing.invoices.document_type).
type DocumentType string

const (
	TypeInvoice    DocumentType = "invoice"
	TypeCreditNote DocumentType = "credit_note"
	TypeDebitNote  DocumentType = "debit_note"
)

// IsNote: las notas de crédito y débito corrigen una factura emitida y llevan su referencia y motivo
// (CHECK invoices_reference_ck).
func (t DocumentType) IsNote() bool { return t == TypeCreditNote || t == TypeDebitNote }

func (t DocumentType) valid() bool { return t == TypeInvoice || t.IsNote() }

// MaxReasonLength: motivo de referencia de una nota y motivo de anulación (issued-note.v1, invoice-cancelled.v1).
const MaxReasonLength = 500

var (
	// ErrNotDraft: la operación exige un borrador (409 invoice-not-draft). Lo emitido no se edita (arquitectura 7.4).
	ErrNotDraft = errors.New("invoice: el documento no es un borrador")
	// ErrNotIssued: anular, o emitir una nota sobre, un documento que no está emitido (409 invoice-not-issued).
	ErrNotIssued = errors.New("invoice: el documento no está emitido")
	// ErrNoteNotCancellable: solo una factura se anula (state-machines/invoice.yaml). TODO(fiscal): el contrato no
	// define si una nota se puede anular ni tiene evento para eso.
	ErrNoteNotCancellable = errors.New("invoice: una nota de crédito o débito no se anula")
	// ErrInvalid agrupa los errores de validación; FieldError indica el campo con su nombre JSON.
	ErrInvalid = errors.New("invoice: datos inválidos")
)

type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

var fiscalCodePattern = regexp.MustCompile(`^[0-9A-Za-z._-]{1,20}$`)

// MaxLines acota un documento: evita que un borrador crezca sin límite en una sola petición.
const MaxLines = 1000

// Header son los datos del encabezado que el usuario edita en borrador. En una nota, ReferencedInvoiceID es la
// factura que corrige (fija desde que se crea) y ReferenceReason su motivo; en una factura los dos van vacíos.
type Header struct {
	DocumentType        DocumentType
	CustomerID          uuid.UUID
	BranchID            *uuid.UUID
	ReferencedInvoiceID *uuid.UUID
	ReferenceReason     string
	SaleConditionCode   string
	CreditTermDays      *int
	Currency            money.Currency
	ExchangeRate        money.ExchangeRate
	Notes               string
}

// CustomerSnapshot se copia al emitir (arquitectura 4.4). Vacío en borrador.
type CustomerSnapshot struct {
	IdentificationTypeCode string
	IdentificationNumber   string
	LegalName              string
	Email                  string
	Phone                  string
	Address                string
}

// Invoice es el agregado: encabezado, líneas y totales. Sus transiciones son métodos que protegen las invariantes;
// nadie cambia Status a mano.
type Invoice struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Header
	Number                string
	Status                Status
	Customer              CustomerSnapshot
	IssuedAt              *time.Time
	DueDate               string // YYYY-MM-DD en la zona de la organización; vacío en borrador
	Lines                 []Line
	Totals                Totals
	RequiresCorrection    bool
	FiscalRejectionReason string
	// Anulación (CHECK invoices_cancelled_ck): solo en status cancelled.
	CancellationReason string
	CancelledAt        *time.Time
	CancelledByUserID  *uuid.UUID
	CreatedByUserID    uuid.UUID
	IssuedByUserID     *uuid.UUID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Line es una línea con su snapshot del producto y sus montos ya calculados.
type Line struct {
	Number            int
	ProductID         *uuid.UUID
	ProductCode       string
	CabysCode         string
	Description       string
	UnitOfMeasureCode string
	IsService         bool
	Quantity          money.Quantity
	UnitPrice         money.Amount
	Discount          money.Amount
	DiscountReason    string
	Subtotal          money.Amount
	Tax               money.Amount // neto
	Total             money.Amount
	Taxes             []LineTax
}

type LineTax struct {
	TypeCode    string
	RateCode    string
	Rate        money.Percentage
	TaxableBase money.Amount
	Amount      money.Amount
	// Exoneration llega con F4 (exoneraciones reales); el cálculo ya la soporta.
	Exoneration *Exoneration
}

type Exoneration struct {
	DocumentTypeCode string
	DocumentNumber   string
	Institution      string
	IssuedAt         time.Time
	ExoneratedRate   money.TaxRate // tarifa exonerada en puntos, no % del impuesto (ADR 0007 de contratos)
	Amount           money.Amount
}

// Gross es el MontoTotal de la línea (cantidad × precio, redondeado): subtotal + descuento, exacto porque
// subtotal = MontoTotal − descuento. La base no lo guarda: se deriva.
func (l Line) Gross() money.Amount {
	a, err := money.ToAmount(money.Dec(l.Subtotal).Add(money.Dec(l.Discount)))
	if err != nil {
		panic(fmt.Sprintf("invoice: línea %d con montos inconsistentes: %v", l.Number, err))
	}
	return money.CanonicalAmount(a)
}

// LineDraft es lo que el caso de uso arma para una línea: el snapshot del producto (ya validado contra la organización)
// más lo que el usuario decide. Los montos los calcula ReplaceLines.
type LineDraft struct {
	ProductID         *uuid.UUID
	ProductCode       string
	CabysCode         string
	Description       string
	UnitOfMeasureCode string
	IsService         bool
	Quantity          money.Quantity
	UnitPrice         money.Amount
	Discount          money.Amount
	DiscountReason    string
	Taxes             []TaxDraft
}

type TaxDraft struct {
	TypeCode string
	RateCode string
	Rate     money.Percentage // del catálogo fiscal.tax_rates, nunca del cliente
}

// NewDraft crea un borrador sin líneas. Las líneas se agregan con ReplaceLines.
func NewDraft(organizationID, createdBy uuid.UUID, h Header) (Invoice, error) {
	h = h.normalized()
	if err := h.validate(); err != nil {
		return Invoice{}, err
	}
	inv := Invoice{
		OrganizationID: organizationID, Header: h, Status: StatusDraft, CreatedByUserID: createdBy, Lines: []Line{},
	}
	inv.Totals = zeroTotals()
	return inv, nil
}

// HeaderPatch: nil = no cambia. El tipo de documento y la factura referenciada no se cambian (una nota sobre otra
// factura es otro documento): si vienen, deben ser los mismos. El motivo de la nota sí se corrige en borrador.
type HeaderPatch struct {
	DocumentType        *DocumentType
	ReferencedInvoiceID *uuid.UUID
	ReferenceReason     *string
	CustomerID          *uuid.UUID
	BranchID            **uuid.UUID // *nil = quitar la sucursal
	SaleConditionCode   *string
	CreditTermDays      **int // *nil = sin plazo
	Currency            *money.Currency
	ExchangeRate        *money.ExchangeRate
	Notes               *string
}

// ApplyHeader edita el encabezado de un borrador.
func (inv Invoice) ApplyHeader(p HeaderPatch) (Invoice, error) {
	if inv.Status != StatusDraft {
		return Invoice{}, ErrNotDraft
	}
	var fixed []error
	if p.DocumentType != nil && *p.DocumentType != inv.DocumentType {
		fixed = append(fixed, FieldError{Field: "documentType", Message: "no se puede cambiar"})
	}
	if p.ReferencedInvoiceID != nil && (inv.ReferencedInvoiceID == nil || *p.ReferencedInvoiceID != *inv.ReferencedInvoiceID) {
		fixed = append(fixed, FieldError{Field: "referencedInvoiceId", Message: "no se puede cambiar: cree otra nota"})
	}
	if err := errors.Join(fixed...); err != nil {
		return Invoice{}, err
	}
	next := inv
	if p.ReferenceReason != nil {
		next.ReferenceReason = *p.ReferenceReason
	}
	if p.CustomerID != nil {
		next.CustomerID = *p.CustomerID
	}
	if p.BranchID != nil {
		next.BranchID = *p.BranchID
	}
	if p.SaleConditionCode != nil {
		next.SaleConditionCode = *p.SaleConditionCode
	}
	if p.CreditTermDays != nil {
		next.CreditTermDays = *p.CreditTermDays
	}
	if p.Currency != nil {
		next.Currency = *p.Currency
	}
	if p.ExchangeRate != nil {
		next.ExchangeRate = *p.ExchangeRate
	}
	if p.Notes != nil {
		next.Notes = *p.Notes
	}
	next.Header = next.normalized()
	if err := next.validate(); err != nil {
		return Invoice{}, err
	}
	return next, nil
}

// ReplaceLines reemplaza todas las líneas del borrador y recalcula montos y totales con las funciones puras de
// calculation.go. Es la única forma de cambiar líneas (PUT /v1/invoices/{id}/lines).
func (inv Invoice) ReplaceLines(drafts []LineDraft) (Invoice, error) {
	if inv.Status != StatusDraft {
		return Invoice{}, ErrNotDraft
	}
	if len(drafts) > MaxLines {
		return Invoice{}, FieldError{Field: "lines", Message: fmt.Sprintf("admite hasta %d líneas", MaxLines)}
	}
	next := inv
	next.Lines = make([]Line, 0, len(drafts))
	amounts := make([]LineAmounts, 0, len(drafts))
	var errs []error
	for i, d := range drafts {
		l, a, err := buildLine(i+1, d)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		next.Lines = append(next.Lines, l)
		amounts = append(amounts, a)
	}
	if err := errors.Join(errs...); err != nil {
		return Invoice{}, err
	}
	totals, err := CalculateTotals(amounts)
	if err != nil {
		return Invoice{}, FieldError{Field: "lines", Message: "los totales exceden 13 dígitos enteros"}
	}
	next.Totals = totals
	return next, nil
}

// CheckExchangeRate: el tipo de cambio es hacia la moneda local de la organización; en la moneda local es 1
// (InvoiceIssued v1: "1 si currency es la moneda local").
func (h Header) CheckExchangeRate(local money.Currency) error {
	isOne := money.Dec(h.ExchangeRate).Equal(money.Dec(money.MustExchangeRateForTest("1")))
	if h.Currency == local && !isOne {
		return FieldError{Field: "exchangeRate", Message: "debe ser 1 cuando la moneda es la local (" + local.String() + ")"}
	}
	return nil
}

// CanDiscard: solo un borrador se descarta (se borra la fila; state-machines/invoice.yaml).
func (inv Invoice) CanDiscard() error {
	if inv.Status != StatusDraft {
		return ErrNotDraft
	}
	return nil
}

func buildLine(number int, d LineDraft) (Line, LineAmounts, error) {
	field := func(name string) string { return fmt.Sprintf("lines[%d].%s", number-1, name) }
	reason := strings.TrimSpace(d.DiscountReason)
	var errs []error
	if !d.Discount.IsZero() && reason == "" {
		errs = append(errs, FieldError{Field: field("discountReason"), Message: "es obligatorio si hay descuento"})
	}
	if utf8.RuneCountInString(reason) > 80 {
		errs = append(errs, FieldError{Field: field("discountReason"), Message: "admite hasta 80 caracteres"})
	}
	if d.Discount.IsZero() {
		reason = ""
	}
	if err := errors.Join(errs...); err != nil {
		return Line{}, LineAmounts{}, err
	}

	in := LineInput{Quantity: d.Quantity, UnitPrice: d.UnitPrice, Discount: d.Discount}
	for _, t := range d.Taxes {
		in.Taxes = append(in.Taxes, TaxInput{TypeCode: t.TypeCode, Rate: t.Rate})
	}
	a, err := CalculateLine(in)
	switch {
	case errors.Is(err, ErrDiscountExceedsGross):
		return Line{}, LineAmounts{}, FieldError{Field: field("discount"), Message: "no puede superar cantidad × precio"}
	case errors.Is(err, ErrAmountOutOfRange):
		return Line{}, LineAmounts{}, FieldError{Field: field("quantity"), Message: "el monto de la línea excede 13 dígitos enteros"}
	case err != nil:
		return Line{}, LineAmounts{}, FieldError{Field: field("taxes"), Message: err.Error()}
	}

	l := Line{
		Number: number, ProductID: d.ProductID, ProductCode: d.ProductCode, CabysCode: d.CabysCode,
		Description: d.Description, UnitOfMeasureCode: d.UnitOfMeasureCode, IsService: d.IsService,
		Quantity: d.Quantity, UnitPrice: money.CanonicalAmount(d.UnitPrice), Discount: a.Discount,
		DiscountReason: reason, Subtotal: a.Subtotal, Tax: a.Tax, Total: a.Total,
		Taxes: make([]LineTax, 0, len(d.Taxes)),
	}
	for j, t := range d.Taxes {
		l.Taxes = append(l.Taxes, LineTax{
			TypeCode: t.TypeCode, RateCode: t.RateCode, Rate: t.Rate,
			TaxableBase: a.Taxes[j].TaxableBase, Amount: a.Taxes[j].Amount,
		})
	}
	return l, a, nil
}

func (h Header) normalized() Header {
	h.SaleConditionCode = strings.TrimSpace(h.SaleConditionCode)
	h.Notes = strings.TrimSpace(h.Notes)
	h.ReferenceReason = strings.TrimSpace(h.ReferenceReason)
	return h
}

func (h Header) validate() error {
	var errs []error
	check := func(ok bool, field, msg string) {
		if !ok {
			errs = append(errs, FieldError{Field: field, Message: msg})
		}
	}
	check(h.DocumentType.valid(), "documentType", "debe ser invoice, credit_note o debit_note")
	if h.DocumentType.IsNote() {
		// Requisitos de la transición de las notas (state-machines/invoice.yaml): factura referenciada y motivo.
		// TODO(fiscal): el código de referencia del comprobante según Hacienda; hoy solo el texto.
		check(h.ReferencedInvoiceID != nil, "referencedInvoiceId", "es obligatorio en una nota")
		check(h.ReferenceReason != "", "referenceReason", "es obligatorio en una nota")
		check(utf8.RuneCountInString(h.ReferenceReason) <= MaxReasonLength, "referenceReason",
			fmt.Sprintf("admite hasta %d caracteres", MaxReasonLength))
	} else {
		check(h.ReferencedInvoiceID == nil, "referencedInvoiceId", "solo aplica a notas de crédito y débito")
		check(h.ReferenceReason == "", "referenceReason", "solo aplica a notas de crédito y débito")
	}
	check(h.CustomerID != uuid.Nil, "customerId", "es obligatorio")
	// TODO(fiscal): catálogo fiscal.sale_conditions (vacío); solo formato.
	check(fiscalCodePattern.MatchString(h.SaleConditionCode), "saleConditionCode", "debe ser un código fiscal válido")
	check(h.CreditTermDays == nil || *h.CreditTermDays >= 0 && *h.CreditTermDays <= 3650, "creditTermDays", "debe estar entre 0 y 3650")
	check(h.Currency.String() != "", "currency", "es obligatoria")
	check(h.ExchangeRate.String() != "", "exchangeRate", "es obligatorio")
	check(utf8.RuneCountInString(h.Notes) <= 2000, "notes", "admite hasta 2000 caracteres")
	return errors.Join(errs...)
}

func zeroTotals() Totals {
	z := zero()
	return Totals{Subtotal: z, Discount: z, Tax: z, Exoneration: z, Total: z}
}

func zero() money.Amount { return money.Zero }
