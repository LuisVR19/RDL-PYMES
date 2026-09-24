// Package config carga y valida la configuración desde variables de entorno.
// Falla al arrancar si falta algo: preferimos no levantar a levantar mal configurados.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	ServiceName string
	HTTP        HTTP
	DB          DB
	Auth        Auth
	Log         Log
	OTel        OTel
}

type HTTP struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type DB struct {
	URL string
	// Password, si viene, reemplaza la de la URL: evita tener que codificar caracteres especiales en DATABASE_URL.
	Password         string
	MaxConns         int32
	StatementTimeout time.Duration
}

type Auth struct {
	Issuer             string
	JWKSURL            string
	Audience           string
	JWKSRefresh        time.Duration
	MembershipCacheTTL time.Duration
}

type Log struct {
	Level string
}

type OTel struct {
	// Endpoint vacío desactiva la exportación; trazas y métricas quedan en no-op.
	Endpoint string
}

// Load lee la configuración de las variables de entorno y la valida.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	r := reader{getenv: getenv}

	supabaseURL := strings.TrimRight(r.required("SUPABASE_URL"), "/")
	issuer := r.optional("AUTH_ISSUER", supabaseURL+"/auth/v1")

	cfg := Config{
		Env:         r.optional("APP_ENV", "dev"),
		ServiceName: r.optional("OTEL_SERVICE_NAME", "platform-api"),
		HTTP: HTTP{
			Addr:              r.optional("HTTP_ADDR", ":8080"),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   r.duration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
		},
		DB: DB{
			URL:              r.optional("DATABASE_URL", ""),
			Password:         r.getenv("DB_PASSWORD"),
			MaxConns:         r.int32InRange("DB_MAX_CONNS", 10, 1, 200),
			StatementTimeout: r.duration("DB_STATEMENT_TIMEOUT", 5*time.Second),
		},
		Auth: Auth{
			Issuer:             issuer,
			JWKSURL:            r.optional("AUTH_JWKS_URL", issuer+"/.well-known/jwks.json"),
			Audience:           r.optional("AUTH_AUDIENCE", "authenticated"),
			JWKSRefresh:        r.duration("AUTH_JWKS_REFRESH", 15*time.Minute),
			MembershipCacheTTL: r.duration("AUTH_MEMBERSHIP_CACHE_TTL", 30*time.Second),
		},
		Log:  Log{Level: r.optional("LOG_LEVEL", "info")},
		OTel: OTel{Endpoint: r.optional("OTEL_EXPORTER_OTLP_ENDPOINT", "")},
	}

	r.validURL("SUPABASE_URL", supabaseURL)
	// Lo normal es no definir DATABASE_URL: se arma con el host de Supavisor y el login platform_api.
	if cfg.DB.URL == "" && supabaseURL != "" {
		u, err := PoolerURL(supabaseURL, getenv("DB_POOLER_HOST"), "platform_api", PoolerTransactionPort)
		if err != nil {
			r.errs = append(r.errs, err)
		}
		cfg.DB.URL = u
	}
	if supabaseURL != "" || getenv("AUTH_JWKS_URL") != "" {
		r.validURL("AUTH_JWKS_URL", cfg.Auth.JWKSURL)
	}
	if cfg.Auth.MembershipCacheTTL <= 0 || cfg.Auth.MembershipCacheTTL > 60*time.Second {
		r.fail("AUTH_MEMBERSHIP_CACHE_TTL debe estar entre 1s y 60s: una membresía revocada no puede seguir válida más de un minuto")
	}
	switch cfg.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		r.fail("LOG_LEVEL debe ser debug, info, warn o error")
	}

	if len(r.errs) > 0 {
		return Config{}, fmt.Errorf("configuración inválida: %w", errors.Join(r.errs...))
	}
	return cfg, nil
}

type reader struct {
	getenv func(string) string
	errs   []error
}

func (r *reader) fail(msg string) { r.errs = append(r.errs, errors.New(msg)) }

func (r *reader) required(key string) string {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		r.fail(key + " es obligatoria")
	}
	return v
}

func (r *reader) optional(key, def string) string {
	if v := strings.TrimSpace(r.getenv(key)); v != "" {
		return v
	}
	return def
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key + " no es una duración válida (ej. 30s)")
		return def
	}
	return d
}

func (r *reader) int32InRange(key string, def, lo, hi int32) int32 {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || int32(n) < lo || int32(n) > hi {
		r.fail(fmt.Sprintf("%s debe ser un entero entre %d y %d", key, lo, hi))
		return def
	}
	return int32(n)
}

func (r *reader) validURL(key, v string) {
	if v == "" {
		return
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "https" && u.Scheme != "http" || u.Host == "" {
		r.fail(key + " no es una URL http(s) válida")
	}
}
