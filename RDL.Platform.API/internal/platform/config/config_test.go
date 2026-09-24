package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDerivesAuthFromSupabaseURL(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"SUPABASE_URL": "https://abc.supabase.co/",
		"DATABASE_URL": "postgres://localhost:6543/postgres",
	}))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Auth.Issuer != "https://abc.supabase.co/auth/v1" {
		t.Errorf("issuer = %q", cfg.Auth.Issuer)
	}
	if cfg.Auth.JWKSURL != "https://abc.supabase.co/auth/v1/.well-known/jwks.json" {
		t.Errorf("jwks = %q", cfg.Auth.JWKSURL)
	}
	if cfg.Auth.Audience != "authenticated" {
		t.Errorf("audience = %q", cfg.Auth.Audience)
	}
	if cfg.Auth.MembershipCacheTTL != 30*time.Second {
		t.Errorf("ttl = %v", cfg.Auth.MembershipCacheTTL)
	}
}

func TestLoadReportsAllMissingVariables(t *testing.T) {
	_, err := load(env(map[string]string{}))
	if err == nil {
		t.Fatal("se esperaba error")
	}
	for _, key := range []string{"SUPABASE_URL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("el error no menciona %s: %v", key, err)
		}
	}
}

func TestLoadRejectsMembershipCacheAboveOneMinute(t *testing.T) {
	_, err := load(env(map[string]string{
		"SUPABASE_URL":              "https://abc.supabase.co",
		"DATABASE_URL":              "postgres://x",
		"AUTH_MEMBERSHIP_CACHE_TTL": "5m",
	}))
	if err == nil || !strings.Contains(err.Error(), "AUTH_MEMBERSHIP_CACHE_TTL") {
		t.Fatalf("se esperaba error de TTL, got %v", err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	_, err := load(env(map[string]string{
		"SUPABASE_URL":      "no-es-url",
		"DATABASE_URL":      "postgres://x",
		"LOG_LEVEL":         "verbose",
		"DB_MAX_CONNS":      "diez",
		"HTTP_READ_TIMEOUT": "rápido",
	}))
	if err == nil {
		t.Fatal("se esperaba error")
	}
	for _, key := range []string{"SUPABASE_URL", "LOG_LEVEL", "DB_MAX_CONNS", "HTTP_READ_TIMEOUT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("el error no menciona %s: %v", key, err)
		}
	}
}
