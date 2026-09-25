package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"rdl/portal-gateway/internal/domain/routes"
)

// El ejemplo de la arquitectura 2.2:
//
//	Factura FE00100034 · Total ₡113 000 · Hacienda: Aceptada · Saldo ₡63 000
//
// El total es de Billing, el estado fiscal de E-Invoice y el saldo de Receivables.

const invoiceID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

func respondJSON(body string) func(http.ResponseWriter, *http.Request, int) {
	return func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

// billingInvoice es el 200 de getInvoiceSummary (openapi/bff-internal.yaml), lo que Billing responde en
// GET /internal/v1/invoices/{id}/summary.
func billingInvoice() string {
	return `{"id":"` + invoiceID + `","documentType":"invoice","number":"FE00100034","status":"issued",
	         "requiresCorrection":false,"customerLegalName":"Comercial Los Almendros S.A.",
	         "currency":"CRC","total":"113000.00"}`
}

// billingInvoicePublic es la parte que usa la vista del detalle público GET /v1/invoices/{id}.
func billingInvoicePublic() string {
	return `{"id":"` + invoiceID + `","documentType":"invoice","number":"FE00100034","status":"issued",
	         "requiresCorrection":false,"currency":"CRC","total":"113000.00","lines":[],
	         "customerSnapshot":{"legalName":"Comercial Los Almendros S.A."}}`
}

// seedOverview deja a las tres APIs respondiendo lo del ejemplo.
func seedOverview(t *testing.T, g *gateway) {
	t.Helper()
	g.up(t, routes.Billing).setResponse(respondJSON(billingInvoice()))
	g.up(t, routes.Fiscal).setResponse(respondJSON(
		`{"electronicDocumentId":"d1f0b3a4-0000-4000-8000-00000000000d","status":"accepted"}`))
	g.up(t, routes.Receivables).setResponse(respondJSON(
		`{"receivableId":"a2e1c5b6-0000-4000-8000-00000000000e","status":"partially_paid",
		  "currency":"CRC","balanceAmount":"63000.00","dueOn":"2026-10-24"}`))
}

func getOverview(t *testing.T, g *gateway) (int, map[string]any) {
	t.Helper()
	rec := g.get("/portal/v1/invoices/" + invoiceID + "/overview")
	if rec.Body.Len() == 0 {
		return rec.Code, nil
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respuesta ilegible (%d): %s", rec.Code, rec.Body.String())
	}
	return rec.Code, body
}

func TestOverviewJoinsTheThreeAPIs(t *testing.T) {
	g := newGateway(t, options{})
	seedOverview(t, g)

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%v", code, body)
	}

	invoice, _ := body["invoice"].(map[string]any)
	if invoice["total"] != "113000.00" || invoice["number"] != "FE00100034" {
		t.Errorf("factura=%v", invoice)
	}
	// El monto viaja como string decimal de punta a punta: nunca un number de JSON.
	if _, isNumber := invoice["total"].(float64); isNumber {
		t.Error("el total llegó como número de JSON: los montos son string decimal")
	}

	fiscal, _ := body["fiscal"].(map[string]any)
	if fiscal["availability"] != "available" {
		t.Errorf("fiscal=%v", fiscal)
	}
	if st, _ := fiscal["status"].(map[string]any); st["status"] != "accepted" {
		t.Errorf("estado fiscal=%v", fiscal["status"])
	}

	receivable, _ := body["receivable"].(map[string]any)
	if receivable["availability"] != "available" {
		t.Errorf("saldo=%v", receivable)
	}
	if b, _ := receivable["balance"].(map[string]any); b["balanceAmount"] != "63000.00" {
		t.Errorf("saldo=%v", receivable["balance"])
	}
}

// Por defecto el resumen sale de la ruta interna del contrato, con el token del usuario tal cual.
func TestOverviewReadsTheContractSummaryFromBilling(t *testing.T) {
	g := newGateway(t, options{})
	seedOverview(t, g)

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%v", code, body)
	}
	got := g.up(t, routes.Billing).last(t)
	if got.Path != "/internal/v1/invoices/"+invoiceID+"/summary" {
		t.Errorf("Billing recibió %s", got.Path)
	}
	if got.Header.Get("Authorization") != "Bearer "+userToken {
		t.Errorf("Authorization=%q", got.Header.Get("Authorization"))
	}
	if inv, _ := body["invoice"].(map[string]any); inv["customerLegalName"] != "Comercial Los Almendros S.A." {
		t.Errorf("factura=%v", inv)
	}
}

// Respaldo para una Billing anterior a la ruta interna: la misma vista, derivada del detalle público.
func TestOverviewCanStillDeriveTheSummaryFromThePublicDetail(t *testing.T) {
	g := newGateway(t, options{summarySource: "public"})
	seedOverview(t, g)
	g.up(t, routes.Billing).setResponse(respondJSON(billingInvoicePublic()))

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%v", code, body)
	}
	if got := g.up(t, routes.Billing).last(t).Path; got != "/v1/invoices/"+invoiceID {
		t.Errorf("Billing recibió %s", got)
	}
	inv, _ := body["invoice"].(map[string]any)
	if inv["customerLegalName"] != "Comercial Los Almendros S.A." || inv["total"] != "113000.00" {
		t.Errorf("factura=%v", inv)
	}
}

