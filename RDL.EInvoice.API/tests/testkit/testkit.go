//go:build integration

// Package testkit es lo común de las suites que corren contra la base de dev (tests/isolation y las que vengan):
// el router real con los adapters reales y un verificador de tokens de prueba. Origen: RDL.Receivables.API.
//
// Base: TEST_DATABASE_URL (+ TEST_DB_PASSWORD) o, si no está, el .env de la raíz (login fiscal_api). Se niega a correr
// con un rol que salte RLS. Organizaciones: ISOLATION_ORG_A e ISOLATION_ORG_B, dos organizaciones activas de dev con
// un owner activo (fiscal no puede crear organizaciones: las crea Platform). Sin ellas, todo se salta con aviso.
package testkit

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
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

	httpadapter "rdl/einvoice-api/internal/adapters/http"
	"rdl/einvoice-api/internal/adapters/postgres"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/platform/config"
	"rdl/einvoice-api/internal/platform/examples"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/internal/platform/logger"
	"rdl/einvoice-api/internal/wiring"
	"rdl/einvoice-api/pkg/tenancy"
)

// Env es el entorno compartido por una suite.
type Env struct {
	Pool      *pgxpool.Pool
	Router    http.Handler
	Tokens    *FakeVerifier
	Processor *app.ProcessEvent
	// SecretsDir es el directorio temporal del SecretStore de la suite (se borra al final con Close).
	SecretsDir string
	A, B       Tenant
}

// Close libera el pool y borra los secretos de la suite.
func (e *Env) Close() {
	e.Pool.Close()
	if e.SecretsDir != "" {
		_ = os.RemoveAll(e.SecretsDir)
	}
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

	// Las organizaciones de prueba de dev comparten usuarios: si el owner de A también es miembro activo de B, los
	// casos cruzados no prueban nada. Se aborta en vez de dar un falso positivo o negativo.
	resolver := postgres.NewMembershipResolver(pool)
	for _, pair := range []struct {
		owner User
		other uuid.UUID
	}{{env.A.Owner, orgB}, {env.B.Owner, orgA}} {
		if _, err := resolver.ActiveMembership(ctx, pair.owner.Subject, pair.other); !errors.Is(err, tenancy.ErrNoMembership) {
			pool.Close()
			return nil, "", fmt.Errorf("el owner %s es miembro activo de la otra organización de prueba (%s): elija otro par ISOLATION_ORG_A/B: %w", pair.owner.ID, pair.other, errors.Join(errors.New("membresía encontrada"), err))
		}
	}

	log := logger.New(os.Stderr, "error", "einvoice-api-test", "test")
	// TTL corto: la suite prueba que una membresía suspendida no entra.
	memberships := tenancy.NewCachedResolver(postgres.NewMembershipResolver(pool), time.Second)
	txm, err := wiring.TxManager(pool)
	if err != nil {
		pool.Close()
		return nil, "", err
	}
	// SecretStore de la suite: directorio temporal y llave aleatoria (nunca la del .env).
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		pool.Close()
		return nil, "", err
	}
	env.SecretsDir, err = os.MkdirTemp("", "einvoice-secrets-")
	if err != nil {
		pool.Close()
		return nil, "", err
	}
	store, err := wiring.SecretStore(env.SecretsDir, base64.StdEncoding.EncodeToString(key))
	if err != nil {
		pool.Close()
		return nil, "", err
	}
	env.Router = httpadapter.NewRouter(wiring.Deps(log, health.New(log, time.Second), env.Tokens, memberships, txm, store))
	policy := app.RetryPolicy{Attempts: 2, Base: 10 * time.Millisecond, Max: 20 * time.Millisecond}
	env.Processor, err = wiring.EventProcessor(pool, txm, policy)
	if err != nil {
		pool.Close()
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
		       pg_has_role(current_user, 'fiscal_app', 'member')
		       and not r.rolsuper and not r.rolbypassrls
		       and pg_get_userbyid(c.relowner) <> current_user
		  from pg_roles r, pg_class c
		 where r.rolname = current_user and c.oid = 'fiscal.electronic_documents'::regclass`).Scan(&role, &ok)
	if err != nil {
		return fmt.Errorf("verificando el rol de la sesión: %w", err)
	}
	if !ok {
		return fmt.Errorf("la sesión corre como %q: se requiere un login que herede fiscal_app, sin BYPASSRLS ni ser dueño", role)
	}
	return nil
}

// loadTenant busca el owner activo (y un miembro suspendido) con la sesión de la organización: fiscal_app solo ve
// los miembros de la organización de la sesión (políticas de core).
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
	Status int
	Body   map[string]any
}

func (r Response) Str(key string) string {
	s, _ := r.Body[key].(string)
	return s
}

// Call hace una petición al router.
func (e *Env) Call(t *testing.T, method, path, token string, headers ...string) Response {
	t.Helper()
	return e.CallBody(t, method, path, token, nil, headers...)
}

// CallBody hace una petición con un cuerpo JSON.
func (e *Env) CallBody(t *testing.T, method, path, token string, body any, headers ...string) Response {
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	out := Response{Status: rec.Code}
	raw, _ := io.ReadAll(rec.Body)
	if len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
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

// IssueInvoice arma un InvoiceIssued con una factura nueva para la organización.
func IssueInvoice(t *testing.T, org uuid.UUID) (uuid.UUID, []byte) {
	t.Helper()
	id := uuid.New()
	return id, Event(t, "invoice-issued.v1.json", org, map[string]any{
		"invoiceId": id.String(), "invoiceNumber": "FAC-T" + id.String()[:8],
	})
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

// EnsureProfile deja un perfil fiscal de PRUEBA (datos ficticios) en la organización, si no tiene uno. Queda en dev:
// fiscal_app no borra perfiles. Requiere el catálogo de tipos de identificación (cmd/import-catalogs).
func (e *Env) EnsureProfile(ctx context.Context, t *testing.T, tn Tenant) {
	t.Helper()
	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		tn.Org.String(), tn.Owner.ID.String()); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `insert into fiscal.taxpayer_profiles
		(organization_id, identification_type_code, identification_number, legal_name, email)
		values ($1, '02', '3101000001', 'Organización de prueba fiscal S.A.', 'fiscal.pruebas@example.test')
		on conflict (organization_id) do nothing`, tn.Org)
	if err != nil {
		t.Fatalf("creando el perfil fiscal de prueba: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// CallRaw hace una petición con un cuerpo y un Content-Type cualesquiera (multipart del certificado).
func (e *Env) CallRaw(t *testing.T, method, path, token, contentType string, body []byte) Response {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.Router.ServeHTTP(rec, req)
	out := Response{Status: rec.Code}
	if raw := rec.Body.Bytes(); len(bytes.TrimSpace(raw)) > 0 {
		_ = json.Unmarshal(raw, &out.Body)
	}
	return out
}
