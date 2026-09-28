package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/aging"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
	"rdl/receivables-api/pkg/correlation"
)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func sept(day int) civil.Date { return civil.Date{Year: 2026, Month: time.September, Day: day} }

func newCreatePayment(s *memStore) *CreatePayment {
	uc := NewCreatePayment(s)
	uc.now = fixedNow
	return uc
}

// Un pago con dos aplicaciones: una salda su cuenta (ReceivableSettled) y la otra la deja parcial. PaymentReceived
// lleva las dos, todo con el correlationId del request, y las cuentas se bloquean en orden de id.
func TestCreatePaymentWithApplications(t *testing.T) {
	s := newMemStore()
	r1, r2 := s.seedReceivable(t, "100"), s.seedReceivable(t, "300")
	cid := uuid.New()
	ctx := correlation.WithID(context.Background(), cid)
	in := PaymentInput{
		CustomerID: customerA, ReceivedOn: sept(25), Amount: dec("250.5"), Currency: "CRC", PaymentMethodCode: "04",
		Applications: []ApplicationInput{{ReceivableID: r2, Amount: dec("150.5")}, {ReceivableID: r1, Amount: dec("100")}},
	}
	res, err := newCreatePayment(s).Execute(ctx, s.tenant("collector"), "k1", in)
	if err != nil {
		t.Fatal(err)
	}
	if s.status(r1) != receivable.StatusPaid || s.balance(r2) != "149.5" || s.status(r2) != receivable.StatusPartiallyPaid {
		t.Errorf("r1 %s, r2 %s %s", s.status(r1), s.balance(r2), s.status(r2))
	}
	if len(s.received) != 1 || len(s.received[0].Applications) != 2 || s.received[0].CorrelationID != cid {
		t.Fatalf("PaymentReceived: %+v", s.received)
	}
	if len(s.settled) != 1 || s.settled[0].ReceivableID != r1 || s.settled[0].CorrelationID != cid || s.settled[0].DocumentNumber == "" {
		t.Errorf("ReceivableSettled: %+v", s.settled)
	}
	if len(s.audits) != 1 || s.audits[0].Action != "payment.created" || s.audits[0].ActorType != ActorUser || s.audits[0].CorrelationID != cid {
		t.Errorf("audit: %+v", s.audits)
	}
	want := sortedUnique([]uuid.UUID{r1, r2})
	if !slices.Equal(s.locks, []string{"R:" + want[0].String(), "R:" + want[1].String()}) {
		t.Errorf("orden de bloqueo: %v", s.locks)
	}

	// Repetición: el mismo pago, sin efectos nuevos. Otra petición con la misma clave: 422.
	again, err := newCreatePayment(s).Execute(ctx, s.tenant("collector"), "k1", in)
	if err != nil || !again.Replayed || again.Value.ID != res.Value.ID || len(s.received) != 1 || len(s.payments) != 1 {
		t.Errorf("repetición: %+v, %v", again, err)
	}
	in.Reference = "otra"
	if _, err := newCreatePayment(s).Execute(ctx, s.tenant("collector"), "k1", in); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("clave reutilizada: %v", err)
	}
}

// Si una aplicación falla, no queda nada: ni el pago, ni las aplicaciones anteriores, ni eventos, ni la clave.
func TestCreatePaymentIsAtomic(t *testing.T) {
	s := newMemStore()
	r1, r2 := s.seedReceivable(t, "100"), s.seedReceivable(t, "10")
	in := PaymentInput{
		CustomerID: customerA, ReceivedOn: sept(25), Amount: dec("200"), Currency: "CRC", PaymentMethodCode: "04",
		Applications: []ApplicationInput{{ReceivableID: r1, Amount: dec("100")}, {ReceivableID: r2, Amount: dec("11")}},
	}
	_, err := newCreatePayment(s).Execute(context.Background(), s.tenant("owner"), "k", in)
	if !errors.Is(err, receivable.ErrExceedsBalance) {
		t.Fatalf("se esperaba ErrExceedsBalance: %v", err)
	}
	if len(s.payments) != 0 || len(s.apps) != 0 || len(s.received) != 0 || len(s.audits) != 0 || len(s.idem) != 0 {
		t.Errorf("quedaron efectos: pagos %d, aplicaciones %d, eventos %d, audits %d, claves %d",
			len(s.payments), len(s.apps), len(s.received), len(s.audits), len(s.idem))
	}
	if s.status(r1) != receivable.StatusOpen {
		t.Error("la primera cuenta no debía quedar pagada")
	}
}

