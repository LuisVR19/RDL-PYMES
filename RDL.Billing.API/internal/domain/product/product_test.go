package product

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/money"
)

func validInput() NewInput {
	return NewInput{
		Code: "SERV-01", Description: "Consultoría", CabysCode: "8314100000100", UnitOfMeasureCode: "Sp",
		UnitPrice: money.MustAmountForTest("25000.50"), Currency: money.MustCurrencyForTest("CRC"), IsService: true,
		Taxes: []Tax{{TypeCode: "01", RateCode: "08"}},
	}
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

func TestNew(t *testing.T) {
	org := uuid.New()
	in := validInput()
	in.UnitPrice = money.MustAmountForTest("25000.50000")
	in.Taxes = []Tax{{TypeCode: "02", RateCode: "01"}, {TypeCode: " 01 ", RateCode: "08"}}
	p, err := New(org, in)
	if err != nil {
		t.Fatal(err)
	}
	if !p.IsActive || p.OrganizationID != org || p.UnitPrice.String() != "25000.5" {
		t.Fatalf("producto = %+v", p)
	}
	if p.Taxes[0].TypeCode != "01" || p.Taxes[1].TypeCode != "02" {
		t.Fatalf("impuestos ordenados y sin espacios: %+v", p.Taxes)
	}
}

func TestNewCanStartInactive(t *testing.T) {
	in := validInput()
	off := false
	in.IsActive = &off
	p, err := New(uuid.New(), in)
	if err != nil || p.IsActive {
		t.Fatalf("p=%+v err=%v", p, err)
	}
}

func TestNewValidation(t *testing.T) {
	cases := map[string]struct {
		mut   func(*NewInput)
		field string
	}{
		"sin código":             {func(in *NewInput) { in.Code = "" }, "code"},
		"código de 51":           {func(in *NewInput) { in.Code = strings.Repeat("A", 51) }, "code"},
		"sin descripción":        {func(in *NewInput) { in.Description = " " }, "description"},
		"descripción larga":      {func(in *NewInput) { in.Description = strings.Repeat("é", 201) }, "description"},
		"CABYS de 12":            {func(in *NewInput) { in.CabysCode = "831410000010" }, "cabysCode"},
		"CABYS con letras":       {func(in *NewInput) { in.CabysCode = "83141000001AB" }, "cabysCode"},
		"unidad vacía":           {func(in *NewInput) { in.UnitOfMeasureCode = "" }, "unitOfMeasureCode"},
		"unidad con espacios":    {func(in *NewInput) { in.UnitOfMeasureCode = "kilo gramo" }, "unitOfMeasureCode"},
		"sin moneda":             {func(in *NewInput) { in.Currency = money.Currency{} }, "currency"},
		"tipo de impuesto vacío": {func(in *NewInput) { in.Taxes = []Tax{{RateCode: "08"}} }, "taxes[0].taxTypeCode"},
		"tarifa inválida":        {func(in *NewInput) { in.Taxes = []Tax{{TypeCode: "01", RateCode: "8 %"}} }, "taxes[0].taxRateCode"},
		"tipo de impuesto doble": {func(in *NewInput) { in.Taxes = []Tax{{"01", "08"}, {"01", "04"}} }, "taxes[1].taxTypeCode"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := validInput()
			tc.mut(&in)
			_, err := New(uuid.New(), in)
			if !errors.Is(err, ErrInvalid) || !fields(err)[tc.field] {
				t.Fatalf("err = %v, se esperaba un error en %s", err, tc.field)
			}
		})
	}
}

func TestApply(t *testing.T) {
	p, _ := New(uuid.New(), validInput())
	price := money.MustAmountForTest("30000")
	off := false
	none := []Tax{}
	next, err := p.Apply(Patch{UnitPrice: &price, IsActive: &off, Taxes: &none})
	if err != nil {
		t.Fatal(err)
	}
	if next.UnitPrice.String() != "30000" || next.IsActive || len(next.Taxes) != 0 || next.Code != p.Code {
		t.Fatalf("resultado = %+v", next)
	}
	if len(p.Taxes) != 1 || p.UnitPrice.String() != "25000.5" {
		t.Fatal("Apply no debe modificar el original")
	}
}

func TestApplyWithoutTaxesKeepsThem(t *testing.T) {
	p, _ := New(uuid.New(), validInput())
	desc := "Consultoría senior"
	next, err := p.Apply(Patch{Description: &desc})
	if err != nil || len(next.Taxes) != 1 {
		t.Fatalf("taxes ausente no cambia los impuestos: %+v %v", next.Taxes, err)
	}
}

func TestEqualIgnoresRepresentation(t *testing.T) {
	p, _ := New(uuid.New(), validInput())
	same := money.MustAmountForTest("25000.50000")
	reordered := []Tax{{TypeCode: "01", RateCode: "08"}}
	next, err := p.Apply(Patch{UnitPrice: &same, Taxes: &reordered})
	if err != nil || !next.Equal(p) {
		t.Fatalf("el mismo precio con otros ceros y los mismos impuestos no es un cambio: %+v %v", next, err)
	}
	code := "SERV-02"
	changed, _ := p.Apply(Patch{Code: &code})
	if changed.Equal(p) {
		t.Fatal("cambiar el código es un cambio")
	}
}
