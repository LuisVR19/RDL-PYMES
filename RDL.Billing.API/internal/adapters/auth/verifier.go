// Package auth verifica los JWT emitidos por Supabase Auth contra su JWKS.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"rdl/billing-api/pkg/tenancy"
)

// Solo algoritmos asimétricos: un token HS256 (secreto legado compartido) nunca se acepta aunque alguien lo firme.
var allowedAlgs = []string{"ES256", "RS256", "EdDSA"}

// leeway tolera desfases de reloj entre Supabase y este servicio.
const leeway = 30 * time.Second

type Verifier struct {
	keyfunc  keyfunc.Keyfunc
	issuer   string
	audience string
}

// NewVerifier arranca la caché de JWKS con refresco periódico. ctx controla la vida de la goroutine de refresco.
// Si el JWKS no responde al arrancar, no falla: /readyz lo reporta y se reintenta en el refresco.
func NewVerifier(ctx context.Context, log *slog.Logger, jwksURL, issuer, audience string, refresh time.Duration) (*Verifier, error) {
	kf, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{jwksURL}, keyfunc.Override{
		Client:          &http.Client{Timeout: 5 * time.Second},
		HTTPTimeout:     5 * time.Second,
		RefreshInterval: refresh,
		RefreshErrorHandlerFunc: func(u string) func(context.Context, error) {
			return func(ctx context.Context, err error) {
				log.WarnContext(ctx, "no se pudo refrescar el JWKS", slog.String("url", u), slog.Any("error", err))
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("inicializando JWKS: %w", err)
	}
	return newVerifier(kf, issuer, audience), nil
}

func newVerifier(kf keyfunc.Keyfunc, issuer, audience string) *Verifier {
	return &Verifier{keyfunc: kf, issuer: issuer, audience: audience}
}

// supabaseClaims recoge solo lo que usamos del access token de Supabase.
type supabaseClaims struct {
	jwt.RegisteredClaims
	Email        string         `json:"email"`
	IsAnonymous  bool           `json:"is_anonymous"`
	OrgID        string         `json:"org_id"`
	UserMetadata map[string]any `json:"user_metadata"`
}

var ErrInvalidToken = errors.New("token inválido")

func (v *Verifier) Verify(ctx context.Context, raw string) (tenancy.Identity, error) {
	var c supabaseClaims
	_, err := jwt.ParseWithClaims(raw, &c, v.keyfunc.KeyfuncCtx(ctx),
		jwt.WithValidMethods(allowedAlgs),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(leeway),
	)
	if err != nil {
		return tenancy.Identity{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	if strings.TrimSpace(c.Subject) == "" {
		return tenancy.Identity{}, fmt.Errorf("%w: sin sub", ErrInvalidToken)
	}
	// Las sesiones anónimas de Supabase tienen sub pero no representan a una persona: no pueden operar una organización.
	if c.IsAnonymous {
		return tenancy.Identity{}, fmt.Errorf("%w: sesión anónima", ErrInvalidToken)
	}

	id := tenancy.Identity{
		Subject:  c.Subject,
		Email:    strings.TrimSpace(c.Email),
		FullName: fullName(c.UserMetadata),
	}
	// Un org_id malformado se trata como ausente: nunca como error de autenticación ni como organización válida.
	if org, err := uuid.Parse(c.OrgID); err == nil && org != uuid.Nil {
		id.OrganizationID = org
	}
	return id, nil
}

func fullName(meta map[string]any) string {
	for _, k := range []string{"full_name", "name"} {
		if s, ok := meta[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
