package taxpayer

import (
	"strings"
	"testing"
)

func valid() Profile {
	return Profile{
		Environment: EnvironmentSandbox, IdentificationTypeCode: "02", IdentificationNumber: "3101123456",
		LegalName: "PYME de Prueba S.A.", Email: "facturas@pyme.example", Phone: "22223333",
		Location:             &Location{ProvinceCode: "1", CantonCode: "01", DistrictCode: "03", AddressDetails: "Del parque 100 m al norte"},
		EconomicActivityCode: "620100",
	}
}

func fieldsOf(errs []FieldError) string {
	var out []string
	for _, e := range errs {
		out = append(out, e.Field)
	}
	return strings.Join(out, ",")
}

func TestValidProfile(t *testing.T) {
	if errs := valid().Validate(); len(errs) != 0 {
		t.Fatalf("perfil válido con errores: %v", errs)
	}
	p := valid()
	p.Location, p.Phone, p.EconomicActivityCode, p.TradeName = nil, "", "", ""
	if errs := p.Validate(); len(errs) != 0 {
		t.Fatalf("los campos opcionales pueden faltar: %v", errs)
	}
}

// Anexo 1, pág. 20: formato del número por tipo de identificación.
func TestIdentificationFormats(t *testing.T) {
	cases := []struct {
		typ, number string
		ok          bool
	}{
		{"01", "109870654", true}, {"01", "009870654", false}, {"01", "1-0987-0654", false}, {"01", "10987065", false},
		{"02", "3101123456", true}, {"02", "3102ABC123", true}, {"02", "3-101-123456", false}, {"02", "310112345", false},
		{"03", "17201234567", true}, {"03", "172012345678", true}, {"03", "07201234567", false}, {"03", "1720123456", false},
		{"04", "1234567890", true}, {"04", "12345678AB", false},
		{"05", "ABC123", false}, {"06", "123", false},
	}
	for _, c := range cases {
		p := valid()
		p.IdentificationTypeCode, p.IdentificationNumber = c.typ, c.number
		errs := p.Validate()
		if (len(errs) == 0) != c.ok {
			t.Errorf("%s %q: errores %v, se esperaba válido = %v", c.typ, c.number, fieldsOf(errs), c.ok)
		}
	}
}

func TestRejectsProductionAndBadFields(t *testing.T) {
	p := valid()
	p.Environment = EnvironmentProduction
	p.LegalName = "Ab"
	p.Email = "no-es-correo"
	p.Phone = "2222-3333"
	p.Location = &Location{ProvinceCode: "10", CantonCode: "1", DistrictCode: "x", AddressDetails: "x"}
	p.EconomicActivityCode = "1234"
	got := fieldsOf(p.Validate())
	want := "environment,legalName,email,phone,location.provinceCode,location.cantonCode,location.districtCode,location.addressDetails,economicActivityCode"
	if got != want {
		t.Errorf("campos = %s\nse esperaba %s", got, want)
	}
}

func TestEmailRegexFromAnnex(t *testing.T) {
	for email, ok := range map[string]bool{
		"a@b.cr": true, "nombre.apellido@empresa.co.cr": true, `"raro"@x.com`: true,
		"sin-arroba": false, "dos@@x.com": false, "a@b": false, "a b@c.cr": false,
	} {
		p := valid()
		p.Email = email
		if (len(p.Validate()) == 0) != ok {
			t.Errorf("%q: se esperaba válido = %v", email, ok)
		}
	}
}
