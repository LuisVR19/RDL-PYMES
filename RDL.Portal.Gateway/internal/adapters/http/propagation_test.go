package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rdl/portal-gateway/internal/domain/routes"
)

// Paso 2 del prompt P7: identidad, tenancy y propagación. Un error aquí mezcla datos de dos empresas en una
// misma pantalla, así que cada regla tiene su prueba.

func TestForwardsOnlyWhitelistedHeaders(t *testing.T) {
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/customers", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("X-Correlation-Id", "0f7c4a58-0000-4000-8000-00000000000c")
	req.Header.Set("Accept-Language", "es-CR")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	// Nada de esto puede llegar a la API: ni cabeceras de infraestructura ni intentos de forzar la organización.
	req.Header.Set("X-Organization-Id", "11111111-1111-4111-8111-111111111111")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("Cookie", "session=lo-que-sea")
	req.Header.Set("X-Real-Ip", "203.0.113.9")

	if rec := g.do(req); rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}

	got := g.up(t, routes.Billing).last(t)
	for _, name := range []string{"Authorization", "X-Correlation-Id", "Accept-Language", "Traceparent"} {
		if got.Header.Get(name) == "" {
			t.Errorf("%s no se propagó", name)
		}
	}
	for _, name := range []string{"X-Organization-Id", "X-Forwarded-For", "Cookie", "X-Real-Ip"} {
		if v := got.Header.Get(name); v != "" {
			t.Errorf("%s llegó a la API con %q: no está en la lista blanca", name, v)
		}
	}
}

// El token que recibe la API es el del usuario, byte por byte. Nunca uno de servicio.
func TestForwardsTheUsersTokenVerbatim(t *testing.T) {
	g := newGateway(t, options{})
	g.get("/portal/v1/customers")

	if got := g.up(t, routes.Billing).last(t).Header.Get("Authorization"); got != "Bearer "+userToken {
		t.Errorf("Authorization=%q, want %q", got, "Bearer "+userToken)
	}
}

// Un correlation id válido del cliente se respeta; uno inventado se reemplaza (audit lo guarda como uuid).
func TestCorrelationIDIsPropagatedAndReturned(t *testing.T) {
	const valid = "0f7c4a58-0000-4000-8000-00000000000c"
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/customers", nil)
	req.Header.Set("X-Correlation-Id", valid)
	rec := g.do(req)

	if got := g.up(t, routes.Billing).last(t).Header.Get("X-Correlation-Id"); got != valid {
		t.Errorf("la API recibió %q, want %q", got, valid)
	}
	if got := rec.Header().Get("X-Correlation-Id"); got != valid {
		t.Errorf("la respuesta devolvió %q, want %q", got, valid)
	}

	req = httptest.NewRequest(http.MethodGet, "/portal/v1/customers", nil)
	req.Header.Set("X-Correlation-Id", "no-es-uuid")
	rec = g.do(req)

	got := g.up(t, routes.Billing).last(t).Header.Get("X-Correlation-Id")
	if got == "no-es-uuid" || got == "" {
		t.Errorf("un correlation id inválido debe reemplazarse, llegó %q", got)
	}
	if rec.Header().Get("X-Correlation-Id") != got {
		t.Errorf("la respuesta y la API deben llevar el mismo id")
	}
}

// Una composición llama a tres APIs: las tres tienen que ver el MISMO correlation id.
func TestOneCorrelationIDReachesEveryAPIOfAComposition(t *testing.T) {
	g := newGateway(t, options{})
	rec := g.get("/portal/v1/invoices/inv-1/overview")
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}

	want := rec.Header().Get("X-Correlation-Id")
	if want == "" {
		t.Fatal("la respuesta no trae correlation id")
	}
	for _, s := range []routes.Service{routes.Billing, routes.Fiscal, routes.Receivables} {
		if got := g.up(t, s).last(t).Header.Get("X-Correlation-Id"); got != want {
			t.Errorf("%s recibió %q, want %q", s, got, want)
		}
	}
}

// Un token inválido se corta en el gateway: ninguna API se entera.
func TestInvalidTokenIsRejectedBeforeCallingAnyAPI(t *testing.T) {
	g := newGateway(t, options{verifier: fakeVerifier{err: errInvalidToken}})

	rec := g.get("/portal/v1/customers")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code=%d, want 401", rec.Code)
	}
	if n := g.totalCalls(); n != 0 {
		t.Errorf("%d llamadas a las APIs con un token inválido", n)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type=%q", ct)
	}
}

func TestMissingAuthorizationIsRejected(t *testing.T) {
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/customers", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // no es Bearer
	if rec := g.do(req); rec.Code != http.StatusUnauthorized {
		t.Errorf("code=%d, want 401", rec.Code)
	}
	if n := g.totalCalls(); n != 0 {
		t.Errorf("%d llamadas sin token válido", n)
	}
}

