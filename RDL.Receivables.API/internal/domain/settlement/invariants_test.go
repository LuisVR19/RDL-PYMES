package settlement

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

// Las 5 invariantes de docs/contexto (prompt P6, paso 4), comprobadas después de cada paso de una secuencia aleatoria
// de operaciones sobre varias cuentas y varios pagos:
//  1. lo aplicado vigente de un pago nunca supera su monto;
//  2. el saldo nunca es negativo y es original + débitos − créditos − cancelación − aplicaciones vigentes;
//  3. revertir una aplicación deja los saldos exactamente como estaban antes de aplicarla;
//  4. reprocesar un evento no cambia nada;
//  5. el estado corresponde al saldo.
//
// Además: una operación válida se acepta y una inválida no cambia nada, los dos lados de cada aplicación coinciden y
// todo cambio de estado de una cuenta es una transición de state-machines/receivable.yaml (o de las pendientes de
// agregar al contrato, ver receivable.TestTransitionsPendingInContract).
// Las sumas se recalculan aquí, sin receivable.Totals ni Derive, para no probar la implementación contra sí misma.

var (
	customers  = []uuid.UUID{uuid.MustParse("00000000-0000-0000-0000-00000000c001"), uuid.MustParse("00000000-0000-0000-0000-00000000c002")}
	currencies = []string{"CRC", "USD"}
	issuedOn   = civil.Date{Year: 2026, Month: time.September, Day: 1}
	dueOn      = civil.Date{Year: 2026, Month: time.October, Day: 1}
)

// sentEvent es un ajuste ya aceptado, para reenviarlo tal cual (invariante 4).
type sentEvent struct {
	receivableID uuid.UUID
	adj          receivable.Adjustment
}

type model struct {
	receivables map[uuid.UUID]*receivable.Receivable
	payments    map[uuid.UUID]*payment.Payment
	events      []sentEvent
	clock       time.Time
	// beforeApply guarda, por aplicación, cómo estaban la cuenta y el pago justo antes de aplicarla.
	beforeApply map[uuid.UUID]applySnapshot
	lastApplied uuid.UUID
	seq         uint64
	allowed     map[receivable.Change]bool
	lastStatus  map[uuid.UUID]receivable.Status
}

type applySnapshot struct{ receivable, payment string }

// pendingInContract son las transiciones que el agregado hace por R1 y R2 y que receivable.yaml v1 no tiene.
// TODO(contratos): agregarlas al YAML (docs/ESTADO.md) y quitar esta lista.
var pendingInContract = []receivable.Change{
	{From: receivable.StatusPaid, To: receivable.StatusOpen},
	{From: receivable.StatusPaid, To: receivable.StatusCancelled},
}

func contractTransitions(t *testing.T) map[receivable.Change]bool {
	raw, err := os.ReadFile("../../../../RDL.Contracts/state-machines/receivable.yaml")
	if err != nil {
		t.Fatalf("no se pudo leer la máquina de estados de contratos: %v", err)
	}
	var sm struct {
		Transitions []struct{ From, To string } `yaml:"transitions"`
	}
	if err := yaml.Unmarshal(raw, &sm); err != nil {
		t.Fatal(err)
	}
	allowed := map[receivable.Change]bool{}
	for _, tr := range sm.Transitions {
		allowed[receivable.Change{From: receivable.Status(tr.From), To: receivable.Status(tr.To)}] = true
	}
	for _, c := range pendingInContract {
		allowed[c] = true
	}
	return allowed
}

