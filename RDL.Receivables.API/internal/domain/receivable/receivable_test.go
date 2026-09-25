package receivable

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
)

var customer = uuid.MustParse("00000000-0000-0000-0000-00000000c001")

func invoice(original string) Invoice {
	return Invoice{
		ID:         newID(),
		CustomerID: customer,
		Currency:   "CRC",
		Original:   dec(original),
		IssuedOn:   civil.Date{Year: 2026, Month: time.September, Day: 1},
		DueOn:      civil.Date{Year: 2026, Month: time.October, Day: 1},
	}
}

// El estado se deriva del saldo frente al total adeudado (original + débitos), no de si hay aplicaciones (R1).
func TestDerive(t *testing.T) {
	cases := []struct {
		name   string
		totals Totals
		want   Status
		saldo  string
	}{
		{"sin movimientos", Totals{Original: dec("100")}, StatusOpen, "100"},
		{"nota de débito sin pagos sigue open", Totals{Original: dec("100"), Debits: dec("20")}, StatusOpen, "120"},
		{"nota de crédito parcial sin pagos", Totals{Original: dec("100"), Credits: dec("0.00001")}, StatusPartiallyPaid, "99.99999"},
		{"pago parcial", Totals{Original: dec("100"), Applied: dec("40")}, StatusPartiallyPaid, "60"},
		{"pagada con pagos y notas", Totals{Original: dec("100"), Debits: dec("5"), Credits: dec("30"), Applied: dec("75")}, StatusPaid, "0"},
		{"anulada", Totals{Original: dec("100"), Cancellation: dec("100"), Cancelled: true}, StatusCancelled, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			saldo, status, err := Derive(c.totals)
			if err != nil || status != c.want || !saldo.Equal(dec(c.saldo)) {
				t.Errorf("Derive = %s, %s, %v; se esperaba %s, %s", saldo, status, err, c.saldo, c.want)
			}
		})
	}
	if _, _, err := Derive(Totals{Original: dec("100"), Applied: dec("100.00001")}); !errors.Is(err, ErrNegativeBalance) {
		t.Errorf("saldo negativo: %v", err)
	}
}

func TestNewValidatesInvoice(t *testing.T) {
	bad := map[string]func(*Invoice){
		"sin id":                  func(i *Invoice) { i.ID = uuid.Nil },
		"sin cliente":             func(i *Invoice) { i.CustomerID = uuid.Nil },
		"sin moneda":              func(i *Invoice) { i.Currency = "" },
		"monto cero":              func(i *Invoice) { i.Original = decimal.Zero },
		"seis decimales":          func(i *Invoice) { i.Original = dec("1.000001") },
		"vence antes de emitirse": func(i *Invoice) { i.DueOn = civil.Date{Year: 2026, Month: time.August, Day: 31} },
		"sin fecha de emisión":    func(i *Invoice) { i.IssuedOn = civil.Date{} },
	}
	for name, mutate := range bad {
		inv := invoice("100")
		mutate(&inv)
		if _, err := New(newID(), inv); err == nil {
			t.Errorf("%s: se aceptó", name)
		}
	}
	inv := invoice("100")
	inv.DueOn = inv.IssuedOn // contado: vence el mismo día
	if _, err := New(newID(), inv); err != nil {
		t.Errorf("vence el día de emisión: %v", err)
	}
}

func TestRehydrateRejectsInconsistentData(t *testing.T) {
	inv := invoice("100")
	apps := []Application{{ID: newID(), PaymentID: newID(), Amount: dec("150"), AppliedAt: tick()}}
	if _, err := Rehydrate(newID(), inv, nil, apps); !errors.Is(err, ErrInconsistent) {
		t.Errorf("aplicado mayor que la deuda: %v", err)
	}
	r, err := Rehydrate(newID(), inv, []Adjustment{{ID: newID(), Type: AdjustmentDebitNote, Amount: dec("50")}}, apps)
	if err != nil || !r.Balance().IsZero() || r.Status() != StatusPaid {
		t.Errorf("con nota de débito: %v, %v", r, err)
	}
}

func TestApplyRules(t *testing.T) {
	r := mustNew(t, "100")
	pay := newID()
	if _, err := r.Apply(Application{ID: newID(), PaymentID: pay, Amount: dec("30"), AppliedAt: tick()}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		pay  uuid.UUID
		amt  string
		want error
	}{
		{"mismo pago con aplicación vigente", pay, "10", ErrDuplicateApplication},
		{"mayor que el saldo", newID(), "70.00001", ErrExceedsBalance},
		{"cero", newID(), "0", amount.ErrInvalid},
		{"negativo", newID(), "-1", amount.ErrInvalid},
		{"seis decimales", newID(), "0.000001", amount.ErrInvalid},
	}
	for _, c := range cases {
		before := r.Balance()
		_, err := r.Apply(Application{ID: newID(), PaymentID: c.pay, Amount: dec(c.amt), AppliedAt: tick()})
		if !errors.Is(err, c.want) || !r.Balance().Equal(before) {
			t.Errorf("%s: error %v (se esperaba %v), saldo %s", c.name, err, c.want, r.Balance())
		}
	}
	if _, err := r.Apply(Application{ID: newID(), PaymentID: newID(), Amount: dec("70"), AppliedAt: tick()}); err != nil {
		t.Errorf("exactamente el saldo: %v", err)
	}
}

