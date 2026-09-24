//go:build integration

package isolation

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// 1) Con el token de A, ningún endpoint alcanza recursos de B: 404 si el recurso "no existe" para A,
// 403 si el token apunta a una organización donde A no es miembro. Y B queda intacto.
func TestEndpointsDoNotReachOtherTenant(t *testing.T) {
	requireDB(t)
	a, b := newTenant(t, "A"), newTenant(t, "B")
	tokA := a.ownerToken()

	cases := []struct {
		name         string
		method, path string
		token        string
		body         any
		want         int
	}{
		{"GET sucursal de B", http.MethodGet, "/v1/organizations/current/branches/" + b.Branch.String(), tokA, nil, 404},
		{"PATCH sucursal de B", http.MethodPatch, "/v1/organizations/current/branches/" + b.Branch.String(), tokA, map[string]any{"name": "hack"}, 404},
		{"PATCH rol del owner de B", http.MethodPatch, "/v1/organizations/current/users/" + b.Owner.ID.String(), tokA, map[string]any{"role": "read_only"}, 404},
		{"suspender miembro de B", http.MethodPatch, "/v1/organizations/current/users/" + b.Member.ID.String(), tokA, map[string]any{"status": "suspended"}, 404},
		{"revocar invitación de B", http.MethodDelete, "/v1/organizations/current/invitations/" + b.Invitation.String(), tokA, nil, 404},
		{"activar la organización B", http.MethodPut, "/v1/me/active-organization", tokA, map[string]any{"organizationId": b.Org.String()}, 404},
		{"aceptar invitación de B dirigida a otro email", http.MethodPost, "/v1/invitations/" + b.InvToken + "/accept", tokA, nil, 404},

		{"token de A con org_id de B: GET current", http.MethodGet, "/v1/organizations/current", tokens.issue(a.Owner, b.Org), nil, 403},
		{"token de A con org_id de B: PATCH current", http.MethodPatch, "/v1/organizations/current", tokens.issue(a.Owner, b.Org), map[string]any{"tradeName": "hack"}, 403},
		{"token de A con org_id de B: miembros", http.MethodGet, "/v1/organizations/current/users", tokens.issue(a.Owner, b.Org), nil, 403},
		{"token de A con org_id de B: sucursales", http.MethodGet, "/v1/organizations/current/branches", tokens.issue(a.Owner, b.Org), nil, 403},
		{"token de A con org_id de B: invitaciones", http.MethodGet, "/v1/organizations/current/invitations", tokens.issue(a.Owner, b.Org), nil, 403},
		{"token de A con org_id de B: GET sucursal", http.MethodGet, "/v1/organizations/current/branches/" + b.Branch.String(), tokens.issue(a.Owner, b.Org), nil, 403},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var headers []string
			if c.method == http.MethodPost {
				headers = []string{"Idempotency-Key", "iso-" + uuid.NewString()}
			}
			expectStatus(t, c.name, c.want, call(t, c.method, c.path, c.token, c.body, headers...))
		})
	}

	// Los listados de A no contienen nada de B.
	for _, path := range []string{"/v1/organizations/current/branches", "/v1/organizations/current/users", "/v1/organizations/current/invitations"} {
		r := call(t, http.MethodGet, path+"?limit=100", tokA, nil)
		expectStatus(t, "listado "+path, http.StatusOK, r)
		for _, it := range r.items() {
			for _, id := range []uuid.UUID{b.Branch, b.Invitation, b.Owner.ID, b.Member.ID} {
				if it["id"] == id.String() || it["userId"] == id.String() {
					t.Errorf("%s de A expone %s de B", path, id)
				}
			}
		}
	}
	r := call(t, http.MethodGet, "/v1/me/memberships", tokA, nil)
	for _, it := range r.items() {
		if it["organizationId"] == b.Org.String() {
			t.Error("las membresías de A incluyen la organización B")
		}
	}

	// B no cambió.
	tokB := b.ownerToken()
	r = call(t, http.MethodGet, "/v1/organizations/current/branches/"+b.Branch.String(), tokB, nil)
	if r.str("name") != "Sucursal B" {
		t.Errorf("la sucursal de B cambió: %v", r.Body)
	}
	r = call(t, http.MethodGet, "/v1/organizations/current/users?limit=100", tokB, nil)
	for _, it := range r.items() {
		if it["userId"] == b.Member.ID.String() && it["status"] != "active" {
			t.Errorf("el miembro de B cambió de estado: %v", it)
		}
		if it["userId"] == b.Owner.ID.String() && len(it["roles"].([]any)) == 1 && it["roles"].([]any)[0] != "owner" {
			t.Errorf("el owner de B cambió de rol: %v", it)
		}
	}
	r = call(t, http.MethodGet, "/v1/organizations/current/invitations?status=pending", tokB, nil)
	found := false
	for _, it := range r.items() {
		found = found || it["id"] == b.Invitation.String()
	}
	if !found {
		t.Error("la invitación pendiente de B ya no está pendiente")
	}
}

