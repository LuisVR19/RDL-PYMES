package app

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/product"
)

// 2026-09-25 05:30 UTC = 2026-09-24 23:30 en Costa Rica: la fecha de negocio es la del 24.
var issueClock = time.Date(2026, 9, 25, 5, 30, 0, 123456789, time.UTC)

func newIssuer(s scenario) *IssueInvoice {
	uc := NewIssueInvoice(s.f)
	uc.now = func() time.Time { return issueClock }
	return uc
}

func draftWithLine(t *testing.T, s scenario) invoice.Invoice {
	t.Helper()
	res, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, uuid.NewString(), s.header(), []LineRequest{s.line(s.crcProduct, "2")})
	if err != nil {
		t.Fatal(err)
	}
	return res.Invoice
}

func TestIssueHappyPath(t *testing.T) {
	s := newScenario(t)
	draft := draftWithLine(t, s)
	res, err := newIssuer(s).Execute(ctx, s.tn, "issue-1", draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	inv := res.Invoice
	if res.Replayed || inv.Status != invoice.StatusIssued || inv.Number != "00000001" || inv.DueDate != "2026-09-24" ||
		!inv.IssuedAt.Equal(issueClock.Truncate(time.Microsecond)) || *inv.IssuedByUserID != s.tn.UserID() {
		t.Fatalf("emitido = %+v", inv)
	}
	// Snapshot del cliente copiado de la base.
	if inv.Customer.LegalName != "Ana Pérez" || inv.Customer.IdentificationNumber != "1" {
		t.Fatalf("snapshot = %+v", inv.Customer)
	}
	// Historial, audit y outbox en la misma transacción.
	h := s.f.state.history[draft.ID]
	if len(h) != 1 || h[0].From != invoice.StatusDraft || h[0].To != invoice.StatusIssued || *h[0].ChangedByUserID != s.tn.UserID() {
		t.Fatalf("historial = %+v", h)
	}
	last := s.f.state.audit[len(s.f.state.audit)-1]
	if last.Action != "invoice.issued" || last.Before.(map[string]any)["status"] != "draft" || last.After.(map[string]any)["number"] != "00000001" {
		t.Fatalf("audit = %+v", last)
	}
	if len(s.f.state.outbox) != 1 || s.f.state.outbox[0].Invoice.ID != draft.ID || s.f.state.outbox[0].IssueDate != "2026-09-24" {
		t.Fatalf("outbox = %+v", s.f.state.outbox)
	}
	// La secuencia de la organización se creó y avanzó.
	if len(s.f.state.sequences) != 1 || s.f.state.sequences[0].NextNumber != 2 || !s.f.state.sequences[0].Used() {
		t.Fatalf("secuencia = %+v", s.f.state.sequences)
	}
}

func TestIssueSameKeyReturnsSameResult(t *testing.T) {
	s := newScenario(t)
	draft := draftWithLine(t, s)
	uc := newIssuer(s)
	first, err := uc.Execute(ctx, s.tn, "issue-1", draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := uc.Execute(ctx, s.tn, "issue-1", draft.ID)
	if err != nil || !again.Replayed || again.Invoice.Number != first.Invoice.Number {
		t.Fatalf("reintento: %+v err=%v", again, err)
	}
	if len(s.f.state.outbox) != 1 || len(s.f.state.history[draft.ID]) != 1 || s.f.state.sequences[0].NextNumber != 2 {
		t.Fatal("el reintento no emite de nuevo ni consume otro número")
	}
}

func TestIssueTwiceWithOtherKeyIsConflict(t *testing.T) {
	s := newScenario(t)
	draft := draftWithLine(t, s)
	uc := newIssuer(s)
	if _, err := uc.Execute(ctx, s.tn, "issue-1", draft.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, s.tn, "issue-2", draft.ID); !errors.Is(err, invoice.ErrNotDraft) {
		t.Fatalf("err = %v, se esperaba 409", err)
	}
	// La misma clave para otro documento: 422.
	other := draftWithLine(t, s)
	if _, err := uc.Execute(ctx, s.tn, "issue-1", other.ID); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("clave reutilizada: err = %v", err)
	}
}

func TestIssueNumbersAreConsecutive(t *testing.T) {
	s := newScenario(t)
	uc := newIssuer(s)
	if _, err := NewConfigureSequence(s.f).Execute(ctx, tenant(s.org, "admin"), SequenceInput{
		DocumentType: invoice.TypeInvoice, Prefix: "FAC-", NextNumber: 41,
	}); err != nil {
		t.Fatal(err)
	}
	var numbers []string
	for range 3 {
		res, err := uc.Execute(ctx, s.tn, uuid.NewString(), draftWithLine(t, s).ID)
		if err != nil {
			t.Fatal(err)
		}
		numbers = append(numbers, res.Invoice.Number)
	}
	if numbers[0] != "FAC-00000041" || numbers[1] != "FAC-00000042" || numbers[2] != "FAC-00000043" {
		t.Fatalf("números = %v", numbers)
	}
	if _, err := NewConfigureSequence(s.f).Execute(ctx, tenant(s.org, "admin"), SequenceInput{
		DocumentType: invoice.TypeInvoice, Prefix: "X-", NextNumber: 1,
	}); !errors.Is(err, numbering.ErrInUse) {
		t.Fatalf("reconfigurar una secuencia usada: err = %v", err)
	}
}

func TestIssueUsesBranchSequence(t *testing.T) {
	s := newScenario(t)
	branch := s.f.addBranch(s.org, true)
	if _, err := NewConfigureSequence(s.f).Execute(ctx, tenant(s.org, "admin"), SequenceInput{
		DocumentType: invoice.TypeInvoice, BranchID: &branch, Prefix: "S1-", NextNumber: 1,
	}); err != nil {
		t.Fatal(err)
	}
	h := s.header()
	h.BranchID = &branch
	d, err := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "d", h, []LineRequest{s.line(s.crcProduct, "1")})
	if err != nil {
		t.Fatal(err)
	}
	res, err := newIssuer(s).Execute(ctx, s.tn, "i", d.Invoice.ID)
	if err != nil || res.Invoice.Number != "S1-00000001" {
		t.Fatalf("número = %q err=%v", res.Invoice.Number, err)
	}
}

