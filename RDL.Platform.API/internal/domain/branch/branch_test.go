package branch

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNew(t *testing.T) {
	b, err := New(uuid.New(), NewInput{Code: " SJ-01 ", Name: " San José Centro ", Email: "sj@alfa.test"})
	if err != nil {
		t.Fatal(err)
	}
	if b.Code != "SJ-01" || b.Name != "San José Centro" || !b.IsActive {
		t.Fatalf("b=%+v", b)
	}
}

func TestNewValidatesEveryField(t *testing.T) {
	_, err := New(uuid.New(), NewInput{Code: "con espacio", Name: "", Email: "no-es-email"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err=%v", err)
	}
	fields := map[string]bool{}
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var fe FieldError
		if errors.As(e, &fe) {
			fields[fe.Field] = true
		}
	}
	for _, f := range []string{"code", "name", "email"} {
		if !fields[f] {
			t.Errorf("falta el error de %s", f)
		}
	}
}

func TestApplyDeactivatesAndClearsOptionalFields(t *testing.T) {
	b, _ := New(uuid.New(), NewInput{Code: "C1", Name: "Centro", Phone: "2222-2222"})
	off, empty := false, ""
	next, err := b.Apply(Patch{IsActive: &off, Phone: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if next.IsActive || next.Phone != "" || next.Code != "C1" || !b.IsActive {
		t.Fatalf("next=%+v original=%+v", next, b)
	}
	blank := "  "
	if _, err := b.Apply(Patch{Name: &blank}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nombre vacío: err=%v", err)
	}
}
