package http_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"rdl/portal-gateway/internal/domain/routes"
)

// api/openapi.yaml es lo que lee quien construye el portal. Si se separa de la tabla de rutas, el portal
// llama a algo que no existe o se pierde una pantalla. Esta prueba lo impide sin meter un parser de YAML:
// compara el conjunto de rutas declaradas en cada lado.
//
// TODO(contracts): cuando `contractsctl` valide también este archivo, esta prueba puede simplificarse.

// Una ruta de primer nivel del bloque `paths:`: dos espacios de sangría y termina en dos puntos.
var pathLine = regexp.MustCompile(`(?m)^  (/[^\s:]*):`)

func openAPIPaths(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("no se pudo leer api/openapi.yaml: %v", err)
	}
	body := string(raw)
	start := strings.Index(body, "\npaths:")
	if start < 0 {
		t.Fatal("api/openapi.yaml no tiene bloque paths:")
	}
	end := strings.Index(body, "\ncomponents:")
	if end < 0 {
		end = len(body)
	}

	var out []string
	for _, m := range pathLine.FindAllStringSubmatch(body[start:end], -1) {
		if strings.HasPrefix(m[1], "/portal/v1/") {
			out = append(out, m[1])
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func tablePaths() []string {
	var out []string
	for _, r := range routes.Table() {
		out = append(out, r.Path)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestOpenAPIMatchesRouteTable(t *testing.T) {
	documented := openAPIPaths(t)
	declared := tablePaths()

	if len(documented) == 0 {
		t.Fatal("no se leyó ninguna ruta de api/openapi.yaml: revise el formato del archivo")
	}
	for _, p := range declared {
		if !slices.Contains(documented, p) {
			t.Errorf("%s está en la tabla de rutas pero no en api/openapi.yaml", p)
		}
	}
	for _, p := range documented {
		if !slices.Contains(declared, p) {
			t.Errorf("%s está en api/openapi.yaml pero el gateway no lo sirve (no está en la tabla)", p)
		}
	}
}

// Los montos son string decimal de punta a punta: un `number` en el contrato del gateway sería un bug
// de redondeo esperando a pasar (convenciones §3 del repo de contratos).
func TestOpenAPIHasNoNumericMoney(t *testing.T) {
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("no se pudo leer api/openapi.yaml: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "amount") && !strings.Contains(low, "total") {
			continue
		}
		if strings.Contains(low, "type: number") || strings.Contains(low, "type: integer") {
			t.Errorf("monto declarado como número: %s", strings.TrimSpace(line))
		}
	}
}
