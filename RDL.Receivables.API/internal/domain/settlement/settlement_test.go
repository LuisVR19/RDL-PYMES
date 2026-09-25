package settlement

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

var at = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func newReceivable(t *testing.T, customer uuid.UUID, currency, original string) *receivable.Receivable {
	t.Helper()
	r, err := receivable.New(uuid.New(), receivable.Invoice{
		ID: uuid.New(), CustomerID: customer, Currency: currency, Original: dec(original), IssuedOn: issuedOn, DueOn: dueOn,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func newPayment(t *testing.T, customer uuid.UUID, currency, amt string) *payment.Payment {
	t.Helper()
	p, err := payment.New(payment.Data{ID: uuid.New(), CustomerID: customer, Currency: currency, Amount: dec(amt)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustApply(t *testing.T, p *payment.Payment, r *receivable.Receivable, amt string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := Apply(p, r, id, dec(amt), at); err != nil {
		t.Fatalf("Apply(%s): %v", amt, err)
	}
	return id
}

func adjustment(amt string) receivable.Adjustment {
	return receivable.Adjustment{ID: uuid.New(), Amount: dec(amt), SourceDocumentID: uuid.New(), SourceEventID: uuid.New()}
}

func TestApplyCrossRules(t *testing.T) {
	c := customers[0]
	cases := []struct {
		name string
		p    *payment.Payment
		r    *receivable.Receivable
		want error
	}{
		{"otro cliente (R11)", newPayment(t, customers[1], "CRC", "10"), newReceivable(t, c, "CRC", "10"), ErrCustomerMismatch},
		{"otra moneda", newPayment(t, c, "USD", "10"), newReceivable(t, c, "CRC", "10"), ErrCurrencyMismatch},
		{"mayor que el disponible", newPayment(t, c, "CRC", "5"), newReceivable(t, c, "CRC", "10"), payment.ErrExceedsPayment},
		{"mayor que el saldo", newPayment(t, c, "CRC", "50"), newReceivable(t, c, "CRC", "5"), receivable.ErrExceedsBalance},
	}
	for _, tc := range cases {
		if _, err := Apply(tc.p, tc.r, uuid.New(), dec("10"), at); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", tc.name, err)
		}
		if len(tc.p.Applications()) != 0 || len(tc.r.Applications()) != 0 {
			t.Errorf("%s: una aplicación rechazada quedó registrada", tc.name)
		}
	}
}

// Un pago a varias cuentas y varios pagos a una cuenta.
func TestManyToMany(t *testing.T) {
	c := customers[0]
	p1, p2 := newPayment(t, c, "CRC", "100"), newPayment(t, c, "CRC", "50")
	r1, r2 := newReceivable(t, c, "CRC", "80"), newReceivable(t, c, "CRC", "70")
	mustApply(t, p1, r1, "60")
	ch, err := Apply(p1, r2, uuid.New(), dec("40"), at)
	if err != nil || ch.To != receivable.StatusPartiallyPaid {
		t.Fatalf("%+v, %v", ch, err)
	}
	mustApply(t, p2, r1, "20")
	ch, err = Apply(p2, r2, uuid.New(), dec("30"), at)
	if err != nil || !ch.Settled() {
		t.Fatalf("la segunda cuenta debía quedar pagada: %+v, %v", ch, err)
	}
	if r1.Status() != receivable.StatusPaid || !p1.Available().IsZero() || !p2.Available().IsZero() {
		t.Errorf("r1 %s, disponibles %s y %s", r1.Status(), p1.Available(), p2.Available())
	}
}

func TestVoidPaymentReversesEverything(t *testing.T) {
	c := customers[0]
	p := newPayment(t, c, "CRC", "100")
	r1, r2 := newReceivable(t, c, "CRC", "30"), newReceivable(t, c, "CRC", "100")
	mustApply(t, p, r1, "30")
	mustApply(t, p, r2, "70")
	loaded := map[uuid.UUID]*receivable.Receivable{r1.ID(): r1, r2.ID(): r2}

	if _, err := VoidPayment(p, map[uuid.UUID]*receivable.Receivable{r1.ID(): r1}, "Cheque rechazado", at); !errors.Is(err, ErrNotLoaded) {
		t.Errorf("sin todas las cuentas cargadas: %v", err)
	}
	if p.Status() != payment.StatusPosted || r1.Status() != receivable.StatusPaid {
		t.Fatal("un VoidPayment rechazado no debe revertir nada")
	}
	effects, err := VoidPayment(p, loaded, "Cheque rechazado", at)
	if err != nil || len(effects) != 2 {
		t.Fatalf("%+v, %v", effects, err)
	}
	if p.Status() != payment.StatusVoided || r1.Status() != receivable.StatusOpen || r2.Status() != receivable.StatusOpen {
		t.Errorf("pago %s, cuentas %s y %s", p.Status(), r1.Status(), r2.Status())
	}
	if _, err := Apply(p, r1, uuid.New(), dec("1"), at); !errors.Is(err, payment.ErrVoided) {
		t.Errorf("aplicar un pago anulado: %v", err)
	}
}

// R3: lo que la nota de crédito revierte en la cuenta también se revierte en el pago y vuelve a estar disponible.
func TestCreditNoteReleasesPayment(t *testing.T) {
	c := customers[0]
	p := newPayment(t, c, "CRC", "100")
	r := newReceivable(t, c, "CRC", "100")
	mustApply(t, p, r, "100")
	res, err := CreditNote(r, map[uuid.UUID]*payment.Payment{p.ID(): p}, adjustment("30"), at)
	if err != nil || len(res.Reversed) != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if !p.Available().Equal(dec("100")) || !r.Balance().Equal(dec("70")) {
		t.Errorf("disponible %s, saldo %s", p.Available(), r.Balance())
	}
	if _, err := CreditNote(r, nil, adjustment("1"), at); err != nil {
		t.Errorf("sin aplicaciones vigentes no hace falta cargar pagos: %v", err)
	}
}

// R2: anular la factura libera lo aplicado en cada pago, que se puede aplicar a otra cuenta.
func TestCancelInvoiceReleasesPayments(t *testing.T) {
	c := customers[0]
	p := newPayment(t, c, "CRC", "100")
	r, other := newReceivable(t, c, "CRC", "100"), newReceivable(t, c, "CRC", "60")
	mustApply(t, p, r, "60")
	if _, err := CancelInvoice(r, map[uuid.UUID]*payment.Payment{}, adjustment("0"), at); !errors.Is(err, ErrNotLoaded) {
		t.Errorf("sin el pago cargado: %v", err)
	}
	if _, err := CancelInvoice(r, map[uuid.UUID]*payment.Payment{p.ID(): p}, adjustment("0"), at); err != nil {
		t.Fatal(err)
	}
	if !p.Available().Equal(dec("100")) || r.Status() != receivable.StatusCancelled {
		t.Errorf("disponible %s, estado %s", p.Available(), r.Status())
	}
	if ch := mustApplyChange(t, p, other, "60"); !ch.Settled() {
		t.Errorf("el saldo a favor se aplica a otra factura: %+v", ch)
	}
}

func TestReverseMismatch(t *testing.T) {
	c := customers[0]
	p1, p2 := newPayment(t, c, "CRC", "100"), newPayment(t, c, "CRC", "100")
	r := newReceivable(t, c, "CRC", "100")
	id := mustApply(t, p1, r, "10")
	if _, err := Reverse(p2, r, id, "Motivo", at); !errors.Is(err, payment.ErrApplicationNotFound) {
		t.Errorf("con otro pago: %v", err)
	}
	if !r.Applications()[0].Active() {
		t.Error("una reversión rechazada no debe tocar la cuenta")
	}
}

func mustApplyChange(t *testing.T, p *payment.Payment, r *receivable.Receivable, amt string) receivable.Change {
	t.Helper()
	ch, err := Apply(p, r, uuid.New(), dec(amt), at)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}