// Criterio 2: el portal sigue mostrando la factura aunque E-Invoice esté caída.
func TestOverviewSurvivesAFallenFiscalAPI(t *testing.T) {
	g := newGateway(t, options{})
	seedOverview(t, g)
	g.up(t, routes.Fiscal).setResponse(status(http.StatusInternalServerError, "urn:rdl:fiscal:problem:internal"))

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d: la vista no puede fallar entera", code)
	}
	if invoice, _ := body["invoice"].(map[string]any); invoice["total"] != "113000.00" {
		t.Errorf("la parte principal tiene que salir igual: %v", invoice)
	}
	fiscal, _ := body["fiscal"].(map[string]any)
	if fiscal["availability"] != "unavailable" {
		t.Errorf("fiscal=%v, want availability unavailable", fiscal)
	}
	if _, hasData := fiscal["status"]; hasData {
		t.Errorf("no puede publicar datos de una parte que no pudo consultar: %v", fiscal)
	}
	if receivable, _ := body["receivable"].(map[string]any); receivable["availability"] != "available" {
		t.Errorf("el saldo sí estaba: %v", receivable)
	}
}

// Hoy E-Invoice y Receivables no existen: la vista sale igual, marcada, sin tocar nada del código.
func TestOverviewWorksWithOnlyBillingDeployed(t *testing.T) {
	g := newGateway(t, options{unconfigured: []routes.Service{routes.Fiscal, routes.Receivables}})
	g.up(t, routes.Billing).setResponse(respondJSON(billingInvoice()))

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d body=%v", code, body)
	}
	if invoice, _ := body["invoice"].(map[string]any); invoice["number"] != "FE00100034" {
		t.Errorf("factura=%v", invoice)
	}
	for _, part := range []string{"fiscal", "receivable"} {
		p, _ := body[part].(map[string]any)
		if p["availability"] != "unavailable" {
			t.Errorf("%s=%v, want unavailable", part, p)
		}
	}
}

// Un borrador todavía no tiene documento electrónico ni cuenta por cobrar: eso es "absent", no una falla.
func TestOverviewMarksMissingPartsAsAbsent(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(respondJSON(billingInvoice()))
	g.up(t, routes.Fiscal).setResponse(status(http.StatusNotFound, "urn:rdl:fiscal:problem:not-found"))
	g.up(t, routes.Receivables).setResponse(status(http.StatusNotFound, "urn:rdl:receivables:problem:not-found"))

	code, body := getOverview(t, g)
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	for _, part := range []string{"fiscal", "receivable"} {
		p, _ := body[part].(map[string]any)
		if p["availability"] != "absent" {
			t.Errorf("%s=%v, want absent", part, p)
		}
	}
}

// Receivables lento: se respeta su timeout y la vista sale sin saldo, sin esperar a que conteste.
func TestOverviewRespectsPerAPITimeout(t *testing.T) {
	g := newGateway(t, options{timeouts: map[routes.Service]time.Duration{routes.Receivables: 40 * time.Millisecond}})
	seedOverview(t, g)
	g.up(t, routes.Receivables).setResponse(func(w http.ResponseWriter, _ *http.Request, _ int) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	start := time.Now()
	code, body := getOverview(t, g)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("esperó %v: no respetó el timeout de la API", elapsed)
	}
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	if receivable, _ := body["receivable"].(map[string]any); receivable["availability"] != "unavailable" {
		t.Errorf("saldo=%v", receivable)
	}
	if fiscal, _ := body["fiscal"].(map[string]any); fiscal["availability"] != "available" {
		t.Errorf("la lentitud de una API no puede afectar a otra: %v", fiscal)
	}
}

// Si falla la fuente PRINCIPAL, se responde el error de Billing tal cual: mismo status y mismo `type`.
func TestOverviewForwardsBillingErrorWhenPrimaryFails(t *testing.T) {
	for _, c := range []struct {
		code        int
		problemType string
	}{
		{http.StatusNotFound, "urn:rdl:billing:problem:not-found"},
		{http.StatusForbidden, "urn:rdl:billing:problem:forbidden"},
	} {
		g := newGateway(t, options{})
		seedOverview(t, g)
		g.up(t, routes.Billing).setResponse(status(c.code, c.problemType))

		code, body := getOverview(t, g)
		if code != c.code {
			t.Errorf("code=%d, want %d", code, c.code)
		}
		if body["type"] != c.problemType {
			t.Errorf("type=%v, want %v", body["type"], c.problemType)
		}
	}
}

// Una composición hace UNA llamada por API: nada de N+1 escondido.
func TestOverviewCallsEachAPIOnce(t *testing.T) {
	g := newGateway(t, options{})
	seedOverview(t, g)

	if code, _ := getOverview(t, g); code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	for _, s := range []routes.Service{routes.Billing, routes.Fiscal, routes.Receivables} {
		if n := g.up(t, s).calls(); n != 1 {
			t.Errorf("%s recibió %d llamadas, want 1", s, n)
		}
	}
	// Y a Platform no se le pregunta nada: la membresía la revalida cada API, no el gateway.
	if n := g.up(t, routes.Platform).calls(); n != 0 {
		t.Errorf("platform recibió %d llamadas en una vista de factura", n)
	}
}
