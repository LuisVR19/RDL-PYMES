package document

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestTransitionsFollowTheContract(t *testing.T) {
	allowed := map[[2]Status]bool{
		{StatusProcessing, StatusSigned}: true, {StatusProcessing, StatusError}: true,
		{StatusSigned, StatusSent}: true, {StatusSigned, StatusContingency}: true,
		{StatusContingency, StatusSent}: true,
		{StatusSent, StatusAccepted}:    true, {StatusSent, StatusRejected}: true,
		{StatusError, StatusProcessing}: true,
	}
	all := []Status{StatusProcessing, StatusSigned, StatusSent, StatusAccepted, StatusRejected, StatusContingency, StatusError}
	for _, from := range all {
		if !from.Valid() {
			t.Errorf("%s debería ser válido", from)
		}
		for _, to := range all {
			want := allowed[[2]Status{from, to}]
			if got := CanTransition(from, to); got != want {
				t.Errorf("%s → %s: %v, se esperaba %v", from, to, got, want)
			}
			if err := Transition(from, to); (err == nil) != want || (err != nil && !errors.Is(err, ErrInvalidTransition)) {
				t.Errorf("Transition(%s, %s) = %v", from, to, err)
			}
		}
	}
	for _, s := range []Status{StatusAccepted, StatusRejected} {
		if !s.IsFinal() {
			t.Errorf("%s debería ser final", s)
		}
	}
	if Status("cancelled").Valid() {
		t.Error("cancelled no es un estado de la base")
	}
}

func TestTypeCodesFromNote3(t *testing.T) {
	for src, want := range map[SourceType]string{SourceInvoice: "01", SourceDebitNote: "02", SourceCreditNote: "03"} {
		if got := src.TypeCode(); got != want {
			t.Errorf("%s: %s, se esperaba %s (Nota 3)", src, got, want)
		}
	}
}

func line(n int) Line {
	return Line{Number: n, Quantity: decimal.NewFromInt(1), UnitPrice: decimal.NewFromInt(1)}
}

func TestNewChecksStructure(t *testing.T) {
	d, err := New(Document{Lines: []Line{line(1), line(2)}, PaymentMethods: make([]PaymentMethod, 4)})
	if err != nil || d.Status != StatusProcessing || d.ID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("documento válido: %+v, %v", d.Status, err)
	}
	for name, bad := range map[string]Document{
		"sin líneas":         {},
		"cinco medios":       {Lines: []Line{line(1)}, PaymentMethods: make([]PaymentMethod, 5)},
		"línea repetida":     {Lines: []Line{line(1), line(1)}},
		"más de 1000 líneas": {Lines: make([]Line, 1001)},
	} {
		if _, err := New(bad); !errors.Is(err, ErrStructure) {
			t.Errorf("%s: err = %v, se esperaba ErrStructure", name, err)
		}
	}
}
