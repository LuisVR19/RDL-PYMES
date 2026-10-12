package catalogs

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

var (
	annotationRe  = regexp.MustCompile(`(?s)<xs:annotation>.*?</xs:annotation>`)
	enumerationRe = regexp.MustCompile(`<xs:enumeration value="([^"]*)"`)
)

// xsdEnum devuelve los valores enumerados después de marker (un simpleType con nombre o un elemento anónimo).
func xsdEnum(t *testing.T, file, marker string) []string {
	t.Helper()
	raw, err := os.ReadFile("../docs/hacienda/xsd/" + file) //nolint:gosec // G304: XSD del repo
	if err != nil {
		t.Fatal(err)
	}
	s := annotationRe.ReplaceAllString(string(raw), "")
	i := regexp.MustCompile(regexp.QuoteMeta(marker)).FindStringIndex(s)
	if i == nil {
		t.Fatalf("%s: no está %s", file, marker)
	}
	end := regexp.MustCompile(`</xs:simpleType>`).FindStringIndex(s[i[1]:])
	var out []string
	for _, m := range enumerationRe.FindAllStringSubmatch(s[i[1]:i[1]+end[0]], -1) {
		out = append(out, m[1])
	}
	slices.Sort(out)
	return out
}

// Cada catálogo transcrito del anexo coincide con la enumeración del XSD oficial (que es el que manda ante una
// diferencia, CLAUDE.md). Las únicas diferencias admitidas están documentadas en el anexo.
func TestCatalogsMatchXSDEnumerations(t *testing.T) {
	cs, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string][]string{}
	for _, c := range cs {
		for _, r := range c.Rows {
			codes[c.Table] = append(codes[c.Table], r.Code)
		}
		slices.Sort(codes[c.Table])
	}
	const fe, nc = "FacturaElectronica_V4.4.xsd", "NotaCreditoElectronica_V4.4.xsd"
	cases := []struct {
		table, file, marker string
		onlyCatalog         []string // códigos del anexo que este XSD no admite, con su razón en el comentario
	}{
		{"units_of_measure", fe, `<xs:simpleType name="UnidadMedidaType">`, nil},
		{"currencies", fe, `<xs:element name="CodigoMoneda"`, nil},
		{"discount_codes", fe, `<xs:simpleType name="CodigoDescuentoType">`, nil},
		{"tax_types", fe, `<xs:simpleType name="CodigoImpuestoType">`, nil},
		{"tax_rates", fe, `<xs:simpleType name="CodigoTarifaIVAType">`, nil},
		{"exoneration_document_types", fe, `<xs:simpleType name="TipoExoneracionType">`, nil},
		{"reference_document_types", fe, `<xs:simpleType name="TipoDocReferenciaType">`, nil},
		// 17 «Pago a comprobante electrónico» es solo del recibo electrónico de pago (Nota 9, pie 35).
		{"reference_codes", fe, `<xs:simpleType name="CodigoReferenciaType">`, []string{"17"}},
		// La nota de crédito admite las 16 condiciones; la factura no admite 09 ni 11 (solo REP, Nota 5 pies 20 y 22).
		{"sale_conditions", nc, `<xs:element name="CondicionVenta"`, nil},
		{"payment_methods", fe, `<xs:element name="TipoMedioPago"`, nil},
		{"exoneration_institutions", fe, `<xs:element name="NombreInstitucion"`, nil},
	}
	for _, c := range cases {
		want := xsdEnum(t, c.file, c.marker)
		got := slices.DeleteFunc(slices.Clone(codes[c.table]), func(s string) bool { return slices.Contains(c.onlyCatalog, s) })
		if !slices.Equal(got, want) {
			t.Errorf("%s no coincide con %s en %s:\ncatálogo %v\nXSD      %v", c.table, c.marker, c.file, got, want)
		}
	}
}