// 2) SELECT directo como platform_app con la sesión de A: ninguna tabla con organization_id devuelve filas de B.
func TestDirectSelectDoesNotSeeOtherTenant(t *testing.T) {
	requireDB(t)
	a, b := newTenant(t, "A"), newTenant(t, "B")
	ctx := context.Background()

	withSession(t, a.Org, a.Owner.ID, func(tx pgx.Tx) {
		tables := tenantTables(t, tx)
		t.Logf("tablas con organization_id revisadas: %v", tables)
		if len(tables) < 5 {
			t.Fatalf("se esperaban al menos 5 tablas con organization_id visibles, hay %d: %v", len(tables), tables)
		}
		for _, tbl := range tables {
			var n int
			if err := tx.QueryRow(ctx, "select count(*) from "+tbl+" where organization_id = $1", b.Org).Scan(&n); err != nil {
				t.Fatalf("%s: %v", tbl, err)
			}
			if n != 0 {
				t.Errorf("%s: la sesión de A ve %d filas de B", tbl, n)
			}
		}

		var orgs, users, ownUsers int
		must(t, tx.QueryRow(ctx, "select count(*) from core.organizations where id = $1", b.Org).Scan(&orgs))
		must(t, tx.QueryRow(ctx, "select count(*) from core.users where id in ($1, $2)", b.Owner.ID, b.Member.ID).Scan(&users))
		must(t, tx.QueryRow(ctx, "select count(*) from core.users where id = $1", a.Owner.ID).Scan(&ownUsers))
		if orgs != 0 || users != 0 {
			t.Errorf("la sesión de A ve la organización B (%d) o sus usuarios (%d)", orgs, users)
		}
		if ownUsers != 1 {
			t.Errorf("control: la sesión de A debería verse a sí misma, ve %d", ownUsers)
		}
	})

	// Sin sesión no se ve ningún usuario (core.users ya no es `using (true)`).
	withSession(t, uuid.Nil, uuid.Nil, func(tx pgx.Tx) {
		var n int
		must(t, tx.QueryRow(ctx, "select count(*) from core.users where id in ($1, $2)", a.Owner.ID, b.Owner.ID).Scan(&n))
		if n != 0 {
			t.Errorf("sin sesión se ven %d usuarios", n)
		}
	})
}

// 3) Una FK compuesta que cruza organizaciones se rechaza, y no se puede escribir con organization_id ajeno.
func TestCrossTenantReferencesAreRejected(t *testing.T) {
	requireDB(t)
	a, b := newTenant(t, "A"), newTenant(t, "B")
	ctx := context.Background()

	var membershipB uuid.UUID
	withSession(t, b.Org, b.Owner.ID, func(tx pgx.Tx) {
		must(t, tx.QueryRow(ctx, "select id from core.organization_users where organization_id = $1 and user_id = $2",
			b.Org, b.Member.ID).Scan(&membershipB))
	})

	withSession(t, a.Org, a.Owner.ID, func(tx pgx.Tx) {
		_, err := tx.Exec(ctx, `insert into core.organization_user_roles (organization_id, organization_user_id, role_code, granted_by_user_id)
			values ($1, $2, 'admin', $3)`, a.Org, membershipB, a.Owner.ID)
		if !isCode(err, "23503") {
			t.Errorf("rol de A sobre membresía de B: se esperaba FK violada (23503), se obtuvo %v", err)
		}
	})

	writes := []struct {
		name, sql string
		args      []any
	}{
		{"membresía en B", "insert into core.organization_users (organization_id, user_id) values ($1, $2)", []any{b.Org, a.Owner.ID}},
		{"sucursal en B", "insert into core.branches (organization_id, code, name) values ($1, 'HACK', 'hack')", []any{b.Org}},
		{"invitación en B", `insert into core.invitations (organization_id, email, role_code, token_hash, invited_by_user_id, expires_at)
			values ($1, 'x@isolation.test', 'owner', repeat('a', 64), $2, now() + interval '1 day')`, []any{b.Org, a.Owner.ID}},
	}
	for _, w := range writes {
		withSession(t, a.Org, a.Owner.ID, func(tx pgx.Tx) {
			_, err := tx.Exec(ctx, w.sql, w.args...)
			if !isCode(err, "42501") {
				t.Errorf("%s desde la sesión de A: se esperaba rechazo de RLS (42501), se obtuvo %v", w.name, err)
			}
		})
	}

	withSession(t, a.Org, a.Owner.ID, func(tx pgx.Tx) {
		tag, err := tx.Exec(ctx, "update core.branches set name = 'hack' where id = $1", b.Branch)
		must(t, err)
		if tag.RowsAffected() != 0 {
			t.Error("la sesión de A actualizó una sucursal de B")
		}
	})
}