func TestInvariants(t *testing.T) {
	allowed := contractTransitions(t)
	rapid.Check(t, func(t *rapid.T) {
		m := &model{
			receivables: map[uuid.UUID]*receivable.Receivable{},
			payments:    map[uuid.UUID]*payment.Payment{},
			beforeApply: map[uuid.UUID]applySnapshot{},
			allowed:     allowed,
			lastStatus:  map[uuid.UUID]receivable.Status{},
			clock:       time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC),
		}
		t.Repeat(map[string]func(*rapid.T){
			"issueInvoice":   m.step(m.issueInvoice),
			"receivePayment": m.step(m.receivePayment),
			"apply":          m.step(m.apply),
			"reverse":        m.step(m.reverse),
			"undoLastApply":  m.undoLastApply,
			"voidPayment":    m.step(m.voidPayment),
			"creditNote":     m.step(m.creditNote),
			"debitNote":      m.step(m.debitNote),
			"cancelInvoice":  m.step(m.cancelInvoice),
			"replayEvent":    m.step(m.replayEvent),
			"":               m.check,
		})
	})
}

// step olvida la última aplicación: después de otra operación, su foto de antes ya no es comparable.
func (m *model) step(action func(*rapid.T)) func(*rapid.T) {
	return func(t *rapid.T) {
		m.lastApplied = uuid.Nil
		action(t)
	}
}

// newID da ids deterministas: rapid repite y achica secuencias, y el orden por id decide qué se revierte primero.
func (m *model) newID() uuid.UUID {
	m.seq++
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], m.seq)
	return id
}

func (m *model) now() time.Time {
	m.clock = m.clock.Add(time.Minute)
	return m.clock
}

