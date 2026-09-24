package numbering

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestFormat(t *testing.T) {
	for _, c := range []struct {
		prefix string
		n      int64
		want   string
	}{
		{"FAC-", 1, "FAC-00000001"},
		{"", 42, "00000042"},
		{"NC-", 99_999_999, "NC-99999999"},
		{"FAC-", 100_000_000, "FAC-100000000"}, // pasado el ancho, crece sin truncar
	} {
		if got := Format(c.prefix, c.n); got != c.want {
			t.Errorf("Format(%q, %d) = %q, se esperaba %q", c.prefix, c.n, got, c.want)
		}
	}
}

func TestAssignAdvancesWithoutGaps(t *testing.T) {
	s := New(uuid.New(), Scope{DocumentType: "invoice"})
	s, _ = s.Configure("FAC-", 1000, nil)
	var numbers []string
	for range 3 {
		var n string
		n, s = s.Assign()
		numbers = append(numbers, n)
	}
	if numbers[0] != "FAC-00001000" || numbers[2] != "FAC-00001002" || s.NextNumber != 1003 || *s.LastAssigned != 1002 {
		t.Fatalf("números = %v, secuencia = %+v", numbers, s)
	}
}

func TestConfigureOnlyWhenUnused(t *testing.T) {
	s := New(uuid.New(), Scope{DocumentType: "invoice"})
	s, err := s.Configure("FAC-", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Reconfigurar antes de usarla está bien.
	if s, err = s.Configure("F-", 500, nil); err != nil || s.Prefix != "F-" || s.NextNumber != 500 {
		t.Fatalf("reconfigurar sin uso: %+v %v", s, err)
	}
	_, s = s.Assign()
	if _, err := s.Configure("X-", 1, nil); !errors.Is(err, ErrInUse) {
		t.Fatalf("configurar una usada: err = %v", err)
	}
}

func TestConfigureValidation(t *testing.T) {
	s := New(uuid.New(), Scope{DocumentType: "invoice"})
	for name, c := range map[string]struct {
		prefix string
		next   int64
		field  string
	}{
		"prefijo largo":       {"FACTURA-2026", 1, "prefix"},
		"prefijo con espacio": {"FAC 1", 1, "prefix"},
		"número cero":         {"F-", 0, "nextNumber"},
		"número enorme":       {"F-", MaxNextNumber + 1, "nextNumber"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := s.Configure(c.prefix, c.next, nil)
			var fe FieldError
			if !errors.As(err, &fe) || fe.Field != c.field {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestPrefixCannotRepeatWithinType(t *testing.T) {
	org := uuid.New()
	b1, b2 := uuid.New(), uuid.New()
	branch1, _ := New(org, Scope{DocumentType: "invoice", BranchID: &b1}).Configure("S1-", 1, nil)
	orgWide, _ := New(org, Scope{DocumentType: "invoice"}).Configure("", 1, nil)
	siblings := []Sequence{branch1, orgWide}

	if _, err := New(org, Scope{DocumentType: "invoice", BranchID: &b2}).Configure("S1-", 1, siblings); !errors.Is(err, ErrPrefixTaken) {
		t.Fatalf("prefijo de otra sucursal: err = %v", err)
	}
	if _, err := New(org, Scope{DocumentType: "invoice", BranchID: &b2}).Configure("", 1, siblings); !errors.Is(err, ErrPrefixTaken) {
		t.Fatalf("prefijo vacío repetido: err = %v", err)
	}
	// Mismo prefijo en otro tipo de documento: no chocan (invoices_number_uk incluye document_type).
	if _, err := New(org, Scope{DocumentType: "credit_note", BranchID: &b2}).Configure("S1-", 1, siblings); err != nil {
		t.Fatalf("otro tipo: %v", err)
	}
	// Reconfigurar la misma secuencia con su propio prefijo no es un choque.
	if _, err := branch1.Configure("S1-", 50, siblings); err != nil {
		t.Fatalf("misma secuencia: %v", err)
	}
}

func TestChoose(t *testing.T) {
	org := uuid.New()
	b1, b2 := uuid.New(), uuid.New()
	branchSeq, _ := New(org, Scope{DocumentType: "invoice", BranchID: &b1}).Configure("S1-", 1, nil)
	orgSeq, _ := New(org, Scope{DocumentType: "invoice"}).Configure("FAC-", 1, nil)
	existing := []Sequence{branchSeq, orgSeq}

	if got := Choose(org, "invoice", &b1, existing); got.Prefix != "S1-" {
		t.Fatalf("sucursal con secuencia propia: %+v", got)
	}
	if got := Choose(org, "invoice", &b2, existing); got.Prefix != "FAC-" {
		t.Fatalf("sucursal sin secuencia: usa la de la organización, llegó %+v", got)
	}
	if got := Choose(org, "invoice", nil, existing); got.Prefix != "FAC-" {
		t.Fatalf("sin sucursal: %+v", got)
	}
	got := Choose(org, "debit_note", nil, existing)
	if got.ID != uuid.Nil || got.Prefix != "" || got.NextNumber != 1 || got.BranchID != nil {
		t.Fatalf("sin ninguna secuencia: nueva de la organización, llegó %+v", got)
	}
}
