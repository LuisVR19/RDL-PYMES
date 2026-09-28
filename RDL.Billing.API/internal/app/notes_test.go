package app

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
)

// issuedInvoice emite una factura del escenario y devuelve el documento emitido.
func issuedInvoice(t *testing.T, s scenario) invoice.Invoice {
	t.Helper()
	res, err := newIssuer(s).Execute(ctx, s.tn, uuid.NewString(), draftWithLine(t, s).ID)
	if err != nil {
		t.Fatal(err)
	}
	return res.Invoice
}

func (s scenario) noteHeader(dt invoice.DocumentType, ref uuid.UUID) HeaderInput {
	h := s.header()
	h.DocumentType, h.ReferencedInvoiceID, h.ReferenceReason = dt, &ref, "Devolución de un servicio"
	return h
}

func createNote(t *testing.T, s scenario, h HeaderInput) invoice.Invoice {
	t.Helper()
	res, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, uuid.NewString(), h, []LineRequest{s.line(s.crcProduct, "1")})
	if err != nil {
		t.Fatal(err)
	}
	return res.Invoice
}

func TestCreateNoteChecksItsInvoice(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	note := createNote(t, s, s.noteHeader(invoice.TypeCreditNote, fac.ID))
	if note.Status != invoice.StatusDraft || *note.ReferencedInvoiceID != fac.ID || note.ReferenceReason != "Devolución de un servicio" {
		t.Fatalf("nota = %+v", note)
	}

	other, err := NewCreateCustomer(s.f).Execute(ctx, s.tn, "c3", newCustomerInput("3"))
	if err != nil {
		t.Fatal(err)
	}
	draft := draftWithLine(t, s)
	create := NewCreateInvoiceDraft(s.f)
	for name, tc := range map[string]struct {
		mut  func(*HeaderInput)
		want func(error) bool
	}{
		"inexistente o de otra organización": {func(h *HeaderInput) { id := uuid.New(); h.ReferencedInvoiceID = &id },
			func(err error) bool { return errors.Is(err, ErrInvalidReference) }},
		"un borrador": {func(h *HeaderInput) { h.ReferencedInvoiceID = &draft.ID },
			func(err error) bool { return errors.Is(err, ErrInvalidReference) }},
		"otra nota": {func(h *HeaderInput) { h.ReferencedInvoiceID = &note.ID },
			func(err error) bool { return errors.Is(err, ErrInvalidReference) }},
		"otro cliente": {func(h *HeaderInput) { h.CustomerID = other.Customer.ID },
			func(err error) bool { return errors.Is(err, ErrInvalidReference) }},
		"otra moneda": {func(h *HeaderInput) {
			h.Currency = money.MustCurrencyForTest("USD")
			rate := money.MustExchangeRateForTest("515.5")
			h.ExchangeRate = &rate
		}, func(err error) bool { return fieldOf(err, "currency") }},
		"sin motivo": {func(h *HeaderInput) { h.ReferenceReason = " " },
			func(err error) bool { return fieldOf(err, "referenceReason") }},
	} {
		t.Run(name, func(t *testing.T) {
			h := s.noteHeader(invoice.TypeCreditNote, fac.ID)
			tc.mut(&h)
			if _, err := create.Execute(ctx, s.tn, uuid.NewString(), h, nil); !tc.want(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestUpdateNoteKeepsItsInvoice(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	note := createNote(t, s, s.noteHeader(invoice.TypeCreditNote, fac.ID))
	reason := "Precio mal cobrado"
	got, err := NewUpdateInvoiceDraft(s.f).Execute(ctx, s.tn, note.ID, invoice.HeaderPatch{ReferenceReason: &reason}, nil)
	if err != nil || got.ReferenceReason != reason {
		t.Fatalf("motivo = %q, err = %v", got.ReferenceReason, err)
	}
	other, _ := NewCreateCustomer(s.f).Execute(ctx, s.tn, "c3", newCustomerInput("3"))
	if _, err := NewUpdateInvoiceDraft(s.f).Execute(ctx, s.tn, note.ID,
		invoice.HeaderPatch{CustomerID: &other.Customer.ID}, nil); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("cambiar el cliente de la nota: err = %v", err)
	}
}

func TestIssueCreditNote(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	note := createNote(t, s, s.noteHeader(invoice.TypeCreditNote, fac.ID))
	res, err := newIssuer(s).Execute(ctx, s.tn, "issue-note", note.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Numeración propia del tipo: la primera nota es 00000001 aunque ya haya facturas.
	if res.Invoice.Status != invoice.StatusIssued || res.Invoice.Number != "00000001" {
		t.Fatalf("nota emitida = %+v", res.Invoice)
	}
	last := s.f.state.outbox[len(s.f.state.outbox)-1]
	if last.Event != "CreditNoteIssued" || last.ReferencedNumber != fac.Number || last.Invoice.ID != note.ID {
		t.Fatalf("outbox = %+v", last)
	}
	audit := s.f.state.audit[len(s.f.state.audit)-1]
	if audit.After.(map[string]any)["documentType"] != "credit_note" {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestIssueDebitNoteHasItsOwnDueDate(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	h := s.noteHeader(invoice.TypeDebitNote, fac.ID)
	days := 15
	h.CreditTermDays = &days
	note := createNote(t, s, h)
	res, err := newIssuer(s).Execute(ctx, s.tn, "issue-debit", note.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Fecha de emisión en Costa Rica: 2026-09-24 (issueClock) + 15 días.
	if res.Invoice.DueDate != "2026-10-09" {
		t.Fatalf("vencimiento = %s", res.Invoice.DueDate)
	}
	if last := s.f.state.outbox[len(s.f.state.outbox)-1]; last.Event != "DebitNoteIssued" {
		t.Fatalf("outbox = %+v", last)
	}
}

func TestIssueNoteOnCancelledInvoiceIsConflict(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	note := createNote(t, s, s.noteHeader(invoice.TypeCreditNote, fac.ID))
	if _, err := NewCancelInvoice(s.f).Execute(ctx, tenant(s.org, "admin"), "cancel-1", fac.ID, "Cliente equivocado"); err != nil {
		t.Fatal(err)
	}
	if _, err := newIssuer(s).Execute(ctx, s.tn, "issue-note", note.ID); !errors.Is(err, invoice.ErrNotIssued) {
		t.Fatalf("err = %v, se esperaba 409 invoice-not-issued", err)
	}
	// Y ya no se le crean notas nuevas.
	if _, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, uuid.NewString(),
		s.noteHeader(invoice.TypeDebitNote, fac.ID), nil); !errors.Is(err, invoice.ErrNotIssued) {
		t.Fatalf("crear nota: err = %v", err)
	}
}

func TestCancelInvoice(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	uc := NewCancelInvoice(s.f)
	owner := tenant(s.org, "owner")
	at := time.Date(2026, 9, 25, 16, 0, 0, 999, time.UTC)
	uc.now = func() time.Time { return at }

	res, err := uc.Execute(ctx, owner, "cancel-1", fac.ID, "  Se facturó al cliente equivocado ")
	if err != nil {
		t.Fatal(err)
	}
	inv := res.Invoice
	if res.Replayed || inv.Status != invoice.StatusCancelled || inv.CancellationReason != "Se facturó al cliente equivocado" ||
		!inv.CancelledAt.Equal(at.Truncate(time.Microsecond)) || *inv.CancelledByUserID != owner.UserID() {
		t.Fatalf("anulada = %+v", inv)
	}
	h := s.f.state.history[fac.ID]
	if last := h[len(h)-1]; last.From != invoice.StatusIssued || last.To != invoice.StatusCancelled ||
		last.Reason != "Se facturó al cliente equivocado" {
		t.Fatalf("historial = %+v", h)
	}
	audit := s.f.state.audit[len(s.f.state.audit)-1]
	if audit.Action != "invoice.cancelled" || audit.Reason != "Se facturó al cliente equivocado" {
		t.Fatalf("audit = %+v", audit)
	}
	events := len(s.f.state.outbox)
	if last := s.f.state.outbox[events-1]; last.Event != "InvoiceCancelled" || last.Invoice.ID != fac.ID {
		t.Fatalf("outbox = %+v", last)
	}

	again, err := uc.Execute(ctx, owner, "cancel-1", fac.ID, "  Se facturó al cliente equivocado ")
	if err != nil || !again.Replayed || len(s.f.state.outbox) != events {
		t.Fatalf("reintento: %+v err=%v", again, err)
	}
	if _, err := uc.Execute(ctx, owner, "cancel-2", fac.ID, "otra vez"); !errors.Is(err, invoice.ErrNotIssued) {
		t.Fatalf("anular dos veces: err = %v", err)
	}
}

func TestCancelRules(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	uc := NewCancelInvoice(s.f)
	owner := tenant(s.org, "owner")
	if _, err := uc.Execute(ctx, tenant(s.org, "biller"), "k1", fac.ID, "motivo"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("facturador: err = %v", err)
	}
	if _, err := uc.Execute(ctx, owner, "k2", draftWithLine(t, s).ID, "motivo"); !errors.Is(err, invoice.ErrNotIssued) {
		t.Fatalf("borrador: err = %v", err)
	}
	note := createNote(t, s, s.noteHeader(invoice.TypeCreditNote, fac.ID))
	if _, err := newIssuer(s).Execute(ctx, s.tn, "issue-note", note.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, owner, "k3", note.ID, "motivo"); !errors.Is(err, invoice.ErrNoteNotCancellable) {
		t.Fatalf("nota: err = %v", err)
	}
	if _, err := uc.Execute(ctx, owner, "k4", fac.ID, ""); !fieldOf(err, "reason") {
		t.Fatalf("sin motivo: err = %v", err)
	}
	// Otra organización: 404, no 409.
	if _, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "k5", fac.ID, "motivo"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ajena: err = %v", err)
	}
}

func TestCancelRollsBackIfTheEventFails(t *testing.T) {
	s := newScenario(t)
	fac := issuedInvoice(t, s)
	s.f.outboxErr = errors.New("schema")
	if _, err := NewCancelInvoice(s.f).Execute(ctx, tenant(s.org, "owner"), "k", fac.ID, "motivo"); err == nil {
		t.Fatal("se esperaba error")
	}
	s.f.outboxErr = nil
	got, err := NewGetInvoice(s.f).Execute(ctx, s.tn, fac.ID)
	if err != nil || got.Status != invoice.StatusIssued {
		t.Fatalf("la anulación sin evento quedó guardada: %+v %v", got.Status, err)
	}
}

// fieldOf dice si el error de validación (posiblemente varios unidos) incluye el campo.
func fieldOf(err error, field string) bool {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, e := range joined.Unwrap() {
			if fieldOf(e, field) {
				return true
			}
		}
		return false
	}
	var fe invoice.FieldError
	return errors.As(err, &fe) && fe.Field == field
}