func TestCreatePaymentValidation(t *testing.T) {
	s := newMemStore()
	r := s.seedReceivable(t, "100")
	base := PaymentInput{CustomerID: customerA, ReceivedOn: sept(25), Amount: dec("1"), Currency: "CRC", PaymentMethodCode: "04"}

	// 18:00 UTC del 25 es 12:00 del 25 en Costa Rica: el 26 todavía es futuro para la organización.
	future := base
	future.ReceivedOn = sept(26)
	var verr ValidationError
	if _, err := newCreatePayment(s).Execute(context.Background(), s.tenant("owner"), "a", future); !errors.As(err, &verr) || verr.Field != "receivedOn" {
		t.Errorf("fecha futura: %v", err)
	}
	dup := base
	dup.Applications = []ApplicationInput{{ReceivableID: r, Amount: dec("1")}, {ReceivableID: r, Amount: dec("1")}}
	if _, err := newCreatePayment(s).Execute(context.Background(), s.tenant("owner"), "b", dup); !errors.As(err, &verr) || verr.Field != "applications" {
		t.Errorf("cuenta repetida: %v", err)
	}
	for _, role := range []string{"accountant", "read_only", "biller"} {
		if _, err := newCreatePayment(s).Execute(context.Background(), s.tenant(role), "c", base); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: %v", role, err)
		}
	}
	other := base
	other.Applications = []ApplicationInput{{ReceivableID: uuid.New(), Amount: dec("1")}}
	if _, err := newCreatePayment(s).Execute(context.Background(), s.tenant("owner"), "d", other); !errors.Is(err, ErrNotFound) {
		t.Errorf("cuenta de otra organización o inexistente: %v", err)
	}
	usd := base
	usd.Currency = "USD"
	usd.Applications = []ApplicationInput{{ReceivableID: r, Amount: dec("1")}}
	if _, err := newCreatePayment(s).Execute(context.Background(), s.tenant("owner"), "e", usd); !errors.Is(err, settlement.ErrCurrencyMismatch) {
		t.Errorf("otra moneda: %v", err)
	}
}

// Aplicar bloquea primero el pago y después la cuenta; si la salda, ReceivableSettled.
func TestApplyAndReverse(t *testing.T) {
	s := newMemStore()
	r, p := s.seedReceivable(t, "100"), s.seedPayment(t, "100")
	apply := NewApplyPayment(s)
	res, err := apply.Execute(context.Background(), s.tenant("collector"), "k", ApplyInput{PaymentID: p, ReceivableID: r, Amount: dec("100")})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.locks, []string{"P:" + p.String(), "R:" + r.String()}) {
		t.Errorf("orden de bloqueo: %v", s.locks)
	}
	if s.status(r) != receivable.StatusPaid || len(s.settled) != 1 {
		t.Errorf("estado %s, eventos %d", s.status(r), len(s.settled))
	}
	if _, err := apply.Execute(context.Background(), s.tenant("accountant"), "x", ApplyInput{PaymentID: p, ReceivableID: r, Amount: dec("1")}); !errors.Is(err, ErrForbidden) {
		t.Errorf("accountant: %v", err)
	}

	rev := NewReversePaymentApplication(s)
	if _, err := rev.Execute(context.Background(), s.tenant("collector"), "r", res.Value.ID, "Error"); !errors.Is(err, ErrForbidden) {
		t.Errorf("collector no revierte: %v", err)
	}
	if _, err := rev.Execute(context.Background(), s.tenant("admin"), "r", res.Value.ID, "Error de digitación"); err != nil {
		t.Fatal(err)
	}
	if s.status(r) != receivable.StatusOpen || s.balance(r) != "100" || len(s.settled) != 1 {
		t.Errorf("después del reverso: %s %s, eventos %d", s.status(r), s.balance(r), len(s.settled))
	}
	if _, err := rev.Execute(context.Background(), s.tenant("admin"), "r2", res.Value.ID, "otra vez"); !errors.Is(err, payment.ErrApplicationReversed) {
		t.Errorf("revertir dos veces: %v", err)
	}
	if _, err := rev.Execute(context.Background(), s.tenant("admin"), "r3", uuid.New(), "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("aplicación inexistente: %v", err)
	}
}