// 4) platform_app no escribe en billing, fiscal ni receivables, y no modifica ni borra la auditoría.
func TestNoWritesOutsideOwnedSchemas(t *testing.T) {
	requireDB(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		select n.nspname || '.' || c.relname, p.priv
		  from pg_class c
		  join pg_namespace n on n.oid = c.relnamespace
		  cross join unnest(array['INSERT', 'UPDATE', 'DELETE', 'TRUNCATE']) as p(priv)
		 where c.relkind in ('r', 'p')
		   and (n.nspname in ('billing', 'fiscal', 'receivables')
		        or (n.nspname = 'audit' and p.priv <> 'INSERT'))
		   and has_table_privilege(c.oid, p.priv)`)
	must(t, err)
	defer rows.Close()
	for rows.Next() {
		var tbl, priv string
		must(t, rows.Scan(&tbl, &priv))
		t.Errorf("la sesión tiene %s sobre %s", priv, tbl)
	}
	must(t, rows.Err())

	var foreign int
	must(t, pool.QueryRow(ctx, `select count(*) from pg_class c join pg_namespace n on n.oid = c.relnamespace
		where c.relkind in ('r', 'p') and n.nspname in ('billing', 'fiscal', 'receivables')`).Scan(&foreign))
	if foreign == 0 {
		t.Error("control: no se encontraron tablas de billing/fiscal/receivables; ¿es la base correcta?")
	}

	// Además del catálogo, el intento real: el chequeo de permisos ocurre antes de evaluar el WHERE.
	for _, sql := range []string{
		"update audit.audit_events set action = action where false",
		"delete from audit.audit_events where false",
	} {
		withSession(t, uuid.Nil, uuid.Nil, func(tx pgx.Tx) {
			_, err := tx.Exec(ctx, sql)
			if !isCode(err, "42501") {
				t.Errorf("%q: se esperaba 42501, se obtuvo %v", sql, err)
			}
		})
	}
}

// 5) Sin org_id en el token, o con la membresía suspendida, no se entra a /current.
func TestNoOrganizationOrSuspendedMembershipIsRejected(t *testing.T) {
	requireDB(t)
	b := newTenant(t, "B")
	current := "/v1/organizations/current"

	r := call(t, http.MethodGet, current, tokens.issue(b.Owner, uuid.Nil), nil)
	expectStatus(t, "token sin org_id", http.StatusForbidden, r)
	if r.str("type") != "urn:rdl:platform:problem:no-active-organization" {
		t.Errorf("type = %q", r.str("type"))
	}
	expectStatus(t, "org_id inexistente", http.StatusForbidden, call(t, http.MethodGet, current, tokens.issue(b.Owner, uuid.New()), nil))
	expectStatus(t, "sin token", http.StatusUnauthorized, call(t, http.MethodGet, current, "", nil))
	expectStatus(t, "token desconocido", http.StatusUnauthorized, call(t, http.MethodGet, current, "iso-forjado", nil))

	member := b.memberToken()
	expectStatus(t, "miembro activo (control)", http.StatusOK, call(t, http.MethodGet, current, member, nil))
	expectStatus(t, "owner suspende al miembro", http.StatusOK,
		call(t, http.MethodPatch, current+"/users/"+b.Member.ID.String(), b.ownerToken(), map[string]any{"status": "suspended"}))

	for _, path := range []string{current, current + "/branches", current + "/branches/" + b.Branch.String()} {
		r := call(t, http.MethodGet, path, member, nil)
		expectStatus(t, "suspendido: "+path, http.StatusForbidden, r)
		if r.str("type") != "urn:rdl:platform:problem:membership-inactive" {
			t.Errorf("%s: type = %q", path, r.str("type"))
		}
	}
	r = call(t, http.MethodGet, "/v1/me/memberships", member, nil)
	for _, it := range r.items() {
		if it["organizationId"] == b.Org.String() {
			t.Error("la membresía suspendida sigue listada")
		}
	}
}

// 6) Un organization_id en body, query o header no cambia el tenant: manda el org_id del token.
func TestClientSuppliedOrganizationIsIgnored(t *testing.T) {
	requireDB(t)
	a, b := newTenant(t, "A"), newTenant(t, "B")
	tokA, tokB := a.ownerToken(), b.ownerToken()
	spoof := []string{"X-Organization-Id", b.Org.String(), "X-Tenant-Id", b.Org.String(), "Organization-Id", b.Org.String()}
	q := "?organization_id=" + b.Org.String() + "&organizationId=" + b.Org.String()

	r := call(t, http.MethodGet, "/v1/organizations/current"+q, tokA, nil, spoof...)
	if r.str("id") != a.Org.String() {
		t.Errorf("GET current con query/header de B devolvió %q, se esperaba A", r.str("id"))
	}

	r = call(t, http.MethodPost, "/v1/organizations/current/branches"+q, tokA,
		map[string]any{"code": "SPOOF", "name": "Spoof"}, append(spoof, "Idempotency-Key", "iso-"+uuid.NewString())...)
	expectStatus(t, "POST sucursal con query/header de B", http.StatusCreated, r)
	spoofed := r.str("id")
	expectStatus(t, "la sucursal quedó en A", http.StatusOK, call(t, http.MethodGet, "/v1/organizations/current/branches/"+spoofed, tokA, nil))
	expectStatus(t, "la sucursal no está en B", http.StatusNotFound, call(t, http.MethodGet, "/v1/organizations/current/branches/"+spoofed, tokB, nil))

	// En el body, un campo de organización se rechaza por desconocido (DisallowUnknownFields) y nada se crea.
	for _, field := range []string{"organizationId", "organization_id"} {
		r = call(t, http.MethodPost, "/v1/organizations/current/branches", tokA,
			map[string]any{"code": "BODY", "name": "Body", field: b.Org.String()}, "Idempotency-Key", "iso-"+uuid.NewString())
		expectStatus(t, "body con "+field, http.StatusBadRequest, r)
		r = call(t, http.MethodPatch, "/v1/organizations/current", tokA, map[string]any{"tradeName": "hack", field: b.Org.String()}, spoof...)
		expectStatus(t, "PATCH current con "+field, http.StatusBadRequest, r)
	}

	r = call(t, http.MethodPatch, "/v1/organizations/current"+q, tokA, map[string]any{"tradeName": "Solo A"}, spoof...)
	expectStatus(t, "PATCH current con query/header de B", http.StatusOK, r)
	if r.str("id") != a.Org.String() {
		t.Errorf("PATCH current actualizó %q, se esperaba A", r.str("id"))
	}
	if got := call(t, http.MethodGet, "/v1/organizations/current", tokB, nil).str("tradeName"); got == "Solo A" {
		t.Error("el PATCH de A modificó la organización B")
	}
	for _, it := range call(t, http.MethodGet, "/v1/organizations/current/branches?limit=100", tokB, nil).items() {
		if it["code"] == "SPOOF" || it["code"] == "BODY" {
			t.Errorf("B tiene una sucursal creada por A: %v", it)
		}
	}
}

// --- SQL directo ---

// withSession abre una transacción como la API (set_config local) y siempre la revierte.
func withSession(t *testing.T, org, userID uuid.UUID, fn func(pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	must(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		emptyIfNil(org), emptyIfNil(userID))
	must(t, err)
	fn(tx)
}

// tenantTables lista las tablas con organization_id de los schemas que toca este servicio, más las de dominios
// ajenos que platform_app pueda leer (si alguna fuera legible, tampoco debe mostrar filas de B).
func tenantTables(t *testing.T, tx pgx.Tx) []string {
	t.Helper()
	rows, err := tx.Query(context.Background(), `
		select format('%I.%I', c.table_schema, c.table_name)
		  from information_schema.columns c
		  join information_schema.tables tb using (table_schema, table_name)
		 where c.column_name = 'organization_id'
		   and tb.table_type = 'BASE TABLE'
		   and c.table_schema in ('core', 'subscriptions', 'integration', 'audit', 'billing', 'fiscal', 'receivables')
		   and case when has_schema_privilege(c.table_schema, 'USAGE')
		            then has_table_privilege(format('%I.%I', c.table_schema, c.table_name), 'SELECT') end
		 order by 1`)
	must(t, err)
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	must(t, err)
	return tables
}

func emptyIfNil(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func isCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
