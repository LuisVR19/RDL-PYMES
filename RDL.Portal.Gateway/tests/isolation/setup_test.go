//go:build integration

// Package isolation es la suite de aislamiento entre organizaciones del Portal Gateway (P7 · incremento 8).
//
// A diferencia de las pruebas de internal/adapters/http, aquí no hay nada simulado: el router real del gateway
// (armado por wiring), el verificador real contra el JWKS de Supabase dev, tokens reales de dos usuarios de
// prueba y las APIs reales corriendo en local. Lo que se prueba es que, pasando por el gateway, un usuario de la
// organización A nunca ve ni toca nada de la organización B, y que el gateway no mezcla identidades entre
// peticiones concurrentes.
//
// Requisitos (si falta alguno, la suite se salta con aviso, no falla):
//   - Platform corriendo (PLATFORM_API_URL, por defecto http://localhost:8080).
//   - Dos usuarios de prueba de Supabase dev: E2E_EMAIL/E2E_PASSWORD y E2E_EMAIL2/E2E_PASSWORD2, del entorno o
//     de .e2e.local (los mismos del E2E de Platform).
//   - Billing es opcional (BILLING_API_URL, por defecto http://localhost:8081): sin ella, sus casos se saltan.
//
// Nunca borra datos: cada corrida crea dos organizaciones nuevas en dev, una por usuario, y trabaja dentro de ellas.
package isolation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/portal-gateway/internal/adapters/auth"
	httpadapter "rdl/portal-gateway/internal/adapters/http"
	"rdl/portal-gateway/internal/platform/config"
	"rdl/portal-gateway/internal/platform/logger"
	"rdl/portal-gateway/internal/wiring"
)

// La publishable key de Supabase dev es pública por diseño (la usa el navegador); es la misma del E2E de Platform.
const defaultPublishableKey = "sb_publishable_X0tQqmJnXotXyvec0kCOtw_0h5U2iby"

// tenant es un usuario real con su propia organización, ya activa en su token.
type tenant struct {
	Email string
	Token string
	OrgID string
}

var (
	router     http.Handler
	skipReason string
	billingUp  bool
	a, b       tenant
)

func TestMain(m *testing.M) {
	if err := setup(); err != nil {
		var skip skipError
		if !errors.As(err, &skip) {
			fmt.Fprintln(os.Stderr, "suite de aislamiento:", err)
			os.Exit(1)
		}
		skipReason = skip.reason
		fmt.Fprintln(os.Stderr, "AVISO: suite de aislamiento saltada:", skipReason)
	}
	os.Exit(m.Run())
}

// skipError distingue «falta el entorno» (se salta) de «el entorno está y algo salió mal» (falla).
type skipError struct{ reason string }

func (e skipError) Error() string { return e.reason }

