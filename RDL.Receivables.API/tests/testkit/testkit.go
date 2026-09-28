//go:build integration

// Package testkit es lo común de las suites que corren contra la base de dev (tests/e2e, tests/isolation): el
// router real con los adapters reales, un verificador de tokens de prueba y constructores de eventos a partir de
// los ejemplos de RDL.Contracts.
//
// Base: TEST_DATABASE_URL (+ TEST_DB_PASSWORD) o, si no está, el .env de la raíz (login receivables_api). Se niega a
// correr con un rol que salte RLS. Organizaciones: ISOLATION_ORG_A e ISOLATION_ORG_B, dos organizaciones activas de
// dev con un owner activo (Receivables no puede crear organizaciones: las crea Platform). Sin ellas, todo se salta.
//
// Nunca borra datos (la app no puede): cada prueba crea sus propias cuentas y pagos con ids aleatorios.
package testkit

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
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "rdl/receivables-api/internal/adapters/http"
	"rdl/receivables-api/internal/adapters/postgres"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/platform/config"
	"rdl/receivables-api/internal/platform/examples"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/internal/platform/logger"
	"rdl/receivables-api/internal/wiring"
	"rdl/receivables-api/pkg/tenancy"
)

// Env es el entorno compartido por una suite.
type Env struct {
	Pool      *pgxpool.Pool
	Router    http.Handler
	Tokens    *FakeVerifier
	Processor *app.ProcessEvent
	A, B      Tenant
}

// Tenant es una organización de prueba con su owner y, si existe, un miembro suspendido.
type Tenant struct {
	Org       uuid.UUID
	Owner     User
	Suspended *User
}

type User struct {
	ID      uuid.UUID
	Subject string
}

// Setup conecta y arma el entorno; devuelve un motivo si la suite debe saltarse.
func Setup(ctx context.Context) (*Env, string, error) {
	// El .env de la raíz también puede traer ISOLATION_ORG_A/B (no pisa lo que ya esté en el entorno).
	if err := config.LoadDotEnv("../../.env"); err != nil {
		return nil, "", err
	}
	orgA, errA := uuid.Parse(os.Getenv("ISOLATION_ORG_A"))
	orgB, errB := uuid.Parse(os.Getenv("ISOLATION_ORG_B"))
	if errA != nil || errB != nil || orgA == orgB {
		return nil, "sin ISOLATION_ORG_A e ISOLATION_ORG_B (dos organizaciones activas de dev con owner)", nil
	}
	pool, reason, err := connect(ctx)
	if reason != "" || err != nil {
		return nil, reason, err
	}
	if err := requireAppRole(ctx, pool); err != nil {
		pool.Close()
		return nil, "", err
	}
	env := &Env{Pool: pool, Tokens: NewFakeVerifier()}
	for _, t := range []struct {
		org uuid.UUID
		dst *Tenant
	}{{orgA, &env.A}, {orgB, &env.B}} {
		tn, err := loadTenant(ctx, pool, t.org)
		if err != nil {
			pool.Close()
			return nil, "", err
		}
		*t.dst = tn
	}

	log := logger.New(os.Stderr, "error", "receivables-api-test", "test")
	txm, err := wiring.TxManager(pool)
	if err != nil {
		return nil, "", err
	}
	// TTL corto: la suite prueba que una membresía suspendida no entra.
	memberships := tenancy.NewCachedResolver(postgres.NewMembershipResolver(pool), time.Second)
	env.Router = httpadapter.NewRouter(wiring.Deps(log, health.New(log, time.Second), env.Tokens, memberships, txm))
	policy := app.RetryPolicy{Attempts: 2, Base: 10 * time.Millisecond, Max: 20 * time.Millisecond}
	env.Processor, err = wiring.EventProcessor(pool, txm, policy)
	if err != nil {
		return nil, "", err
	}
	return env, "", nil
}

