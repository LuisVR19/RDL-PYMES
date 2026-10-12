package catalogs

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func manifest(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open("../docs/hacienda/manifest.sha256")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		sum, name, _ := strings.Cut(sc.Text(), "  ")
		out[name] = sum
	}
	return out
}

// Cada catálogo cita un archivo de docs/hacienda/ con el mismo sha256 que el manifiesto: si el PDF cambia, la
// transcripción se revisa.
func TestCatalogsTraceToHaciendaSources(t *testing.T) {
	cs, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	m := manifest(t)
	tables := map[string]bool{}
	for _, c := range cs {
		if m[c.Source] != c.SourceSHA256 {
			t.Errorf("%s: fuente %s con sha256 %s, el manifiesto dice %q", c.File, c.Source, c.SourceSHA256, m[c.Source])
		}
		if tables[c.Table] {
			t.Errorf("%s: la tabla %s ya la carga otro CSV", c.File, c.Table)
		}
		tables[c.Table] = true
	}
	for want := range Tables {
		if !tables[want] {
			t.Errorf("la tabla %s está admitida pero no hay CSV que la cargue", want)
		}
	}
}

// Los códigos que el resto del código usa como constantes tienen que existir en su catálogo.
func TestKnownCodes(t *testing.T) {
	cs, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	byTable := map[string]map[string]string{}
	for _, c := range cs {
		byTable[c.Table] = map[string]string{}
		for _, r := range c.Rows {
			byTable[c.Table][r.Code] = r.Name
		}
	}
	for _, want := range []struct{ table, code, name string }{
		{"document_types", "01", "Factura electrónica"},
		{"document_types", "02", "Nota de débito electrónica"},
		{"document_types", "03", "Nota de crédito electrónica"},
		{"identification_types", "02", "Cédula Jurídica"},
		{"payment_methods", "05", "Recaudado por terceros"},
		{"payment_methods", "99", "Otros"},
		{"currencies", "CRC", "Colón costarricense"},
		{"currencies", "USD", "Dólar Americano"},
		{"tax_types", "01", "Impuesto al Valor Agregado"},
		{"tax_rates", "08", "Tarifa general 13%"},
		{"tax_rates", "10", "Tarifa Exenta"},
		{"reference_codes", "01", "Anula Documento de Referencia"},
		{"reference_codes", "16", "Sustituye comprobante electrónico rechazado"},
		{"reference_document_types", "10", "Comprobante electrónico rechazado por el Ministerio de Hacienda"},
		{"discount_codes", "04", "Descuento por volumen"},
		{"units_of_measure", "cm", "centímetro"},
		{"units_of_measure", "Cm", "Comisiones"},
		{"units_of_measure", "Unid", "Unidad"},
		{"sale_conditions", "02", "Crédito"},
	} {
		if got := byTable[want.table][want.code]; got != want.name {
			t.Errorf("%s %s = %q, se esperaba %q", want.table, want.code, got, want.name)
		}
	}
}

func TestParseRejects(t *testing.T) {
	header := "# fuente: x.pdf\n# fuente_sha256: " + strings.Repeat("a", 64) + "\n# version: 1\n# cita: Nota 1\n"
	for name, raw := range map[string]string{
		"tabla no admitida": "# tabla: core.users\n" + header + "code,name\n01,A\n",
		"sin cita":          "# tabla: payment_methods\n# fuente: x.pdf\n# fuente_sha256: " + strings.Repeat("a", 64) + "\n# version: 1\ncode,name\n01,A\n",
		"código repetido":   "# tabla: payment_methods\n" + header + "code,name\n01,A\n01,B\n",
		"sin filas":         "# tabla: payment_methods\n" + header + "code,name\n",
		"encabezado":        "# tabla: payment_methods\n" + header + "codigo,nombre\n01,A\n",
	} {
		if _, err := Parse(name, raw); err == nil {
			t.Errorf("%s: se esperaba un error", name)
		}
	}
}

// Las tarifas de la Nota 8.1 y las condiciones con plazo (pág. 25) llevan su columna adicional.
func TestExtraColumns(t *testing.T) {
	cs, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		switch c.Table {
		case "tax_rates":
			want := map[string]string{"08": "13", "09": "0.5", "10": "0", "04": "4"}
			for _, r := range c.Rows {
				if w, ok := want[r.Code]; ok && r.Extra[0] != w {
					t.Errorf("tarifa %s = %s, se esperaba %s", r.Code, r.Extra[0], w)
				}
			}
		case "sale_conditions":
			for _, r := range c.Rows {
				if (r.Extra[0] == "true") != (r.Code == "02" || r.Code == "10") {
					t.Errorf("condición %s: requires_credit_term = %s", r.Code, r.Extra[0])
				}
			}
		}
	}
}
