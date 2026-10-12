// Package config carga y valida la configuración desde variables de entorno.
// Falla al arrancar si falta algo: preferimos no levantar a levantar mal configurados.
// Origen: RDL.Receivables.API (que a su vez copió de Platform); se agregan Worker y Fiscal.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
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
	Consumer    Consumer
	Worker      Worker
	Fiscal      Fiscal
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

// Consumer es la configuración de cmd/consumer (y de cmd/replay, que usa el mismo procesador).
type Consumer struct {
	// HTTPAddr sirve /healthz y /readyz del consumidor (no expone la API).
	HTTPAddr string
	// MessageTimeout es el tiempo máximo de un mensaje con todos sus reintentos (6 intentos, hasta ~63 s de espera).
	MessageTimeout time.Duration
}

// Worker es la configuración de cmd/worker (firma, envío y consulta a Hacienda).
type Worker struct {
	// HTTPAddr sirve /healthz y /readyz del worker.
	HTTPAddr string
}

// Fiscal reúne lo que depende de Hacienda y de la plataforma como proveedor de sistemas.
type Fiscal struct {
	// HaciendaEnvironment es el ambiente de Hacienda permitido a este proceso. Solo "sandbox" en V1: el prompt P5
	// prohíbe conectarse a producción, y así es imposible de confundir aunque un perfil diga otra cosa.
	HaciendaEnvironment string
	// ProviderIdentification es la identificación de RDL como proveedor de sistemas (ProveedorSistemas, Anexo 1
	// pág. 18). Vacía hasta que exista: sin ella el worker no envía a Hacienda (decisión del 2026-10-12).
	ProviderIdentification string
	// RetentionYears es el plazo de conservación de XML y respuestas (decisión 7). TODO(legal): confirmar el plazo.
	RetentionYears int32
	// SecretsDir y SecretsMasterKey son el SecretStore de desarrollo (decisión 6): archivos cifrados con AES-256-GCM.
	// Sin llave no hay SecretStore: las rutas de credenciales y certificado responden 503. TODO(P2): gestor real.
	SecretsDir       string
	SecretsMasterKey string
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
		ServiceName: r.optional("OTEL_SERVICE_NAME", "einvoice-api"),
		HTTP: HTTP{
			Addr:              r.optional("HTTP_ADDR", ":8082"),
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
		Consumer: Consumer{
			HTTPAddr:       r.optional("CONSUMER_HTTP_ADDR", ":8085"),
			MessageTimeout: r.duration("CONSUMER_MESSAGE_TIMEOUT", 2*time.Minute),
		},
		Worker: Worker{HTTPAddr: r.optional("WORKER_HTTP_ADDR", ":8086")},
		Fiscal: Fiscal{
			HaciendaEnvironment:    r.optional("HACIENDA_ENVIRONMENT", "sandbox"),
			ProviderIdentification: r.optional("PROVIDER_IDENTIFICATION", ""),
			RetentionYears:         r.int32InRange("RETENTION_YEARS", 5, 1, 50),
			SecretsDir:             r.optional("SECRETS_DIR", ".secrets"),
			SecretsMasterKey:       strings.TrimSpace(r.getenv("SECRETS_MASTER_KEY")),
		},
	}

	r.validURL("SUPABASE_URL", supabaseURL)
	// Lo normal es no definir DATABASE_URL: se arma con el host de Supavisor y el login fiscal_api.
	if cfg.DB.URL == "" && supabaseURL != "" {
		u, err := PoolerURL(supabaseURL, getenv("DB_POOLER_HOST"), "fiscal_api", PoolerTransactionPort)
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
	if cfg.Consumer.MessageTimeout < 30*time.Second {
		r.fail("CONSUMER_MESSAGE_TIMEOUT debe ser de al menos 30s: los reintentos esperan hasta ~63s en total")
	}
	if cfg.Fiscal.HaciendaEnvironment != "sandbox" {
		r.fail("HACIENDA_ENVIRONMENT solo admite sandbox: el ambiente de producción de Hacienda está prohibido en V1")
	}
	if id := cfg.Fiscal.ProviderIdentification; id != "" && !providerID.MatchString(id) {
		r.fail("PROVIDER_IDENTIFICATION debe tener de 9 a 20 letras o dígitos, sin guiones (ProveedorSistemas, String 20)")
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

// providerID admite letras porque la cédula jurídica es alfanumérica desde la revisión de 2026 (informe 0001).
var providerID = regexp.MustCompile(`^[0-9A-Za-z]{9,20}$`)

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
