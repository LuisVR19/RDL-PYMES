package app

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
)

func invoiceIssued(total string) InvoiceIssued {
	return InvoiceIssued{
		InvoiceID: uuid.New(), InvoiceNumber: "FAC-0000001", CustomerID: uuid.New(),
		CustomerIdentification: "3101123456", CustomerLegalName: "Cliente Ejemplo S.A.", SaleConditionCode: "02",
		Currency: "CRC", Total: decimal.RequireFromString(total),
		IssueDate: civil.Date{Year: 2026, Month: time.September, Day: 24},
		DueDate:   civil.Date{Year: 2026, Month: time.October, Day: 24},
	}
}

func handleInvoice(t *testing.T, s *fakeEventStore, ev IncomingEvent) (Outcome, error) {
	t.Helper()
	dec := fakeDecoder{events: map[string]IncomingEvent{"e": ev}}
	uc := NewHandleEvent(dec, s, map[string]EventHandler{"InvoiceIssued": NewCreateReceivableFromInvoice()})
	return uc.Handle(t.Context(), []byte("e"))
}

func TestInvoiceIssuedCreatesOpenReceivableWithAudit(t *testing.T) {
	s := newFakeEventStore()
	ev := event("InvoiceIssued")
	cmd := invoiceIssued("11300.50")
	ev.Body = cmd
	if out, err := handleInvoice(t, s, ev); err != nil || out != OutcomeProcessed {
		t.Fatalf("%v, %v", out, err)
	}
	got, ok := s.receivables[ev.Ref.OrganizationID][cmd.InvoiceID]
	if !ok {
		t.Fatal("no se creó la cuenta en la organización del evento")
	}
	r := got.Receivable
	if r.ID() == cmd.InvoiceID {
		t.Error("el id de la cuenta no debe ser el de la factura")
	}
	if r.Status() != receivable.StatusOpen || !r.Balance().Equal(cmd.Total) || r.Invoice().DueOn != cmd.DueDate {
		t.Errorf("cuenta %s, saldo %s, vence %s", r.Status(), r.Balance(), r.Invoice().DueOn)
	}
	if got.SourceEventID != ev.Ref.ID || got.InvoiceNumber != cmd.InvoiceNumber || got.CustomerLegalName != cmd.CustomerLegalName {
		t.Errorf("snapshot: %+v", got)
	}
	if len(s.audits) != 1 {
		t.Fatalf("auditoría: %+v", s.audits)
	}
	a := s.audits[0]
	if a.ActorType != ActorService || a.ActorUserID != uuid.Nil || a.CorrelationID != ev.Ref.CorrelationID ||
		a.OrganizationID != ev.Ref.OrganizationID || a.EntityID != r.ID() || a.Action != "receivable.created" {
		t.Errorf("auditoría: %+v", a)
	}
}

// Invariante 4: el mismo evento dos veces, o la misma factura en otro evento con los mismos datos, no cambia nada.
func TestInvoiceIssuedIsIdempotent(t *testing.T) {
	s := newFakeEventStore()
	ev := event("InvoiceIssued")
	cmd := invoiceIssued("100")
	ev.Body = cmd
	if _, err := handleInvoice(t, s, ev); err != nil {
		t.Fatal(err)
	}
	if out, err := handleInvoice(t, s, ev); err != nil || out != OutcomeDuplicate {
		t.Errorf("mismo eventId: %v, %v", out, err)
	}
	republished := ev
	republished.Ref.ID = uuid.New()
	if out, err := handleInvoice(t, s, republished); err != nil || out != OutcomeProcessed {
		t.Errorf("otro eventId, misma factura: %v, %v", out, err)
	}
	if len(s.receivables[ev.Ref.OrganizationID]) != 1 || len(s.audits) != 1 {
		t.Errorf("cuentas %d, auditorías %d", len(s.receivables[ev.Ref.OrganizationID]), len(s.audits))
	}

	conflicting := republished
	conflicting.Ref.ID = uuid.New()
	other := cmd
	other.Total = decimal.RequireFromString("100.00001")
	conflicting.Body = other
	if _, err := handleInvoice(t, s, conflicting); !errors.Is(err, ErrRejectedEvent) || !errors.Is(err, ErrInvoiceConflict) {
		t.Errorf("misma factura con otro total: %v", err)
	}
}

// El tenant sale del evento: la misma factura en otra organización es otra cuenta y no toca la primera.
func TestInvoiceIssuedIsScopedToTheEventOrganization(t *testing.T) {
	s := newFakeEventStore()
	a := event("InvoiceIssued")
	cmd := invoiceIssued("100")
	a.Body = cmd
	b := event("InvoiceIssued")
	other := cmd
	other.Total = decimal.RequireFromString("5")
	b.Body = other
	for _, ev := range []IncomingEvent{a, b} {
		if _, err := handleInvoice(t, s, ev); err != nil {
			t.Fatal(err)
		}
	}
	if !s.receivables[a.Ref.OrganizationID][cmd.InvoiceID].Receivable.Balance().Equal(cmd.Total) ||
		!s.receivables[b.Ref.OrganizationID][cmd.InvoiceID].Receivable.Balance().Equal(other.Total) {
		t.Error("cada organización debe tener su propia cuenta")
	}
}

func TestInvoiceIssuedRejectsWhatTheDatabaseCannotStore(t *testing.T) {
	for name, mutate := range map[string]func(*InvoiceIssued){
		"total cero":             func(c *InvoiceIssued) { c.Total = decimal.Zero },
		"vence antes de emitida": func(c *InvoiceIssued) { c.DueDate = civil.Date{Year: 2026, Month: time.January, Day: 1} },
	} {
		s := newFakeEventStore()
		ev := event("InvoiceIssued")
		cmd := invoiceIssued("100")
		mutate(&cmd)
		ev.Body = cmd
		_, err := handleInvoice(t, s, ev)
		if !errors.Is(err, ErrRejectedEvent) {
			t.Errorf("%s: %v", name, err)
		}
		if len(s.receivables) != 0 || len(s.audits) != 0 || len(s.inbox) != 0 {
			t.Errorf("%s: dejó efectos", name)
		}
	}
	s := newFakeEventStore()
	ev := event("InvoiceIssued")
	cmd := invoiceIssued("100")
	cmd.Total = decimal.RequireFromString("0.000001")
	ev.Body = cmd
	if _, err := handleInvoice(t, s, ev); !errors.Is(err, amount.ErrInvalid) {
		t.Errorf("seis decimales: %v", err)
	}
}
