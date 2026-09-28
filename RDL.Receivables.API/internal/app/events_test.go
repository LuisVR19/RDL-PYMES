package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeEventStore simula la base del consumidor: inbox, cuentas, auditoría y dead letters. Una transacción que
// falla no deja nada (se aplica sobre una copia y solo se confirma si fn termina bien).
type fakeEventStore struct {
	inbox       map[uuid.UUID]bool
	receivables map[uuid.UUID]map[uuid.UUID]NewReceivable // organización → factura → cuenta
	audits      []AuditEvent
	dead        []DeadLetter
	txOrgs      []uuid.UUID

	failTx   []error // errores de las próximas transacciones, en orden
	deadFail error
}

func newFakeEventStore() *fakeEventStore {
	return &fakeEventStore{inbox: map[uuid.UUID]bool{}, receivables: map[uuid.UUID]map[uuid.UUID]NewReceivable{}}
}

// fakeEventTx embebe EventTx: los métodos que estos tests no usan (Ledger, Outbox) fallan si se llaman.
type fakeEventTx struct {
	EventTx
	s           *fakeEventStore
	org         uuid.UUID
	claimed     []uuid.UUID
	created     []NewReceivable
	audits      []AuditEvent
	processedOK bool
}

func (s *fakeEventStore) WithinServiceTx(ctx context.Context, org uuid.UUID, fn func(context.Context, EventTx) error) error {
	s.txOrgs = append(s.txOrgs, org)
	if len(s.failTx) > 0 {
		err := s.failTx[0]
		s.failTx = s.failTx[1:]
		if err != nil {
			return err
		}
	}
	tx := &fakeEventTx{s: s, org: org}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	for _, id := range tx.claimed {
		s.inbox[id] = true
	}
	for _, r := range tx.created {
		if s.receivables[org] == nil {
			s.receivables[org] = map[uuid.UUID]NewReceivable{}
		}
		s.receivables[org][r.Receivable.Invoice().ID] = r
	}
	s.audits = append(s.audits, tx.audits...)
	return nil
}

func (s *fakeEventStore) Record(_ context.Context, d DeadLetter) error {
	if s.deadFail != nil {
		return s.deadFail
	}
	s.dead = append(s.dead, d)
	return nil
}

func (t *fakeEventTx) Inbox() Inbox                     { return t }
func (t *fakeEventTx) ReceivableStore() ReceivableStore { return t }
func (t *fakeEventTx) Audit() AuditRecorder             { return fakeAudit{t} }

func (t *fakeEventTx) Claim(_ context.Context, ref EventRef) (bool, error) {
	if t.s.inbox[ref.ID] {
		return false, nil
	}
	t.claimed = append(t.claimed, ref.ID)
	return true, nil
}

func (t *fakeEventTx) MarkProcessed(context.Context, EventRef) error {
	t.processedOK = true
	return nil
}

func (t *fakeEventTx) FindByInvoice(_ context.Context, org, invoiceID uuid.UUID) (ReceivableRecord, bool, error) {
	if org != t.org {
		panic("FindByInvoice con una organización distinta de la de la transacción")
	}
	r, ok := t.s.receivables[org][invoiceID]
	if !ok {
		return ReceivableRecord{}, false, nil
	}
	inv := r.Receivable.Invoice()
	return ReceivableRecord{ID: r.Receivable.ID(), CustomerID: inv.CustomerID, Currency: inv.Currency, Original: inv.Original}, true, nil
}

func (t *fakeEventTx) Create(_ context.Context, org uuid.UUID, r NewReceivable) error {
	if org != t.org {
		panic("Create con una organización distinta de la de la transacción")
	}
	t.created = append(t.created, r)
	return nil
}

type fakeAudit struct{ t *fakeEventTx }

func (a fakeAudit) Record(_ context.Context, e AuditEvent) error {
	a.t.audits = append(a.t.audits, e)
	return nil
}

// fakeDecoder devuelve el evento que se le indique para cada payload.
type fakeDecoder struct {
	events map[string]IncomingEvent
	errs   map[string]error
}

func (d fakeDecoder) Decode(p []byte) (IncomingEvent, error) {
	if err, ok := d.errs[string(p)]; ok {
		return IncomingEvent{}, err
	}
	return d.events[string(p)], nil
}

func (d fakeDecoder) Peek(p []byte) EventRef { return d.events[string(p)].Ref }

type countingHandler struct {
	calls int
	err   []error
}

func (h *countingHandler) Handle(context.Context, EventTx, IncomingEvent) error {
	h.calls++
	if len(h.err) > 0 {
		err := h.err[0]
		h.err = h.err[1:]
		return err
	}
	return nil
}

func event(eventType string) IncomingEvent {
	return IncomingEvent{Ref: EventRef{
		ID: uuid.New(), Type: eventType, Version: 1, OrganizationID: uuid.New(), CorrelationID: uuid.New(),
	}}
}

type harness struct {
	store   *fakeEventStore
	handler *countingHandler
	proc    *ProcessEvent
	sleeps  []time.Duration
}

func newHarness(dec fakeDecoder) *harness {
	h := &harness{store: newFakeEventStore(), handler: &countingHandler{}}
	handle := NewHandleEvent(dec, h.store, map[string]EventHandler{"InvoiceIssued": h.handler})
	h.proc = NewProcessEvent(handle, dec, h.store, RetryPolicy{Attempts: 6, Base: time.Second, Max: 32 * time.Second})
	h.proc.sleep = func(_ context.Context, d time.Duration) error {
		h.sleeps = append(h.sleeps, d)
		return nil
	}
	return h
}

