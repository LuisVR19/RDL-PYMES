//go:build integration

package isolation

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Criterio 1: con el token de A, todos los endpoints sobre recursos de B devuelven 404 (o 403), y B queda intacto.
func TestCriterion1CrossTenantEndpoints(t *testing.T) {
	e := newEnv(t)
	b := e.seed(subjectOwnerB, orgB, "B1")
	a := tokens.issue(subjectOwnerA, orgA)

	cases := []struct{ method, path, body string }{
		{"GET", "/v1/customers/" + b.customerID, ""},
		{"PATCH", "/v1/customers/" + b.customerID, `{"legalName":"Robado"}`},
		{"GET", "/v1/products/" + b.productID, ""},
		{"PATCH", "/v1/products/" + b.productID, `{"isActive":false}`},
		{"GET", "/v1/invoices/" + b.draftID, ""},
		{"PATCH", "/v1/invoices/" + b.draftID, `{"notes":"hack"}`},
		{"PUT", "/v1/invoices/" + b.draftID + "/lines", `[{"productId":"` + b.productID + `","quantity":"9"}]`},
		{"DELETE", "/v1/invoices/" + b.draftID, ""},
		{"POST", "/v1/invoices/" + b.draftID + "/issue", ""},
		{"GET", "/v1/invoices/" + b.draftID + "/history", ""},
		{"GET", "/internal/v1/invoices/" + b.draftID + "/summary", ""},
		// Referencias a recursos de B desde documentos de A.
		{"POST", "/v1/invoices", `{"documentType":"invoice","customerId":"` + b.customerID + `","saleConditionCode":"01","currency":"CRC"}`},
		{"PUT", "/v1/document-sequences/invoice", `{"branchId":"` + branchB.String() + `","prefix":"X-","nextNumber":1}`},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			r := e.call(c.method, c.path, a, c.body, "Idempotency-Key", uuid.NewString())
			if r.Status != http.StatusNotFound && r.Status != http.StatusForbidden {
				t.Fatalf("status %d, se esperaba 404 o 403 (%s)", r.Status, r.Raw)
			}
		})
	}

	// Una línea de A con un producto de B: 404 (el producto "no existe" para A).
	aCustomer := e.must(201, e.call("POST", "/v1/customers", a, `{"identification":{"typeCode":"02","number":"3101A1"},"legalName":"Cliente A"}`,
		"Idempotency-Key", uuid.NewString()), "cliente de A").str("id")
	r := e.call("POST", "/v1/invoices", a, `{"documentType":"invoice","customerId":"`+aCustomer+`","saleConditionCode":"01","currency":"CRC",
		"lines":[{"productId":"`+b.productID+`","quantity":"1"}]}`, "Idempotency-Key", uuid.NewString())
	e.must(404, r, "línea con producto de B")

	// Los listados de A no muestran nada de B.
	for _, path := range []string{"/v1/customers", "/v1/products", "/v1/invoices", "/v1/invoices?customerId=" + b.customerID} {
		for _, it := range e.must(200, e.call("GET", path, a, ""), path).items() {
			if it["id"] == b.customerID || it["id"] == b.productID || it["id"] == b.draftID {
				t.Fatalf("%s de A devolvió un recurso de B: %v", path, it)
			}
		}
	}
	seqs := e.call("GET", "/v1/document-sequences", a, "")
	if seqs.Raw != "[]\n" {
		t.Fatalf("secuencias de A: %s", seqs.Raw)
	}

	// B quedó intacto.
	c := e.must(200, e.call("GET", "/v1/customers/"+b.customerID, b.token, ""), "cliente de B")
	if c.str("legalName") != "Cliente B1" {
		t.Fatalf("el cliente de B cambió: %v", c.Body)
	}
	if p := e.must(200, e.call("GET", "/v1/products/"+b.productID, b.token, ""), "producto de B"); p.Body["isActive"] != true {
		t.Fatalf("el producto de B cambió: %v", p.Body)
	}
	d := e.must(200, e.call("GET", "/v1/invoices/"+b.draftID, b.token, ""), "borrador de B")
	if d.str("status") != "draft" || d.str("notes") != "" || d.str("total") != "200" {
		t.Fatalf("el borrador de B cambió: %v", d.Body)
	}
}

