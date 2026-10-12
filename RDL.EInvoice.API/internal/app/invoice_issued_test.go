package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/einvoice-api/internal/domain/document"
)

// Fakes del consumidor: inbox, perfiles, documentos y auditoría en memoria, con commit y rollback por transacción.
type memState struct {
	inbox    map[uuid.UUID]bool
	docs     map[uuid.UUID]document.Document // por documento comercial
	audits   []AuditEvent
	profiles map[uuid.UUID]FiscalProfile
}

type memTxm struct {
	state *memState
	dead  []DeadLetter
}

func newMem() *memTxm {
	return &memTxm{state: &memState{inbox: map[uuid.UUID]bool{}, docs: map[uuid.UUID]document.Document{}, profiles: map[uuid.UUID]FiscalProfile{}}}
}

func (m *memTxm) WithinServiceTx(ctx context.Context, _ uuid.UUID, fn func(context.Context, EventTx) error) error {
	work := &memState{inbox: map[uuid.UUID]bool{}, docs: map[uuid.UUID]document.Document{}, profiles: m.state.profiles}
	for k, v := range m.state.inbox {
		work.inbox[k] = v
	}
	for k, v := range m.state.docs {
		work.docs[k] = v
	}
	work.audits = append(work.audits, m.state.audits...)
	if err := fn(ctx, memTx{work}); err != nil {
		return err // rollback: work se descarta
	}
	m.state = work
	return nil
}

func (m *memTxm) Record(_ context.Context, d DeadLetter) error {
	m.dead = append(m.dead, d)
	return nil
}

type memTx struct{ s *memState }

func (t memTx) Inbox() Inbox                        { return memInbox(t) }
func (t memTx) FiscalProfiles() FiscalProfileReader { return memProfiles(t) }
func (t memTx) Documents() DocumentStore            { return memDocs(t) }
func (t memTx) Audit() AuditRecorder                { return memAudit(t) }

type memInbox memTx

func (i memInbox) Claim(_ context.Context, ref EventRef) (bool, error) {
	if i.s.inbox[ref.ID] {
		return false, nil
	}
	i.s.inbox[ref.ID] = true
	return true, nil
}
func (i memInbox) MarkProcessed(context.Context, EventRef) error { return nil }

type memProfiles memTx

func (p memProfiles) Get(_ context.Context, org uuid.UUID) (FiscalProfile, error) {
	fp, ok := p.s.profiles[org]
	if !ok {
		return FiscalProfile{}, ErrNotFound
	}
	return fp, nil
}

type memDocs memTx

func (d memDocs) ExistsBySource(_ context.Context, _ document.SourceType, id uuid.UUID) (bool, error) {
	_, ok := d.s.docs[id]
	return ok, nil
}
func (d memDocs) Create(_ context.Context, doc document.Document) error {
	d.s.docs[doc.SourceDocumentID] = doc
	return nil
}

type memAudit memTx

func (a memAudit) Record(_ context.Context, e AuditEvent) error {
	a.s.audits = append(a.s.audits, e)
	return nil
}

// fakeDecoder entrega el evento armado por la prueba sin pasar por JSON (el decoder real tiene sus propias pruebas).
type fakeDecoder struct{ events map[string]IncomingEvent }

func (f fakeDecoder) Decode(payload []byte) (IncomingEvent, error) {
	ev, ok := f.events[string(payload)]
	if !ok {
		return IncomingEvent{}, ErrInvalidEvent
	}
	return ev, nil
}
func (f fakeDecoder) Peek(payload []byte) EventRef { return f.events[string(payload)].Ref }

func invoiceEvent(org uuid.UUID, lines, methods int) IncomingEvent {
	d := document.Document{
		SourceDocumentID: uuid.New(), SourceDocumentNumber: "00000001", SaleConditionCode: "01", CurrencyCode: "CRC",
		ExchangeRate: decimal.NewFromInt(1), Total: decimal.RequireFromString("1130"),
		Receiver: document.Party{IdentificationTypeCode: "02", IdentificationNumber: "3101999999", LegalName: "Cliente S.A."},
	}
	for i := 1; i <= lines; i++ {
		d.Lines = append(d.Lines, document.Line{Number: i, Quantity: decimal.NewFromInt(1)})
	}
	d.PaymentMethods = make([]document.PaymentMethod, methods)
	return IncomingEvent{
		Ref:  EventRef{ID: uuid.New(), Type: "InvoiceIssued", Version: 1, OrganizationID: org, CorrelationID: uuid.New()},
		Body: InvoiceIssued{Document: d},
	}
}

