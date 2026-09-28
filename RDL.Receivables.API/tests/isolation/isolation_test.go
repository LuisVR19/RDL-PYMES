//go:build integration

// Package isolation es la suite de aislamiento entre tenants (make test-isolation): los 6 criterios del prompt P6
// más el bypass del guard de ADR 0001 §4.2, contra la base de dev con el login receivables_api.
// Si un caso falla por falta de una política en la base, no se debilita el test: se reporta y se propone la migración.
package isolation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/tests/testkit"
)

var env *testkit.Env

func TestMain(m *testing.M) {
	e, reason, err := testkit.Setup(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "aislamiento:", err)
		os.Exit(1)
	}
	if reason != "" {
		fmt.Fprintln(os.Stderr, "AVISO: suite de aislamiento saltada:", reason)
		os.Exit(0)
	}
	env = e
	code := m.Run()
	env.Pool.Close()
	os.Exit(code)
}

// bResources crea en B una cuenta, un pago aplicado, un seguimiento y una promesa.
type bResources struct {
	invoice     testkit.Invoice
	receivable  string
	payment     string
	application string
	followUp    string
	promise     string
}

func seedB(t *testing.T) bResources {
	t.Helper()
	b := env.B
	tok := b.OwnerToken(env)
	res := bResources{invoice: env.IssueInvoice(t, b.Org, "500", "2099-01-01")}
	res.receivable = env.ReceivableOf(t, b, res.invoice.ID).Str("receivableId")
	pay := env.Call(t, http.MethodPost, "/v1/payments", tok, map[string]any{
		"customerId": res.invoice.Customer.String(), "receivedOn": "2026-09-20", "amount": "100", "currency": "CRC",
		"paymentMethodCode": "01", "applications": []map[string]any{{"receivableId": res.receivable, "amount": "100"}},
	})
	testkit.Expect(t, "pago de B", http.StatusCreated, pay)
	res.payment = pay.Str("id")
	res.application = pay.Body["applications"].([]any)[0].(map[string]any)["id"].(string)
	fu := env.Call(t, http.MethodPost, "/v1/receivables/"+res.receivable+"/follow-ups", tok, map[string]any{"followupType": "note", "notes": "B"})
	testkit.Expect(t, "seguimiento de B", http.StatusCreated, fu)
	res.followUp = fu.Str("id")
	pr := env.Call(t, http.MethodPost, "/v1/receivables/"+res.receivable+"/promises", tok, map[string]any{"promisedAmount": "10", "promisedOn": "2099-01-01"})
	testkit.Expect(t, "promesa de B", http.StatusCreated, pr)
	res.promise = pr.Str("id")
	return res
}