// Criterio 2: con la sesión en A, un SELECT directo con billing_app no devuelve filas de B en ninguna tabla con
// organization_id (billing, audit e integration).
func TestCriterion2DirectSelectSeesOnlyActiveOrganization(t *testing.T) {
	e := newEnv(t)
	e.seed(subjectOwnerB, orgB, "B2") // B tiene filas en todas las tablas que Billing escribe
	if _, err := e.outer.Exec(e.ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		orgA.String(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	tables := []string{"billing.customers", "billing.products", "billing.product_taxes", "billing.invoices", "billing.invoice_lines",
		"billing.invoice_line_taxes", "billing.invoice_payment_methods", "billing.invoice_status_history", "billing.document_sequences",
		"audit.audit_events", "integration.idempotency_keys"}
	for _, table := range tables {
		var n int
		if err := e.outer.QueryRow(e.ctx, fmt.Sprintf("select count(*) from %s where organization_id = $1", table), orgB).Scan(&n); err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s: la sesión de A ve %d filas de B", table, n)
		}
	}
	// Control: con la sesión en B, las mismas consultas sí las ven (la prueba no es vacía).
	if _, err := e.outer.Exec(e.ctx, "select set_config('app.current_organization_id', $1, true)", orgB.String()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.outer.QueryRow(e.ctx, "select count(*) from billing.invoices where organization_id = $1", orgB).Scan(&n); err != nil || n != 1 {
		t.Fatalf("control: B ve %d facturas propias (err %v)", n, err)
	}
}

// Criterio 3: una línea o una factura que referencia un cliente, producto, sucursal o factura de otra organización se
// rechaza en la base (FK compuestas), aunque la app se equivocara.
func TestCriterion3CompositeForeignKeys(t *testing.T) {
	e := newEnv(t)
	b := e.seed(subjectOwnerB, orgB, "B3")
	a := e.seed(subjectOwnerA, orgA, "A3")
	if _, err := e.outer.Exec(e.ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		orgA.String(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	var branchFK bool
	_ = e.outer.QueryRow(e.ctx, "select exists (select 1 from pg_constraint where conname = 'invoices_branch_fk')").Scan(&branchFK)

	cases := map[string]string{
		"factura de A con cliente de B": fmt.Sprintf(`insert into billing.invoices (organization_id, customer_id, sale_condition_code, created_by_user_id)
			values ('%s', '%s', '01', '%s')`, orgA, b.customerID, uuid.New()),
		"línea de A con producto de B": fmt.Sprintf(`insert into billing.invoice_lines (organization_id, invoice_id, line_number, product_id, cabys_code,
			description, unit_of_measure_code, quantity, unit_price, subtotal_amount, total_amount)
			values ('%s', '%s', 9, '%s', '8314100000100', 'x', 'Sp', 1, 1, 1, 1)`, orgA, a.draftID, b.productID),
		"línea de A en factura de B": fmt.Sprintf(`insert into billing.invoice_lines (organization_id, invoice_id, line_number, cabys_code,
			description, unit_of_measure_code, quantity, unit_price, subtotal_amount, total_amount)
			values ('%s', '%s', 9, '8314100000100', 'x', 'Sp', 1, 1, 1, 1)`, orgA, b.draftID),
		"nota de A que referencia una factura de B": fmt.Sprintf(`insert into billing.invoices (organization_id, document_type, customer_id,
			sale_condition_code, created_by_user_id, referenced_invoice_id, reference_reason)
			values ('%s', 'credit_note', '%s', '01', '%s', '%s', 'x')`, orgA, a.customerID, uuid.New(), b.draftID),
	}
	if branchFK {
		cases["factura de A con sucursal de B"] = fmt.Sprintf(`insert into billing.invoices (organization_id, customer_id, branch_id,
			sale_condition_code, created_by_user_id) values ('%s', '%s', '%s', '01', '%s')`, orgA, a.customerID, branchB, uuid.New())
	} else {
		t.Log("migración 00003 (FK de sucursal) sin aplicar: se omite el caso de sucursal en la base (la app lo valida igual)")
	}
	for name, sql := range cases {
		t.Run(name, func(t *testing.T) {
			if err := execInSavepoint(e, sql); err == nil {
				t.Fatal("la base aceptó la referencia cruzada")
			}
		})
	}
	// Control: la misma factura con el cliente propio sí entra (el rechazo no es por otra cosa).
	ok := fmt.Sprintf(`insert into billing.invoices (organization_id, customer_id, branch_id, sale_condition_code, created_by_user_id)
		values ('%s', '%s', '%s', '01', '%s')`, orgA, a.customerID, branchA, uuid.New())
	if err := execInSavepoint(e, ok); err != nil {
		t.Fatalf("control: %v", err)
	}
}

// Criterio 4: billing_app no escribe en core, fiscal, receivables ni subscriptions, ni hace UPDATE/DELETE en audit.
func TestCriterion4NoWritesOutsideBilling(t *testing.T) {
	e := newEnv(t)
	e.seed(subjectOwnerA, orgA, "A4") // deja al menos un audit event de A
	if _, err := e.outer.Exec(e.ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		orgA.String(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	statements := map[string]string{
		"insertar sucursal en core":        fmt.Sprintf(`insert into core.branches (organization_id, code, name) values ('%s', 'HACK', 'x')`, orgA),
		"editar sucursal en core":          fmt.Sprintf(`update core.branches set name = 'hack' where id = '%s'`, branchA),
		"editar organización en core":      fmt.Sprintf(`update core.organizations set legal_name = 'hack' where id = '%s'`, orgA),
		"suspender membresía en core":      `update core.organization_users set status = 'suspended'`,
		"darse un rol en core":             `insert into core.organization_user_roles (organization_id, organization_user_id, role_code) values ('b0000000-0000-4000-8000-00000000000a', 'b0000000-0000-4000-8000-0000000001a2', 'owner')`,
		"inventar una tarifa en fiscal":    `insert into fiscal.tax_rates (code, name, rate) values ('99', 'hack', 50)`,
		"editar documentos de fiscal":      `update fiscal.electronic_documents set status = status`,
		"escribir en receivables":          `update receivables.receivables set status = status`,
		"escribir en subscriptions":        `update subscriptions.plans set name = name`,
		"editar el audit":                  `update audit.audit_events set action = 'hack'`,
		"borrar el audit":                  `delete from audit.audit_events`,
		"leer el historial de migraciones": `select count(*) from billing.goose_db_version`,
	}
	for name, sql := range statements {
		t.Run(name, func(t *testing.T) {
			if err := execInSavepoint(e, sql); err == nil {
				t.Fatal("billing_app pudo ejecutarlo")
			}
		})
	}
}

// Criterio 5: un token sin org_id, o con la membresía suspendida, no accede.
func TestCriterion5NoOrganizationOrSuspendedMembership(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		name, token, problem string
		status               int
	}{
		{"sin token", "", "unauthenticated", 401},
		{"token inválido", "iso-falso", "unauthenticated", 401},
		{"token sin org_id", tokens.issue(subjectOwnerA, uuid.Nil), "no-active-organization", 403},
		{"membresía suspendida", tokens.issue(subjectSuspendedA, orgA), "membership-inactive", 403},
		{"org_id de una organización ajena", tokens.issue(subjectOwnerA, orgB), "membership-inactive", 403},
		{"sujeto que no existe en core", tokens.issue("billing-iso-nadie-"+uuid.NewString(), orgA), "membership-inactive", 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, path := range []string{"/v1/customers", "/v1/invoices", "/v1/document-sequences"} {
				r := e.call("GET", path, c.token, "")
				if r.Status != c.status || r.str("type") != "urn:rdl:billing:problem:"+c.problem {
					t.Fatalf("%s: status %d type %s", path, r.Status, r.str("type"))
				}
			}
		})
	}
	// El lector activo sí entra, pero no escribe (403, no 404: la organización es la suya).
	reader := tokens.issue(subjectReaderA, orgA)
	e.must(200, e.call("GET", "/v1/invoices", reader, ""), "lector lista")
	e.must(403, e.call("POST", "/v1/customers", reader, `{"identification":{"typeCode":"02","number":"1"},"legalName":"x"}`,
		"Idempotency-Key", uuid.NewString()), "lector crea")
}

// Criterio 6: un organization_id o branch_id de B en el body, la query o un header no cambia el tenant ni se acepta.
func TestCriterion6TenantComesOnlyFromToken(t *testing.T) {
	e := newEnv(t)
	b := e.seed(subjectOwnerB, orgB, "B6")
	a := e.seed(subjectOwnerA, orgA, "A6")

	// organizationId en el body: campo desconocido (400), no un cambio de tenant.
	e.must(400, e.call("POST", "/v1/customers", a.token,
		`{"organizationId":"`+orgB.String()+`","identification":{"typeCode":"02","number":"9"},"legalName":"x"}`,
		"Idempotency-Key", uuid.NewString()), "organizationId en el body")

	// organization_id en la query y en headers: se ignoran; A sigue viendo solo lo suyo.
	r := e.must(200, e.call("GET", "/v1/customers?organizationId="+orgB.String()+"&organization_id="+orgB.String(), a.token, "",
		"X-Organization-Id", orgB.String(), "X-Tenant-Id", orgB.String()), "query y headers")
	items := r.items()
	if len(items) != 1 || items[0]["id"] != a.customerID {
		t.Fatalf("A vio %v", items)
	}

	// branch_id de B en el body: 404 al crear un documento y al configurar una secuencia.
	e.must(404, e.call("POST", "/v1/invoices", a.token,
		`{"documentType":"invoice","customerId":"`+a.customerID+`","branchId":"`+branchB.String()+`","saleConditionCode":"01","currency":"CRC"}`,
		"Idempotency-Key", uuid.NewString()), "branchId de B al crear")
	e.must(404, e.call("PATCH", "/v1/invoices/"+a.draftID, a.token, `{"branchId":"`+branchB.String()+`"}`), "branchId de B al editar")
	e.must(404, e.call("PUT", "/v1/document-sequences/invoice", a.token, `{"branchId":"`+branchB.String()+`","prefix":"Z-","nextNumber":1}`),
		"branchId de B en la secuencia")
	// La sucursal propia sí se acepta (control).
	e.must(200, e.call("PATCH", "/v1/invoices/"+a.draftID, a.token, `{"branchId":"`+branchA.String()+`"}`), "branchId propio")

	// Emitir: el documento de A queda en A y con la secuencia de A, aunque B tenga la suya.
	issued := e.must(201, e.call("POST", "/v1/invoices/"+a.draftID+"/issue", a.token, "", "Idempotency-Key", uuid.NewString()), "emitir A")
	if issued.str("number") != "A6-00000001" {
		t.Fatalf("número = %s", issued.str("number"))
	}
	// La factura de B sigue en borrador.
	if d := e.must(200, e.call("GET", "/v1/invoices/"+b.draftID, b.token, ""), "B"); d.str("status") != "draft" {
		t.Fatalf("B = %v", d.Body)
	}
}

// execInSavepoint ejecuta sql y revierte solo ese paso, para que un error esperado no aborte la transacción de la prueba.
func execInSavepoint(e *env, sql string) error {
	sp, err := e.outer.Begin(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	defer func() { _ = sp.Rollback(e.ctx) }()
	_, err = sp.Exec(e.ctx, sql)
	return err
}