func newProcessor(m *memTxm, events map[string]IncomingEvent) *ProcessEvent {
	dec := fakeDecoder{events}
	handle := NewHandleEvent(dec, m, map[string]EventHandler{"InvoiceIssued": NewCreateDocumentFromInvoice()})
	return NewProcessEvent(handle, dec, m, RetryPolicy{Attempts: 2})
}

func TestInvoiceIssuedCreatesOneDocument(t *testing.T) {
	org := uuid.New()
	m := newMem()
	m.state.profiles[org] = FiscalProfile{IdentificationTypeCode: "02", IdentificationNumber: "3101123456", LegalName: "PYME S.A.", EconomicActivityCode: "620100"}
	ev := invoiceEvent(org, 2, 1)
	p := newProcessor(m, map[string]IncomingEvent{"e1": ev})

	res, err := p.Process(context.Background(), []byte("e1"))
	if err != nil || res.Outcome != OutcomeProcessed {
		t.Fatalf("primera vez: %+v, %v", res, err)
	}
	doc := m.state.docs[ev.Body.(InvoiceIssued).Document.SourceDocumentID]
	if doc.Status != document.StatusProcessing || doc.TypeCode() != "01" || doc.Issuer.LegalName != "PYME S.A." ||
		doc.Issuer.EconomicActivityCode != "620100" || doc.OrganizationID != org || doc.SourceEventID != ev.Ref.ID {
		t.Errorf("documento = %+v", doc)
	}
	if len(m.state.audits) != 1 || m.state.audits[0].Action != "electronic_document.created" ||
		m.state.audits[0].CorrelationID != ev.Ref.CorrelationID || m.state.audits[0].ActorType != ActorService {
		t.Errorf("auditoría = %+v", m.state.audits)
	}

	// Criterio 3: el mismo evento otra vez no crea un segundo documento ni otra auditoría.
	res, err = p.Process(context.Background(), []byte("e1"))
	if err != nil || res.Outcome != OutcomeDuplicate || len(m.state.docs) != 1 || len(m.state.audits) != 1 {
		t.Fatalf("reproceso: %+v, %v, docs=%d, audits=%d", res, err, len(m.state.docs), len(m.state.audits))
	}
}

func TestInvoiceIssuedPermanentFailuresGoToDeadLetter(t *testing.T) {
	org := uuid.New()
	m := newMem()
	m.state.profiles[org] = FiscalProfile{LegalName: "PYME S.A."}

	dup := invoiceEvent(org, 1, 0)
	other := dup
	other.Ref.ID = uuid.New() // otro evento del mismo documento comercial
	events := map[string]IncomingEvent{
		"sin-perfil":   invoiceEvent(uuid.New(), 1, 0),
		"cinco-medios": invoiceEvent(org, 1, 5),
		"dup":          dup,
		"otro":         other,
	}
	p := newProcessor(m, events)
	if res, err := p.Process(context.Background(), []byte("dup")); err != nil || res.Outcome != OutcomeProcessed {
		t.Fatalf("dup: %+v, %v", res, err)
	}
	for _, name := range []string{"sin-perfil", "cinco-medios", "otro"} {
		res, err := p.Process(context.Background(), []byte(name))
		if err != nil || res.Outcome != OutcomeDeadLettered || res.Attempts != 1 || !errors.Is(res.Err, ErrRejectedEvent) {
			t.Errorf("%s: %+v, %v (se esperaba dead letter al primer intento)", name, res, err)
		}
	}
	if len(m.dead) != 3 || len(m.state.docs) != 1 {
		t.Errorf("dead letters = %d, documentos = %d", len(m.dead), len(m.state.docs))
	}
}
