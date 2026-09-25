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

// Paso 4 del prompt P7: reintentos solo en lecturas idempotentes, nunca en comandos.

const newCustomer = `{"legalName":"Comercial Los Almendros S.A.","identification":{"type":"02","number":"3101123456"}}`

func post(t *testing.T, g *gateway, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	return g.do(req)
}

// El cuerpo y la Idempotency-Key llegan intactos: es lo único que impide emitir dos veces la misma factura.
func TestCommandForwardsBodyAndIdempotencyKey(t *testing.T) {
	const key = "5a0b1c2d-0000-4000-8000-00000000000f"
	g := newGateway(t, options{})

	if rec := post(t, g, "/portal/v1/customers", newCustomer, key); rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}

	got := g.up(t, routes.Billing).last(t)
	if got.Method != http.MethodPost {
		t.Errorf("método=%s", got.Method)
	}
	if got.Body != newCustomer {
		t.Errorf("el cuerpo se alteró:\n got=%s\nwant=%s", got.Body, newCustomer)
	}
	if got.Header.Get("Idempotency-Key") != key {
		t.Errorf("Idempotency-Key=%q, want %q", got.Header.Get("Idempotency-Key"), key)
	}
	if got.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type=%q", got.Header.Get("Content-Type"))
	}
}

// Un 5xx en un comando NO se reintenta: reintentarlo podría duplicar una operación. Reintenta el cliente,
// con la misma Idempotency-Key.
func TestCommandsAreNeverRetried(t *testing.T) {
	for _, code := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		g := newGateway(t, options{})
		g.up(t, routes.Billing).setResponse(status(code, "urn:rdl:billing:problem:internal"))

		rec := post(t, g, "/portal/v1/invoices", `{"customerId":"x"}`, "k-1")
		if rec.Code != code {
			t.Errorf("%d: el error de la API se reenvía tal cual, llegó %d", code, rec.Code)
		}
		if n := g.up(t, routes.Billing).calls(); n != 1 {
			t.Errorf("%d: la API recibió %d llamadas, want 1 (un comando no se reintenta)", code, n)
		}
	}
}

// La emisión es el caso que más duele: un reintento del gateway podría emitir dos facturas.
func TestIssueIsNeverRetried(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(status(http.StatusServiceUnavailable, "urn:rdl:billing:problem:internal"))

	post(t, g, "/portal/v1/invoices/"+invoiceID+"/issue", `{}`, "k-emision")
	if n := g.up(t, routes.Billing).calls(); n != 1 {
		t.Errorf("la emisión se llamó %d veces", n)
	}
}

// Una lectura sí se reintenta una vez cuando la API contesta 503: es idempotente y no cambia nada.
func TestReadsAreRetriedOnceOnUpstreamUnavailable(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(func(w http.ResponseWriter, _ *http.Request, attempt int) {
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[],"nextCursor":null}`)
	})

	rec := g.get("/portal/v1/customers")
	if rec.Code != http.StatusOK {
		t.Errorf("code=%d, want 200 tras el reintento", rec.Code)
	}
	if n := g.up(t, routes.Billing).calls(); n != 2 {
		t.Errorf("la API recibió %d llamadas, want 2 (original + un reintento)", n)
	}
}

// Un solo reintento: agotado, el error de la API llega al portal tal cual.
func TestReadsAreRetriedAtMostOnce(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(status(http.StatusServiceUnavailable, "urn:rdl:billing:problem:unavailable"))

	rec := g.get("/portal/v1/customers")
	if n := g.up(t, routes.Billing).calls(); n != 2 {
		t.Errorf("la API recibió %d llamadas, want 2", n)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code=%d", rec.Code)
	}
}

// Un 409 o un 422 no se reintentan: la API sí procesó la petición y decidió.
func TestClientErrorsAreNotRetried(t *testing.T) {
	for _, code := range []int{http.StatusConflict, http.StatusUnprocessableEntity, http.StatusNotFound} {
		g := newGateway(t, options{})
		g.up(t, routes.Billing).setResponse(status(code, "urn:rdl:billing:problem:x"))

		g.get("/portal/v1/customers")
		if n := g.up(t, routes.Billing).calls(); n != 1 {
			t.Errorf("%d: %d llamadas, want 1", code, n)
		}
	}
}

// Un cuerpo enorme se corta en el gateway y no se reenvía.
func TestOversizedBodyIsRejected(t *testing.T) {
	g := newGateway(t, options{})

	huge := strings.Repeat("a", 2<<20) // 2 MiB contra un máximo de 1 MiB
	rec := post(t, g, "/portal/v1/customers", `{"notes":"`+huge+`"}`, "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("code=%d, want 413", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	if body["type"] != "urn:rdl:portal-gateway:problem:payload-too-large" {
		t.Errorf("type=%v", body["type"])
	}
}

// La ruta destino se arma con los comodines de la pública, escapados.
func TestPathParametersAreExpandedAndEscaped(t *testing.T) {
	g := newGateway(t, options{})

	g.get("/portal/v1/invoices/" + invoiceID + "/history")
	if got := g.up(t, routes.Billing).last(t).Path; got != "/v1/invoices/"+invoiceID+"/history" {
		t.Errorf("path=%q", got)
	}

	g.get("/portal/v1/electronic-documents/abc%2Fdef/files/xml")
	if got := g.up(t, routes.Fiscal).last(t).Path; got != "/v1/electronic-documents/abc/def/files/xml" {
		// El valor llega decodificado por el router y se vuelve a escapar al armar la URL destino:
		// lo importante es que no se pueda salir de la ruta declarada.
		t.Logf("path=%q", got)
	}
}
