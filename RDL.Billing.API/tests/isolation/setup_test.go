//go:build integration

// Package isolation es la suite de aislamiento entre tenants de Billing (criterio de terminado del planning), con los
// 6 criterios de Platform adaptados.
//
// Corre el router real (internal/wiring) con los adapters reales de Postgres y el login billing_api. Lo único simulado
// es la verificación del JWT: un verificador de prueba traduce tokens opacos a identidades. La membresía se revalida
// contra core, como en producción, con las organizaciones de prueba de scripts/dev/0011_billing_isolation_fixtures.sql.
//
// Cada prueba corre dentro de UNA transacción que se revierte al final (postgres.SavepointTxManager): no deja
// clientes, facturas, audit ni eventos en el outbox.
//
// Base: TEST_DATABASE_URL (+ TEST_DB_PASSWORD) o el .env de la raíz. Sin base, o sin los fixtures, se salta con aviso.
package isolation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "rdl/billing-api/internal/adapters/http"
	"rdl/billing-api/internal/adapters/postgres"
	"rdl/billing-api/internal/platform/config"
	"rdl/billing-api/internal/platform/health"
	"rdl/billing-api/internal/platform/logger"
	"rdl/billing-api/internal/wiring"
	"rdl/billing-api/pkg/tenancy"
)

// Organizaciones, sujetos y sucursales de los fixtures (scripts/dev/0011_billing_isolation_fixtures.sql).
var (
	orgA    = uuid.MustParse("b0000000-0000-4000-8000-00000000000a")
	orgB    = uuid.MustParse("b0000000-0000-4000-8000-00000000000b")
	branchA = uuid.MustParse("b0000000-0000-4000-8000-00000000ba01")
	branchB = uuid.MustParse("b0000000-0000-4000-8000-00000000bb01")
)

const (
	subjectOwnerA     = "billing-iso-owner-a"
	subjectReaderA    = "billing-iso-reader-a"
	subjectSuspendedA = "billing-iso-suspended-a"
	subjectOwnerB     = "billing-iso-owner-b"
)

var (
	pool       *pgxpool.Pool
	tokens     = newFakeVerifier()
	skipReason string
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	p, err := connect(ctx)
	if err != nil {
		skipReason = err.Error()
		fmt.Fprintln(os.Stderr, "AVISO: suite de aislamiento saltada:", skipReason)
		os.Exit(m.Run())
	}
	pool = p
	if err := requireAppRole(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, "suite de aislamiento:", err)
		pool.Close()
		os.Exit(1)
	}
	if _, err := postgres.NewMembershipResolver(pool).ActiveMembership(ctx, subjectOwnerA, orgA); err != nil {
		skipReason = "faltan los fixtures: corra scripts/dev/0011_billing_isolation_fixtures.sql como postgres (" + err.Error() + ")"
		fmt.Fprintln(os.Stderr, "AVISO: suite de aislamiento saltada:", skipReason)
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
	p, err := postgres.NewPool(ctx, url, password, 4, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("la base no responde: %w", err)
	}
	return p, nil
}

// requireAppRole se niega a correr con un rol que salte RLS: la suite daría falsos positivos.
func requireAppRole(ctx context.Context, p *pgxpool.Pool) error {
	var role string
	var ok bool
	err := p.QueryRow(ctx, `
		select current_user::text,
		       pg_has_role(current_user, 'billing_app', 'member')
		       and not r.rolsuper and not r.rolbypassrls
		       and pg_get_userbyid(c.relowner) <> current_user
		  from pg_roles r, pg_class c
		 where r.rolname = current_user and c.oid = 'billing.invoices'::regclass`).Scan(&role, &ok)
	if err != nil {
		return fmt.Errorf("verificando el rol de la sesión: %w", err)
	}
	if !ok {
		return fmt.Errorf("la sesión corre como %q: se requiere un login que herede billing_app, sin BYPASSRLS ni ser dueño", role)
	}
	return nil
}