func setup() error {
	for _, f := range []string{"../../.env", "../../.e2e.local"} {
		if err := config.LoadDotEnv(f); err != nil {
			return err
		}
	}
	setDefault("SUPABASE_URL", "https://dzlsnsstuqpxvwegeqcy.supabase.co")
	setDefault("PLATFORM_API_URL", "http://localhost:8080")
	setDefault("BILLING_API_URL", "http://localhost:8081")
	setDefault("SUPABASE_PUBLISHABLE_KEY", defaultPublishableKey)

	for _, k := range []string{"E2E_EMAIL", "E2E_PASSWORD", "E2E_EMAIL2", "E2E_PASSWORD2"} {
		if os.Getenv(k) == "" {
			return skipError{"falta " + k + " (entorno o .e2e.local)"}
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	platformURL, billingURL := os.Getenv("PLATFORM_API_URL"), os.Getenv("BILLING_API_URL")
	if !healthy(platformURL) {
		return skipError{"Platform no responde en " + platformURL + " (make run en RDL.Platform.API)"}
	}
	billingUp = healthy(billingURL)
	if !billingUp {
		fmt.Fprintln(os.Stderr, "AVISO: Billing no responde en", billingURL, "— sus casos se saltan")
	}

	ctx := context.Background()
	log := logger.New(io.Discard, "error", "portal-gateway-isolation", "test")
	verifier, err := auth.NewVerifier(ctx, log, cfg.Auth.JWKSURL, cfg.Auth.Issuer, cfg.Auth.Audience, cfg.Auth.JWKSRefresh)
	if err != nil {
		return err
	}
	router, err = httpadapter.NewRouter(wiring.Deps(cfg, log, verifier, wiring.Clients(cfg, log), wiring.HealthChecks(cfg, log)))
	if err != nil {
		return err
	}

	supabase := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	if a, err = newTenant(supabase, os.Getenv("E2E_EMAIL"), os.Getenv("E2E_PASSWORD"), "A"); err != nil {
		return err
	}
	if b, err = newTenant(supabase, os.Getenv("E2E_EMAIL2"), os.Getenv("E2E_PASSWORD2"), "B"); err != nil {
		return err
	}
	if a.OrgID == b.OrgID {
		return errors.New("los dos usuarios quedaron en la misma organización: la suite no probaría nada")
	}
	return nil
}

// newTenant inicia sesión, crea una organización nueva a través del gateway, la activa y refresca el token para
// que traiga su org_id: exactamente el flujo del portal en las pantallas 3 y 5.
func newTenant(supabase, email, password, label string) (tenant, error) {
	sess, err := supabaseToken(supabase, "password", map[string]string{"email": email, "password": password})
	if err != nil {
		return tenant{}, fmt.Errorf("login de %s: %w", label, err)
	}

	run := randomDigits(6)
	r := do(http.MethodPost, "/portal/v1/organizations", sess.AccessToken, map[string]string{
		"legalName":              "Aislamiento GW " + label + " " + run + " S.A.",
		"identificationTypeCode": "02",
		"identificationNumber":   "3101" + run,
		"email":                  "gw-iso-" + run + "@rdlpymes.invalid",
	}, "Idempotency-Key", "gw-iso-org-"+uuid.NewString())
	if r.Status != http.StatusCreated {
		return tenant{}, fmt.Errorf("crear la organización de %s: %d %v", label, r.Status, r.Body)
	}
	org := r.str("id")

	r = do(http.MethodPut, "/portal/v1/me/active-organization", sess.AccessToken, map[string]string{"organizationId": org})
	if r.Status != http.StatusOK {
		return tenant{}, fmt.Errorf("activar la organización de %s: %d %v", label, r.Status, r.Body)
	}
	sess, err = supabaseToken(supabase, "refresh_token", map[string]string{"refresh_token": sess.RefreshToken})
	if err != nil {
		return tenant{}, fmt.Errorf("refrescar el token de %s: %w", label, err)
	}
	if got := orgClaim(sess.AccessToken); got != org {
		return tenant{}, fmt.Errorf("el token de %s trae org_id %q, se esperaba %q", label, got, org)
	}
	return tenant{Email: email, Token: sess.AccessToken, OrgID: org}, nil
}

func requireEnv(t *testing.T) {
	t.Helper()
	if skipReason != "" {
		t.Skip("sin entorno: " + skipReason)
	}
}

func requireBilling(t *testing.T) {
	t.Helper()
	requireEnv(t)
	if !billingUp {
		t.Skip("Billing no responde: " + os.Getenv("BILLING_API_URL"))
	}
}

// --- Supabase Auth ---

type session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func supabaseToken(base, grant string, body map[string]string) (session, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return session{}, err
	}
	// #nosec G704 -- la URL es SUPABASE_URL de la configuración de dev, no un dato de una petición.
	req, err := http.NewRequest(http.MethodPost, base+"/auth/v1/token?grant_type="+grant, bytes.NewReader(raw))
	if err != nil {
		return session{}, err
	}
	req.Header.Set("apikey", os.Getenv("SUPABASE_PUBLISHABLE_KEY"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req) // #nosec G704 -- ídem

	if err != nil {
		return session{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Sin el cuerpo: podría repetir el correo; el status basta para saber que fallaron las credenciales.
		return session{}, fmt.Errorf("Supabase Auth respondió %d", resp.StatusCode)
	}
	var s session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return session{}, err
	}
	if s.AccessToken == "" {
		return session{}, errors.New("Supabase Auth no devolvió access_token")
	}
	return s, nil
}

// orgClaim lee org_id del payload sin verificar la firma: solo sirve para comprobar el setup, la verificación
// de verdad la hace el gateway.
func orgClaim(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var c struct {
		OrgID string `json:"org_id"`
	}
	_ = json.Unmarshal(payload, &c)
	return c.OrgID
}

// --- cliente HTTP en proceso contra el router real ---

type response struct {
	Status int
	Body   map[string]any
}

func (r response) str(key string) string {
	s, _ := r.Body[key].(string)
	return s
}

// ids devuelve el campo `id` de cada elemento de `items` (la forma de todas las listas del contrato).
func (r response) ids(field string) []string {
	raw, _ := r.Body["items"].([]any)
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			if s, ok := m[field].(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func (r response) problemType() string { return r.str("type") }

func do(method, path, token string, body any, headers ...string) response {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		rdr = bytes.NewReader(raw)
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
	_ = json.Unmarshal(rec.Body.Bytes(), &out.Body)
	return out
}

func call(t *testing.T, method, path, token string, body any, headers ...string) response {
	t.Helper()
	return do(method, path, token, body, headers...)
}

func expect(t *testing.T, what string, want int, got response) {
	t.Helper()
	if got.Status != want {
		t.Fatalf("%s: status %d, se esperaba %d (%v)", what, got.Status, want, got.Body)
	}
}

// --- utilidades ---

func healthy(base string) bool {
	// #nosec G704 -- las URLs son las de las APIs de la configuración local de la suite.
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get(strings.TrimRight(base, "/") + "/healthz")
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func setDefault(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}

func randomDigits(n int) string {
	var sb strings.Builder
	for range n {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic(err)
		}
		sb.WriteString(d.String())
	}
	return sb.String()
}