// Anular bloquea el pago y después sus cuentas en orden de id, revierte todo y no emite eventos (R7).
func TestVoidPayment(t *testing.T) {
	s := newMemStore()
	r1, r2, p := s.seedReceivable(t, "100"), s.seedReceivable(t, "100"), s.seedPayment(t, "150")
	apply := NewApplyPayment(s)
	for _, a := range []struct {
		r   uuid.UUID
		amt string
	}{{r1, "100"}, {r2, "50"}} {
		if _, err := apply.Execute(context.Background(), s.tenant("owner"), uuid.NewString(), ApplyInput{PaymentID: p, ReceivableID: a.r, Amount: dec(a.amt)}); err != nil {
			t.Fatal(err)
		}
	}
	events := len(s.settled)
	res, err := NewVoidPayment(s).Execute(context.Background(), s.tenant("owner"), "v", p, "Cheque rechazado")
	if err != nil {
		t.Fatal(err)
	}
	want := sortedUnique([]uuid.UUID{r1, r2})
	if !slices.Equal(s.locks, []string{"P:" + p.String(), "R:" + want[0].String(), "R:" + want[1].String()}) {
		t.Errorf("orden de bloqueo: %v", s.locks)
	}
	if res.Value.Status != payment.StatusVoided || s.balance(r1) != "100" || s.balance(r2) != "100" || len(s.settled) != events {
		t.Errorf("pago %s, saldos %s %s, eventos %d", res.Value.Status, s.balance(r1), s.balance(r2), len(s.settled)-events)
	}
	for _, a := range s.apps {
		if a.reversedAt.IsZero() || a.reason != "Cheque rechazado" {
			t.Errorf("aplicación sin revertir: %+v", a)
		}
	}
	if _, err := NewVoidPayment(s).Execute(context.Background(), s.tenant("owner"), "v2", p, "otra"); !errors.Is(err, payment.ErrVoided) {
		t.Errorf("anular dos veces: %v", err)
	}
}

func noteEvent(s *memStore, body any) IncomingEvent {
	return IncomingEvent{Ref: EventRef{ID: uuid.New(), Type: "x", OrganizationID: s.org, CorrelationID: uuid.New()}, Body: body}
}

func handleOnce(t *testing.T, s *memStore, h EventHandler, ev IncomingEvent) error {
	t.Helper()
	return s.WithinServiceTx(context.Background(), s.org, func(ctx context.Context, tx EventTx) error {
		return h.Handle(ctx, tx, ev)
	})
}

// R3 con varios pagos: la nota revierte primero la aplicación más reciente y los pagos se bloquean antes que la cuenta.
func TestCreditNoteEvent(t *testing.T) {
	s := newMemStore()
	r := s.seedReceivable(t, "100")
	p1, p2 := s.seedPayment(t, "60"), s.seedPayment(t, "40")
	apply := NewApplyPayment(s)
	for i, p := range []uuid.UUID{p1, p2} {
		apply.now = func() time.Time { return fixedNow().Add(time.Duration(i) * time.Minute) }
		if _, err := apply.Execute(context.Background(), s.tenant("owner"), uuid.NewString(), ApplyInput{PaymentID: p, ReceivableID: r, Amount: s.payments[p].data.Amount}); err != nil {
			t.Fatal(err)
		}
	}
	inv := s.receivables[r].inv.ID
	ev := noteEvent(s, CreditNoteIssued{DocumentID: uuid.New(), DocumentNumber: "NC-1", InvoiceID: inv, Currency: "CRC", Total: dec("30")})
	if err := handleOnce(t, s, NewApplyCreditNote(), ev); err != nil {
		t.Fatal(err)
	}
	pays := sortedUnique([]uuid.UUID{p1, p2})
	if !slices.Equal(s.locks, []string{"P:" + pays[0].String(), "P:" + pays[1].String(), "R:" + r.String()}) {
		t.Errorf("orden de bloqueo: %v", s.locks)
	}
	// Se revirtió solo la de p2 (la más reciente, 40): saldo 100 − 30 − 60 = 10.
	if s.balance(r) != "10" || s.status(r) != receivable.StatusPartiallyPaid {
		t.Errorf("saldo %s %s", s.balance(r), s.status(r))
	}
	for _, a := range s.apps {
		if (a.paymentID == p2) == a.reversedAt.IsZero() {
			t.Errorf("reversión equivocada: %+v", a)
		}
	}
	// El mismo documento en otro evento no cambia nada.
	dup := ev
	dup.Ref.ID = uuid.New()
	before := s.balance(r)
	if err := handleOnce(t, s, NewApplyCreditNote(), dup); err != nil || s.balance(r) != before {
		t.Errorf("documento repetido: %v, saldo %s", err, s.balance(r))
	}
	// Otra nota que la deja en cero: ReceivableSettled con el correlationId del evento.
	last := noteEvent(s, CreditNoteIssued{DocumentID: uuid.New(), InvoiceID: inv, Currency: "CRC", Total: dec("10")})
	settledBefore := len(s.settled)
	if err := handleOnce(t, s, NewApplyCreditNote(), last); err != nil {
		t.Fatal(err)
	}
	if s.status(r) != receivable.StatusPaid || len(s.settled) != settledBefore+1 || s.settled[len(s.settled)-1].CorrelationID != last.Ref.CorrelationID {
		t.Errorf("saldada por nota: %s, eventos %d", s.status(r), len(s.settled)-settledBefore)
	}
}

