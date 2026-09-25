package routes

import (
	"slices"
	"strings"
	"testing"
)

// La tabla es la superficie pública del Portal Gateway: estas pruebas son su contrato de forma.
func TestTableIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Table() {
		key := r.Method + " " + r.Path
		if seen[key] {
			t.Errorf("%s: ruta duplicada (el mux entraría en pánico al registrarla)", key)
		}
		seen[key] = true

		if !strings.HasPrefix(r.Path, "/portal/v1/") {
			t.Errorf("%s: toda ruta pública cuelga de /portal/v1/", key)
		}
		if r.Why == "" {
			t.Errorf("%s: sin pantalla que la justifique", key)
		}
		switch r.Method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
		default:
			t.Errorf("%s: método inesperado", key)
		}
	}
}

func TestPassthroughRoutesTargetOneService(t *testing.T) {
	for _, r := range Table() {
		key := r.Method + " " + r.Path
		if r.Kind != Passthrough {
			continue
		}
		if r.Service == "" || r.Upstream == "" {
			t.Errorf("%s: un paso directo necesita servicio y ruta destino", key)
			continue
		}
		if !slices.Contains(Services(), r.Service) {
			t.Errorf("%s: servicio desconocido %q", key, r.Service)
		}
		if !strings.HasPrefix(r.Upstream, "/v1/") {
			t.Errorf("%s: la ruta destino %q no es /v1/...", key, r.Upstream)
		}
	}
}

func TestComposedRoutesDeclareNoSingleService(t *testing.T) {
	for _, r := range Table() {
		if r.Kind == Composed && (r.Service != "" || r.Upstream != "") {
			t.Errorf("%s %s: una composición no tiene una sola API destino", r.Method, r.Path)
		}
	}
}

// Un comodín del destino que no exista en la ruta pública se expandiría vacío y armaría una URL equivocada.
func TestUpstreamParamsExistInPublicPath(t *testing.T) {
	for _, r := range Table() {
		public := Params(r.Path)
		for _, p := range r.PathParams() {
			if !slices.Contains(public, p) {
				t.Errorf("%s %s: el destino usa {%s}, que no está en la ruta pública", r.Method, r.Path, p)
			}
		}
	}
}

// Condición del incremento: fiscal y receivables están declarados aunque sus repos no existan (ADR 0002).
func TestEveryServiceIsRepresented(t *testing.T) {
	byService := map[Service]int{}
	for _, r := range Table() {
		if r.Kind == Passthrough {
			byService[r.Service]++
		}
	}
	for _, s := range Services() {
		if byService[s] == 0 {
			t.Errorf("el servicio %q no tiene ninguna ruta declarada", s)
		}
	}
}

func TestIsCommand(t *testing.T) {
	cases := map[string]bool{"GET": false, "HEAD": false, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	for method, want := range cases {
		if got := (Route{Method: method}).IsCommand(); got != want {
			t.Errorf("%s: IsCommand()=%v, want %v", method, got, want)
		}
	}
}

func TestParams(t *testing.T) {
	cases := []struct {
		pattern string
		want    []string
	}{
		{"/v1/customers", nil},
		{"/v1/customers/{id}", []string{"id"}},
		{"/v1/electronic-documents/{id}/files/{kind}", []string{"id", "kind"}},
		{"/v1/roto/{sin-cerrar", nil},
	}
	for _, c := range cases {
		if got := Params(c.pattern); !slices.Equal(got, c.want) {
			t.Errorf("Params(%q)=%v, want %v", c.pattern, got, c.want)
		}
	}
}

// La organización sale del token: ninguna ruta pública la recibe como parámetro.
func TestNoRouteTakesAnOrganization(t *testing.T) {
	for _, r := range Table() {
		for _, p := range Params(r.Path) {
			if strings.Contains(strings.ToLower(p), "organization") || strings.EqualFold(p, "orgId") {
				t.Errorf("%s %s: la organización nunca viaja en la ruta", r.Method, r.Path)
			}
		}
	}
}