func TestSamePaymentCanApplyAgainAfterReversal(t *testing.T) {
	r := mustNew(t, "100")
	pay, first := newID(), newID()
	if _, err := r.Apply(Application{ID: first, PaymentID: pay, Amount: dec("30"), AppliedAt: tick()}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReverseApplication(first, "Monto equivocado", tick()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Apply(Application{ID: newID(), PaymentID: pay, Amount: dec("35"), AppliedAt: tick()}); err != nil {
		t.Errorf("volver a aplicar el mismo pago después de revertir: %v", err)
	}
	if len(r.Applications()) != 2 {
		t.Errorf("la aplicación revertida debe quedar como historial: %d", len(r.Applications()))
	}
}

func TestReverseRules(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "30")
	id := r.Applications()[0].ID
	if _, err := r.ReverseApplication(id, "", tick()); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("sin motivo: %v", err)
	}
	if _, err := r.ReverseApplication(newID(), "Motivo", tick()); !errors.Is(err, ErrApplicationNotFound) {
		t.Errorf("de otra cuenta: %v", err)
	}
	if _, err := r.ReverseApplication(id, "Motivo", time.Time{}); !errors.Is(err, ErrInconsistent) {
		t.Errorf("sin instante: %v", err)
	}
	if _, err := r.ReverseApplication(id, "Motivo", tick()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReverseApplication(id, "Motivo", tick()); !errors.Is(err, ErrApplicationReversed) {
		t.Errorf("dos veces: %v", err)
	}
	if got := r.Applications()[0]; got.ReversalReason != "Motivo" || got.Active() {
		t.Errorf("reversión no registrada: %+v", got)
	}
}

// R3: la nota de crédito mayor que el saldo revierte de la aplicación más reciente a la más antigua, solo hasta
// donde haga falta.
func TestCreditNoteReversesNewestFirstOnlyAsNeeded(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "20") // la más antigua: queda vigente
	apply(t, r, "30")
	apply(t, r, "40") // la más reciente
	apps := r.Applications()
	// saldo 10; nota de 60 → faltan 50: revertir 40 no alcanza, 40 + 30 sí.
	res, err := r.AddCreditNote(adjustment("60"), tick())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Reversed) != 2 || res.Reversed[0].ApplicationID != apps[2].ID || res.Reversed[1].ApplicationID != apps[1].ID {
		t.Fatalf("reversiones: %+v", res.Reversed)
	}
	for _, rev := range res.Reversed {
		if rev.Reason != ReasonCreditNote {
			t.Errorf("motivo %q", rev.Reason)
		}
	}
	// 100 − 60 − 20 = 20
	if !r.Balance().Equal(dec("20")) || r.Status() != StatusPartiallyPaid || !r.Applications()[0].Active() {
		t.Errorf("saldo %s, estado %s", r.Balance(), r.Status())
	}
}

func TestCreditNoteLimits(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "100")
	before := r.Balance()
	if _, err := r.AddCreditNote(adjustment("100.00001"), tick()); !errors.Is(err, ErrCreditExceedsDebt) {
		t.Errorf("mayor que el total adeudado: %v", err)
	}
	if !r.Balance().Equal(before) || !r.Applications()[0].Active() {
		t.Error("una nota rechazada no debe revertir nada")
	}
	res, err := r.AddCreditNote(adjustment("100"), tick())
	if err != nil || len(res.Reversed) != 1 || r.Status() != StatusPaid || res.Settled() {
		t.Errorf("igual al total: %+v, %v (sigue paid, no se vuelve a emitir ReceivableSettled)", res, err)
	}
}

// R4: la nota de débito reabre una cuenta pagada y no toca el vencimiento de la factura.
func TestDebitNoteReopens(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "100")
	due := r.Invoice().DueOn
	res, err := r.AddDebitNote(adjustment("15.5"))
	if err != nil || res.Change != (Change{StatusPaid, StatusPartiallyPaid}) || !r.Balance().Equal(dec("15.5")) {
		t.Errorf("%+v, %v, saldo %s", res, err, r.Balance())
	}
	if r.Invoice().DueOn != due {
		t.Error("la nota de débito no cambia el vencimiento")
	}
}