func TestNoteEventErrors(t *testing.T) {
	s := newMemStore()
	r := s.seedReceivable(t, "100")
	inv := s.receivables[r].inv.ID
	cases := []struct {
		name      string
		h         EventHandler
		body      any
		permanent bool
		want      error
	}{
		{"factura que no llegó", NewApplyCreditNote(), CreditNoteIssued{DocumentID: uuid.New(), InvoiceID: uuid.New(), Currency: "CRC", Total: dec("1")}, false, ErrInvoiceNotYetReceived},
		{"otra moneda", NewApplyDebitNote(), DebitNoteIssued{DocumentID: uuid.New(), InvoiceID: inv, Currency: "USD", Total: dec("1")}, true, ErrDocumentMismatch},
		{"crédito mayor que la deuda", NewApplyCreditNote(), CreditNoteIssued{DocumentID: uuid.New(), InvoiceID: inv, Currency: "CRC", Total: dec("100.00001")}, true, receivable.ErrCreditExceedsDebt},
		{"anulación con otro total", NewCancelReceivable(), InvoiceCancelled{InvoiceID: inv, Currency: "CRC", Total: dec("99")}, true, ErrDocumentMismatch},
		{"cuerpo de otro tipo", NewCancelReceivable(), InvoiceIssued{}, true, ErrInvalidEvent},
	}
	for _, c := range cases {
		err := handleOnce(t, s, c.h, noteEvent(s, c.body))
		if !errors.Is(err, c.want) || permanent(err) != c.permanent {
			t.Errorf("%s: %v (permanente=%v)", c.name, err, permanent(err))
		}
	}
	if s.balance(r) != "100" || len(s.receivables[r].adjs) != 0 {
		t.Error("un evento rechazado no debe dejar ajustes")
	}
}

// R2 y R4: la nota de débito reabre una cuenta pagada sin cambiar el vencimiento; la anulación revierte todo.
func TestDebitNoteAndCancellation(t *testing.T) {
	s := newMemStore()
	r, p := s.seedReceivable(t, "100"), s.seedPayment(t, "100")
	if _, err := NewApplyPayment(s).Execute(context.Background(), s.tenant("owner"), "k", ApplyInput{PaymentID: p, ReceivableID: r, Amount: dec("100")}); err != nil {
		t.Fatal(err)
	}
	inv := s.receivables[r].inv
	if err := handleOnce(t, s, NewApplyDebitNote(), noteEvent(s, DebitNoteIssued{DocumentID: uuid.New(), InvoiceID: inv.ID, Currency: "CRC", Total: dec("25"), DueDate: civil.Date{Year: 2027, Month: 1, Day: 1}})); err != nil {
		t.Fatal(err)
	}
	if s.status(r) != receivable.StatusPartiallyPaid || s.balance(r) != "25" || s.rehydrate(r).Invoice().DueOn != inv.DueOn {
		t.Errorf("después de la nota de débito: %s %s", s.status(r), s.balance(r))
	}
	if err := handleOnce(t, s, NewCancelReceivable(), noteEvent(s, InvoiceCancelled{InvoiceID: inv.ID, Currency: "CRC", Total: dec("100"), Reason: "Error"})); err != nil {
		t.Fatal(err)
	}
	if s.status(r) != receivable.StatusCancelled || s.balance(r) != "0" {
		t.Errorf("anulada: %s %s", s.status(r), s.balance(r))
	}
	pv, _ := memPayments{s}.Get(context.Background(), s.org, p)
	if len(pv.Applications) != 1 || pv.Applications[0].ReversedAt == nil || pv.Applications[0].ReversalReason != receivable.ReasonInvoiceCancelled {
		t.Errorf("la aplicación debía revertirse con el motivo de R2: %+v", pv.Applications)
	}
	if last := s.audits[len(s.audits)-1]; last.Action != "receivable.cancelled" || last.ActorType != ActorService {
		t.Errorf("audit: %+v", last)
	}
}

