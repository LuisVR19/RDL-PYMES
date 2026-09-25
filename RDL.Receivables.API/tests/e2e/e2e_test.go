//go:build integration

// Package e2e recorre los flujos de punta a punta contra la base de dev con el router y el procesador de eventos
// reales (docs/PLAN.md §6, incremento 10): factura emitida, pago que la cancela, ReceivableSettled en el outbox,
// anulación del pago con el saldo de vuelta, notas, anulación de factura, aging y cobranza.
//
//	ISOLATION_ORG_A=... ISOLATION_ORG_B=... go test -count=1 -tags=integration ./tests/e2e/...
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/tests/testkit"
)

var env *testkit.Env

func TestMain(m *testing.M) {
	e, reason, err := testkit.Setup(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
	if reason != "" {
		fmt.Fprintln(os.Stderr, "AVISO: e2e saltada:", reason)
		os.Exit(0)
	}
	env = e
	code := m.Run()
	env.Pool.Close()
	os.Exit(code)
}

// Factura → pago que la cancela (ReceivableSettled) → repetición idempotente → anulación del pago (saldo de vuelta).
func TestPaymentSettlesAndVoidRestoresBalance(t *testing.T) {
	a := env.A
	tok := a.OwnerToken(env)
	inv := env.IssueInvoice(t, a.Org, "11300.50", "2026-10-01")
	acc := env.ReceivableOf(t, a, inv.ID)
	if acc.Str("status") != "open" || acc.Str("balanceAmount") != "11300.5" {
		t.Fatalf("cuenta nueva: %v", acc.Body)
	}
	receivableID := acc.Str("receivableId")

	body := map[string]any{
		"customerId": inv.Customer.String(), "receivedOn": "2026-09-20", "amount": "12000", "currency": "CRC",
		"paymentMethodCode": "04", "reference": "TRF-E2E",
		"applications": []map[string]any{{"receivableId": receivableID, "amount": "11300.50"}},
	}
	key := "e2e-" + uuid.NewString()
	pay := env.Call(t, http.MethodPost, "/v1/payments", tok, body, "Idempotency-Key", key)
	testkit.Expect(t, "registrar pago", http.StatusCreated, pay)
	paymentID := uuid.MustParse(pay.Str("id"))
	if apps, _ := pay.Body["applications"].([]any); len(apps) != 1 || pay.Str("status") != "posted" {
		t.Fatalf("pago: %v", pay.Body)
	}
	if acc = env.ReceivableOf(t, a, inv.ID); acc.Str("status") != "paid" || acc.Str("balanceAmount") != "0" {
		t.Fatalf("después del pago: %v", acc.Body)
	}

	// Outbox en la misma transacción: PaymentReceived y ReceivableSettled, con el mismo correlationId.
	received := env.Outbox(t, a.Org, "PaymentReceived", paymentID)
	settled := env.Outbox(t, a.Org, "ReceivableSettled", uuid.MustParse(receivableID))
	if len(received) != 1 || len(settled) != 1 {
		t.Fatalf("outbox: %d PaymentReceived, %d ReceivableSettled", len(received), len(settled))
	}
	if received[0]["correlationId"] != settled[0]["correlationId"] || settled[0]["sourceInvoiceId"] != inv.ID.String() {
		t.Errorf("eventos: %v / %v", received[0], settled[0])
	}

	// La misma Idempotency-Key con el mismo cuerpo no registra otro pago; con otro cuerpo, 422.
	again := env.Call(t, http.MethodPost, "/v1/payments", tok, body, "Idempotency-Key", key)
	testkit.Expect(t, "repetición idempotente", http.StatusCreated, again)
	if again.Str("id") != paymentID.String() || len(env.Outbox(t, a.Org, "PaymentReceived", paymentID)) != 1 {
		t.Errorf("la repetición creó otro pago o otro evento: %v", again.Body)
	}
	body["reference"] = "OTRA"
	testkit.Expect(t, "misma clave, otro cuerpo", http.StatusUnprocessableEntity,
		env.Call(t, http.MethodPost, "/v1/payments", tok, body, "Idempotency-Key", key))

	// Anular revierte la aplicación: la cuenta vuelve exactamente a su saldo y a open.
	void := env.Call(t, http.MethodPost, "/v1/payments/"+paymentID.String()+"/void", tok, map[string]any{"reason": "Cheque rechazado"})
	testkit.Expect(t, "anular pago", http.StatusOK, void)
	if void.Str("status") != "voided" {
		t.Fatalf("pago anulado: %v", void.Body)
	}
	if acc = env.ReceivableOf(t, a, inv.ID); acc.Str("status") != "open" || acc.Str("balanceAmount") != "11300.5" {
		t.Fatalf("después de anular: %v", acc.Body)
	}
	testkit.Expect(t, "anular dos veces", http.StatusConflict,
		env.Call(t, http.MethodPost, "/v1/payments/"+paymentID.String()+"/void", tok, map[string]any{"reason": "otra vez"}))

	detail := env.Call(t, http.MethodGet, "/v1/receivables/"+receivableID, tok, nil)
	testkit.Expect(t, "detalle", http.StatusOK, detail)
	apps, _ := detail.Body["applications"].([]any)
	if len(apps) != 1 || apps[0].(map[string]any)["reversalReason"] != "Cheque rechazado" {
		t.Errorf("la aplicación revertida queda como historial: %v", detail.Body["applications"])
	}
}

// Un pago a dos cuentas, aplicación posterior, reverso y reglas del contrato (saldo, disponible, moneda, cliente).
func TestApplicationsAndReversal(t *testing.T) {
	a := env.A
	tok := a.OwnerToken(env)
	inv1 := env.IssueInvoice(t, a.Org, "100", "2026-10-01")
	r1 := env.ReceivableOf(t, a, inv1.ID).Str("receivableId")
	// Segunda factura del mismo cliente.
	inv2 := testkit.Invoice{ID: uuid.New(), Customer: inv1.Customer}
	env.MustProcess(t, testkit.Event(t, "invoice-issued.v1.json", a.Org, map[string]any{
		"invoiceId": inv2.ID.String(), "invoiceNumber": "FAC-T2" + inv2.ID.String()[:6], "total": "50",
		"issueDate": "2026-09-01", "dueDate": "2026-10-01",
		"customerSnapshot": map[string]any{"customerId": inv1.Customer.String(), "legalName": "Cliente de prueba S.A.",
			"identification": map[string]any{"typeCode": "02", "number": "3101999999"}},
	}), app.OutcomeProcessed)
	r2 := env.ReceivableOf(t, a, inv2.ID).Str("receivableId")

	pay := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": inv1.Customer.String(), "receivedOn": "2026-09-20", "amount": "120", "currency": "CRC",
		"paymentMethodCode": "01",
		"applications":      []map[string]any{{"receivableId": r1, "amount": "80"}},
	})
	testkit.Expect(t, "pago parcial", http.StatusCreated, pay)
	pid := pay.Str("id")
	// Pago grande sin aplicar: para probar el saldo de la cuenta sin chocar antes con el disponible del pago.
	big := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": inv1.Customer.String(), "receivedOn": "2026-09-20", "amount": "1000", "currency": "CRC",
		"paymentMethodCode": "01",
	})
	testkit.Expect(t, "pago grande", http.StatusCreated, big)

	cases := []struct {
		name string
		body map[string]any
		want int
		typ  string
	}{
		{"supera el saldo", map[string]any{"paymentId": big.Str("id"), "receivableId": r2, "amount": "50.00001"}, 422, "application-exceeds-balance"},
		{"mismo pago a la misma cuenta", map[string]any{"paymentId": pid, "receivableId": r1, "amount": "20"}, 409, "conflict"},
		{"supera el disponible", map[string]any{"paymentId": pid, "receivableId": r2, "amount": "40.00001"}, 422, "application-exceeds-payment"},
		{"monto con 6 decimales", map[string]any{"paymentId": pid, "receivableId": r2, "amount": "1.000001"}, 422, "validation"},
		{"cuenta que no existe", map[string]any{"paymentId": pid, "receivableId": uuid.NewString(), "amount": "1"}, 404, "not-found"},
	}
	for _, c := range cases {
		r := env.Call(t, http.MethodPost, "/v1/payment-applications", tok, c.body)
		if r.Status != c.want || r.Str("type") != "urn:rdl:receivables:problem:"+c.typ {
			t.Errorf("%s: %d %s", c.name, r.Status, r.Str("type"))
		}
	}
	ok := env.Call(t, http.MethodPost, "/v1/payment-applications", tok, map[string]any{"paymentId": pid, "receivableId": r2, "amount": "40"})
	testkit.Expect(t, "aplicar el resto", http.StatusCreated, ok)
	r := env.Call(t, http.MethodPost, "/v1/payment-applications", tok, map[string]any{"paymentId": pid, "receivableId": r2, "amount": "1"})
	if r.Status != 409 && r.Status != 422 {
		t.Errorf("sin disponible: %d", r.Status)
	}

	rev := env.Call(t, http.MethodPost, "/v1/payment-applications/"+ok.Str("id")+"/reverse", tok, map[string]any{"reason": "Monto equivocado"})
	testkit.Expect(t, "revertir", http.StatusOK, rev)
	if rev.Str("reversalReason") != "Monto equivocado" {
		t.Errorf("reverso: %v", rev.Body)
	}
	if b := env.ReceivableOf(t, a, inv2.ID); b.Str("status") != "open" || b.Str("balanceAmount") != "50" {
		t.Errorf("cuenta 2 después del reverso: %v", b.Body)
	}
	again := env.Call(t, http.MethodPost, "/v1/payment-applications/"+ok.Str("id")+"/reverse", tok, map[string]any{"reason": "otra"})
	if again.Status != 409 || again.Str("type") != "urn:rdl:receivables:problem:application-reversed" {
		t.Errorf("revertir dos veces: %d %v", again.Status, again.Body)
	}

	// Otro cliente y otra moneda.
	other := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": uuid.NewString(), "receivedOn": "2026-09-20", "amount": "10", "currency": "CRC", "paymentMethodCode": "01",
		"applications": []map[string]any{{"receivableId": r2, "amount": "5"}},
	})
	if other.Status != 422 || other.Str("type") != "urn:rdl:receivables:problem:customer-mismatch" {
		t.Errorf("otro cliente: %d %v", other.Status, other.Body)
	}
	usd := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": inv1.Customer.String(), "receivedOn": "2026-09-20", "amount": "10", "currency": "USD", "paymentMethodCode": "01",
		"applications": []map[string]any{{"receivableId": r2, "amount": "5"}},
	})
	if usd.Status != 422 || usd.Str("type") != "urn:rdl:receivables:problem:currency-mismatch" {
		t.Errorf("otra moneda: %d %v", usd.Status, usd.Body)
	}
	future := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": inv1.Customer.String(), "receivedOn": "2099-01-01", "amount": "10", "currency": "CRC", "paymentMethodCode": "01",
	})
	testkit.Expect(t, "fecha futura", http.StatusUnprocessableEntity, future)
}

