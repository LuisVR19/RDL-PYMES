package http_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"rdl/portal-gateway/internal/domain/routes"
)

// Listado de documentos (pantalla 12): una página de Billing enriquecida con el estado fiscal y el saldo, con
// UNA llamada por API por página. Las rutas por lote son las de `openapi/bff-internal.yaml` del contrato.

func rowID(i int) string { return fmt.Sprintf("7c9e6679-7425-40de-944b-%012d", i) }

// billingPage responde como GET /v1/invoices de Billing: n facturas y un cursor.
func billingPage(n int) string {
	items := make([]string, 0, n)
	for i := range n {
		items = append(items, `{"id":"`+rowID(i)+`","documentType":"invoice","number":"FE`+fmt.Sprint(i)+`","status":"issued",
		  "requiresCorrection":false,"customerId":"c1","customerSnapshot":{"legalName":"Cliente `+fmt.Sprint(i)+`"},
		  "issuedAt":"2026-09-25T15:00:00Z","dueDate":"2026-10-25","currency":"CRC","total":"113000.00",
		  "createdAt":"2026-09-25T14:00:00Z","lines":[{"lineNumber":1}]}`)
	}
	return `{"items":[` + strings.Join(items, ",") + `],"nextCursor":"sig"}`
}

func seedList(t *testing.T, g *gateway, n int) {
	t.Helper()
	g.up(t, routes.Billing).setResponse(respondJSON(billingPage(n)))
	// Fiscal conoce solo la primera factura; Receivables, solo la segunda.
	g.up(t, routes.Fiscal).setResponse(respondJSON(
		`{"items":[{"sourceDocumentId":"` + rowID(0) + `","electronicDocumentId":"d1f0b3a4-0000-4000-8000-00000000000d","status":"accepted"}]}`))
	g.up(t, routes.Receivables).setResponse(respondJSON(
		`{"items":[{"invoiceId":"` + rowID(1) + `","receivableId":"a2e1c5b6-0000-4000-8000-00000000000e","status":"open",
		  "currency":"CRC","balanceAmount":"63000.00","dueOn":"2026-10-25"}]}`))
}