func connect(ctx context.Context) (*pgxpool.Pool, string, error) {
	url, password := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_DB_PASSWORD")
	if url == "" {
		cfg, err := config.Load()
		if err != nil {
			return nil, "sin TEST_DATABASE_URL y sin configuración de la API: " + err.Error(), nil
		}
		url, password = cfg.DB.URL, cfg.DB.Password
	}
	if url == "" {
		return nil, "sin TEST_DATABASE_URL ni DB_POOLER_HOST", nil
	}
	p, err := postgres.NewPool(ctx, url, password, 10, 15*time.Second)
	if err != nil {
		return nil, "", err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, "la base no responde: " + err.Error(), nil
	}
	return p, "", nil
}

// requireAppRole se niega a correr con un rol que salte RLS (superusuario, BYPASSRLS o dueño de las tablas).
func requireAppRole(ctx context.Context, p *pgxpool.Pool) error {
	var role string
	var ok bool
	err := p.QueryRow(ctx, `
		select current_user::text,
		       pg_has_role(current_user, 'receivables_app', 'member')
		       and not r.rolsuper and not r.rolbypassrls
		       and pg_get_userbyid(c.relowner) <> current_user
		  from pg_roles r, pg_class c
		 where r.rolname = current_user and c.oid = 'receivables.receivables'::regclass`).Scan(&role, &ok)
	if err != nil {
		return fmt.Errorf("verificando el rol de la sesión: %w", err)
	}
	if !ok {
		return fmt.Errorf("la sesión corre como %q: se requiere un login que herede receivables_app, sin BYPASSRLS ni ser dueño", role)
	}
	return nil
}

