package receivable

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"
)

// contractsDir es el mismo directorio del replace de go.mod: los tests leen la máquina de estados de la versión de
// contratos con la que se compila.
const contractsDir = "../../../../RDL.Contracts"

type stateMachine struct {
	Created struct {
		State string `yaml:"state"`
	} `yaml:"created"`
	States      map[string]struct{} `yaml:"states"`
	Transitions []struct {
		From     string   `yaml:"from"`
		To       string   `yaml:"to"`
		Emits    string   `yaml:"emits"`
		Requires []string `yaml:"requires"`
		Trigger  struct {
			Name string `yaml:"name"`
		} `yaml:"trigger"`
	} `yaml:"transitions"`
}

func loadStateMachine(t *testing.T, name string) stateMachine {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractsDir, "state-machines", name)) //nolint:gosec // G304: ruta fija del repo de contratos
	if err != nil {
		t.Fatalf("no se pudo leer la máquina de estados de contratos: %v", err)
	}
	var sm stateMachine
	if err := yaml.Unmarshal(raw, &sm); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return sm
}

// Los estados del contrato son exactamente los del dominio, y la cuenta nace en el estado inicial del contrato.
func TestStatesMatchContract(t *testing.T) {
	sm := loadStateMachine(t, "receivable.yaml")
	for s := range sm.States {
		if !Status(s).Valid() {
			t.Errorf("el contrato tiene el estado %q y el dominio no", s)
		}
	}
	for _, s := range []Status{StatusOpen, StatusPartiallyPaid, StatusPaid, StatusCancelled} {
		if _, ok := sm.States[string(s)]; !ok {
			t.Errorf("el dominio tiene el estado %q y el contrato no", s)
		}
	}
	if r := mustNew(t, "100"); string(r.Status()) != sm.Created.State {
		t.Errorf("estado inicial %s, el contrato dice %s", r.Status(), sm.Created.State)
	}
}

// Cada transición del YAML se reproduce con el agregado: se arma una cuenta en el estado de origen, se dispara la
// operación del trigger cumpliendo sus requires y se comprueba el destino y si hay que emitir ReceivableSettled.
// Una transición nueva en el contrato hace fallar el test hasta que se le agregue su caso aquí.
func TestTransitionsFromContract(t *testing.T) {
	type key struct{ from, to, trigger string }
	cases := map[key]func(t *testing.T) Change{
		{"open", "partially_paid", "POST /v1/payments"}: func(t *testing.T) Change {
			return apply(t, mustNew(t, "100"), "40")
		},
		{"open", "partially_paid", "CreditNoteIssued"}: func(t *testing.T) Change {
			return creditNote(t, mustNew(t, "100"), "40")
		},
		{"open", "paid", "POST /v1/payments"}: func(t *testing.T) Change {
			return apply(t, mustNew(t, "100"), "100")
		},
		{"open", "paid", "CreditNoteIssued"}: func(t *testing.T) Change {
			return creditNote(t, mustNew(t, "100"), "100")
		},
		{"partially_paid", "paid", "POST /v1/payments"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "40")
			return apply(t, r, "60")
		},
		{"partially_paid", "paid", "CreditNoteIssued"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "40")
			return creditNote(t, r, "60")
		},
		{"partially_paid", "open", "POST /v1/payment-applications/{id}/reverse"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "40")
			return reverseLast(t, r)
		},
		{"paid", "partially_paid", "POST /v1/payment-applications/{id}/reverse"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "60")
			apply(t, r, "40")
			return reverseLast(t, r)
		},
		{"paid", "partially_paid", "DebitNoteIssued"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "100")
			res, err := r.AddDebitNote(adjustment("10"))
			if err != nil {
				t.Fatal(err)
			}
			return res.Change
		},
		{"open", "cancelled", "InvoiceCancelled"}: func(t *testing.T) Change {
			return cancel(t, mustNew(t, "100"))
		},
		{"partially_paid", "cancelled", "InvoiceCancelled"}: func(t *testing.T) Change {
			r := mustNew(t, "100")
			apply(t, r, "40")
			return cancel(t, r)
		},
	}

	sm := loadStateMachine(t, "receivable.yaml")
	for _, tr := range sm.Transitions {
		k := key{tr.From, tr.To, tr.Trigger.Name}
		t.Run(tr.From+"→"+tr.To+" "+tr.Trigger.Name, func(t *testing.T) {
			run, ok := cases[k]
			if !ok {
				t.Fatalf("transición del contrato sin caso en este test: %+v", k)
			}
			ch := run(t)
			if ch != (Change{From: Status(tr.From), To: Status(tr.To)}) {
				t.Errorf("el agregado hizo %s → %s", ch.From, ch.To)
			}
			if want := tr.Emits == "ReceivableSettled"; ch.Settled() != want {
				t.Errorf("Settled()=%v, el contrato dice emits=%q", ch.Settled(), tr.Emits)
			}
		})
	}
}

