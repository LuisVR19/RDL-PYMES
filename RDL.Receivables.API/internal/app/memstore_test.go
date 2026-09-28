package app

import (
	"context"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/pkg/tenancy"
)

// memStore imita a la base para los casos de uso: filas por organización, saldo y estado recalculados con la misma
// fórmula que los triggers (receivable.Rehydrate), rollback completo si la transacción falla y registro del orden
// de bloqueo. Implementa TxManager, EventTxManager, Tx y EventTx.
type memStore struct {
	org         uuid.UUID
	timezone    string
	receivables map[uuid.UUID]*memReceivable
	payments    map[uuid.UUID]*memPayment
	apps        []memApp
	followUps   map[uuid.UUID]FollowUpView
	promises    map[uuid.UUID]PromiseView
	idem        map[string]IdempotencyRecord
	inbox       map[uuid.UUID]bool
	audits      []AuditEvent
	received    []PaymentReceivedEvent
	settled     []ReceivableSettledEvent
	locks       []string
	commits     int
}

type memReceivable struct {
	inv    receivable.Invoice
	number string
	adjs   []receivable.Adjustment
}

type memPayment struct {
	data       payment.Data
	status     payment.Status
	voidReason string
	voidedAt   time.Time
	receivedOn civil.Date
}

type memApp struct {
	id, paymentID, receivableID uuid.UUID
	amount                      decimal.Decimal
	appliedAt, reversedAt       time.Time
	reason                      string
}

func newMemStore() *memStore {
	return &memStore{
		org: uuid.New(), timezone: "America/Costa_Rica",
		receivables: map[uuid.UUID]*memReceivable{}, payments: map[uuid.UUID]*memPayment{},
		followUps: map[uuid.UUID]FollowUpView{}, promises: map[uuid.UUID]PromiseView{},
		idem: map[string]IdempotencyRecord{}, inbox: map[uuid.UUID]bool{},
	}
}

// snapshot y restore: copia superficial de todo lo mutable (las filas se reemplazan, nunca se editan en sitio,
// salvo las de memApp y memPayment, que se copian por valor).
type memSnapshot struct {
	receivables map[uuid.UUID]memReceivable
	payments    map[uuid.UUID]memPayment
	apps        []memApp
	followUps   map[uuid.UUID]FollowUpView
	promises    map[uuid.UUID]PromiseView
	idem        map[string]IdempotencyRecord
	inbox       map[uuid.UUID]bool
	audits      []AuditEvent
	received    []PaymentReceivedEvent
	settled     []ReceivableSettledEvent
}

func (s *memStore) snapshot() memSnapshot {
	snap := memSnapshot{
		receivables: map[uuid.UUID]memReceivable{}, payments: map[uuid.UUID]memPayment{},
		apps: slices.Clone(s.apps), followUps: maps.Clone(s.followUps), promises: maps.Clone(s.promises),
		idem: maps.Clone(s.idem), inbox: maps.Clone(s.inbox), audits: slices.Clone(s.audits),
		received: slices.Clone(s.received), settled: slices.Clone(s.settled),
	}
	for id, r := range s.receivables {
		c := *r
		c.adjs = slices.Clone(r.adjs)
		snap.receivables[id] = c
	}
	for id, p := range s.payments {
		snap.payments[id] = *p
	}
	return snap
}

func (s *memStore) restore(snap memSnapshot) {
	s.receivables, s.payments = map[uuid.UUID]*memReceivable{}, map[uuid.UUID]*memPayment{}
	for id, r := range snap.receivables {
		c := r
		s.receivables[id] = &c
	}
	for id, p := range snap.payments {
		c := p
		s.payments[id] = &c
	}
	s.apps, s.followUps, s.promises, s.idem, s.inbox = snap.apps, snap.followUps, snap.promises, snap.idem, snap.inbox
	s.audits, s.received, s.settled = snap.audits, snap.received, snap.settled
}

func (s *memStore) tx(ctx context.Context, org uuid.UUID, fn func(context.Context) error) error {
	if org != s.org {
		panic("transacción en una organización distinta de la del store")
	}
	snap := s.snapshot()
	s.locks = nil
	if err := fn(ctx); err != nil {
		s.restore(snap)
		return err
	}
	s.commits++
	return nil
}

func (s *memStore) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, Tx) error) error {
	return s.tx(ctx, t.OrganizationID(), func(ctx context.Context) error { return fn(ctx, s) })
}

func (s *memStore) WithinServiceTx(ctx context.Context, org uuid.UUID, fn func(context.Context, EventTx) error) error {
	return s.tx(ctx, org, func(ctx context.Context) error { return fn(ctx, s) })
}