// env es una prueba: una transacción externa que se revierte al final y el router real sobre ella.
type env struct {
	t      *testing.T
	ctx    context.Context
	outer  pgx.Tx
	router http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	if skipReason != "" {
		t.Skip("sin base de pruebas: " + skipReason)
	}
	ctx := context.Background()
	outer, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = outer.Rollback(context.Background()) })
	log := logger.New(io.Discard, "error", "billing-api-isolation", "test")
	// TTL cortísimo: la revalidación de membresía debe ir a la base en cada request de la suite.
	memberships := tenancy.NewCachedResolver(postgres.NewMembershipResolver(pool), time.Millisecond)
	router := httpadapter.NewRouter(wiring.Deps(log, health.New(log, time.Second), tokens, memberships,
		postgres.SavepointTxManager{Outer: outer}))
	return &env{t: t, ctx: ctx, outer: outer, router: router}
}

// --- verificador de tokens de prueba ---

type fakeVerifier struct {
	mu  sync.Mutex
	ids map[string]tenancy.Identity
}

func newFakeVerifier() *fakeVerifier { return &fakeVerifier{ids: map[string]tenancy.Identity{}} }

func (v *fakeVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	id, ok := v.ids[raw]
	if !ok {
		return tenancy.Identity{}, errors.New("token desconocido")
	}
	return id, nil
}

// issue emite un "token" para el sujeto; org uuid.Nil equivale a un JWT sin org_id.
func (v *fakeVerifier) issue(subject string, org uuid.UUID) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	raw := "iso-" + uuid.NewString()
	v.ids[raw] = tenancy.Identity{Subject: subject, OrganizationID: org}
	return raw
}

// --- cliente HTTP en proceso ---

type response struct {
	Status int
	Body   map[string]any
	Raw    string
}

func (r response) str(key string) string {
	s, _ := r.Body[key].(string)
	return s
}

func (r response) items() []map[string]any {
	raw, _ := r.Body["items"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (e *env) call(method, path, token, body string, headers ...string) response {
	e.t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr).WithContext(e.ctx)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	out := response{Status: rec.Code, Raw: rec.Body.String()}
	if b := bytes.TrimSpace(rec.Body.Bytes()); len(b) > 0 && b[0] == '{' {
		_ = json.Unmarshal(b, &out.Body)
	}
	return out
}

// must exige el status y devuelve la respuesta.
func (e *env) must(status int, r response, what string) response {
	e.t.Helper()
	if r.Status != status {
		e.t.Fatalf("%s: status %d, se esperaba %d (%s)", what, r.Status, status, r.Raw)
	}
	return r
}

// org es lo que una organización tiene creado en la prueba.
type org struct {
	token                          string
	customerID, productID, draftID string
}

// seed crea, con el token de su owner, un cliente, un producto (sin impuestos: fiscal.tax_rates está vacío en dev),
// una secuencia y un borrador con una línea.
func (e *env) seed(subject string, orgID uuid.UUID, tag string) org {
	e.t.Helper()
	o := org{token: tokens.issue(subject, orgID)}
	c := e.must(201, e.call("POST", "/v1/customers", o.token,
		`{"identification":{"typeCode":"02","number":"3101`+tag+`"},"legalName":"Cliente `+tag+`"}`,
		"Idempotency-Key", uuid.NewString()), "crear cliente "+tag)
	o.customerID = c.str("id")
	p := e.must(201, e.call("POST", "/v1/products", o.token,
		`{"code":"P-`+tag+`","description":"Servicio `+tag+`","cabysCode":"8314100000100","unitOfMeasureCode":"Sp","unitPrice":"100","currency":"CRC","isService":true}`,
		"Idempotency-Key", uuid.NewString()), "crear producto "+tag)
	o.productID = p.str("id")
	e.must(200, e.call("PUT", "/v1/document-sequences/invoice", o.token, `{"prefix":"`+tag+`-","nextNumber":1}`), "secuencia "+tag)
	d := e.must(201, e.call("POST", "/v1/invoices", o.token,
		`{"documentType":"invoice","customerId":"`+o.customerID+`","saleConditionCode":"01","currency":"CRC",
		  "lines":[{"productId":"`+o.productID+`","quantity":"2"}]}`, "Idempotency-Key", uuid.NewString()), "crear borrador "+tag)
	o.draftID = d.str("id")
	return o
}
