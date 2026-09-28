package events

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
)

func issuedNote(t *testing.T, dt invoice.DocumentType) invoice.Invoice {
	t.Helper()
	note := issuedInvoice(t, []invoice.LineInput{
		{Quantity: money.MustQuantityForTest("1"), UnitPrice: money.MustAmountForTest("1500"), Discount: money.Zero,
			Taxes: []invoice.TaxInput{{TypeCode: "01", Rate: money.MustPercentageForTest("13")}}},
	}, nil)
	ref := uuid.New()
	note.DocumentType, note.Number, note.ReferencedInvoiceID, note.ReferenceReason = dt, "NC-00000001", &ref, "Devolución parcial"
	return note
}

func payloadOf(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNotesValidateAgainstSchema(t *testing.T) {
	cid := uuid.New()
	for _, tc := range []struct {
		dt   invoice.DocumentType
		want string
	}{{invoice.TypeCreditNote, "CreditNoteIssued"}, {invoice.TypeDebitNote, "DebitNoteIssued"}} {
		t.Run(tc.want, func(t *testing.T) {
			note := issuedNote(t, tc.dt)
			e, err := NoteIssued(note, "FAC-00000042", "2026-09-24", cid)
			if err != nil {
				t.Fatal(err)
			}
			row, err := OutboxRow(e)
			if err != nil {
				t.Fatalf("no valida contra el schema: %v", err)
			}
			p := payloadOf(t, row.Payload)
			if row.EventType != tc.want || p["referencedInvoiceNumber"] != "FAC-00000042" ||
				p["referenceReason"] != "Devolución parcial" || p["documentId"] != note.ID.String() ||
				p["issueDate"] != "2026-09-24" || p["correlationId"] != cid.String() {
				t.Fatalf("%s = %s", row.EventType, row.Payload)
			}
			// Solo la de débito lleva vencimiento (el schema de la de crédito no lo admite).
			if _, has := p["dueDate"]; has != (tc.dt == invoice.TypeDebitNote) {
				t.Fatalf("dueDate en %s: %v", tc.want, p["dueDate"])
			}
		})
	}
}

func TestNoteIssuedRejectsWhatIsNotAnIssuedNote(t *testing.T) {
	fac := issuedNote(t, invoice.TypeInvoice)
	fac.ReferencedInvoiceID = nil
	if _, err := NoteIssued(fac, "X", "2026-09-24", uuid.New()); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("factura: err = %v", err)
	}
	draft := issuedNote(t, invoice.TypeCreditNote)
	draft.Status = invoice.StatusDraft
	if _, err := NoteIssued(draft, "X", "2026-09-24", uuid.New()); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("borrador: err = %v", err)
	}
	// Un motivo vacío no cumple el schema: la emisión fallaría entera en lugar de mandar un evento inválido.
	empty := issuedNote(t, invoice.TypeCreditNote)
	empty.ReferenceReason = ""
	e, err := NoteIssued(empty, "FAC-00000042", "2026-09-24", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OutboxRow(e); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("motivo vacío: err = %v", err)
	}
}

func TestInvoiceCancelledValidatesAgainstSchema(t *testing.T) {
	inv := issuedInvoice(t, []invoice.LineInput{
		{Quantity: money.MustQuantityForTest("2"), UnitPrice: money.MustAmountForTest("1000"), Discount: money.Zero},
	}, nil)
	at, by := time.Date(2026, 9, 25, 16, 0, 0, 0, time.UTC), uuid.New()
	inv.Status, inv.CancellationReason, inv.CancelledAt, inv.CancelledByUserID = invoice.StatusCancelled, "Cliente equivocado", &at, &by
	e, err := InvoiceCancelled(inv, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	row, err := OutboxRow(e)
	if err != nil {
		t.Fatalf("no valida contra el schema: %v", err)
	}
	p := payloadOf(t, row.Payload)
	if row.EventType != "InvoiceCancelled" || p["invoiceNumber"] != inv.Number || p["reason"] != "Cliente equivocado" ||
		p["cancelledAt"] != "2026-09-25T16:00:00Z" || p["occurredAt"] != "2026-09-25T16:00:00Z" ||
		p["total"] != inv.Totals.Total.String() || p["currency"] != "USD" {
		t.Fatalf("payload = %s", row.Payload)
	}
	inv.Status = invoice.StatusIssued
	if _, err := InvoiceCancelled(inv, uuid.New()); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("no anulada: err = %v", err)
	}
}