type listBody struct {
	Items []struct {
		Invoice    map[string]any `json:"invoice"`
		Fiscal     map[string]any `json:"fiscal"`
		Receivable map[string]any `json:"receivable"`
	} `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func getList(t *testing.T, g *gateway, query string) (int, listBody) {
	t.Helper()
	rec := g.get("/portal/v1/invoices" + query)
	var body listBody
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("respuesta ilegible: %s", rec.Body.String())
		}
	}
	return rec.Code, body
}

// El criterio del prompt: una página de N facturas hace exactamente una llamada por API.
func TestInvoiceListMakesOneCallPerAPI(t *testing.T) {
	const n = 25
	g := newGateway(t, options{})
	seedList(t, g, n)

	code, body := getList(t, g, "")
	if code != http.StatusOK || len(body.Items) != n {
		t.Fatalf("code=%d items=%d", code, len(body.Items))
	}
	for _, s := range []routes.Service{routes.Billing, routes.Fiscal, routes.Receivables} {
		if got := len(g.up(t, s).requests()); got != 1 {
			t.Errorf("%s recibió %d llamadas, se esperaba 1", s, got)
		}
	}

	fis := g.up(t, routes.Fiscal).last(t)
	rec := g.up(t, routes.Receivables).last(t)
	if fis.Method != http.MethodPost || fis.Path != "/internal/v1/electronic-documents/by-source" ||
		rec.Method != http.MethodPost || rec.Path != "/internal/v1/receivables/by-invoice" {
		t.Fatalf("lotes: %s %s · %s %s", fis.Method, fis.Path, rec.Method, rec.Path)
	}
	for _, r := range []recorded{fis, rec} {
		var batch struct {
			IDs []string `json:"ids"`
		}
		if err := json.Unmarshal([]byte(r.Body), &batch); err != nil || len(batch.IDs) != n || batch.IDs[0] != rowID(0) {
			t.Errorf("cuerpo del lote %s = %s", r.Path, r.Body)
		}
		if r.Header.Get("Authorization") != "Bearer "+userToken || r.Header.Get("Idempotency-Key") != "" {
			t.Errorf("%s: headers %v", r.Path, r.Header)
		}
	}
}

func TestInvoiceListJoinsEachRow(t *testing.T) {
	g := newGateway(t, options{})
	seedList(t, g, 3)

	_, body := getList(t, g, "")
	first, second, third := body.Items[0], body.Items[1], body.Items[2]
	if first.Invoice["number"] != "FE0" || first.Invoice["customerLegalName"] != "Cliente 0" ||
		first.Invoice["total"] != "113000.00" || first.Invoice["issuedAt"] != "2026-09-25T15:00:00Z" {
		t.Errorf("factura = %v", first.Invoice)
	}
	if _, hasLines := first.Invoice["lines"]; hasLines {
		t.Error("las líneas de Billing no viajan al listado")
	}
	if first.Fiscal["availability"] != "available" || second.Fiscal["availability"] != "absent" {
		t.Errorf("fiscal: %v / %v", first.Fiscal, second.Fiscal)
	}
	if b, _ := second.Receivable["balance"].(map[string]any); second.Receivable["availability"] != "available" || b["balanceAmount"] != "63000.00" {
		t.Errorf("saldo = %v", second.Receivable)
	}
	if third.Fiscal["availability"] != "absent" || third.Receivable["availability"] != "absent" {
		t.Errorf("tercera = %v / %v", third.Fiscal, third.Receivable)
	}
	if body.NextCursor == nil || *body.NextCursor != "sig" {
		t.Errorf("nextCursor = %v", body.NextCursor)
	}
}

// Hoy en dev: E-Invoice y Receivables no existen. El listado sale igual, con esas partes unavailable.
func TestInvoiceListWithOnlyBillingDeployed(t *testing.T) {
	g := newGateway(t, options{unconfigured: []routes.Service{routes.Fiscal, routes.Receivables}})
	g.up(t, routes.Billing).setResponse(respondJSON(billingPage(2)))

	code, body := getList(t, g, "")
	if code != http.StatusOK || len(body.Items) != 2 {
		t.Fatalf("code=%d body=%+v", code, body)
	}
	for _, it := range body.Items {
		if it.Fiscal["availability"] != "unavailable" || it.Receivable["availability"] != "unavailable" {
			t.Errorf("fila = %v / %v", it.Fiscal, it.Receivable)
		}
	}
}

// Un lote que falla no reintenta (regla del repo: solo GET) y no tumba la tabla.
func TestInvoiceListSurvivesAFailingBatch(t *testing.T) {
	g := newGateway(t, options{})
	seedList(t, g, 2)
	g.up(t, routes.Fiscal).setResponse(func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	code, body := getList(t, g, "")
	if code != http.StatusOK || body.Items[0].Fiscal["availability"] != "unavailable" ||
		body.Items[1].Receivable["availability"] != "available" {
		t.Fatalf("code=%d body=%+v", code, body)
	}
	if got := len(g.up(t, routes.Fiscal).requests()); got != 1 {
		t.Errorf("el lote se llamó %d veces: un POST no se reintenta", got)
	}
}

func TestInvoiceListForwardsOnlyDeclaredFilters(t *testing.T) {
	g := newGateway(t, options{})
	seedList(t, g, 1)

	getList(t, g, "?status=issued&limit=50&cursor=abc&issuedFrom=2026-09-01&organizationId="+orgID+"&org_id=x&otro=1")
	got := g.up(t, routes.Billing).last(t)
	if got.Path != "/v1/invoices" {
		t.Fatalf("Billing recibió %s", got.Path)
	}
	q, _ := url.ParseQuery(got.Query)
	if q.Get("status") != "issued" || q.Get("limit") != "50" || q.Get("cursor") != "abc" || q.Get("issuedFrom") != "2026-09-01" {
		t.Errorf("filtros = %v", q)
	}
	for _, k := range []string{"organizationId", "org_id", "otro"} {
		if q.Has(k) {
			t.Errorf("%s llegó a Billing: %v", k, q)
		}
	}
	if strings.Contains(got.Query, orgID) {
		t.Error("la organización viajó en la query")
	}
}

func TestInvoiceListEmptyPageCallsOnlyBilling(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(respondJSON(`{"items":[],"nextCursor":null}`))

	code, body := getList(t, g, "")
	if code != http.StatusOK || len(body.Items) != 0 || body.NextCursor != nil {
		t.Fatalf("code=%d body=%+v", code, body)
	}
	if n := len(g.up(t, routes.Fiscal).requests()) + len(g.up(t, routes.Receivables).requests()); n != 0 {
		t.Errorf("una página vacía llamó %d veces a los lotes", n)
	}
}

// Si Billing responde un error (un limit fuera de rango, un rol sin permiso), el portal recibe el suyo tal cual.
func TestInvoiceListForwardsBillingProblem(t *testing.T) {
	g := newGateway(t, options{})
	g.up(t, routes.Billing).setResponse(func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"type":"urn:rdl:billing:problem:validation","status":422,"title":"Datos inválidos"}`)
	})

	rec := g.get("/portal/v1/invoices?limit=500")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "urn:rdl:billing:problem:validation") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if n := len(g.up(t, routes.Fiscal).requests()) + len(g.up(t, routes.Receivables).requests()); n != 0 {
		t.Errorf("sin página no se consultan lotes (%d llamadas)", n)
	}
}