// 1) Con el token de A, ningún endpoint alcanza recursos de B (404), un token de A con el org_id de B no entra
// (403) y B queda intacto.
func TestEndpointsDoNotReachOtherTenant(t *testing.T) {
	a, b := env.A, env.B
	br := seedB(t)
	tokA := a.OwnerToken(env)
	// Un pago de A del mismo cliente, para intentar aplicarlo a la cuenta de B.
	payA := env.Call(t, http.MethodPost, "/v1/payments", tokA, map[string]any{
		"customerId": br.invoice.Customer.String(), "receivedOn": "2026-09-20", "amount": "50", "currency": "CRC", "paymentMethodCode": "01",
	})
	testkit.Expect(t, "pago de A", http.StatusCreated, payA)

	cases := []struct {
		name, method, path string
		body               any
	}{
		{"detalle de la cuenta", http.MethodGet, "/v1/receivables/" + br.receivable, nil},
		{"saldo por factura (BFF)", http.MethodGet, "/internal/v1/receivables/by-invoice/" + br.invoice.ID.String(), nil},
		{"detalle del pago", http.MethodGet, "/v1/payments/" + br.payment, nil},
		{"anular el pago", http.MethodPost, "/v1/payments/" + br.payment + "/void", map[string]any{"reason": "hack"}},
		{"revertir la aplicación", http.MethodPost, "/v1/payment-applications/" + br.application + "/reverse", map[string]any{"reason": "hack"}},
		{"aplicar un pago de A a la cuenta de B", http.MethodPost, "/v1/payment-applications", map[string]any{"paymentId": payA.Str("id"), "receivableId": br.receivable, "amount": "1"}},
		{"aplicar el pago de B", http.MethodPost, "/v1/payment-applications", map[string]any{"paymentId": br.payment, "receivableId": br.receivable, "amount": "1"}},
		{"pago de A aplicado a la cuenta de B al registrarlo", http.MethodPost, "/v1/payments", map[string]any{
			"customerId": br.invoice.Customer.String(), "receivedOn": "2026-09-20", "amount": "5", "currency": "CRC", "paymentMethodCode": "01",
			"applications": []map[string]any{{"receivableId": br.receivable, "amount": "5"}},
		}},
		{"seguimientos de B", http.MethodGet, "/v1/receivables/" + br.receivable + "/follow-ups", nil},
		{"crear seguimiento en B", http.MethodPost, "/v1/receivables/" + br.receivable + "/follow-ups", map[string]any{"followupType": "call", "notes": "hack"}},
		{"promesas de B", http.MethodGet, "/v1/receivables/" + br.receivable + "/promises", nil},
		{"crear promesa en B", http.MethodPost, "/v1/receivables/" + br.receivable + "/promises", map[string]any{"promisedAmount": "1", "promisedOn": "2099-01-01"}},
		{"cerrar la promesa de B", http.MethodPost, "/v1/payment-promises/" + br.promise + "/status", map[string]any{"status": "cancelled"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testkit.Expect(t, c.name, http.StatusNotFound, env.Call(t, c.method, c.path, tokA, c.body))
		})
	}

	// Token de A con el org_id de B: A no es miembro de B.
	tokAinB := env.Tokens.Issue(a.Owner, b.Org)
	for _, path := range []string{"/v1/receivables", "/v1/payments", "/v1/receivables/" + br.receivable, "/internal/v1/receivables/by-invoice/" + br.invoice.ID.String()} {
		r := env.Call(t, http.MethodGet, path, tokAinB, nil)
		if r.Status != http.StatusForbidden || r.Str("type") != "urn:rdl:receivables:problem:membership-inactive" {
			t.Errorf("A con org_id de B en %s: %d %s", path, r.Status, r.Str("type"))
		}
	}

	// Los listados de A no traen nada de B.
	for _, path := range []string{"/v1/receivables?limit=100", "/v1/payments?limit=100", "/v1/receivables?limit=100&customerId=" + br.invoice.Customer.String()} {
		r := env.Call(t, http.MethodGet, path, tokA, nil)
		testkit.Expect(t, path, http.StatusOK, r)
		for _, it := range r.Body["items"].([]any) {
			id := it.(map[string]any)["id"]
			if id == br.receivable || id == br.payment {
				t.Errorf("%s de A expone %s de B", path, id)
			}
		}
	}

	// B no cambió: la cuenta sigue con su aplicación vigente y la promesa pendiente.
	tokB := b.OwnerToken(env)
	acc := env.ReceivableOf(t, b, br.invoice.ID)
	if acc.Str("balanceAmount") != "400" || acc.Str("status") != "partially_paid" {
		t.Errorf("la cuenta de B cambió: %v", acc.Body)
	}
	p := env.Call(t, http.MethodGet, "/v1/payments/"+br.payment, tokB, nil)
	if p.Str("status") != "posted" {
		t.Errorf("el pago de B cambió: %v", p.Body)
	}
	pr := env.Call(t, http.MethodGet, "/v1/receivables/"+br.receivable+"/promises", tokB, nil)
	if len(pr.List) != 1 || pr.List[0].(map[string]any)["status"] != "pending" {
		t.Errorf("la promesa de B cambió: %v", pr.List)
	}
	fu := env.Call(t, http.MethodGet, "/v1/receivables/"+br.receivable+"/follow-ups", tokB, nil)
	if len(fu.List) != 1 {
		t.Errorf("A agregó seguimientos a B: %v", fu.List)
	}
}