// El gateway nunca agrega una organización, y tampoco deja que el cliente la cuele por la query.
func TestOrganizationNeverTravelsToTheAPIs(t *testing.T) {
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodGet,
		"/portal/v1/customers?q=almendros&organizationId=11111111-1111-4111-8111-111111111111&org_id=otra&limit=20", nil)
	if rec := g.do(req); rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}

	got := g.up(t, routes.Billing).last(t)
	if strings.Contains(strings.ToLower(got.Query), "organization") || strings.Contains(got.Query, "org_id") {
		t.Errorf("la query llegó con la organización: %q", got.Query)
	}
	// Lo legítimo sí pasa.
	if !strings.Contains(got.Query, "q=almendros") || !strings.Contains(got.Query, "limit=20") {
		t.Errorf("se perdieron parámetros legítimos: %q", got.Query)
	}
}

// Los errores de las APIs se reenvían sin cambios: mismo status y mismo `type`.
func TestUpstreamProblemDetailsAreForwardedUnchanged(t *testing.T) {
	cases := []struct {
		code        int
		problemType string
	}{
		{http.StatusNotFound, "urn:rdl:billing:problem:not-found"},
		{http.StatusForbidden, "urn:rdl:billing:problem:forbidden"},
		{http.StatusConflict, "urn:rdl:billing:problem:conflict"},
		{http.StatusUnprocessableEntity, "urn:rdl:billing:problem:validation"},
	}
	for _, c := range cases {
		g := newGateway(t, options{})
		g.up(t, routes.Billing).setResponse(status(c.code, c.problemType))

		rec := g.get("/portal/v1/customers/7c9e6679-7425-40de-944b-e07fc1f90ae7")
		if rec.Code != c.code {
			t.Errorf("code=%d, want %d", rec.Code, c.code)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("respuesta ilegible: %v", err)
		}
		if body["type"] != c.problemType {
			t.Errorf("type=%v, want %v: el gateway no reescribe el error de la API", body["type"], c.problemType)
		}
	}
}

// Solo existe lo que está en la tabla de rutas: nada de reverse proxy abierto.
func TestUndeclaredRoutesAreNotProxied(t *testing.T) {
	g := newGateway(t, options{})

	for _, path := range []string{
		"/portal/v1/no-existe",
		"/portal/v1/internal/v1/invoices/x/summary", // ruta interna: jamás se expone al portal
		"/portal/v1/organizations/otra/users",
	} {
		if rec := g.get(path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d, want 404", path, rec.Code)
		}
	}
	if n := g.totalCalls(); n != 0 {
		t.Errorf("%d llamadas a las APIs por rutas no declaradas", n)
	}
}

// Una ruta declarada con otro método responde 405 en Problem Details, no un 404 genérico.
func TestDeclaredPathWithWrongMethodIs405(t *testing.T) {
	g := newGateway(t, options{})

	rec := g.do(httptest.NewRequest(http.MethodDelete, "/portal/v1/customers", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("code=%d, want 405", rec.Code)
	}
	if n := g.totalCalls(); n != 0 {
		t.Errorf("%d llamadas con un método no declarado", n)
	}
}

// Las rutas de E-Invoice y Receivables existen desde hoy: responden 503 "todavía no", no 404.
// Así el portal distingue "esta función aún no está" de "esta ruta no existe".
func TestUnconfiguredUpstreamAnswersNotConfigured(t *testing.T) {
	g := newGateway(t, options{unconfigured: []routes.Service{routes.Fiscal, routes.Receivables}})

	for _, path := range []string{"/portal/v1/fiscal-profile", "/portal/v1/receivables", "/portal/v1/payments"} {
		rec := g.get(path)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: code=%d, want 503", path, rec.Code)
		}
		var body map[string]any
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("%s: respuesta ilegible: %v", path, err)
		}
		if body["type"] != "urn:rdl:portal-gateway:problem:upstream-not-configured" {
			t.Errorf("%s: type=%v", path, body["type"])
		}
	}
}

// El preflight se responde antes de autenticar: el navegador no manda el token en un OPTIONS.
func TestCORSPreflightDoesNotRequireAToken(t *testing.T) {
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodOptions, "/portal/v1/customers", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Del("Authorization")

	rec := httptest.NewRecorder()
	g.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("code=%d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Allow-Origin=%q", got)
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Idempotency-Key") {
		t.Errorf("Allow-Headers=%q", rec.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	g := newGateway(t, options{})

	req := httptest.NewRequest(http.MethodGet, "/portal/v1/customers", nil)
	req.Header.Set("Origin", "https://sitio-de-otro.example")
	rec := g.do(req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin=%q para un origen no declarado", got)
	}
}

func TestResponseHeadersAreFiltered(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/v1/customers/nuevo")
		w.Header().Set("X-Powered-By", "postgrest") // detalle interno
		w.Header().Set("Set-Cookie", "interna=1")   // jamás hacia el navegador
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"nuevo"}`)
	})

	rec := g.do(httptest.NewRequest(http.MethodPost, "/portal/v1/customers", strings.NewReader(`{}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d", rec.Code)
	}
	if rec.Header().Get("Location") == "" {
		t.Error("Location debería propagarse")
	}
	for _, name := range []string{"X-Powered-By", "Set-Cookie"} {
		if v := rec.Header().Get(name); v != "" {
			t.Errorf("%s llegó al navegador con %q", name, v)
		}
	}
}
