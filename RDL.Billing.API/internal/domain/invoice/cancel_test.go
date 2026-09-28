package invoice

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func ptr[T any](v T) *T { return &v }

func noteHeader(t DocumentType) Header {
	h := header()
	h.DocumentType = t
	h.ReferencedInvoiceID = ptr(uuid.New())
	h.ReferenceReason = "  Devolución de mercadería  "
	return h
}

func TestNewDraftNote(t *testing.T) {
	for _, dt := range []DocumentType{TypeCreditNote, TypeDebitNote} {
		inv, err := NewDraft(uuid.New(), uuid.New(), noteHeader(dt))
		if err != nil {
			t.Fatalf("%s: %v", dt, err)
		}
		if inv.ReferenceReason != "Devolución de mercadería" || inv.ReferencedInvoiceID == nil {
			t.Fatalf("%s: referencia = %v %q", dt, inv.ReferencedInvoiceID, inv.ReferenceReason)
		}
	}
}

func TestApplyHeaderNote(t *testing.T) {
	inv, err := NewDraft(uuid.New(), uuid.New(), noteHeader(TypeCreditNote))
	if err != nil {
		t.Fatal(err)
	}
	// El motivo se corrige en borrador; el tipo y la factura, si vienen iguales (cuerpo completo), no molestan.
	same, ref := TypeCreditNote, *inv.ReferencedInvoiceID
	next, err := inv.ApplyHeader(HeaderPatch{DocumentType: &same, ReferencedInvoiceID: &ref, ReferenceReason: ptr("Precio mal cobrado")})
	if err != nil || next.ReferenceReason != "Precio mal cobrado" {
		t.Fatalf("next = %q, err = %v", next.ReferenceReason, err)
	}
	other, debit := uuid.New(), TypeDebitNote
	if _, err := inv.ApplyHeader(HeaderPatch{DocumentType: &debit, ReferencedInvoiceID: &other}); !fieldsOf(err)["documentType"] ||
		!fieldsOf(err)["referencedInvoiceId"] {
		t.Fatalf("err = %v, se esperaba documentType y referencedInvoiceId", err)
	}
	if _, err := inv.ApplyHeader(HeaderPatch{ReferenceReason: ptr("  ")}); !fieldsOf(err)["referenceReason"] {
		t.Fatalf("err = %v, se esperaba referenceReason", err)
	}
	// A una factura no se le agrega una referencia por PATCH.
	fac, _ := NewDraft(uuid.New(), uuid.New(), header())
	if _, err := fac.ApplyHeader(HeaderPatch{ReferencedInvoiceID: &other}); !fieldsOf(err)["referencedInvoiceId"] {
		t.Fatalf("err = %v, se esperaba referencedInvoiceId", err)
	}
}

func issuedInvoice(t *testing.T) Invoice {
	t.Helper()
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	inv, err := inv.ReplaceLines([]LineDraft{draftLine("1", "1000", "0", "", "13")})
	if err != nil {
		t.Fatal(err)
	}
	inv, err = inv.Issue(IssueInput{
		Number: "00000001", IssuedAt: time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC), IssueDate: "2026-09-24",
		Customer: CustomerSnapshot{IdentificationTypeCode: "02", IdentificationNumber: "3101123456", LegalName: "Cliente S.A."},
		IssuedBy: uuid.New(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestCancel(t *testing.T) {
	inv := issuedInvoice(t)
	at, by := time.Date(2026, 9, 25, 15, 0, 0, 0, time.FixedZone("CR", -6*3600)), uuid.New()
	got, err := inv.Cancel(CancelInput{Reason: "  Se facturó al cliente equivocado ", At: at, By: by})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCancelled || got.CancellationReason != "Se facturó al cliente equivocado" ||
		!got.CancelledAt.Equal(at) || got.CancelledAt.Location() != time.UTC || *got.CancelledByUserID != by {
		t.Fatalf("anulada = %+v", got)
	}
	// Lo emitido no cambia.
	if got.Number != inv.Number || got.Totals != inv.Totals || len(got.Lines) != len(inv.Lines) {
		t.Fatal("la anulación cambió datos del documento emitido")
	}
	if _, err := got.Cancel(CancelInput{Reason: "otra vez", At: at, By: by}); !errors.Is(err, ErrNotIssued) {
		t.Fatalf("anular dos veces: err = %v", err)
	}
}

func TestCancelRules(t *testing.T) {
	draft, _ := NewDraft(uuid.New(), uuid.New(), header())
	if _, err := draft.Cancel(CancelInput{Reason: "x"}); !errors.Is(err, ErrNotIssued) {
		t.Fatalf("borrador: err = %v", err)
	}
	note, _ := NewDraft(uuid.New(), uuid.New(), noteHeader(TypeCreditNote))
	note.Status = StatusIssued
	if _, err := note.Cancel(CancelInput{Reason: "x"}); !errors.Is(err, ErrNoteNotCancellable) {
		t.Fatalf("nota: err = %v", err)
	}
	inv := issuedInvoice(t)
	for _, reason := range []string{"", "   ", strings.Repeat("m", 501)} {
		if _, err := inv.Cancel(CancelInput{Reason: reason}); !fieldsOf(err)["reason"] {
			t.Fatalf("motivo %q: err = %v", reason, err)
		}
	}
}
