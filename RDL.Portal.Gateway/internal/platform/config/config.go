// Package config carga y valida la configuración desde variables de entorno.
// Falla al arrancar si falta algo: preferimos no levantar a levantar mal configurados.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"rdl/portal-gateway/internal/domain/routes"
)

type Config struct {
	Env         string
	ServiceName string
	HTTP        HTTP
	Auth        Auth
	Upstreams   Upstreams
	CORS        CORS
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
	// MaxRequestBody acota el cuerpo que se acepta y se reenvía (igual que Platform).
	MaxRequestBody int64
}

type Auth struct {
	Issuer      string
	JWKSURL     string
	Audience    string
	JWKSRefresh time.Duration
}

// Upstream es una API destino. URL vacía = todavía no desplegada en este ambiente.
type Upstream struct {
	URL     string
	Timeout time.Duration
}

func (u Upstream) Configured() bool { return u.URL != "" }

type Upstreams struct {
	Services map[routes.Service]Upstream
	// Budget es el presupuesto total de una petición, incluidas las composiciones. Siempre mayor que el
	// timeout de una sola API, si no una composición nunca alcanzaría a degradar.
	Budget time.Duration
	// BillingSummarySource: de dónde sale el resumen de factura de la vista transversal.
	//   "internal" → GET /internal/v1/invoices/{id}/summary (la ruta del contrato, lo definitivo);
	//   "public"   → GET /v1/invoices/{id} (temporal, mientras Billing no implemente la interna).
	BillingSummarySource string
}

func (u Upstreams) Get(s routes.Service) Upstream { return u.Services[s] }

type CORS struct {
	AllowedOrigins []string
}

type Log struct {
	Level string
}

type OTel struct {
	// Endpoint vacío desactiva la exportación; trazas y métricas quedan en no-op.
	Endpoint string
}

const (
	// SummarySourceInternal es la ruta de `openapi/bff-internal.yaml` del repo de contratos.
	SummarySourceInternal = "internal"
	// SummarySourcePublic deriva el resumen del detalle público de la factura. Queda como respaldo para apuntar
	// el gateway a una Billing anterior a la ruta interna; pesa más, porque trae las líneas.
	SummarySourcePublic = "public"
)

// Load lee la configuración de las variables de entorno y la valida.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	r := reader{getenv: getenv}

	supabaseURL := strings.TrimRight(r.required("SUPABASE_URL"), "/")
	issuer := r.optional("AUTH_ISSUER", supabaseURL+"/auth/v1")
	defaultTimeout := r.duration("UPSTREAM_TIMEOUT", 5*time.Second)

	cfg := Config{
		Env:         r.optional("APP_ENV", "dev"),
		ServiceName: r.optional("OTEL_SERVICE_NAME", "portal-gateway"),
		HTTP: HTTP{
			Addr:              r.optional("HTTP_ADDR", ":8090"),
			ReadHeaderTimeout: r.duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			ReadTimeout:       r.duration("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:      r.duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       r.duration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   r.duration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			MaxRequestBody:    int64(r.intInRange("HTTP_MAX_REQUEST_BODY", 1<<20, 1024, 32<<20)),
		},
		Auth: Auth{
			Issuer:      issuer,
			JWKSURL:     r.optional("AUTH_JWKS_URL", issuer+"/.well-known/jwks.json"),
			Audience:    r.optional("AUTH_AUDIENCE", "authenticated"),
			JWKSRefresh: r.duration("AUTH_JWKS_REFRESH", 15*time.Minute),
		},
		Upstreams: Upstreams{
			Services:             map[routes.Service]Upstream{},
			Budget:               r.duration("UPSTREAM_BUDGET", 10*time.Second),
			BillingSummarySource: r.optional("BILLING_SUMMARY_SOURCE", SummarySourceInternal),
		},
		CORS: CORS{AllowedOrigins: splitList(r.optional("CORS_ALLOWED_ORIGINS", "http://localhost:5173"))},
		Log:  Log{Level: r.optional("LOG_LEVEL", "info")},
		OTel: OTel{Endpoint: r.optional("OTEL_EXPORTER_OTLP_ENDPOINT", "")},
	}

	// Platform y Billing son obligatorias: sin ellas el portal no puede ni abrir sesión ni facturar.
	// E-Invoice y Receivables son opcionales mientras sus repos no existan: su ausencia degrada, no tumba
	// el arranque ni el readiness (ADR 0002).
	for _, s := range routes.Services() {
		key := upstreamEnvPrefix(s)
		u := Upstream{
			URL:     strings.TrimRight(r.optional(key+"_API_URL", ""), "/"),
			Timeout: r.duration(key+"_API_TIMEOUT", defaultTimeout),
		}
		if u.URL == "" && (s == routes.Platform || s == routes.Billing) {
			r.fail(key + "_API_URL es obligatoria")
		}
		if u.URL != "" {
			r.validURL(key+"_API_URL", u.URL)
		}
		if u.Timeout <= 0 {
			r.fail(key + "_API_TIMEOUT debe ser mayor que cero: ninguna llamada interna puede quedarse sin timeout")
		}
		cfg.Upstreams.Services[s] = u
	}

	r.validURL("SUPABASE_URL", supabaseURL)
	if supabaseURL != "" || getenv("AUTH_JWKS_URL") != "" {
		r.validURL("AUTH_JWKS_URL", cfg.Auth.JWKSURL)
	}
	if cfg.Upstreams.Budget <= longestUpstreamTimeout(cfg.Upstreams) {
		r.fail("UPSTREAM_BUDGET tiene que ser mayor que el timeout de cada API: si no, una composición se corta antes de poder degradar")
	}
	switch cfg.Upstreams.BillingSummarySource {
	case SummarySourceInternal, SummarySourcePublic:
	default:
		r.fail("BILLING_SUMMARY_SOURCE debe ser " + SummarySourceInternal + " o " + SummarySourcePublic)
	}
	if len(cfg.CORS.AllowedOrigins) == 0 {
		r.fail("CORS_ALLOWED_ORIGINS no puede quedar vacío: el portal no podría llamar al gateway")
	}
	for _, o := range cfg.CORS.AllowedOrigins {
		if o == "*" {
			r.fail("CORS_ALLOWED_ORIGINS no admite *: el token viaja en un header y el origen se declara")
		}
		r.validURL("CORS_ALLOWED_ORIGINS", o)
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

// upstreamEnvPrefix: platform → PLATFORM, receivables → RECEIVABLES.
func upstreamEnvPrefix(s routes.Service) string { return strings.ToUpper(string(s)) }

func longestUpstreamTimeout(u Upstreams) time.Duration {
	var max time.Duration
	for _, up := range u.Services {
		if up.Timeout > max {
			max = up.Timeout
		}
	}
	return max
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