func TestBuildAging(t *testing.T) {
	asOf := civil.Date{Year: 2026, Month: time.September, Day: 30}
	rows := []AgingRow{
		{Currency: "USD", DueOn: sept(30), Balance: dec("10.5")},
		{Currency: "CRC", DueOn: sept(29), Balance: dec("0.00001")},
		{Currency: "CRC", DueOn: civil.Date{Year: 2026, Month: time.August, Day: 31}, Balance: dec("100")}, // 30 días
		{Currency: "CRC", DueOn: civil.Date{Year: 2026, Month: time.August, Day: 30}, Balance: dec("1")},   // 31 días
		{Currency: "CRC", DueOn: civil.Date{Year: 2025, Month: time.January, Day: 1}, Balance: dec("7")},
	}
	rep := buildAging(asOf, rows)
	got := map[string]string{}
	for _, b := range rep.Buckets {
		got[b.Currency+"/"+string(b.Bucket)] = b.Balance.String()
	}
	want := map[string]string{
		"CRC/current": "0", "CRC/1_30": "100.00001", "CRC/31_60": "1", "CRC/61_90": "0", "CRC/90_plus": "7",
		"USD/current": "10.5", "USD/1_30": "0", "USD/31_60": "0", "USD/61_90": "0", "USD/90_plus": "0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %s, se esperaba %s", k, got[k], v)
		}
	}
	if len(rep.Buckets) != 2*len(aging.Buckets) || rep.Buckets[0].Currency != "CRC" {
		t.Errorf("orden: %+v", rep.Buckets)
	}
}

func TestPromises(t *testing.T) {
	s := newMemStore()
	r := s.seedReceivable(t, "100")
	create := NewCreatePromise(s)
	create.now = fixedNow
	if _, err := create.Execute(context.Background(), s.tenant("collector"), "a", r, PromiseInput{Amount: dec("100.00001"), PromisedOn: sept(30)}); !errors.Is(err, collection.ErrPromiseExceedsBalance) {
		t.Errorf("mayor que el saldo: %v", err)
	}
	if _, err := create.Execute(context.Background(), s.tenant("collector"), "b", r, PromiseInput{Amount: dec("10"), PromisedOn: sept(24)}); !errors.Is(err, collection.ErrPromiseInPast) {
		t.Errorf("fecha pasada: %v", err)
	}
	res, err := create.Execute(context.Background(), s.tenant("collector"), "c", r, PromiseInput{Amount: dec("40"), PromisedOn: sept(25)})
	if err != nil {
		t.Fatal(err)
	}
	closeUC := NewClosePromise(s)
	if _, err := closeUC.Execute(context.Background(), s.tenant("read_only"), "d", res.Value.ID, collection.PromiseKept); !errors.Is(err, ErrForbidden) {
		t.Errorf("read_only: %v", err)
	}
	if _, err := closeUC.Execute(context.Background(), s.tenant("collector"), "e", res.Value.ID, collection.PromiseBroken); err != nil {
		t.Fatal(err)
	}
	if _, err := closeUC.Execute(context.Background(), s.tenant("collector"), "f", res.Value.ID, collection.PromiseKept); !errors.Is(err, collection.ErrPromiseClosed) {
		t.Errorf("cerrar dos veces: %v", err)
	}
	if !strings.HasPrefix(s.audits[len(s.audits)-1].Action, "payment_promise.broken") {
		t.Errorf("audit: %+v", s.audits[len(s.audits)-1])
	}
}