func TestProcessEventHappyPathAndDuplicate(t *testing.T) {
	ev := event("InvoiceIssued")
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": ev}})
	res, err := h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeProcessed || res.Attempts != 1 || h.handler.calls != 1 {
		t.Fatalf("%+v, %v, llamadas %d", res, err, h.handler.calls)
	}
	// El tenant de la transacción es el organizationId del evento.
	if len(h.store.txOrgs) != 1 || h.store.txOrgs[0] != ev.Ref.OrganizationID {
		t.Errorf("transacción en %v, el evento es de %s", h.store.txOrgs, ev.Ref.OrganizationID)
	}
	res, err = h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeDuplicate || h.handler.calls != 1 {
		t.Errorf("reenvío: %+v, %v, llamadas %d", res, err, h.handler.calls)
	}
}

func TestProcessEventPermanentErrorsGoStraightToDeadLetter(t *testing.T) {
	for name, err := range map[string]error{
		"inválido":       ErrInvalidEvent,
		"sin consumidor": ErrUnsupportedEvent,
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(fakeDecoder{errs: map[string]error{"e": err}})
			res, perr := h.proc.Process(t.Context(), []byte("e"))
			if perr != nil || res.Outcome != OutcomeDeadLettered || res.Attempts != 1 || len(h.sleeps) != 0 {
				t.Fatalf("%+v, %v, esperas %v", res, perr, h.sleeps)
			}
			if len(h.store.dead) != 1 || string(h.store.dead[0].Payload) != "e" || h.store.dead[0].Attempts != 1 {
				t.Errorf("dead letter: %+v", h.store.dead)
			}
			if len(h.store.txOrgs) != 0 {
				t.Error("un evento inválido no debe abrir la transacción del tenant")
			}
		})
	}
}

func TestProcessEventRejectedByDomain(t *testing.T) {
	ev := event("InvoiceIssued")
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": ev}})
	h.handler.err = []error{reject(ErrInvoiceConflict)}
	res, err := h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeDeadLettered || res.Attempts != 1 || !errors.Is(res.Err, ErrInvoiceConflict) {
		t.Fatalf("%+v, %v", res, err)
	}
	if h.store.inbox[ev.Ref.ID] {
		t.Error("el inbox no debe quedar marcado si el efecto falló")
	}
	if h.store.dead[0].Ref.OrganizationID != ev.Ref.OrganizationID {
		t.Error("la dead letter conserva la organización del evento")
	}
}

func TestProcessEventUnknownTypeHasNoHandler(t *testing.T) {
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": event("PaymentReceived")}})
	res, err := h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeDeadLettered || !errors.Is(res.Err, ErrUnsupportedEvent) {
		t.Fatalf("%+v, %v", res, err)
	}
}

func TestProcessEventRetriesTransientErrors(t *testing.T) {
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": event("InvoiceIssued")}})
	transient := errors.New("conexión cerrada")
	h.store.failTx = []error{transient, transient}
	res, err := h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeProcessed || res.Attempts != 3 {
		t.Fatalf("%+v, %v", res, err)
	}
	if len(h.sleeps) != 2 || h.sleeps[0] != time.Second || h.sleeps[1] != 2*time.Second {
		t.Errorf("esperas %v", h.sleeps)
	}
}

func TestProcessEventDeadLettersAfterExhaustingRetries(t *testing.T) {
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": event("InvoiceIssued")}})
	h.handler.err = []error{errors.New("1"), errors.New("2"), errors.New("3"), errors.New("4"), errors.New("5"), errors.New("deadlock")}
	res, err := h.proc.Process(t.Context(), []byte("e"))
	if err != nil || res.Outcome != OutcomeDeadLettered || res.Attempts != 6 {
		t.Fatalf("%+v, %v", res, err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	if len(h.sleeps) != len(want) {
		t.Fatalf("esperas %v", h.sleeps)
	}
	for i := range want {
		if h.sleeps[i] != want[i] {
			t.Errorf("espera %d: %v, se esperaba %v", i, h.sleeps[i], want[i])
		}
	}
	if d := h.store.dead[0]; d.Attempts != 6 || d.Reason != "deadlock" {
		t.Errorf("dead letter: %+v", d)
	}
}

func TestProcessEventDoesNotAckWhenCancelledOrDeadLetterFails(t *testing.T) {
	h := newHarness(fakeDecoder{events: map[string]IncomingEvent{"e": event("InvoiceIssued")}})
	h.handler.err = []error{errors.New("transitorio")}
	h.proc.sleep = func(context.Context, time.Duration) error { return context.Canceled }
	if _, err := h.proc.Process(t.Context(), []byte("e")); !errors.Is(err, context.Canceled) || len(h.store.dead) != 0 {
		t.Errorf("cancelado durante la espera: %v, dead letters %d", err, len(h.store.dead))
	}

	h = newHarness(fakeDecoder{errs: map[string]error{"e": ErrInvalidEvent}})
	h.store.deadFail = errors.New("base caída")
	if _, err := h.proc.Process(t.Context(), []byte("e")); err == nil {
		t.Error("si no se pudo escribir la dead letter el mensaje no debe darse por consumido")
	}
}

func TestRetryPolicyDelayIsCapped(t *testing.T) {
	p := RetryPolicy{Attempts: 10, Base: time.Second, Max: 32 * time.Second}
	if d := p.delay(9); d != 32*time.Second {
		t.Errorf("delay(9) = %v", d)
	}
	for range 100 {
		if d := EqualJitter(8 * time.Second); d < 4*time.Second || d > 8*time.Second {
			t.Fatalf("jitter fuera de rango: %v", d)
		}
	}
}