// R2: anular revierte todas las aplicaciones vigentes y registra la cancelación por el saldo que queda.
func TestCancel(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "25")
	apply(t, r, "35")
	if _, err := r.AddCreditNote(adjustment("10"), tick()); err != nil {
		t.Fatal(err)
	}
	res, err := r.Cancel(adjustment("999"), tick()) // el monto que llega se ignora
	if err != nil {
		t.Fatal(err)
	}
	adjs := r.Adjustments()
	last := adjs[len(adjs)-1]
	if len(res.Reversed) != 2 || last.Type != AdjustmentCancellation || !last.Amount.Equal(dec("90")) {
		t.Errorf("reversiones %d, ajuste %+v", len(res.Reversed), last)
	}
	if r.Status() != StatusCancelled || !r.Balance().IsZero() {
		t.Errorf("estado %s, saldo %s", r.Status(), r.Balance())
	}
	for _, rev := range res.Reversed {
		if rev.Reason != ReasonInvoiceCancelled {
			t.Errorf("motivo %q", rev.Reason)
		}
	}
}

func TestCancelledAcceptsNothing(t *testing.T) {
	r := mustNew(t, "100")
	cancel(t, r)
	if _, err := r.Apply(Application{ID: newID(), PaymentID: newID(), Amount: dec("1"), AppliedAt: tick()}); !errors.Is(err, ErrCancelled) {
		t.Errorf("aplicar: %v", err)
	}
	if _, err := r.AddDebitNote(adjustment("1")); !errors.Is(err, ErrCancelled) {
		t.Errorf("nota de débito (R4, dead letter): %v", err)
	}
	if _, err := r.AddCreditNote(adjustment("1"), tick()); !errors.Is(err, ErrCancelled) {
		t.Errorf("nota de crédito: %v", err)
	}
	if _, err := r.Cancel(adjustment("0"), tick()); !errors.Is(err, ErrCancelled) {
		t.Errorf("otra anulación con otro documento: %v", err)
	}
}

func TestCancelWithNothingLeft(t *testing.T) {
	r := mustNew(t, "100")
	creditNote(t, r, "100")
	if _, err := r.Cancel(adjustment("0"), tick()); !errors.Is(err, ErrNothingToCancel) {
		t.Errorf("saldada solo con notas de crédito: %v", err)
	}
}

// Invariante 4: el mismo documento reenviado es inocuo; con otro monto o con un evento ya usado, es un conflicto.
func TestAdjustmentIdempotency(t *testing.T) {
	r := mustNew(t, "100")
	adj := adjustment("10")
	if _, err := r.AddCreditNote(adj, tick()); err != nil {
		t.Fatal(err)
	}
	res, err := r.AddCreditNote(adj, tick())
	if err != nil || !res.Duplicate || len(r.Adjustments()) != 1 || !r.Balance().Equal(dec("90")) {
		t.Errorf("reenvío: %+v, %v", res, err)
	}
	other := adj
	other.ID, other.SourceEventID, other.Amount = newID(), newID(), dec("11")
	if _, err := r.AddCreditNote(other, tick()); !errors.Is(err, ErrConflictingAdjustment) {
		t.Errorf("mismo documento, otro monto: %v", err)
	}
	reused := adjustment("5")
	reused.SourceEventID = adj.SourceEventID
	if _, err := r.AddDebitNote(reused); !errors.Is(err, ErrConflictingAdjustment) {
		t.Errorf("evento ya usado por otro documento: %v", err)
	}
}

// Una anulación reenviada es un duplicado aunque la cuenta ya esté cancelled (el evento se reprocesa sin error).
func TestCancelIsIdempotent(t *testing.T) {
	r := mustNew(t, "100")
	adj := adjustment("0")
	if _, err := r.Cancel(adj, tick()); err != nil {
		t.Fatal(err)
	}
	res, err := r.Cancel(adj, tick())
	if err != nil || !res.Duplicate || res.To != StatusCancelled {
		t.Errorf("%+v, %v", res, err)
	}
}

func TestAdjustmentWithoutSource(t *testing.T) {
	r := mustNew(t, "100")
	adj := adjustment("10")
	adj.SourceEventID = uuid.Nil
	if _, err := r.AddDebitNote(adj); !errors.Is(err, ErrInconsistent) {
		t.Errorf("sin evento de origen: %v", err)
	}
}

func TestActivePaymentIDsSortedAndUnique(t *testing.T) {
	r := mustNew(t, "100")
	a, b := uuid.MustParse("00000000-0000-0000-0000-0000000000bb"), uuid.MustParse("00000000-0000-0000-0000-0000000000aa")
	for _, p := range []uuid.UUID{a, b} {
		if _, err := r.Apply(Application{ID: newID(), PaymentID: p, Amount: dec("10"), AppliedAt: tick()}); err != nil {
			t.Fatal(err)
		}
	}
	ids := r.ActivePaymentIDs()
	if len(ids) != 2 || ids[0] != b || ids[1] != a {
		t.Errorf("orden de bloqueo: %v", ids)
	}
}

// Los getters devuelven copias: modificar lo devuelto no toca el agregado.
func TestAccessorsReturnCopies(t *testing.T) {
	r := mustNew(t, "100")
	apply(t, r, "10")
	r.Applications()[0].ReversedAt = tick()
	if !r.Applications()[0].Active() {
		t.Error("Applications() expone el slice interno")
	}
}