// loadTenant busca el owner activo (y un miembro suspendido) con la sesión de la organización: receivables_app
// solo ve los miembros de la organización de la sesión (políticas de core).
func loadTenant(ctx context.Context, p *pgxpool.Pool, org uuid.UUID) (Tenant, error) {
	t := Tenant{Org: org}
	err := WithSession(ctx, p, org, uuid.Nil, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			select u.id, u.external_subject
			  from core.organization_users ou
			  join core.users u on u.id = ou.user_id and u.status = 'active'
			  join core.organization_user_roles r on r.organization_id = ou.organization_id and r.organization_user_id = ou.id
			 where ou.organization_id = $1 and ou.status = 'active' and r.role_code = 'owner'
			 order by u.id limit 1`, org).Scan(&t.Owner.ID, &t.Owner.Subject)
		if err != nil {
			return fmt.Errorf("organización %s sin owner activo visible: %w", org, err)
		}
		var s User
		err = tx.QueryRow(ctx, `
			select u.id, u.external_subject
			  from core.organization_users ou join core.users u on u.id = ou.user_id
			 where ou.organization_id = $1 and ou.status = 'suspended'
			 order by u.id limit 1`, org).Scan(&s.ID, &s.Subject)
		if err == nil {
			t.Suspended = &s
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return nil
	})
	return t, err
}

// WithSession corre fn con la sesión RLS de la organización (y del usuario) y hace rollback siempre.
func WithSession(ctx context.Context, p *pgxpool.Pool, org, user uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	orgS, userS := "", ""
	if org != uuid.Nil {
		orgS = org.String()
	}
	if user != uuid.Nil {
		userS = user.String()
	}
	if _, err := tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)", orgS, userS); err != nil {
		return err
	}
	return fn(tx)
}

// --- tokens de prueba ---

// FakeVerifier traduce tokens opacos a identidades. La membresía se sigue revalidando en la base, como en
// producción: solo la firma del JWT es simulada.
type FakeVerifier struct {
	mu  sync.Mutex
	ids map[string]tenancy.Identity
}

func NewFakeVerifier() *FakeVerifier { return &FakeVerifier{ids: map[string]tenancy.Identity{}} }

func (v *FakeVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	id, ok := v.ids[raw]
	if !ok {
		return tenancy.Identity{}, errors.New("token desconocido")
	}
	return id, nil
}

// Issue emite un token para el usuario; org uuid.Nil equivale a un JWT sin org_id.
func (v *FakeVerifier) Issue(u User, org uuid.UUID) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	raw := "test-" + uuid.NewString()
	v.ids[raw] = tenancy.Identity{Subject: u.Subject, OrganizationID: org}
	return raw
}

func (t Tenant) OwnerToken(e *Env) string { return e.Tokens.Issue(t.Owner, t.Org) }

// --- cliente HTTP en proceso ---

type Response struct {
	Status  int
	Body    map[string]any
	List    []any
	Headers http.Header
}

func (r Response) Str(key string) string {
	s, _ := r.Body[key].(string)
	return s
}

// Call hace una petición al router. Los POST llevan una Idempotency-Key nueva salvo que se pase otra.
func (e *Env) Call(t *testing.T, method, path, token string, body any, headers ...string) Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", "test-"+uuid.NewString())
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	out := Response{Status: rec.Code, Headers: rec.Header()}
	raw := rec.Body.Bytes()
	if len(bytes.TrimSpace(raw)) > 0 {
		if raw[0] == '[' {
			_ = json.Unmarshal(raw, &out.List)
		} else {
			_ = json.Unmarshal(raw, &out.Body)
		}
	}
	return out
}

func Expect(t *testing.T, what string, want int, r Response) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("%s: status %d, se esperaba %d: %v", what, r.Status, want, r.Body)
	}
}

// --- eventos de Billing a partir de los ejemplos de contratos ---

const examplesDir = "../../../RDL.Contracts/examples/events"

// Event arma un evento válido a partir de valid[0] del ejemplo de contratos, con ids nuevos y los campos dados.
func Event(t *testing.T, file string, org uuid.UUID, set map[string]any) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(examplesDir, file)) //nolint:gosec // G304: ruta fija del repo de contratos
	if err != nil {
		t.Fatal(err)
	}
	valid, _, err := examples.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(valid[0]))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	doc["eventId"], doc["correlationId"], doc["organizationId"] = uuid.NewString(), uuid.NewString(), org.String()
	for k, v := range set {
		doc[k] = v
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Invoice es una factura emitida de prueba.
type Invoice struct {
	ID       uuid.UUID
	Customer uuid.UUID
	Total    string
	Payload  []byte
}

// IssueInvoice publica un InvoiceIssued para la organización y devuelve la factura.
func (e *Env) IssueInvoice(t *testing.T, org uuid.UUID, total, dueDate string) Invoice {
	t.Helper()
	inv := Invoice{ID: uuid.New(), Customer: uuid.New(), Total: total}
	inv.Payload = Event(t, "invoice-issued.v1.json", org, map[string]any{
		"invoiceId": inv.ID.String(), "invoiceNumber": "FAC-T" + inv.ID.String()[:8], "total": total,
		"dueDate": dueDate, "issueDate": "2026-01-01",
		"customerSnapshot": map[string]any{
			"customerId": inv.Customer.String(), "legalName": "Cliente de prueba S.A.",
			"identification": map[string]any{"typeCode": "02", "number": "3101999999"},
		},
	})
	e.MustProcess(t, inv.Payload, app.OutcomeProcessed)
	return inv
}

// MustProcess pasa el evento por el mismo procesamiento que el consumidor y exige el resultado.
func (e *Env) MustProcess(t *testing.T, payload []byte, want app.Outcome) app.Result {
	t.Helper()
	res, err := e.Processor.Process(context.Background(), payload)
	if err != nil || res.Outcome != want {
		t.Fatalf("evento: %s (%v), se esperaba %s", res.Outcome, errors.Join(err, res.Err), want)
	}
	return res
}

// ReceivableOf busca la cuenta de una factura con el endpoint interno.
func (e *Env) ReceivableOf(t *testing.T, tn Tenant, invoice uuid.UUID) Response {
	t.Helper()
	r := e.Call(t, http.MethodGet, "/internal/v1/receivables/by-invoice/"+invoice.String(), tn.OwnerToken(e), nil)
	Expect(t, "saldo por factura", http.StatusOK, r)
	return r
}

// Outbox lee los eventos que Receivables escribió con un correlationId (la política deja ver solo los propios).
func (e *Env) Outbox(t *testing.T, org uuid.UUID, eventType string, aggregate uuid.UUID) []map[string]any {
	t.Helper()
	var out []map[string]any
	err := WithSession(context.Background(), e.Pool, org, uuid.Nil, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `
			select payload::text from integration.outbox_messages
			 where source_service = 'receivables' and organization_id = $1 and event_type = $2 and aggregate_id = $3
			 order by occurred_at`, org, eventType, aggregate)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return err
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				return err
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
