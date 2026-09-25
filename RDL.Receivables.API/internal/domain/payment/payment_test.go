package payment

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gopkg.in/yaml.v3"

	"rdl/receivables-api/internal/domain/amount"
)

// contractsDir es el mismo directorio del replace de go.mod.
const contractsDir = "../../../../RDL.Contracts"

var at = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

func dec(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func mustNew(t *testing.T, amt string) *Payment {
	t.Helper()
	p, err := New(Data{ID: uuid.New(), CustomerID: uuid.New(), Currency: "CRC", Amount: dec(amt)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func apply(t *testing.T, p *Payment, amt string) Application {
	t.Helper()
	app := Application{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec(amt), AppliedAt: at}
	if err := p.Apply(app); err != nil {
		t.Fatalf("Apply(%s): %v", amt, err)
	}
	return app
}

// payment.yaml: estados, estado inicial y la única transición (posted → voided, con motivo y sin aplicaciones).
func TestStateMachineFromContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(contractsDir, "state-machines", "payment.yaml")) //nolint:gosec // G304: ruta fija del repo de contratos
	if err != nil {
		t.Fatalf("no se pudo leer la máquina de estados de contratos: %v", err)
	}
	var sm struct {
		Created struct {
			State string `yaml:"state"`
		} `yaml:"created"`
		States      map[string]struct{ Final bool } `yaml:"states"`
		Transitions []struct{ From, To string }     `yaml:"transitions"`
	}
	if err := yaml.Unmarshal(raw, &sm); err != nil {
		t.Fatal(err)
	}
	if len(sm.States) != 2 || sm.Created.State != string(StatusPosted) {
		t.Fatalf("el contrato cambió: estados %v, inicial %s", sm.States, sm.Created.State)
	}
	if p := mustNew(t, "10"); string(p.Status()) != sm.Created.State {
		t.Errorf("estado inicial %s", p.Status())
	}
	for _, tr := range sm.Transitions {
		if tr.From != string(StatusPosted) || tr.To != string(StatusVoided) {
			t.Fatalf("transición del contrato sin caso en este test: %s → %s", tr.From, tr.To)
		}
		p := mustNew(t, "10")
		if err := p.Void("Cheque rechazado", at); err != nil || p.Status() != StatusVoided {
			t.Errorf("posted → voided: %v", err)
		}
	}
	if !sm.States[string(StatusVoided)].Final {
		t.Error("voided debe ser final en el contrato")
	}
}

// Invariante 1: lo aplicado vigente nunca supera el monto; lo revertido vuelve a estar disponible.
func TestAppliedNeverExceedsAmount(t *testing.T) {
	p := mustNew(t, "100.5")
	first := apply(t, p, "60")
	if err := p.Apply(Application{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec("40.50001"), AppliedAt: at}); !errors.Is(err, ErrExceedsPayment) {
		t.Errorf("mayor que el disponible: %v", err)
	}
	apply(t, p, "40.5")
	if !p.Available().IsZero() {
		t.Errorf("disponible %s", p.Available())
	}
	if err := p.ReverseApplication(first.ID, "Cuenta equivocada", at); err != nil {
		t.Fatal(err)
	}
	if !p.Available().Equal(dec("60")) || !p.Applied().Equal(dec("40.5")) {
		t.Errorf("después de revertir: disponible %s, aplicado %s", p.Available(), p.Applied())
	}
}

func TestApplyRules(t *testing.T) {
	p := mustNew(t, "100")
	app := apply(t, p, "10")
	if err := p.Apply(Application{ID: uuid.New(), ReceivableID: app.ReceivableID, Amount: dec("5"), AppliedAt: at}); !errors.Is(err, ErrDuplicateApplication) {
		t.Errorf("misma cuenta con aplicación vigente: %v", err)
	}
	for _, amt := range []string{"0", "-5", "0.000001"} {
		if err := p.Apply(Application{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec(amt), AppliedAt: at}); !errors.Is(err, amount.ErrInvalid) {
			t.Errorf("monto %s: %v", amt, err)
		}
	}
	if err := p.Apply(Application{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec("1"), ReversedAt: at}); !errors.Is(err, ErrInconsistent) {
		t.Errorf("aplicación ya revertida: %v", err)
	}
}

func TestReverseRules(t *testing.T) {
	p := mustNew(t, "100")
	app := apply(t, p, "10")
	if err := p.ReverseApplication(app.ID, "", at); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("sin motivo: %v", err)
	}
	if err := p.ReverseApplication(uuid.New(), "Motivo", at); !errors.Is(err, ErrApplicationNotFound) {
		t.Errorf("de otro pago: %v", err)
	}
	if err := p.ReverseApplication(app.ID, "Motivo", at); err != nil {
		t.Fatal(err)
	}
	if err := p.ReverseApplication(app.ID, "Motivo", at); !errors.Is(err, ErrApplicationReversed) {
		t.Errorf("dos veces: %v", err)
	}
	if len(p.Applications()) != 1 || p.Applications()[0].ReversalReason != "Motivo" {
		t.Errorf("la reversión queda como historial: %+v", p.Applications())
	}
}

func TestVoid(t *testing.T) {
	p := mustNew(t, "100")
	app := apply(t, p, "10")
	if err := p.Void("Cheque rechazado", at); !errors.Is(err, ErrActiveApplications) || p.Status() != StatusPosted {
		t.Errorf("con aplicaciones vigentes: %v", err)
	}
	if err := p.Void("", at); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("sin motivo: %v", err)
	}
	if err := p.ReverseApplication(app.ID, "Cheque rechazado", at); err != nil {
		t.Fatal(err)
	}
	if err := p.Void("Cheque rechazado", at); err != nil || p.VoidReason() != "Cheque rechazado" {
		t.Fatalf("anular: %v", err)
	}
	if err := p.Void("Otra vez", at); !errors.Is(err, ErrVoided) {
		t.Errorf("voided es final: %v", err)
	}
	if err := p.Apply(Application{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec("1"), AppliedAt: at}); !errors.Is(err, ErrVoided) {
		t.Errorf("aplicar un pago anulado: %v", err)
	}
}

func TestRehydrate(t *testing.T) {
	d := Data{ID: uuid.New(), CustomerID: uuid.New(), Currency: "USD", Amount: dec("50")}
	over := []Application{{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec("50.00001"), AppliedAt: at}}
	active := []Application{{ID: uuid.New(), ReceivableID: uuid.New(), Amount: dec("1"), AppliedAt: at}}
	bad := map[string]func() (*Payment, error){
		"aplicado mayor que el monto": func() (*Payment, error) { return Rehydrate(d, StatusPosted, "", time.Time{}, over) },
		"estado desconocido":          func() (*Payment, error) { return Rehydrate(d, "reversed", "", time.Time{}, nil) },
		"anulado sin motivo":          func() (*Payment, error) { return Rehydrate(d, StatusVoided, "", at, nil) },
		"motivo sin anular":           func() (*Payment, error) { return Rehydrate(d, StatusPosted, "x", at, nil) },
		"anulado con aplicaciones":    func() (*Payment, error) { return Rehydrate(d, StatusVoided, "x", at, active) },
	}
	for name, f := range bad {
		if _, err := f(); !errors.Is(err, ErrInconsistent) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := Rehydrate(d, StatusVoided, "Cheque rechazado", at, nil); err != nil {
		t.Errorf("anulado válido: %v", err)
	}
}