// Dos aplicaciones simultáneas a la misma cuenta, cada una por el 60 % del saldo: exactamente una entra y el saldo
// nunca queda negativo (bloqueo en orden estable, ADR 0006).
func TestConcurrentApplicationsNeverOverdraw(t *testing.T) {
	a := env.A
	tok := a.OwnerToken(env)
	inv := env.IssueInvoice(t, a.Org, "100", "2026-10-01")
	rid := env.ReceivableOf(t, a, inv.ID).Str("receivableId")
	var pids []string
	for range 2 {
		p := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
			"customerId": inv.Customer.String(), "receivedOn": "2026-09-20", "amount": "60", "currency": "CRC", "paymentMethodCode": "01",
		})
		testkit.Expect(t, "pago", http.StatusCreated, p)
		pids = append(pids, p.Str("id"))
	}
	var wg sync.WaitGroup
	statuses := make([]int, 2)
	for i, pid := range pids {
		wg.Go(func() {
			r := env.Call(t, http.MethodPost, "/v1/payment-applications", tok, map[string]any{"paymentId": pid, "receivableId": rid, "amount": "60"})
			statuses[i] = r.Status
		})
	}
	wg.Wait()
	created := 0
	for _, s := range statuses {
		if s == http.StatusCreated {
			created++
		} else if s != http.StatusUnprocessableEntity {
			t.Errorf("status inesperado: %d", s)
		}
	}
	if created != 1 {
		t.Fatalf("se esperaba exactamente una aplicación, statuses %v", statuses)
	}
	if b := env.ReceivableOf(t, a, inv.ID); b.Str("balanceAmount") != "40" || b.Str("status") != "partially_paid" {
		t.Errorf("saldo final: %v", b.Body)
	}
}