// 2) Con la sesión de A, un SELECT directo como receivables_app no devuelve filas de B en ninguna tabla.
func TestDirectSelectDoesNotSeeOtherTenant(t *testing.T) {
	seedB(t)
	env.IssueInvoice(t, env.A.Org, "1", "2099-01-01") // control: A ve lo suyo
	tables := []string{"receivables", "receivable_adjustments", "payments", "payment_applications",
		"collection_followups", "payment_promises", "receivable_aging"}
	ctx := context.Background()
	err := testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
		for _, tbl := range tables {
			var foreign, total int
			q := "select count(*) filter (where organization_id = $1), count(*) from receivables." + tbl
			if err := tx.QueryRow(ctx, q, env.B.Org).Scan(&foreign, &total); err != nil {
				return fmt.Errorf("%s: %w", tbl, err)
			}
			if foreign != 0 {
				t.Errorf("%s: la sesión de A ve %d filas de B", tbl, foreign)
			}
		}
		var own int
		if err := tx.QueryRow(ctx, "select count(*) from receivables.receivables where organization_id = $1", env.A.Org).Scan(&own); err != nil {
			return err
		}
		if own == 0 {
			t.Error("control: la sesión de A debería ver sus propias cuentas")
		}
		// Integración: el inbox y la dead letter son por servicio; los audit events solo de la organización.
		var auditB int
		if err := tx.QueryRow(ctx, "select count(*) from audit.audit_events where organization_id = $1", env.B.Org).Scan(&auditB); err != nil {
			return err
		}
		if auditB != 0 {
			t.Errorf("la sesión de A ve %d audit events de B", auditB)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Sin sesión no se ve nada.
	err = testkit.WithSession(ctx, env.Pool, uuid.Nil, uuid.Nil, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, "select count(*) from receivables.receivables").Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("sin sesión se ven %d cuentas", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 3) Una aplicación de un pago de A a una cuenta de B se rechaza en la base (FK compuesta y RLS), aunque se escriba
// directo sin pasar por la API.
func TestCrossTenantApplicationRejectedByDatabase(t *testing.T) {
	br := seedB(t)
	tokA := env.A.OwnerToken(env)
	payA := env.Call(t, http.MethodPost, "/v1/payments", tokA, map[string]any{
		"customerId": br.invoice.Customer.String(), "receivedOn": "2026-09-20", "amount": "50", "currency": "CRC", "paymentMethodCode": "01",
	})
	testkit.Expect(t, "pago de A", http.StatusCreated, payA)
	ctx := context.Background()
	attempts := map[string]uuid.UUID{"organización de A": env.A.Org, "organización de B": env.B.Org}
	for name, org := range attempts {
		err := testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `insert into receivables.payment_applications (organization_id, payment_id, receivable_id, amount)
				values ($1, $2, $3, 1)`, org, uuid.MustParse(payA.Str("id")), uuid.MustParse(br.receivable))
			return err
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "23503" && pgErr.Code != "42501") {
			t.Errorf("aplicación cruzada con %s: se esperaba FK (23503) o RLS (42501), se obtuvo %v", name, err)
		}
	}
}

