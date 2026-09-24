// Package events traduce el dominio de Billing a los eventos del repo de contratos (pkg/events) y los valida contra
// su JSON Schema antes de que lleguen al outbox. Un evento que no valida no se escribe: la emisión falla entera.
package events

import (
	"errors"
	"fmt"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
)

// ErrInvalidEvent: el evento armado no cumple el contrato. Es un error de programación de Billing, nunca del usuario.
var ErrInvalidEvent = errors.New("events: el evento no cumple su schema")

// InvoiceIssued arma InvoiceIssued v1 de un documento recién emitido (docs/eventos/invoice-issued.md del contrato).
// issueDate es la fecha de negocio de IssuedAt en la zona de la organización. Los montos se copian tal como quedaron
// en la factura: el evento no recalcula.
func InvoiceIssued(inv invoice.Invoice, issueDate string, correlationID uuid.UUID) (cevents.InvoiceIssuedV1, error) {
	if inv.Status != invoice.StatusIssued || inv.IssuedAt == nil || inv.IssuedByUserID == nil {
		return cevents.InvoiceIssuedV1{}, fmt.Errorf("%w: el documento %s no está emitido", ErrInvalidEvent, inv.ID)
	}
	issue, err := cevents.ParseDate(issueDate)
	if err != nil {
		return cevents.InvoiceIssuedV1{}, err
	}
	due, err := cevents.ParseDate(inv.DueDate)
	if err != nil {
		return cevents.InvoiceIssuedV1{}, err
	}
	issuedAt := cevents.NewInstant(*inv.IssuedAt)
	e := cevents.InvoiceIssuedV1{
		// occurredAt = issuedAt: el hecho de negocio es la emisión (reglas del productor, punto 2).
		Envelope:          cevents.NewHeader(cevents.InvoiceIssuedSpec, inv.OrganizationID, correlationID, issuedAt),
		InvoiceID:         inv.ID,
		BranchID:          inv.BranchID,
		InvoiceNumber:     inv.Number,
		IssuedAt:          issuedAt,
		IssueDate:         issue,
		DueDate:           due,
		CreditTermDays:    inv.CreditTermDays,
		IssuedByUserID:    *inv.IssuedByUserID,
		SaleConditionCode: inv.SaleConditionCode,
		Currency:          inv.Currency,
		ExchangeRate:      inv.ExchangeRate,
		CustomerSnapshot: cevents.CustomerSnapshot{
			CustomerID: inv.CustomerID,
			Identification: cevents.Identification{
				TypeCode: inv.Customer.IdentificationTypeCode, Number: inv.Customer.IdentificationNumber,
			},
			LegalName: inv.Customer.LegalName, Email: inv.Customer.Email, Phone: inv.Customer.Phone,
			Address: inv.Customer.Address,
		},
		Lines: make([]cevents.DocumentLine, 0, len(inv.Lines)),
		Totals: cevents.Totals{
			Subtotal: inv.Totals.Subtotal, Discount: inv.Totals.Discount, Tax: inv.Totals.Tax,
			Exoneration: inv.Totals.Exoneration, Total: inv.Totals.Total,
		},
		Notes: inv.Notes,
	}
	for _, l := range inv.Lines {
		dl := cevents.DocumentLine{
			LineNumber: l.Number, ProductID: l.ProductID, ProductCode: l.ProductCode, CabysCode: l.CabysCode,
			Description: l.Description, UnitOfMeasureCode: l.UnitOfMeasureCode, IsService: l.IsService,
			Quantity: l.Quantity, UnitPrice: l.UnitPrice, GrossAmount: l.Gross(), Discount: l.Discount, DiscountReason: l.DiscountReason,
			Subtotal: l.Subtotal, Tax: l.Tax, Total: l.Total, Taxes: make([]cevents.LineTax, 0, len(l.Taxes)),
		}
		for _, t := range l.Taxes {
			lt := cevents.LineTax{
				TaxTypeCode: t.TypeCode, TaxRateCode: t.RateCode, Rate: t.Rate, TaxableBase: t.TaxableBase, Amount: t.Amount,
			}
			if x := t.Exoneration; x != nil {
				lt.Exoneration = &cevents.Exoneration{
					DocumentTypeCode: x.DocumentTypeCode, DocumentNumber: x.DocumentNumber, Institution: x.Institution,
					IssuedAt: cevents.NewInstant(x.IssuedAt), ExoneratedRate: x.ExoneratedRate, Amount: x.Amount,
				}
			}
			dl.Taxes = append(dl.Taxes, lt)
		}
		e.Lines = append(e.Lines, dl)
	}
	return e, nil
}

// OutboxRow serializa el evento y lo valida contra su JSON Schema (el embebido en el módulo de contratos, la misma
// versión que el DTO). Solo una fila válida llega a integration.outbox_messages.
func OutboxRow(e cevents.Event) (cevents.OutboxRow, error) {
	row, err := cevents.ToOutboxRow(e)
	if err != nil {
		return cevents.OutboxRow{}, fmt.Errorf("%w: %w", ErrInvalidEvent, err)
	}
	v, err := cevents.DefaultValidator()
	if err != nil {
		return cevents.OutboxRow{}, fmt.Errorf("compilando schemas de eventos: %w", err)
	}
	if err := v.Validate(e.Spec().SchemaFile, row.Payload); err != nil {
		return cevents.OutboxRow{}, fmt.Errorf("%w: %w", ErrInvalidEvent, err)
	}
	return row, nil
}