// Lecturas: permisos de la matriz (R10: biller solo ve el saldo por factura) y 404 para lo que no existe.
func TestReadUseCases(t *testing.T) {
	s := newMemStore()
	r, p := s.seedReceivable(t, "100"), s.seedPayment(t, "10")
	inv := s.receivables[r].inv.ID
	ctx := context.Background()

	if v, err := NewGetBalanceByInvoice(s).Execute(ctx, s.tenant("biller"), inv); err != nil || v.ID != r || v.BalanceAmount != "100" {
		t.Errorf("biller ve el saldo por factura: %+v, %v", v, err)
	}
	if _, err := NewGetBalanceByInvoice(s).Execute(ctx, s.tenant("read_only"), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("factura sin cuenta: %v", err)
	}
	// Por lote: la factura sin cuenta no viene (no es error), y los mismos permisos que la de a una.
	if vs, err := NewGetBalancesByInvoice(s).Execute(ctx, s.tenant("biller"), []uuid.UUID{inv, uuid.New()}); err != nil || len(vs) != 1 || vs[0].ID != r {
		t.Errorf("biller ve los saldos por lote: %+v, %v", vs, err)
	}
	for name, run := range map[string]func(string) error{
		"detalle de la cuenta": func(role string) error {
			_, err := NewGetReceivable(s).Execute(ctx, s.tenant(role), r)
			return err
		},
		"detalle del pago": func(role string) error {
			_, err := NewGetPayment(s).Execute(ctx, s.tenant(role), p)
			return err
		},
		"listado de pagos": func(role string) error {
			_, err := NewListPayments(s).Execute(ctx, s.tenant(role), PaymentQuery{Limit: 500})
			return err
		},
		"aging": func(role string) error {
			_, err := NewGetAging(s).Execute(ctx, s.tenant(role), AgingQuery{})
			return err
		},
		"seguimientos": func(role string) error {
			_, err := NewListFollowUps(s).Execute(ctx, s.tenant(role), r)
			return err
		},
		"promesas": func(role string) error {
			_, err := NewListPromises(s).Execute(ctx, s.tenant(role), r)
			return err
		},
	} {
		if err := run("accountant"); err != nil {
			t.Errorf("%s con accountant: %v", name, err)
		}
		if err := run("biller"); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s con biller: %v", name, err)
		}
	}
	if _, err := NewGetPayment(s).Execute(ctx, s.tenant("owner"), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("pago inexistente: %v", err)
	}
	if _, err := NewListFollowUps(s).Execute(ctx, s.tenant("owner"), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("seguimientos de una cuenta inexistente: %v", err)
	}
}

func TestFollowUps(t *testing.T) {
	s := newMemStore()
	r := s.seedReceivable(t, "100")
	uc := NewCreateFollowUp(s)
	next := sept(30)
	res, err := uc.Execute(context.Background(), s.tenant("collector"), "k", r, FollowUpInput{Type: collection.FollowUpCall, Notes: "Llamó", NextActionOn: &next})
	if err != nil || res.Value.ReceivableID != r || s.audits[0].Action != "collection_followup.created" {
		t.Fatalf("%+v, %v", res, err)
	}
	again, err := uc.Execute(context.Background(), s.tenant("collector"), "k", r, FollowUpInput{Type: collection.FollowUpCall, Notes: "Llamó", NextActionOn: &next})
	if err != nil || !again.Replayed || len(s.followUps) != 1 {
		t.Errorf("repetición: %+v, %v", again, err)
	}
	if _, err := uc.Execute(context.Background(), s.tenant("collector"), "k2", uuid.New(), FollowUpInput{Type: collection.FollowUpNote, Notes: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("cuenta inexistente: %v", err)
	}
	if _, err := uc.Execute(context.Background(), s.tenant("collector"), "k3", r, FollowUpInput{Type: "fax", Notes: "x"}); !errors.Is(err, collection.ErrInvalidType) {
		t.Errorf("tipo inválido: %v", err)
	}
	// Una promesa ligada a un seguimiento de otra cuenta se rechaza.
	other := s.seedReceivable(t, "50")
	create := NewCreatePromise(s)
	create.now = fixedNow
	var verr ValidationError
	if _, err := create.Execute(context.Background(), s.tenant("collector"), "p", other, PromiseInput{Amount: dec("1"), PromisedOn: sept(30), FollowUpID: res.Value.ID}); !errors.As(err, &verr) {
		t.Errorf("seguimiento de otra cuenta: %v", err)
	}
}