func (m *model) issueInvoice(t *rapid.T) {
	r, err := receivable.New(m.newID(), receivable.Invoice{
		ID:         m.newID(),
		CustomerID: rapid.SampledFrom(customers).Draw(t, "customer"),
		Currency:   rapid.SampledFrom(currencies).Draw(t, "currency"),
		Original:   drawPositive(t, "original"),
		IssuedOn:   issuedOn,
		DueOn:      dueOn,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	m.receivables[r.ID()] = r
}

func (m *model) receivePayment(t *rapid.T) {
	p, err := payment.New(payment.Data{
		ID:         m.newID(),
		CustomerID: rapid.SampledFrom(customers).Draw(t, "customer"),
		Currency:   rapid.SampledFrom(currencies).Draw(t, "currency"),
		Amount:     drawPositive(t, "amount"),
	})
	if err != nil {
		t.Fatalf("payment.New: %v", err)
	}
	m.payments[p.ID()] = p
}

func (m *model) apply(t *rapid.T) {
	r := m.drawReceivable(t)
	p := m.drawPaymentFor(t, r)
	amt := drawAmount(t, r.Balance(), p.Available(), decimal.Min(r.Balance(), p.Available()))
	before := applySnapshot{receivable: snapReceivable(r, true), payment: snapPayment(p, true)}
	all := m.snapshot()
	id := m.newID()
	valid := r.Status() != receivable.StatusCancelled && p.Status() == payment.StatusPosted &&
		p.CustomerID() == r.CustomerID() && p.Currency() == r.Currency() && amount.Positive(amt) == nil &&
		amt.LessThanOrEqual(decimal.Min(r.Balance(), p.Available())) && !hasActive(p, r.ID())
	_, err := Apply(p, r, id, amt, m.now())
	if valid != (err == nil) {
		t.Fatalf("Apply de %s (saldo %s, disponible %s): válida=%v, error %v", amt, r.Balance(), p.Available(), valid, err)
	}
	if err != nil {
		m.assertUnchanged(t, all, "Apply", err)
		return
	}
	m.beforeApply[id], m.lastApplied = before, id
}

// reverse revierte una aplicación vigente cualquiera: el saldo sube y el disponible del pago vuelve a crecer
// exactamente en su monto, y el estado sigue al saldo (invariantes 3 y 5).
func (m *model) reverse(t *rapid.T) {
	p, app := m.drawActiveApplication(t)
	r := m.receivables[app.ReceivableID]
	balance, available := r.Balance(), p.Available()
	if _, err := Reverse(p, r, app.ID, rapid.SampledFrom([]string{"", "Error de digitación"}).Draw(t, "reason"),
		m.now()); err != nil {
		if !errors.Is(err, receivable.ErrReasonRequired) && !errors.Is(err, payment.ErrReasonRequired) {
			t.Fatalf("Reverse de una aplicación vigente: %v", err)
		}
		return
	}
	if !r.Balance().Equal(balance.Add(app.Amount)) || !p.Available().Equal(available.Add(app.Amount)) {
		t.Fatalf("revertir %s: saldo %s → %s, disponible %s → %s", app.Amount, balance, r.Balance(), available,
			p.Available())
	}
}

// undoLastApply revierte la última aplicación sin nada en medio: cuenta y pago vuelven a su foto de antes
// (invariante 3, en su forma más estricta: también el estado).
func (m *model) undoLastApply(t *rapid.T) {
	before, ok := m.beforeApply[m.lastApplied]
	if !ok {
		t.Skip("no hay una aplicación recién hecha")
	}
	id := m.lastApplied
	m.lastApplied = uuid.Nil
	p, app := m.findApplication(id)
	r := m.receivables[app.ReceivableID]
	if _, err := Reverse(p, r, id, "Deshacer", m.now()); err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if got := snapReceivable(r, true); got != before.receivable {
		t.Fatalf("la cuenta no volvió a como estaba:\nantes   %s\ndespués %s", before.receivable, got)
	}
	if got := snapPayment(p, true); got != before.payment {
		t.Fatalf("el pago no volvió a como estaba:\nantes   %s\ndespués %s", before.payment, got)
	}
}

func (m *model) voidPayment(t *rapid.T) {
	p := m.drawPayment(t)
	all := m.snapshot()
	loaded := map[uuid.UUID]*receivable.Receivable{}
	for _, id := range p.ActiveReceivableIDs() {
		loaded[id] = m.receivables[id]
	}
	reason := rapid.SampledFrom([]string{"", "Cheque rechazado"}).Draw(t, "reason")
	valid := p.Status() == payment.StatusPosted && reason != ""
	if _, err := VoidPayment(p, loaded, reason, m.now()); valid != (err == nil) {
		t.Fatalf("VoidPayment (%s, motivo %q): válida=%v, error %v", p.Status(), reason, valid, err)
	} else if err != nil {
		m.assertUnchanged(t, all, "VoidPayment", err)
		return
	}
	if p.Status() != payment.StatusVoided || !p.Applied().IsZero() {
		t.Fatalf("pago anulado con estado %s y aplicado %s", p.Status(), p.Applied())
	}
}

func (m *model) creditNote(t *rapid.T) {
	r := m.drawReceivable(t)
	balance := r.Balance()
	capacity := balance.Add(appliedTo(r)) // lo máximo que se puede acreditar revirtiendo todas las aplicaciones
	adj := m.newAdjustment(drawAmount(t, balance, capacity, capacity.Add(decimal.New(1, -5))))
	valid := r.Status() != receivable.StatusCancelled && amount.Positive(adj.Amount) == nil &&
		adj.Amount.LessThanOrEqual(capacity)
	res := m.adjust(t, r, adj, valid, func() (receivable.AdjustmentResult, error) {
		return CreditNote(r, m.paymentsOf(r), adj, m.now())
	})
	// R3: se revierte solo lo necesario; sin la última reversión no alcanzaba.
	missing, released := adj.Amount.Sub(balance), decimal.Zero
	for _, rev := range res.Reversed {
		released = released.Add(rev.Amount)
	}
	if n := len(res.Reversed); n > 0 && (released.LessThan(missing) || !released.Sub(res.Reversed[n-1].Amount).LessThan(missing)) {
		t.Fatalf("nota de %s con saldo %s: se liberaron %s en %d reversiones", adj.Amount, balance, released, n)
	}
}

func (m *model) debitNote(t *rapid.T) {
	r := m.drawReceivable(t)
	adj := m.newAdjustment(drawPositive(t, "debit"))
	valid := r.Status() != receivable.StatusCancelled
	m.adjust(t, r, adj, valid, func() (receivable.AdjustmentResult, error) { return DebitNote(r, adj) })
}

func (m *model) cancelInvoice(t *rapid.T) {
	r := m.drawReceivable(t)
	adj := m.newAdjustment(decimal.Zero)
	// Una cuenta saldada solo con notas de crédito no tiene nada que anular (ErrNothingToCancel).
	valid := r.Status() != receivable.StatusCancelled && r.Balance().Add(appliedTo(r)).IsPositive()
	m.adjust(t, r, adj, valid, func() (receivable.AdjustmentResult, error) {
		return CancelInvoice(r, m.paymentsOf(r), adj, m.now())
	})
	if valid && !r.Balance().IsZero() {
		t.Fatalf("cuenta anulada con saldo %s", r.Balance())
	}
}

// adjust aplica un ajuste por evento y exige que se acepte si y solo si valid; si se acepta, lo guarda para
// reenviarlo después.
func (m *model) adjust(t *rapid.T, r *receivable.Receivable, adj receivable.Adjustment, valid bool,
	op func() (receivable.AdjustmentResult, error),
) receivable.AdjustmentResult {
	all := m.snapshot()
	res, err := op()
	if valid != (err == nil) {
		t.Fatalf("ajuste de %s sobre saldo %s (%s): válido=%v, error %v", adj.Amount, r.Balance(), r.Status(), valid, err)
	}
	if err != nil {
		m.assertUnchanged(t, all, "ajuste", err)
		return res
	}
	if res.Duplicate {
		t.Fatalf("un documento nuevo se tomó como duplicado")
	}
	stored := r.Adjustments()[len(r.Adjustments())-1]
	m.events = append(m.events, sentEvent{receivableID: r.ID(), adj: stored})
	return res
}

// replayEvent reenvía un ajuste ya aceptado (invariante 4): se reconoce como duplicado y no cambia nada.
func (m *model) replayEvent(t *rapid.T) {
	if len(m.events) == 0 {
		t.Skip("no hay eventos que reenviar")
	}
	ev := rapid.SampledFrom(m.events).Draw(t, "event")
	r := m.receivables[ev.receivableID]
	all := m.snapshot()
	var (
		res receivable.AdjustmentResult
		err error
	)
	switch ev.adj.Type {
	case receivable.AdjustmentCreditNote:
		res, err = CreditNote(r, m.paymentsOf(r), ev.adj, m.now())
	case receivable.AdjustmentDebitNote:
		res, err = DebitNote(r, ev.adj)
	case receivable.AdjustmentCancellation:
		res, err = CancelInvoice(r, m.paymentsOf(r), ev.adj, m.now())
	}
	if err != nil || !res.Duplicate || res.From != res.To || len(res.Reversed) > 0 {
		t.Fatalf("reenviar %s: resultado %+v, error %v", ev.adj.Type, res, err)
	}
	if got := m.snapshot(); got != all {
		t.Fatalf("reenviar %s cambió el estado:\nantes   %s\ndespués %s", ev.adj.Type, all, got)
	}
}

func (m *model) check(t *rapid.T) {
	for _, p := range m.payments {
		applied := decimal.Zero
		for _, a := range p.Applications() {
			if a.Active() {
				applied = applied.Add(a.Amount)
				r := m.receivables[a.ReceivableID]
				if !slices.ContainsFunc(r.Applications(), func(ra receivable.Application) bool {
					return ra.ID == a.ID && ra.Active() && ra.PaymentID == p.ID() && ra.Amount.Equal(a.Amount)
				}) {
					t.Fatalf("la aplicación %s del pago no coincide con la de la cuenta", a.ID)
				}
			}
		}
		if applied.GreaterThan(p.Amount()) { // 1
			t.Fatalf("pago %s: aplicado %s mayor que el monto %s", p.ID(), applied, p.Amount())
		}
		if p.Status() == payment.StatusVoided && applied.IsPositive() {
			t.Fatalf("pago %s anulado con %s aplicado", p.ID(), applied)
		}
	}
	for _, r := range m.receivables {
		inv := r.Invoice()
		debits, credits, cancellation, cancelled := decimal.Zero, decimal.Zero, decimal.Zero, false
		for _, a := range r.Adjustments() {
			switch a.Type {
			case receivable.AdjustmentDebitNote:
				debits = debits.Add(a.Amount)
			case receivable.AdjustmentCancellation:
				cancellation, cancelled = cancellation.Add(a.Amount), true
			default:
				credits = credits.Add(a.Amount)
			}
		}
		applied := appliedTo(r)
		want := inv.Original.Add(debits).Sub(credits).Sub(cancellation).Sub(applied)
		if r.Balance().IsNegative() || !r.Balance().Equal(want) { // 2
			t.Fatalf("cuenta %s: saldo %s, se esperaba %s", r.ID(), r.Balance(), want)
		}
		var status receivable.Status // 5
		switch {
		case cancelled:
			status = receivable.StatusCancelled
		case want.IsZero():
			status = receivable.StatusPaid
		case want.Equal(inv.Original.Add(debits)):
			status = receivable.StatusOpen
		default:
			status = receivable.StatusPartiallyPaid
		}
		if r.Status() != status {
			t.Fatalf("cuenta %s: estado %s con saldo %s, se esperaba %s", r.ID(), r.Status(), want, status)
		}
		if cancelled && applied.IsPositive() {
			t.Fatalf("cuenta %s anulada con %s aplicado", r.ID(), applied)
		}
		ch := receivable.Change{From: m.lastStatus[r.ID()], To: r.Status()}
		if ch.From != "" && ch.From != ch.To && !m.allowed[ch] {
			t.Fatalf("cuenta %s: transición %s → %s fuera de la máquina de estados", r.ID(), ch.From, ch.To)
		}
		m.lastStatus[r.ID()] = r.Status()
	}
}

func (m *model) assertUnchanged(t *rapid.T, before, op string, err error) {
	if err == nil {
		return
	}
	if got := m.snapshot(); got != before {
		t.Fatalf("%s falló (%v) pero cambió el estado:\nantes   %s\ndespués %s", op, err, before, got)
	}
}

func (m *model) newAdjustment(amt decimal.Decimal) receivable.Adjustment {
	return receivable.Adjustment{ID: m.newID(), Amount: amt, SourceDocumentID: m.newID(), SourceEventID: m.newID()}
}

func (m *model) paymentsOf(r *receivable.Receivable) map[uuid.UUID]*payment.Payment {
	out := map[uuid.UUID]*payment.Payment{}
	for _, id := range r.ActivePaymentIDs() {
		out[id] = m.payments[id]
	}
	return out
}

func (m *model) drawPayment(t *rapid.T) *payment.Payment {
	if len(m.payments) == 0 {
		t.Skip("no hay pagos")
	}
	return m.payments[rapid.SampledFrom(sortedKeys(m.payments)).Draw(t, "payment")]
}

// drawPaymentFor prefiere un pago del mismo cliente y moneda (si no, casi ninguna aplicación prosperaría), pero a
// veces toma cualquiera para probar customer-mismatch y currency-mismatch.
func (m *model) drawPaymentFor(t *rapid.T, r *receivable.Receivable) *payment.Payment {
	var matching []uuid.UUID
	for _, id := range sortedKeys(m.payments) {
		if p := m.payments[id]; p.CustomerID() == r.CustomerID() && p.Currency() == r.Currency() {
			matching = append(matching, id)
		}
	}
	if len(matching) == 0 || rapid.IntRange(0, 4).Draw(t, "anyPayment") == 0 {
		return m.drawPayment(t)
	}
	return m.payments[rapid.SampledFrom(matching).Draw(t, "payment")]
}

func (m *model) drawReceivable(t *rapid.T) *receivable.Receivable {
	if len(m.receivables) == 0 {
		t.Skip("no hay cuentas")
	}
	return m.receivables[rapid.SampledFrom(sortedKeys(m.receivables)).Draw(t, "receivable")]
}

func (m *model) drawActiveApplication(t *rapid.T) (*payment.Payment, payment.Application) {
	type pair struct {
		p *payment.Payment
		a payment.Application
	}
	var active []pair
	for _, id := range sortedKeys(m.payments) {
		for _, a := range m.payments[id].Applications() {
			if a.Active() {
				active = append(active, pair{m.payments[id], a})
			}
		}
	}
	if len(active) == 0 {
		t.Skip("no hay aplicaciones vigentes")
	}
	i := rapid.IntRange(0, len(active)-1).Draw(t, "application")
	return active[i].p, active[i].a
}

func (m *model) findApplication(id uuid.UUID) (*payment.Payment, payment.Application) {
	for _, p := range m.payments {
		for _, a := range p.Applications() {
			if a.ID == id {
				return p, a
			}
		}
	}
	panic("aplicación inexistente")
}

// snapshot es una foto textual de todo el modelo (decimal.String no depende de la escala interna).
func (m *model) snapshot() string {
	var b strings.Builder
	for _, id := range sortedKeys(m.receivables) {
		b.WriteString(snapReceivable(m.receivables[id], false))
	}
	for _, id := range sortedKeys(m.payments) {
		b.WriteString(snapPayment(m.payments[id], false))
	}
	return b.String()
}

// snapReceivable y snapPayment: con activeOnly, una aplicación revertida no cuenta (la reversión queda como
// historial, pero los saldos y el estado deben ser los de antes de aplicarla).
func snapReceivable(r *receivable.Receivable, activeOnly bool) string {
	s := fmt.Sprintf("R %s %s %s adj=%d;", r.ID(), r.Status(), r.Balance(), len(r.Adjustments()))
	for _, a := range r.Applications() {
		if a.Active() || !activeOnly {
			s += fmt.Sprintf(" %s:%s:%t", a.ID, a.Amount, a.Active())
		}
	}
	return s + "\n"
}

func snapPayment(p *payment.Payment, activeOnly bool) string {
	s := fmt.Sprintf("P %s %s %s;", p.ID(), p.Status(), p.Available())
	for _, a := range p.Applications() {
		if a.Active() || !activeOnly {
			s += fmt.Sprintf(" %s:%s:%t", a.ID, a.Amount, a.Active())
		}
	}
	return s + "\n"
}

func hasActive(p *payment.Payment, receivableID uuid.UUID) bool {
	return slices.ContainsFunc(p.Applications(), func(a payment.Application) bool {
		return a.Active() && a.ReceivableID == receivableID
	})
}

func appliedTo(r *receivable.Receivable) decimal.Decimal {
	sum := decimal.Zero
	for _, a := range r.Applications() {
		if a.Active() {
			sum = sum.Add(a.Amount)
		}
	}
	return sum
}

// drawPositive genera montos válidos de numeric(18,5), con 0 a 5 decimales.
func drawPositive(t *rapid.T, label string) decimal.Decimal {
	return decimal.New(rapid.Int64Range(1, 50_000_000).Draw(t, label), -rapid.Int32Range(0, 5).Draw(t, label+"Scale"))
}

// drawAmount elige entre los bordes que importan (los montos exactos que llevan a paid o al límite del pago) y un
// monto cualquiera, que a veces es inválido (0, negativo o con 6 decimales).
func drawAmount(t *rapid.T, edges ...decimal.Decimal) decimal.Decimal {
	switch rapid.IntRange(0, 5).Draw(t, "amountKind") {
	case 0, 1:
		return rapid.SampledFrom(edges).Draw(t, "edge")
	case 2:
		return rapid.SampledFrom([]decimal.Decimal{decimal.Zero, decimal.New(-1, 0), decimal.New(1, -6)}).Draw(t, "invalid")
	default:
		return drawPositive(t, "amount")
	}
}

func sortedKeys[V any](m map[uuid.UUID]V) []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b uuid.UUID) int { return cmp.Compare(a.String(), b.String()) })
	return keys
}