// --- Tx y EventTx ---

func (s *memStore) Organizations() OrganizationReader { return s }
func (s *memStore) Receivables() ReceivableReader     { return memReceivables{s} }
func (s *memStore) Payments() PaymentReader           { return memPayments{s} }
func (s *memStore) Ledger() Ledger                    { return memLedger{s} }
func (s *memStore) Collection() CollectionStore       { return memCollection{s} }
func (s *memStore) Idempotency() IdempotencyStore     { return s }
func (s *memStore) Audit() AuditRecorder              { return s }
func (s *memStore) Outbox() Outbox                    { return s }
func (s *memStore) Inbox() Inbox                      { return memInbox{s} }
func (s *memStore) ReceivableStore() ReceivableStore  { return memReceivables{s} }

func (s *memStore) Timezone(context.Context, uuid.UUID) (string, error) { return s.timezone, nil }

func (s *memStore) Claim(_ context.Context, org uuid.UUID, key, hash string, _ time.Duration) (*IdempotencyRecord, error) {
	k := org.String() + "/" + key
	if prev, ok := s.idem[k]; ok {
		return &prev, nil
	}
	s.idem[k] = IdempotencyRecord{RequestHash: hash}
	return nil, nil
}

func (s *memStore) Complete(_ context.Context, org uuid.UUID, key string, r IdempotencyRecord) error {
	s.idem[org.String()+"/"+key] = r
	return nil
}

func (s *memStore) Record(_ context.Context, e AuditEvent) error {
	s.audits = append(s.audits, e)
	return nil
}

func (s *memStore) PaymentReceived(_ context.Context, e PaymentReceivedEvent) error {
	s.received = append(s.received, e)
	return nil
}

func (s *memStore) ReceivableSettled(_ context.Context, e ReceivableSettledEvent) error {
	s.settled = append(s.settled, e)
	return nil
}

// memInbox: Inbox.Claim tiene otra firma que IdempotencyStore.Claim, así que va en un tipo aparte.
type memInbox struct{ s *memStore }

func (m memInbox) Claim(_ context.Context, ref EventRef) (bool, error) {
	if m.s.inbox[ref.ID] {
		return false, nil
	}
	m.s.inbox[ref.ID] = true
	return true, nil
}

func (m memInbox) MarkProcessed(context.Context, EventRef) error { return nil }

// --- agregados ---

// rehydrate reconstruye la cuenta como lo haría la base: la misma fórmula que los triggers.
func (s *memStore) rehydrate(id uuid.UUID) *receivable.Receivable {
	row := s.receivables[id]
	var apps []receivable.Application
	for _, a := range s.apps {
		if a.receivableID == id {
			apps = append(apps, receivable.Application{ID: a.id, PaymentID: a.paymentID, Amount: a.amount,
				AppliedAt: a.appliedAt, ReversedAt: a.reversedAt, ReversalReason: a.reason})
		}
	}
	r, err := receivable.Rehydrate(id, row.inv, row.adjs, apps)
	if err != nil {
		panic(err) // la "base" nunca guarda un estado inválido: un check lo habría rechazado
	}
	return r
}

type memLedger struct{ s *memStore }

func (l memLedger) LockPayment(_ context.Context, org, id uuid.UUID) (*payment.Payment, error) {
	p, ok := l.s.payments[id]
	if !ok || org != l.s.org {
		return nil, ErrNotFound
	}
	l.s.locks = append(l.s.locks, "P:"+id.String())
	var apps []payment.Application
	for _, a := range l.s.apps {
		if a.paymentID == id {
			apps = append(apps, payment.Application{ID: a.id, ReceivableID: a.receivableID, Amount: a.amount,
				AppliedAt: a.appliedAt, ReversedAt: a.reversedAt, ReversalReason: a.reason})
		}
	}
	return payment.Rehydrate(p.data, p.status, p.voidReason, p.voidedAt, apps)
}

func (l memLedger) LockReceivable(_ context.Context, org, id uuid.UUID) (*receivable.Receivable, ReceivableMeta, error) {
	row, ok := l.s.receivables[id]
	if !ok || org != l.s.org {
		return nil, ReceivableMeta{}, ErrNotFound
	}
	l.s.locks = append(l.s.locks, "R:"+id.String())
	return l.s.rehydrate(id), ReceivableMeta{DocumentNumber: row.number}, nil
}

func (l memLedger) ActivePaymentIDs(_ context.Context, _, receivableID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, a := range l.s.apps {
		if a.receivableID == receivableID && a.reversedAt.IsZero() {
			ids = append(ids, a.paymentID)
		}
	}
	return sortedUnique(ids), nil
}