// Notas de crédito y débito y anulación de factura por evento (R2, R3, R4), con reproceso sin duplicar.
func TestNotesAndCancellationEvents(t *testing.T) {
	a := env.A
	tok := a.OwnerToken(env)
	inv := env.IssueInvoice(t, a.Org, "100", "2026-10-01")
	rid := env.ReceivableOf(t, a, inv.ID).Str("receivableId")
	pay := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": inv.Customer.String(), "receivedOn": "2026-09-20", "amount": "100", "currency": "CRC", "paymentMethodCode": "01",
		"applications": []map[string]any{{"receivableId": rid, "amount": "100"}},
	})
	testkit.Expect(t, "pago total", http.StatusCreated, pay)

	// R3: nota de crédito de 30 sobre una cuenta pagada: revierte la aplicación y deja saldo 70.
	credit := testkit.Event(t, "credit-note-issued.v1.json", a.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": inv.ID.String(), "total": "30", "currency": "CRC",
	})
	env.MustProcess(t, credit, app.OutcomeProcessed)
	env.MustProcess(t, credit, app.OutcomeDuplicate)
	if b := env.ReceivableOf(t, a, inv.ID); b.Str("balanceAmount") != "70" || b.Str("status") != "partially_paid" {
		t.Fatalf("después de la nota de crédito: %v", b.Body)
	}
	p := env.Call(t, http.MethodGet, "/v1/payments/"+pay.Str("id"), tok, nil)
	if apps := p.Body["applications"].([]any); apps[0].(map[string]any)["reversalReason"] == nil {
		t.Errorf("la aplicación debía revertirse: %v", apps)
	}

	// R4: nota de débito de 20: suma y conserva el vencimiento de la factura.
	debit := testkit.Event(t, "debit-note-issued.v1.json", a.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": inv.ID.String(), "total": "20", "currency": "CRC",
		"dueDate": "2026-12-31",
	})
	env.MustProcess(t, debit, app.OutcomeProcessed)
	if b := env.ReceivableOf(t, a, inv.ID); b.Str("balanceAmount") != "90" || b.Str("dueOn") != "2026-10-01" {
		t.Fatalf("después de la nota de débito: %v", b.Body)
	}

	// Una nota en otra moneda va a dead letter sin tocar la cuenta.
	usd := testkit.Event(t, "credit-note-issued.v1.json", a.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": inv.ID.String(), "total": "1", "currency": "USD",
	})
	env.MustProcess(t, usd, app.OutcomeDeadLettered)

	// R2: la anulación registra la cancelación por el saldo; la cuenta queda cancelled y no admite aplicaciones.
	cancel := testkit.Event(t, "invoice-cancelled.v1.json", a.Org, map[string]any{
		"invoiceId": inv.ID.String(), "total": "100", "currency": "CRC",
	})
	env.MustProcess(t, cancel, app.OutcomeProcessed)
	env.MustProcess(t, cancel, app.OutcomeDuplicate)
	if b := env.ReceivableOf(t, a, inv.ID); b.Str("status") != "cancelled" || b.Str("balanceAmount") != "0" {
		t.Fatalf("después de anular: %v", b.Body)
	}
	late := testkit.Event(t, "debit-note-issued.v1.json", a.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": inv.ID.String(), "total": "5", "currency": "CRC",
	})
	env.MustProcess(t, late, app.OutcomeDeadLettered)

	// Una nota de una factura que no llegó: se reintenta y termina en dead letter.
	orphan := testkit.Event(t, "credit-note-issued.v1.json", a.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": uuid.NewString(), "total": "1", "currency": "CRC",
	})
	if res := env.MustProcess(t, orphan, app.OutcomeDeadLettered); res.Attempts < 2 {
		t.Errorf("la factura faltante debía reintentarse: %d intentos", res.Attempts)
	}
}

