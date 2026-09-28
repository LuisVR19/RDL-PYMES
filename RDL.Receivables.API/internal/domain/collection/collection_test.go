package collection

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
)

func TestValidateFollowUp(t *testing.T) {
	for _, typ := range []FollowUpType{FollowUpCall, FollowUpEmail, FollowUpVisit, FollowUpMessage, FollowUpNote} {
		if err := ValidateFollowUp(typ, "Cliente promete pagar el viernes"); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	if err := ValidateFollowUp("fax", "x"); !errors.Is(err, ErrInvalidType) {
		t.Errorf("tipo fuera del catálogo: %v", err)
	}
	if err := ValidateFollowUp(FollowUpCall, ""); !errors.Is(err, ErrNotesRequired) {
		t.Errorf("sin notas: %v", err)
	}
}

func TestValidatePromise(t *testing.T) {
	today := civil.Date{Year: 2026, Month: time.September, Day: 25}
	tomorrow := civil.Date{Year: 2026, Month: time.September, Day: 26}
	yesterday := civil.Date{Year: 2026, Month: time.September, Day: 24}
	bal := decimal.RequireFromString("100")
	cases := []struct {
		name     string
		status   receivable.Status
		promised string
		on       civil.Date
		want     error
	}{
		{"parcial, hoy", receivable.StatusOpen, "40", today, nil},
		{"todo el saldo, mañana", receivable.StatusPartiallyPaid, "100", tomorrow, nil},
		{"pagada", receivable.StatusPaid, "1", tomorrow, ErrNotCollectible},
		{"anulada", receivable.StatusCancelled, "1", tomorrow, ErrNotCollectible},
		{"más que el saldo", receivable.StatusOpen, "100.00001", tomorrow, ErrPromiseExceedsBalance},
		{"cero", receivable.StatusOpen, "0", tomorrow, amount.ErrInvalid},
		{"ayer", receivable.StatusOpen, "10", yesterday, ErrPromiseInPast},
	}
	for _, c := range cases {
		err := ValidatePromise(c.status, bal, decimal.RequireFromString(c.promised), c.on, today)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, se esperaba %v", c.name, err, c.want)
		}
	}
}

// R8: solo pending cambia, y solo a uno de los tres cierres.
func TestPromiseTransitions(t *testing.T) {
	all := []PromiseStatus{PromisePending, PromiseKept, PromiseBroken, PromiseCancelled}
	for _, from := range all {
		for _, to := range all {
			err := Transition(from, to)
			switch {
			case to == PromisePending:
				if !errors.Is(err, ErrInvalidPromiseStatus) {
					t.Errorf("%s → %s: %v", from, to, err)
				}
			case from == PromisePending:
				if err != nil {
					t.Errorf("%s → %s: %v", from, to, err)
				}
			default:
				if !errors.Is(err, ErrPromiseClosed) {
					t.Errorf("%s → %s debe fallar: estado final", from, to)
				}
			}
		}
	}
	if err := Transition(PromisePending, "expired"); !errors.Is(err, ErrInvalidPromiseStatus) {
		t.Errorf("destino desconocido: %v", err)
	}
}