func (l memLedger) FindApplication(_ context.Context, _, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	for _, a := range l.s.apps {
		if a.id == id {
			return a.paymentID, a.receivableID, nil
		}
	}
	return uuid.Nil, uuid.Nil, ErrNotFound
}

func (l memLedger) InsertPayment(_ context.Context, _ uuid.UUID, n NewPayment) error {
	l.s.payments[n.Payment.ID()] = &memPayment{data: n.Payment.Data(), status: payment.StatusPosted, receivedOn: n.ReceivedOn}
	return nil
}

func (l memLedger) InsertApplication(_ context.Context, _ uuid.UUID, a NewApplication) error {
	l.s.apps = append(l.s.apps, memApp{id: a.ID, paymentID: a.PaymentID, receivableID: a.ReceivableID, amount: a.Amount, appliedAt: a.AppliedAt})
	return nil
}

func (l memLedger) ReverseApplication(_ context.Context, _, id uuid.UUID, reason string, at time.Time, _ uuid.UUID) error {
	for i := range l.s.apps {
		if l.s.apps[i].id == id && l.s.apps[i].reversedAt.IsZero() {
			l.s.apps[i].reversedAt, l.s.apps[i].reason = at, reason
			return nil
		}
	}
	return errNotReversible
}

func (l memLedger) VoidPayment(_ context.Context, _, id uuid.UUID, reason string, at time.Time) error {
	p := l.s.payments[id]
	p.status, p.voidReason, p.voidedAt = payment.StatusVoided, reason, at
	return nil
}

func (l memLedger) InsertAdjustment(_ context.Context, _, receivableID uuid.UUID, adj receivable.Adjustment, _ string) error {
	row := l.s.receivables[receivableID]
	row.adjs = append(row.adjs, adj)
	return nil
}

func (l memLedger) ReceivableState(_ context.Context, _, id uuid.UUID) (ReceivableState, error) {
	r := l.s.rehydrate(id)
	st := ReceivableState{Balance: r.Balance(), Status: r.Status()}
	if r.Balance().IsZero() {
		at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
		st.SettledAt = &at
	}
	return st, nil
}

type memErr string

func (e memErr) Error() string { return string(e) }

const errNotReversible = memErr("la aplicación no estaba vigente")

// --- lecturas ---

type memReceivables struct{ s *memStore }

func (m memReceivables) List(context.Context, uuid.UUID, ReceivableQuery) ([]ReceivableView, error) {
	return nil, nil
}

func (m memReceivables) view(id uuid.UUID) ReceivableView {
	r := m.s.rehydrate(id)
	inv := r.Invoice()
	return ReceivableView{ID: id, SourceInvoiceID: inv.ID, CustomerID: inv.CustomerID, Currency: inv.Currency,
		OriginalAmount: inv.Original.String(), BalanceAmount: r.Balance().String(), IssuedOn: inv.IssuedOn, DueOn: inv.DueOn,
		Status: r.Status(), DocumentNumber: m.s.receivables[id].number}
}

func (m memReceivables) Get(_ context.Context, _, id uuid.UUID) (ReceivableDetail, error) {
	if _, ok := m.s.receivables[id]; !ok {
		return ReceivableDetail{}, ErrNotFound
	}
	return ReceivableDetail{ReceivableView: m.view(id)}, nil
}

func (m memReceivables) GetByInvoice(_ context.Context, _, invoiceID uuid.UUID) (ReceivableView, error) {
	for id, r := range m.s.receivables {
		if r.inv.ID == invoiceID {
			return m.view(id), nil
		}
	}
	return ReceivableView{}, ErrNotFound
}

func (m memReceivables) ListByInvoices(_ context.Context, _ uuid.UUID, invoiceIDs []uuid.UUID) ([]ReceivableView, error) {
	var out []ReceivableView
	for id, r := range m.s.receivables {
		if slices.Contains(invoiceIDs, r.inv.ID) {
			out = append(out, m.view(id))
		}
	}
	return out, nil
}

func (m memReceivables) AgingByDueDate(context.Context, uuid.UUID, string) ([]AgingRow, error) {
	return nil, nil
}

func (m memReceivables) FindByInvoice(_ context.Context, _, invoiceID uuid.UUID) (ReceivableRecord, bool, error) {
	for id, r := range m.s.receivables {
		if r.inv.ID == invoiceID {
			return ReceivableRecord{ID: id, CustomerID: r.inv.CustomerID, Currency: r.inv.Currency, Original: r.inv.Original}, true, nil
		}
	}
	return ReceivableRecord{}, false, nil
}

