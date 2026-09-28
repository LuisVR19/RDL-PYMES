package invoice

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// CancelInput son los datos de la anulación. At y By los fija el caso de uso (reloj y token), nunca el cliente HTTP.
type CancelInput struct {
	Reason string
	At     time.Time
	By     uuid.UUID
}

// Cancel es la transición issued → cancelled de una factura (state-machines/invoice.yaml), con motivo obligatorio
// (CHECK invoices_cancelled_ck). Número, snapshots, líneas y montos no cambian: la base solo deja tocar el estado y
// los campos de la anulación (invoices_guard). Cancelled es final.
//
// Qué hace Hacienda con eso no es de Billing: fiscal lo decide al consumir InvoiceCancelled (ESTADO §3 del repo de
// contratos: un comprobante aceptado se corrige con nota de crédito código 01).
func (inv Invoice) Cancel(in CancelInput) (Invoice, error) {
	if inv.DocumentType != TypeInvoice {
		return Invoice{}, ErrNoteNotCancellable
	}
	if inv.Status != StatusIssued {
		return Invoice{}, ErrNotIssued
	}
	reason := strings.TrimSpace(in.Reason)
	switch {
	case reason == "":
		return Invoice{}, FieldError{Field: "reason", Message: "es obligatorio"}
	case utf8.RuneCountInString(reason) > MaxReasonLength:
		return Invoice{}, FieldError{Field: "reason", Message: fmt.Sprintf("admite hasta %d caracteres", MaxReasonLength)}
	}
	next := inv
	at := in.At.UTC()
	by := in.By
	next.Status = StatusCancelled
	next.CancellationReason = reason
	next.CancelledAt = &at
	next.CancelledByUserID = &by
	return next, nil
}
