package customer

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validInput() NewInput {
	return NewInput{IdentificationTypeCode: "01", IdentificationNumber: "112340567", LegalName: "Ana Pérez"}
}

func fields(err error) map[string]bool {
	out := map[string]bool{}
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		for _, e := range joined.Unwrap() {
			var fe FieldError
			if errors.As(e, &fe) {
				out[fe.Field] = true
			}
		}
	}
	return out
}

func TestNewTrimsAndStartsActive(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	in := validInput()
	in.LegalName, in.Email = "  Ana Pérez ", " ana@example.com "
	c, err := New(org, user, in)
	if err != nil {
		t.Fatal(err)
	}
	if c.LegalName != "Ana Pérez" || c.Email != "ana@example.com" || !c.IsActive ||
		c.OrganizationID != org || c.CreatedByUserID != user {
		t.Fatalf("cliente = %+v", c)
	}
}

func TestNewValidation(t *testing.T) {
	cases := map[string]struct {
		mut   func(*NewInput)
		field string
	}{
		"sin tipo":               {func(in *NewInput) { in.IdentificationTypeCode = " " }, "identification.typeCode"},
		"tipo largo":             {func(in *NewInput) { in.IdentificationTypeCode = strings.Repeat("1", 11) }, "identification.typeCode"},
		"sin número":             {func(in *NewInput) { in.IdentificationNumber = "" }, "identification.number"},
		"número largo":           {func(in *NewInput) { in.IdentificationNumber = strings.Repeat("9", 31) }, "identification.number"},
		"sin nombre":             {func(in *NewInput) { in.LegalName = "" }, "legalName"},
		"nombre de 201":          {func(in *NewInput) { in.LegalName = strings.Repeat("ñ", 201) }, "legalName"},
		"nombre comercial largo": {func(in *NewInput) { in.TradeName = strings.Repeat("a", 201) }, "tradeName"},
		"email inválido":         {func(in *NewInput) { in.Email = "ana@" }, "email"},
		"email con nombre":       {func(in *NewInput) { in.Email = "Ana <ana@example.com>" }, "email"},
		"teléfono largo":         {func(in *NewInput) { in.Phone = strings.Repeat("8", 31) }, "phone"},
		"dirección larga":        {func(in *NewInput) { in.Address = strings.Repeat("a", 501) }, "address"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			tc.mut(&in)
			_, err := New(uuid.New(), uuid.New(), in)
			if !errors.Is(err, ErrInvalid) || !fields(err)[tc.field] {
				t.Fatalf("err = %v, se esperaba un error en %s", err, tc.field)
			}
		})
	}
}

func TestLimitsCountCharactersNotBytes(t *testing.T) {
	in := validInput()
	in.LegalName = strings.Repeat("ñ", 200) // 400 bytes
	if _, err := New(uuid.New(), uuid.New(), in); err != nil {
		t.Fatalf("200 caracteres deben aceptarse: %v", err)
	}
}

func TestApply(t *testing.T) {
	c, _ := New(uuid.New(), uuid.New(), NewInput{
		IdentificationTypeCode: "01", IdentificationNumber: "1", LegalName: "A", Email: "a@example.com", Phone: "8888",
	})
	name, empty, inactive := " B ", "", false
	next, err := c.Apply(Patch{LegalName: &name, Email: &empty, IsActive: &inactive})
	if err != nil {
		t.Fatal(err)
	}
	if next.LegalName != "B" || next.Email != "" || next.Phone != "8888" || next.IsActive {
		t.Fatalf("resultado = %+v", next)
	}
	if next.Identification != c.Identification || next.ID != c.ID {
		t.Fatal("la identificación y el id no cambian")
	}
	if c.LegalName != "A" {
		t.Fatal("Apply no debe modificar el original")
	}
}

func TestApplyValidates(t *testing.T) {
	c, _ := New(uuid.New(), uuid.New(), validInput())
	blank := "  "
	if _, err := c.Apply(Patch{LegalName: &blank}); !fields(err)["legalName"] {
		t.Fatalf("err = %v", err)
	}
}