func TestIssueFailingOutboxLeavesNothingIssued(t *testing.T) {
	s := newScenario(t)
	draft := draftWithLine(t, s)
	s.f.outboxErr = errors.New("el evento no valida contra su schema")
	if _, err := newIssuer(s).Execute(ctx, s.tn, "issue-1", draft.ID); err == nil {
		t.Fatal("debió fallar")
	}
	inv := s.f.state.invoices[0]
	if inv.Status != invoice.StatusDraft || inv.Number != "" || len(s.f.state.history[draft.ID]) != 0 || len(s.f.state.outbox) != 0 {
		t.Fatalf("quedó algo emitido: %+v", inv)
	}
	if len(s.f.state.sequences) != 0 {
		t.Fatal("el número volvió a quedar libre (sin huecos por rollback)")
	}
	// Con el outbox sano, la misma clave emite normalmente: la reserva también se revirtió.
	s.f.outboxErr = nil
	if res, err := newIssuer(s).Execute(ctx, s.tn, "issue-1", draft.ID); err != nil || res.Invoice.Number != "00000001" {
		t.Fatalf("reintento: %+v err=%v", res.Invoice.Number, err)
	}
}

func TestIssueValidations(t *testing.T) {
	s := newScenario(t)
	uc := newIssuer(s)

	empty, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "empty", s.header(), nil)
	if _, err := uc.Execute(ctx, s.tn, "a", empty.Invoice.ID); !errors.Is(err, invoice.ErrWithoutLines) {
		t.Fatalf("sin líneas: err = %v", err)
	}

	d := draftWithLine(t, s)
	off := false
	if _, err := NewUpdateCustomer(s.f).Execute(ctx, tenant(s.org, "owner"), s.customer, customer.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, s.tn, "b", d.ID); !errors.Is(err, ErrCustomerInactive) {
		t.Fatalf("cliente inactivo: err = %v", err)
	}
	on := true
	_, _ = NewUpdateCustomer(s.f).Execute(ctx, tenant(s.org, "owner"), s.customer, customer.Patch{IsActive: &on})

	if _, err := NewUpdateProduct(s.f).Execute(ctx, tenant(s.org, "owner"), s.crcProduct.ID, product.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, s.tn, "c", d.ID); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("producto desactivado: err = %v", err)
	}

	if _, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "d", d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("de otra organización: err = %v", err)
	}
	for _, role := range []string{"collector", "accountant", "read_only"} {
		if _, err := uc.Execute(ctx, tenant(s.org, role), "e", d.ID); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s: err = %v", role, err)
		}
	}
	if s.f.state.invoices[1].Status != invoice.StatusDraft || len(s.f.state.outbox) != 0 {
		t.Fatal("ningún intento rechazado emitió")
	}
}

func TestIssueDueDateWithCredit(t *testing.T) {
	s := newScenario(t)
	h := s.header()
	days := 30
	h.CreditTermDays = &days
	d, _ := NewCreateInvoiceDraft(s.f).Execute(ctx, s.tn, "d", h, []LineRequest{s.line(s.crcProduct, "1")})
	res, err := newIssuer(s).Execute(ctx, s.tn, "i", d.Invoice.ID)
	if err != nil || res.Invoice.DueDate != "2026-10-24" {
		t.Fatalf("vencimiento = %q err=%v", res.Invoice.DueDate, err)
	}
}

// Criterio 5: cambiar el cliente o el producto después de emitir no altera la factura ni su evento.
func TestIssuedInvoiceIgnoresLaterChanges(t *testing.T) {
	s := newScenario(t)
	draft := draftWithLine(t, s)
	res, err := newIssuer(s).Execute(ctx, s.tn, "i", draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	eventBefore := s.f.state.outbox[0]

	owner := tenant(s.org, "owner")
	name, email := "Ana Pérez Solís", "otra@example.com"
	if _, err := NewUpdateCustomer(s.f).Execute(ctx, owner, s.customer, customer.Patch{LegalName: &name, Email: &email}); err != nil {
		t.Fatal(err)
	}
	price := money.MustAmountForTest("99999")
	desc := "Consultoría premium"
	if _, err := NewUpdateProduct(s.f).Execute(ctx, owner, s.crcProduct.ID, product.Patch{UnitPrice: &price, Description: &desc}); err != nil {
		t.Fatal(err)
	}

	after, err := NewGetInvoice(s.f).Execute(ctx, s.tn, draft.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Customer.LegalName != "Ana Pérez" || after.Customer.Email != "" || after.Lines[0].Description != "Consultoría" ||
		after.Lines[0].UnitPrice.String() != "25000.5" || after.Totals != res.Invoice.Totals {
		t.Fatalf("la factura emitida cambió: %+v", after)
	}
	if len(s.f.state.outbox) != 1 || s.f.state.outbox[0].Invoice.Customer != eventBefore.Invoice.Customer {
		t.Fatal("el evento cambió")
	}
}