// Transiciones que el agregado hace por decisiones aprobadas (docs/PLAN.md §3) y que receivable.yaml v1 todavía no
// tiene. TODO(contratos): agregarlas al YAML (docs/ESTADO.md); cuando estén, estos casos pasan al test de arriba.
func TestTransitionsPendingInContract(t *testing.T) {
	t.Run("paid→open al revertir la única aplicación (R1: el saldo vuelve al total)", func(t *testing.T) {
		r := mustNew(t, "100")
		apply(t, r, "100")
		if ch := reverseLast(t, r); ch != (Change{StatusPaid, StatusOpen}) {
			t.Errorf("%+v", ch)
		}
	})
	t.Run("paid→partially_paid por una nota de crédito que revierte aplicaciones (R3)", func(t *testing.T) {
		r := mustNew(t, "100")
		apply(t, r, "100")
		if ch := creditNote(t, r, "30"); ch != (Change{StatusPaid, StatusPartiallyPaid}) || r.Balance().String() != "70" {
			t.Errorf("%+v, saldo %s", ch, r.Balance())
		}
	})
	t.Run("paid→cancelled al anular una factura pagada con pagos (R2)", func(t *testing.T) {
		r := mustNew(t, "100")
		apply(t, r, "100")
		if ch := cancel(t, r); ch != (Change{StatusPaid, StatusCancelled}) || ch.Settled() {
			t.Errorf("%+v", ch)
		}
	})
}

var seq uint64

func newID() uuid.UUID {
	seq++
	var id uuid.UUID
	binary.BigEndian.PutUint64(id[8:], seq)
	return id
}

var testClock = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

func tick() time.Time {
	testClock = testClock.Add(time.Minute)
	return testClock
}

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func mustNew(t *testing.T, original string) *Receivable {
	t.Helper()
	r, err := New(newID(), invoice(original))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func apply(t *testing.T, r *Receivable, amt string) Change {
	t.Helper()
	ch, err := r.Apply(Application{ID: newID(), PaymentID: newID(), Amount: dec(amt), AppliedAt: tick()})
	if err != nil {
		t.Fatalf("Apply(%s): %v", amt, err)
	}
	return ch
}

func reverseLast(t *testing.T, r *Receivable) Change {
	t.Helper()
	apps := r.Applications()
	ch, err := r.ReverseApplication(apps[len(apps)-1].ID, "Error de digitación", tick())
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func creditNote(t *testing.T, r *Receivable, amt string) Change {
	t.Helper()
	res, err := r.AddCreditNote(adjustment(amt), tick())
	if err != nil {
		t.Fatalf("AddCreditNote(%s): %v", amt, err)
	}
	return res.Change
}

func cancel(t *testing.T, r *Receivable) Change {
	t.Helper()
	res, err := r.Cancel(adjustment("0"), tick())
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	return res.Change
}

func adjustment(amt string) Adjustment {
	return Adjustment{ID: newID(), Amount: dec(amt), SourceDocumentID: newID(), SourceEventID: newID()}
}