// 4) receivables_app no escribe en core, billing, fiscal ni subscriptions; no actualiza ni borra audit; no lee el
// outbox de otro servicio. Y el guard de saldos no se salta fijando el GUC (ADR 0001 §4.2, migración 00003).
func TestPrivileges(t *testing.T) {
	ctx := context.Background()
	err := testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select table_schema || '.' || table_name, p.priv
			  from information_schema.tables t
			  cross join (values ('INSERT'), ('UPDATE'), ('DELETE'), ('TRUNCATE')) p(priv)
			 where t.table_schema in ('core', 'billing', 'fiscal', 'subscriptions') and t.table_type = 'BASE TABLE'
			   and has_table_privilege(current_user, quote_ident(t.table_schema) || '.' || quote_ident(t.table_name), p.priv)
			union all
			select 'audit.audit_events', p.priv from (values ('UPDATE'), ('DELETE'), ('TRUNCATE')) p(priv)
			 where has_table_privilege(current_user, 'audit.audit_events', p.priv)`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var tbl, priv string
			if err := rows.Scan(&tbl, &priv); err != nil {
				return err
			}
			t.Errorf("receivables_app tiene %s sobre %s", priv, tbl)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		var foreign int
		if err := tx.QueryRow(ctx, "select count(*) from integration.outbox_messages where source_service <> 'receivables'").Scan(&foreign); err != nil {
			return err
		}
		if foreign != 0 {
			t.Errorf("receivables_app ve %d mensajes del outbox de otros servicios", foreign)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Escritura real en core rechazada (no solo el catálogo de privilegios).
	err = testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "update core.organizations set status = status where id = $1", env.A.Org)
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Errorf("UPDATE en core: se esperaba permiso denegado (42501), se obtuvo %v", err)
	}

	inv := env.IssueInvoice(t, env.A.Org, "10", "2099-01-01")
	rid := uuid.MustParse(env.ReceivableOf(t, env.A, inv.ID).Str("receivableId"))
	for name, stmt := range map[string]string{
		"saldo":  "update receivables.receivables set balance_amount = 0 where organization_id = $1 and id = $2",
		"estado": "update receivables.receivables set status = 'paid' where organization_id = $1 and id = $2",
	} {
		err := testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "select set_config('receivables.recalculating', 'on', true)"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, stmt, env.A.Org, rid)
			return err
		})
		if !errors.As(err, &pgErr) || pgErr.Code != "23001" {
			t.Errorf("bypass del guard (%s): se esperaba restrict_violation, se obtuvo %v", name, err)
		}
	}
}

// 5) Sin token, con un token sin org_id o con la membresía suspendida no se accede.
func TestAuthenticationAndMembership(t *testing.T) {
	cases := []struct {
		name, token, typ string
		status           int
	}{
		{"sin token", "", "unauthenticated", 401},
		{"token desconocido", "no-es-un-token", "unauthenticated", 401},
		{"token sin org_id", env.Tokens.Issue(env.A.Owner, uuid.Nil), "no-active-organization", 403},
	}
	if s := env.A.Suspended; s != nil {
		cases = append(cases, struct {
			name, token, typ string
			status           int
		}{"membresía suspendida", env.Tokens.Issue(*s, env.A.Org), "membership-inactive", 403})
	} else {
		t.Log("la organización A no tiene un miembro suspendido: ese caso no se prueba")
	}
	for _, c := range cases {
		for _, path := range []string{"/v1/receivables", "/internal/v1/receivables/by-invoice/" + uuid.NewString()} {
			r := env.Call(t, http.MethodGet, path, c.token, nil)
			if r.Status != c.status || r.Str("type") != "urn:rdl:receivables:problem:"+c.typ {
				t.Errorf("%s en %s: %d %s", c.name, path, r.Status, r.Str("type"))
			}
		}
	}
}

// 6) Un organization_id en la query, un header o el body no cambia el tenant; un evento con el organizationId de B
// solo afecta a B.
func TestTenantComesOnlyFromTokenAndEvent(t *testing.T) {
	br := seedB(t)
	tokA := env.A.OwnerToken(env)
	for name, r := range map[string]testkit.Response{
		"query":  env.Call(t, http.MethodGet, "/v1/receivables?limit=100&organization_id="+env.B.Org.String(), tokA, nil),
		"header": env.Call(t, http.MethodGet, "/v1/receivables?limit=100", tokA, nil, "X-Organization-Id", env.B.Org.String()),
	} {
		testkit.Expect(t, name, http.StatusOK, r)
		for _, it := range r.Body["items"].([]any) {
			if it.(map[string]any)["id"] == br.receivable {
				t.Errorf("organization_id en %s expuso la cuenta de B", name)
			}
		}
	}
	body := env.Call(t, http.MethodPost, "/v1/payments", tokA, map[string]any{
		"organizationId": env.B.Org.String(), "customerId": uuid.NewString(), "receivedOn": "2026-09-20", "amount": "1",
		"currency": "CRC", "paymentMethodCode": "01",
	})
	if body.Status != http.StatusBadRequest {
		t.Errorf("organizationId en el body: %d (se rechaza como campo desconocido)", body.Status)
	}

	// La misma factura (mismo invoiceId) emitida en A y en B: son dos cuentas, cada una en su organización.
	invA := env.IssueInvoice(t, env.A.Org, "77", "2099-01-01")
	evB := testkit.Event(t, "invoice-issued.v1.json", env.B.Org, map[string]any{
		"invoiceId": invA.ID.String(), "invoiceNumber": "FAC-B", "total": "5", "issueDate": "2026-01-01", "dueDate": "2099-01-01",
	})
	env.MustProcess(t, evB, app.OutcomeProcessed)
	a := env.ReceivableOf(t, env.A, invA.ID)
	b := env.ReceivableOf(t, env.B, invA.ID)
	if a.Str("balanceAmount") != "77" || b.Str("balanceAmount") != "5" || a.Str("receivableId") == b.Str("receivableId") {
		t.Errorf("A: %v, B: %v", a.Body, b.Body)
	}
	// Una nota de crédito con el organizationId de B no toca la cuenta de A.
	credit := testkit.Event(t, "credit-note-issued.v1.json", env.B.Org, map[string]any{
		"documentId": uuid.NewString(), "referencedInvoiceId": invA.ID.String(), "total": "5", "currency": "CRC",
	})
	env.MustProcess(t, credit, app.OutcomeProcessed)
	if a = env.ReceivableOf(t, env.A, invA.ID); a.Str("balanceAmount") != "77" {
		t.Errorf("un evento de B cambió la cuenta de A: %v", a.Body)
	}
	if b = env.ReceivableOf(t, env.B, invA.ID); b.Str("status") != "paid" {
		t.Errorf("la nota de B no se aplicó a B: %v", b.Body)
	}
}
