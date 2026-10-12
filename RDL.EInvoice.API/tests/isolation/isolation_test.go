//go:build integration

// Package isolation es la suite de aislamiento entre tenants (make test-isolation), contra la base de dev con el
// login fiscal_api. Crece con cada incremento hasta los 6 criterios del prompt P5 (incremento 13). Hoy cubre lo que
// existe en el incremento 2: autenticación, membresía, tenant solo del token, privilegios de fiscal_app y RLS de
// las tablas de fiscal.
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

	"rdl/einvoice-api/tests/testkit"
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
	env.Close()
	os.Exit(code)
}

// Sin token, con un token sin org_id, con la membresía suspendida o con la organización de otro no se accede.
func TestAuthenticationAndMembership(t *testing.T) {
	cases := []struct {
		name, token, typ string
		status           int
	}{
		{"sin token", "", "unauthenticated", 401},
		{"token desconocido", "no-es-un-token", "unauthenticated", 401},
		{"token sin org_id", env.Tokens.Issue(env.A.Owner, uuid.Nil), "no-active-organization", 403},
		// El owner de A con el org_id de B en su token: la membresía se revalida en core y no existe.
		{"organización ajena en el token", env.Tokens.Issue(env.A.Owner, env.B.Org), "membership-inactive", 403},
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
		r := env.Call(t, http.MethodGet, "/v1/fiscal-profile", c.token)
		if r.Status != c.status || r.Str("type") != "urn:rdl:fiscal:problem:"+c.typ {
			t.Errorf("%s: %d %s", c.name, r.Status, r.Str("type"))
		}
	}
}

// El tenant sale solo del token: un organization_id en la query o en un header no cambia la organización.
func TestTenantComesOnlyFromToken(t *testing.T) {
	token := env.A.OwnerToken(env)
	base := env.Call(t, http.MethodGet, "/v1/fiscal-profile", token)
	for name, r := range map[string]testkit.Response{
		"query":  env.Call(t, http.MethodGet, "/v1/fiscal-profile?organizationId="+env.B.Org.String(), token),
		"header": env.Call(t, http.MethodGet, "/v1/fiscal-profile", token, "X-Organization-Id", env.B.Org.String()),
	} {
		if r.Status != base.Status || r.Str("legalName") != base.Str("legalName") {
			t.Errorf("con organization_id de B en %s: %d %v, sin él: %d %v", name, r.Status, r.Body, base.Status, base.Body)
		}
	}
	// Owner de A, organización sin perfil (hoy no hay perfiles: PUT llega en el incremento 4) → 404, no 500 ni 403.
	if base.Status != http.StatusOK && base.Status != http.StatusNotFound {
		t.Errorf("perfil fiscal de A: %d %v", base.Status, base.Body)
	}
}

// fiscal_app no escribe en schemas ajenos, no ve el outbox de otros servicios y no modifica la auditoría.
func TestPrivileges(t *testing.T) {
	ctx := context.Background()
	err := testkit.WithSession(ctx, env.Pool, env.A.Org, env.A.Owner.ID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select table_schema || '.' || table_name, p.priv
			  from information_schema.tables t
			  cross join (values ('INSERT'), ('UPDATE'), ('DELETE'), ('TRUNCATE')) p(priv)
			 where t.table_schema in ('core', 'billing', 'receivables', 'subscriptions') and t.table_type = 'BASE TABLE'
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
			t.Errorf("fiscal_app tiene %s sobre %s", priv, tbl)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		var foreign int
		if err := tx.QueryRow(ctx, "select count(*) from integration.outbox_messages where source_service <> 'fiscal'").Scan(&foreign); err != nil {
			return err
		}
		if foreign != 0 {
			t.Errorf("fiscal_app ve %d mensajes del outbox de otros servicios", foreign)
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
}

// Toda tabla de fiscal con organization_id tiene RLS y una política que filtra por la organización de la sesión;
// los catálogos solo se leen. Sin sesión, una tabla de organización no devuelve filas.
func TestFiscalTablesHaveTenantRLS(t *testing.T) {
	ctx := context.Background()
	err := testkit.WithSession(ctx, env.Pool, uuid.Nil, uuid.Nil, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select c.relname, c.relrowsecurity,
			       exists (select 1 from pg_policy p where p.polrelid = c.oid
			                and pg_get_expr(p.polqual, p.polrelid) like '%current_organization_id()%')
			  from pg_class c
			  join pg_namespace n on n.oid = c.relnamespace
			 where n.nspname = 'fiscal' and c.relkind = 'r'
			   and exists (select 1 from pg_attribute a where a.attrelid = c.oid and a.attname = 'organization_id')`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			var rls, policy bool
			if err := rows.Scan(&name, &rls, &policy); err != nil {
				return err
			}
			if !rls || !policy {
				t.Errorf("fiscal.%s: rls=%v, política por organización=%v", name, rls, policy)
			}
			tables = append(tables, name)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(tables) < 13 {
			t.Errorf("se esperaban al menos 13 tablas de organización en fiscal, hay %d: %v", len(tables), tables)
		}
		for _, name := range tables {
			var n int
			if err := tx.QueryRow(ctx, "select count(*) from fiscal."+pgx.Identifier{name}.Sanitize()).Scan(&n); err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("fiscal.%s devuelve %d filas sin organización en la sesión", name, n)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Con la sesión de B no se leen ni se modifican el perfil, el certificado ni los documentos de A (RLS), aunque se
// pida por el organization_id de A. El certificado de A no puede firmar un documento de B porque B no lo ve.
func TestDirectSelectDoesNotSeeOtherTenant(t *testing.T) {
	ctx := context.Background()
	err := testkit.WithSession(ctx, env.Pool, env.B.Org, env.B.Owner.ID, func(tx pgx.Tx) error {
		for _, table := range []string{"taxpayer_profiles", "certificates", "electronic_documents", "electronic_document_lines",
			"organization_economic_activities", "submission_attempts", "document_files"} {
			var n int
			q := "select count(*) from fiscal." + pgx.Identifier{table}.Sanitize() + " where organization_id = $1"
			if err := tx.QueryRow(ctx, q, env.A.Org).Scan(&n); err != nil {
				return err
			}
			if n != 0 {
				t.Errorf("B ve %d filas de A en fiscal.%s", n, table)
			}
		}
		tag, err := tx.Exec(ctx, "update fiscal.certificates set is_active = false where organization_id = $1", env.A.Org)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			t.Errorf("B modificó %d certificados de A", tag.RowsAffected())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