func (m memReceivables) Create(_ context.Context, _ uuid.UUID, n NewReceivable) error {
	m.s.receivables[n.Receivable.ID()] = &memReceivable{inv: n.Receivable.Invoice(), number: n.InvoiceNumber}
	return nil
}

type memPayments struct{ s *memStore }

func (m memPayments) Get(_ context.Context, _, id uuid.UUID) (PaymentView, error) {
	p, ok := m.s.payments[id]
	if !ok {
		return PaymentView{}, ErrNotFound
	}
	v := PaymentView{ID: id, CustomerID: p.data.CustomerID, Amount: p.data.Amount.String(), Currency: p.data.Currency,
		Status: p.status, VoidReason: p.voidReason, ReceivedOn: p.receivedOn}
	for _, a := range m.s.apps {
		if a.paymentID == id {
			v.Applications = append(v.Applications, m.app(a))
		}
	}
	return v, nil
}

func (m memPayments) app(a memApp) ApplicationView {
	v := ApplicationView{ID: a.id, PaymentID: a.paymentID, ReceivableID: a.receivableID, Amount: a.amount.String(),
		AppliedAt: a.appliedAt, ReversalReason: a.reason}
	if !a.reversedAt.IsZero() {
		at := a.reversedAt
		v.ReversedAt = &at
	}
	return v
}

func (m memPayments) List(context.Context, uuid.UUID, PaymentQuery) ([]PaymentView, error) {
	return nil, nil
}

func (m memPayments) GetApplication(_ context.Context, _, id uuid.UUID) (ApplicationView, error) {
	for _, a := range m.s.apps {
		if a.id == id {
			return m.app(a), nil
		}
	}
	return ApplicationView{}, ErrNotFound
}

type memCollection struct{ s *memStore }

func (m memCollection) CreateFollowUp(_ context.Context, _ uuid.UUID, f FollowUpView) error {
	m.s.followUps[f.ID] = f
	return nil
}

func (m memCollection) GetFollowUp(_ context.Context, _, id uuid.UUID) (FollowUpView, error) {
	f, ok := m.s.followUps[id]
	if !ok {
		return FollowUpView{}, ErrNotFound
	}
	return f, nil
}

func (m memCollection) ListFollowUps(context.Context, uuid.UUID, uuid.UUID) ([]FollowUpView, error) {
	return slices.Collect(maps.Values(m.s.followUps)), nil
}

func (m memCollection) CreatePromise(_ context.Context, _ uuid.UUID, p PromiseView) error {
	m.s.promises[p.ID] = p
	return nil
}

func (m memCollection) GetPromise(_ context.Context, _, id uuid.UUID) (PromiseView, error) {
	p, ok := m.s.promises[id]
	if !ok {
		return PromiseView{}, ErrNotFound
	}
	return p, nil
}

func (m memCollection) LockPromise(ctx context.Context, org, id uuid.UUID) (PromiseView, error) {
	return m.GetPromise(ctx, org, id)
}

func (m memCollection) ListPromises(context.Context, uuid.UUID, uuid.UUID) ([]PromiseView, error) {
	return slices.Collect(maps.Values(m.s.promises)), nil
}

func (m memCollection) SetPromiseStatus(_ context.Context, _, id uuid.UUID, status collection.PromiseStatus) error {
	p := m.s.promises[id]
	p.Status = status
	m.s.promises[id] = p
	return nil
}

// --- ayudas de los tests ---

var customerA = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")

// seedReceivable crea una cuenta open del cliente A en CRC.
func (s *memStore) seedReceivable(t *testing.T, original string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	s.receivables[id] = &memReceivable{number: "FAC-" + id.String()[:4], inv: receivable.Invoice{
		ID: uuid.New(), CustomerID: customerA, Currency: "CRC", Original: decimal.RequireFromString(original),
		IssuedOn: civil.Date{Year: 2026, Month: time.September, Day: 1}, DueOn: civil.Date{Year: 2026, Month: time.October, Day: 1},
	}}
	return id
}

func (s *memStore) seedPayment(t *testing.T, amt string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	s.payments[id] = &memPayment{status: payment.StatusPosted,
		data: payment.Data{ID: id, CustomerID: customerA, Currency: "CRC", Amount: decimal.RequireFromString(amt)}}
	return id
}

func (s *memStore) balance(id uuid.UUID) string { return s.rehydrate(id).Balance().String() }

func (s *memStore) status(id uuid.UUID) receivable.Status { return s.rehydrate(id).Status() }

func (s *memStore) tenant(roles ...string) tenancy.Context {
	return tenancy.NewContext(uuid.New(), "sub", s.org, roles)
}

func fixedNow() time.Time { return time.Date(2026, 9, 25, 18, 0, 0, 0, time.UTC) } // 12:00 en Costa Rica
