//go:build integration

// Package isolation es la suite de aislamiento entre tenants (criterio de terminado del planning, incremento 8).
//
// Corre el router real con los adapters reales de Postgres contra una base con el schema de database-platform.
// Lo único simulado es la verificación del JWT: la suite no puede firmar tokens de Supabase, así que un verificador
// de prueba traduce tokens opacos a identidades. La membresía se sigue revalidando en la base, como en producción.
//
// Base: TEST_DATABASE_URL (+ TEST_DB_PASSWORD opcional) o, si no está, la misma configuración de la API (.env de la
// raíz: login platform_api contra el proyecto dev). Sin ninguna de las dos, las pruebas se saltan con aviso.
// No hay testcontainers: el schema lo crea database-platform (auth, shared, audit, billing, ...), no este repo.
//
// Nunca borra datos: cada prueba crea sus propias organizaciones y usuarios con sujetos aleatorios.
package isolation

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "rdl/platform-api/internal/adapters/http"
	"rdl/platform-api/internal/adapters/postgres"
	"rdl/platform-api/internal/platform/config"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/internal/platform/logger"
	"rdl/platform-api/internal/wiring"
	"rdl/platform-api/pkg/tenancy"
)

var (
	pool       *pgxpool.Pool
	router     http.Handler
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

	log := logger.New(io.Discard, "error", "platform-api-isolation", "test")
	// TTL corto: la suite también prueba que la suspensión surte efecto (la invalidación la hace UpdateMember).
	memberships := tenancy.NewCachedResolver(postgres.NewMembershipResolver(pool), time.Second)
	router = httpadapter.NewRouter(wiring.Deps(log, health.New(log, time.Second), tokens, memberships, postgres.NewTxManager(pool)))

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
	if url == "" {
		return nil, errors.New("sin TEST_DATABASE_URL ni DB_POOLER_HOST")
	}
	p, err := postgres.NewPool(ctx, url, password, 5, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("la base no responde: %w", err)
	}
	return p, nil
}

// requireAppRole se niega a correr con un rol que salte RLS (superusuario, BYPASSRLS o dueño de las tablas):
// la suite daría falsos negativos... o falsos positivos que nadie investigaría.
func requireAppRole(ctx context.Context, p *pgxpool.Pool) error {
	var role string
	var ok bool
	err := p.QueryRow(ctx, `
		select current_user::text,
		       pg_has_role(current_user, 'platform_app', 'member')
		       and not r.rolsuper and not r.rolbypassrls
		       and pg_get_userbyid(c.relowner) <> current_user
		  from pg_roles r, pg_class c
		 where r.rolname = current_user and c.oid = 'core.users'::regclass`).Scan(&role, &ok)
	if err != nil {
		return fmt.Errorf("verificando el rol de la sesión: %w", err)
	}
	if !ok {
		return fmt.Errorf("la sesión corre como %q: se requiere un login que herede platform_app, sin BYPASSRLS ni ser dueño", role)
	}
	return nil
}

func requireDB(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip("sin base de pruebas: " + skipReason)
	}
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

// issue emite un "token" para la identidad; org uuid.Nil equivale a un JWT sin org_id.
func (v *fakeVerifier) issue(u user, org uuid.UUID) string {
	v.mu.Lock()
	defer v.mu.Unlock()
	raw := "iso-" + uuid.NewString()
	v.ids[raw] = tenancy.Identity{Subject: u.Subject, Email: u.Email, FullName: u.Name, OrganizationID: org}
	return raw
}

// --- cliente HTTP en proceso ---

type response struct {
	Status int
	Body   map[string]any
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

func call(t *testing.T, method, path, token string, body any, headers ...string) response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	out := response{Status: rec.Code}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
	}
	return out
}

func expectStatus(t *testing.T, name string, want int, r response) {
	t.Helper()
	if r.Status != want {
		t.Errorf("%s: status %d, se esperaba %d (%v)", name, r.Status, want, r.Body)
	}
}

// --- fixtures ---

type user struct {
	Subject, Email, Name string
	ID                   uuid.UUID
}

func newUser(t *testing.T, label string) user {
	t.Helper()
	tag := uuid.NewString()[:8]
	u := user{Subject: "isolation-" + uuid.NewString(), Email: fmt.Sprintf("iso-%s-%s@isolation.test", label, tag), Name: "Isolation " + label}
	r := call(t, http.MethodGet, "/v1/me", tokens.issue(u, uuid.Nil), nil)
	expectStatus(t, "alta de "+label, http.StatusOK, r)
	u.ID = uuid.MustParse(r.str("id"))
	return u
}

// tenant es una organización con su owner y, opcionalmente, un segundo miembro, una sucursal y una invitación pendiente.
type tenant struct {
	Org        uuid.UUID
	Owner      user
	Member     user // biller que aceptó una invitación
	Branch     uuid.UUID
	Invitation uuid.UUID // pendiente, a un email que no es miembro
	InvToken   string
}

func (tn tenant) ownerToken() string  { return tokens.issue(tn.Owner, tn.Org) }
func (tn tenant) memberToken() string { return tokens.issue(tn.Member, tn.Org) }

func newTenant(t *testing.T, label string) tenant {
	t.Helper()
	tn := tenant{Owner: newUser(t, label+"-owner")}
	n := binary.BigEndian.Uint32(uuid.New().NodeID()[:4]) % 1_000_000_000 // identificación única por corrida
	r := call(t, http.MethodPost, "/v1/organizations", tokens.issue(tn.Owner, uuid.Nil), map[string]any{
		"legalName": fmt.Sprintf("Isolation %s %d S.A.", label, n), "identificationTypeCode": "02",
		"identificationNumber": fmt.Sprintf("3102%09d", n), "email": fmt.Sprintf("org-%d@isolation.test", n),
	}, "Idempotency-Key", "iso-org-"+uuid.NewString())
	expectStatus(t, "alta de organización "+label, http.StatusCreated, r)
	tn.Org = uuid.MustParse(r.str("id"))
	owner := tn.ownerToken()

	r = call(t, http.MethodPost, "/v1/organizations/current/branches", owner,
		map[string]any{"code": "ISO-" + label, "name": "Sucursal " + label}, "Idempotency-Key", "iso-br-"+uuid.NewString())
	expectStatus(t, "sucursal "+label, http.StatusCreated, r)
	tn.Branch = uuid.MustParse(r.str("id"))

	tn.Member = newUser(t, label+"-member")
	r = call(t, http.MethodPost, "/v1/organizations/current/invitations", owner,
		map[string]any{"email": tn.Member.Email, "role": "biller"}, "Idempotency-Key", "iso-inv-"+uuid.NewString())
	expectStatus(t, "invitación al miembro "+label, http.StatusCreated, r)
	r = call(t, http.MethodPost, "/v1/invitations/"+r.str("token")+"/accept", tokens.issue(tn.Member, uuid.Nil), nil,
		"Idempotency-Key", "iso-acc-"+uuid.NewString())
	expectStatus(t, "aceptación del miembro "+label, http.StatusOK, r)

	r = call(t, http.MethodPost, "/v1/organizations/current/invitations", owner,
		map[string]any{"email": fmt.Sprintf("pendiente-%s@isolation.test", uuid.NewString()[:8]), "role": "read_only"},
		"Idempotency-Key", "iso-inv-"+uuid.NewString())
	expectStatus(t, "invitación pendiente "+label, http.StatusCreated, r)
	tn.Invitation = uuid.MustParse(r.str("id"))
	tn.InvToken = r.str("token")

	if t.Failed() {
		t.FailNow()
	}
	return tn
}