// Aging por tramos a una fecha dada, seguimiento y promesa de pago con su cierre.
func TestAgingAndCollection(t *testing.T) {
	a := env.A
	tok := a.OwnerToken(env)
	inv := env.IssueInvoice(t, a.Org, "250", "2026-06-30")
	rid := env.ReceivableOf(t, a, inv.ID).Str("receivableId")

	ag := env.Call(t, http.MethodGet, "/v1/receivables/aging?asOf=2026-07-15&currency=CRC", tok, nil)
	testkit.Expect(t, "aging", http.StatusOK, ag)
	found := false
	for _, it := range ag.List {
		m := it.(map[string]any)
		if m["bucket"] == "1_30" && m["currency"] == "CRC" && m["balance"] != "0" {
			found = true
		}
	}
	if !found || ag.Headers.Get("X-Aging-As-Of") != "2026-07-15" {
		t.Errorf("aging: %v", ag.List)
	}

	fu := env.Call(t, http.MethodPost, "/v1/receivables/"+rid+"/follow-ups", tok, map[string]any{
		"followupType": "call", "notes": "Promete pagar el viernes", "nextActionOn": "2099-01-02",
	})
	testkit.Expect(t, "seguimiento", http.StatusCreated, fu)
	list := env.Call(t, http.MethodGet, "/v1/receivables/"+rid+"/follow-ups", tok, nil)
	if len(list.List) != 1 {
		t.Errorf("seguimientos: %v", list.List)
	}

	pr := env.Call(t, http.MethodPost, "/v1/receivables/"+rid+"/promises", tok, map[string]any{
		"promisedAmount": "100", "promisedOn": "2099-01-02", "followupId": fu.Str("id"),
	})
	testkit.Expect(t, "promesa", http.StatusCreated, pr)
	testkit.Expect(t, "promesa mayor que el saldo", http.StatusUnprocessableEntity,
		env.Call(t, http.MethodPost, "/v1/receivables/"+rid+"/promises", tok, map[string]any{"promisedAmount": "250.00001", "promisedOn": "2099-01-02"}))
	closed := env.Call(t, http.MethodPost, "/v1/payment-promises/"+pr.Str("id")+"/status", tok, map[string]any{"status": "kept"})
	testkit.Expect(t, "cumplir promesa", http.StatusOK, closed)
	if closed.Str("status") != "kept" {
		t.Errorf("promesa: %v", closed.Body)
	}
	testkit.Expect(t, "cerrar dos veces", http.StatusConflict,
		env.Call(t, http.MethodPost, "/v1/payment-promises/"+pr.Str("id")+"/status", tok, map[string]any{"status": "broken"}))
}
