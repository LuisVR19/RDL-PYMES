package events

import (
	"fmt"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
)

// NoteIssued arma CreditNoteIssued v1 o DebitNoteIssued v1 (schemas/events/parts/issued-note.v1.json) de una nota
// recién emitida. referencedNumber es el número visible de la factura que corrige; issueDate, la fecha de negocio de
// IssuedAt en la zona de la organización. La de débito lleva su vencimiento (el DueDate de la nota).
func NoteIssued(note invoice.Invoice, referencedNumber, issueDate string, correlationID uuid.UUID) (cevents.Event, error) {
	if !note.DocumentType.IsNote() || note.ReferencedInvoiceID == nil {
		return nil, fmt.Errorf("%w: el documento %s no es una nota con referencia", ErrInvalidEvent, note.ID)
	}
	if note.Status != invoice.StatusIssued || note.IssuedAt == nil || note.IssuedByUserID == nil {
		return nil, fmt.Errorf("%w: la nota %s no está emitida", ErrInvalidEvent, note.ID)
	}
	issue, err := cevents.ParseDate(issueDate)
	if err != nil {
		return nil, err
	}
	issuedAt := cevents.NewInstant(*note.IssuedAt)
	body := cevents.IssuedNote{
		DocumentID:              note.ID,
		BranchID:                note.BranchID,
		DocumentNumber:          note.Number,
		ReferencedInvoiceID:     *note.ReferencedInvoiceID,
		ReferencedInvoiceNumber: referencedNumber,
		ReferenceReason:         note.ReferenceReason,
		IssuedAt:                issuedAt,
		IssueDate:               issue,
		IssuedByUserID:          *note.IssuedByUserID,
		SaleConditionCode:       note.SaleConditionCode,
		Currency:                note.Currency,
		ExchangeRate:            note.ExchangeRate,
		CustomerSnapshot:        customerSnapshot(note),
		Lines:                   documentLines(note),
		Totals:                  totals(note),
		Notes:                   note.Notes,
	}
	// occurredAt = issuedAt: el hecho de negocio es la emisión (reglas del productor, punto 2).
	if note.DocumentType == invoice.TypeCreditNote {
		return cevents.CreditNoteIssuedV1{
			Envelope:   cevents.NewHeader(cevents.CreditNoteIssuedSpec, note.OrganizationID, correlationID, issuedAt),
			IssuedNote: body,
		}, nil
	}
	due, err := cevents.ParseDate(note.DueDate)
	if err != nil {
		return nil, err
	}
	return cevents.DebitNoteIssuedV1{
		Envelope:   cevents.NewHeader(cevents.DebitNoteIssuedSpec, note.OrganizationID, correlationID, issuedAt),
		IssuedNote: body,
		DueDate:    due,
	}, nil
}

// InvoiceCancelled arma InvoiceCancelled v1 de una factura recién anulada. Sin líneas (D13): el consumidor ya las
// recibió en InvoiceIssued; el total y la moneda van para que verifique contra lo que registró.
func InvoiceCancelled(inv invoice.Invoice, correlationID uuid.UUID) (cevents.InvoiceCancelledV1, error) {
	if inv.Status != invoice.StatusCancelled || inv.CancelledAt == nil || inv.CancelledByUserID == nil {
		return cevents.InvoiceCancelledV1{}, fmt.Errorf("%w: el documento %s no está anulado", ErrInvalidEvent, inv.ID)
	}
	at := cevents.NewInstant(*inv.CancelledAt)
	return cevents.InvoiceCancelledV1{
		// occurredAt = cancelledAt: el hecho de negocio es la anulación.
		Envelope:          cevents.NewHeader(cevents.InvoiceCancelledSpec, inv.OrganizationID, correlationID, at),
		InvoiceID:         inv.ID,
		InvoiceNumber:     inv.Number,
		CancelledAt:       at,
		CancelledByUserID: *inv.CancelledByUserID,
		Reason:            inv.CancellationReason,
		Currency:          inv.Currency,
		Total:             inv.Totals.Total,
	}, nil
}
