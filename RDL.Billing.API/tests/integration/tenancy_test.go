//go:build integration

// Package integration prueba los adapters de Postgres contra la base dev con el login billing_api (no superusuario),
// igual que la suite de Platform. Base: TEST_DATABASE_URL (+ TEST_DB_PASSWORD) o el .env de la raíz del repo.
// Sin base, se salta con aviso. Nunca escribe en core; lo que escribe en billing lo revierte.
//
// TEST_MEMBER_SUBJECT y TEST_MEMBER_ORG (opcionales): sujeto de Supabase y organización de una membresía activa
// de prueba, para verificar el camino feliz de la revalidación. Billing no puede listar usuarios de core.
package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/billing-api/internal/adapters/postgres"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/platform/config"
	"rdl/billing-api/pkg/tenancy"
)

var (
	pool       *pgxpool.Pool
	skipReason string
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	p, err := connect(ctx)
	if err != nil {
		skipReason = err.Error()
		fmt.Fprintln(os.Stderr, "AVISO: pruebas de integración saltadas:", skipReason)
		os.Exit(m.Run())
	}
	pool = p
	if err := requireAppRole(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "integración:", err)
		pool.Close()
		os.Exit(1)
	}
	code := m.Run()
	pool.Close()
	os.Exit(code)
}

func connect(ctx context.Context) (*pgxpool.Pool, error) {
	url, password := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_DB_PASSWORD")
	if url == "" {
		if err := config.LoadDotEnv("../../.env"); err != nil {
			return nil, err
		}
		cfg, err := config.Load()
		if err != nil {
			return nil, fmt.Errorf("sin TEST_DATABASE_URL y sin configuración de la API: %w", err)
		}
		url, password = cfg.DB.URL, cfg.DB.Password
	}
	p, err := postgres.NewPool(ctx, url, password, 3, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("la base no responde: %w", err)
	}
	return p, nil
}

// requireAppRole se niega a correr con un rol que salte RLS: las pruebas darían falsos positivos.
func requireAppRole(ctx context.Context, p *pgxpool.Pool) error {
	var role string
	var ok bool
	err := p.QueryRow(ctx, `
		select current_user::text,
		       pg_has_role(current_user, 'billing_app', 'member')
		       and not r.rolsuper and not r.rolbypassrls
		       and pg_get_userbyid(c.relowner) <> current_user
		  from pg_roles r, pg_class c
		 where r.rolname = current_user and c.oid = 'billing.customers'::regclass`).Scan(&role, &ok)
	if err != nil {
		return fmt.Errorf("verificando el rol de la sesión: %w", err)
	}
	if !ok {
		return fmt.Errorf("la sesión corre como %q: se requiere un login que herede billing_app, sin BYPASSRLS ni ser dueño", role)
	}
	return nil
}

func requireDB(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip("sin base de pruebas: " + skipReason)
	}
}

func TestMembershipUnknownSubjectIsRejected(t *testing.T) {
	requireDB(t)
	// Si faltaran los GRANT de core, esto sería un error de permisos y no ErrNoMembership.
	_, err := postgres.NewMembershipResolver(pool).ActiveMembership(context.Background(), "sub-"+uuid.NewString(), uuid.New())
	if !errors.Is(err, tenancy.ErrNoMembership) {
		t.Fatalf("err = %v, se esperaba ErrNoMembership", err)
	}
}

func TestMembershipActiveMember(t *testing.T) {
	requireDB(t)
	subject, org := os.Getenv("TEST_MEMBER_SUBJECT"), os.Getenv("TEST_MEMBER_ORG")
	if subject == "" || org == "" {
		t.Skip("defina TEST_MEMBER_SUBJECT y TEST_MEMBER_ORG para probar el camino feliz")
	}
	orgID := uuid.MustParse(org)
	r := postgres.NewMembershipResolver(pool)
	m, err := r.ActiveMembership(context.Background(), subject, orgID)
	if err != nil {
		t.Fatalf("membresía activa rechazada: %v", err)
	}
	if m.UserID == uuid.Nil || len(m.Roles) == 0 {
		t.Fatalf("membresía = %+v", m)
	}
	// La misma persona en una organización donde no es miembro.
	if _, err := r.ActiveMembership(context.Background(), subject, uuid.New()); !errors.Is(err, tenancy.ErrNoMembership) {
		t.Fatalf("otra organización: err = %v", err)
	}
}

func TestTenantSessionIsolatesCustomers(t *testing.T) {
	requireDB(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()

	// Con la sesión en A, una fila de B no se puede escribir (WITH CHECK) ni leer (USING).
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		orgA.String(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `insert into billing.customers (organization_id, identification_type_code, identification_number, legal_name)
	                       values ($1, '01', '999999999', 'Intruso')`, orgB)
	if err == nil {
		t.Fatal("RLS permitió insertar un cliente de otra organización")
	}

	// El caso de uso real, vía TxManager, con una organización sin clientes.
	uc := app.NewListCustomers(postgres.NewTxManager(pool))
	page, err := uc.Execute(ctx, tenancy.NewContext(uuid.New(), "sub", orgA, []string{"read_only"}), app.CustomerQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.Next != nil {
		t.Fatalf("página = %+v", page)
	}
}
