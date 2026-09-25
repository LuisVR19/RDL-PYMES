package config

import (
	"strings"
	"testing"
	"time"

	"rdl/portal-gateway/internal/domain/routes"
)

// env mínimo y válido. Las pruebas lo modifican para probar una regla a la vez.
func baseEnv() map[string]string {
	return map[string]string{
		"SUPABASE_URL":     "https://dzlsnsstuqpxvwegeqcy.supabase.co",
		"PLATFORM_API_URL": "http://localhost:8080",
		"BILLING_API_URL":  "http://localhost:8081",
	}
}

func loadWith(env map[string]string) (Config, error) {
	return load(func(k string) string { return env[k] })
}

func mustLoad(t *testing.T, env map[string]string) Config {
	t.Helper()
	cfg, err := loadWith(env)
	if err != nil {
		t.Fatalf("configuración válida rechazada: %v", err)
	}
	return cfg
}

func wantError(t *testing.T, env map[string]string, fragment string) {
	t.Helper()
	_, err := loadWith(env)
	if err == nil {
		t.Fatalf("se esperaba un error que mencionara %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Errorf("error=%v, se esperaba que mencionara %q", err, fragment)
	}
}

// Condición del incremento: E-Invoice y Receivables son opcionales mientras sus repos no existan.
func TestFiscalAndReceivablesAreOptional(t *testing.T) {
	cfg := mustLoad(t, baseEnv())

	for _, s := range []routes.Service{routes.Platform, routes.Billing} {
		if !cfg.Upstreams.Get(s).Configured() {
			t.Errorf("%s debería estar configurada", s)
		}
	}
	for _, s := range []routes.Service{routes.Fiscal, routes.Receivables} {
		if cfg.Upstreams.Get(s).Configured() {
			t.Errorf("%s no estaba en el entorno y quedó configurada", s)
		}
	}
}

// Platform y Billing sí son obligatorias: sin ellas el portal no abre sesión ni factura.
func TestPlatformAndBillingAreRequired(t *testing.T) {
	for _, key := range []string{"PLATFORM_API_URL", "BILLING_API_URL"} {
		env := baseEnv()
		delete(env, key)
		wantError(t, env, key+" es obligatoria")
	}
}

func TestUpstreamTimeoutsDefaultAndOverride(t *testing.T) {
	env := baseEnv()
	env["UPSTREAM_TIMEOUT"] = "3s"
	env["BILLING_API_TIMEOUT"] = "8s"
	env["UPSTREAM_BUDGET"] = "20s"

	cfg := mustLoad(t, env)
	if got := cfg.Upstreams.Get(routes.Platform).Timeout; got != 3*time.Second {
		t.Errorf("platform timeout=%v, want 3s", got)
	}
	if got := cfg.Upstreams.Get(routes.Billing).Timeout; got != 8*time.Second {
		t.Errorf("billing timeout=%v, want 8s", got)
	}
}

// Si el presupuesto total no supera al timeout de una API, una composición se corta antes de poder degradar.
func TestBudgetMustExceedEveryUpstreamTimeout(t *testing.T) {
	env := baseEnv()
	env["UPSTREAM_TIMEOUT"] = "10s"
	env["UPSTREAM_BUDGET"] = "5s"
	wantError(t, env, "UPSTREAM_BUDGET")
}

func TestCORSRejectsWildcard(t *testing.T) {
	env := baseEnv()
	env["CORS_ALLOWED_ORIGINS"] = "*"
	wantError(t, env, "no admite *")
}

func TestCORSDefaultsToThePortalInDev(t *testing.T) {
	cfg := mustLoad(t, baseEnv())
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("origins=%v", cfg.CORS.AllowedOrigins)
	}
}

func TestBillingSummarySourceIsValidated(t *testing.T) {
	env := baseEnv()
	env["BILLING_SUMMARY_SOURCE"] = "inventado"
	wantError(t, env, "BILLING_SUMMARY_SOURCE")

	// Por ahora, el detalle público: Billing todavía no expone la ruta interna del contrato.
	if got := mustLoad(t, baseEnv()).Upstreams.BillingSummarySource; got != SummarySourceInternal {
		t.Errorf("source=%q, want %q", got, SummarySourceInternal)
	}
}

func TestDefaultPortIs8090(t *testing.T) {
	// Platform :8080, Billing :8081, fiscal :8082, Receivables :8083, gateway :8090.
	if got := mustLoad(t, baseEnv()).HTTP.Addr; got != ":8090" {
		t.Errorf("addr=%q", got)
	}
}

// Los errores de configuración se reportan todos juntos: corregir el .env de una sola vez.
func TestAllConfigErrorsAreReportedTogether(t *testing.T) {
	env := map[string]string{"LOG_LEVEL": "gritando"}
	_, err := loadWith(env)
	if err == nil {
		t.Fatal("se esperaba un error")
	}
	for _, fragment := range []string{"SUPABASE_URL", "PLATFORM_API_URL", "BILLING_API_URL", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Errorf("el error no menciona %s: %v", fragment, err)
		}
	}
}

func TestJWKSURLDerivesFromSupabaseURL(t *testing.T) {
	cfg := mustLoad(t, baseEnv())
	want := "https://dzlsnsstuqpxvwegeqcy.supabase.co/auth/v1/.well-known/jwks.json"
	if cfg.Auth.JWKSURL != want {
		t.Errorf("jwks=%q, want %q", cfg.Auth.JWKSURL, want)
	}
}
