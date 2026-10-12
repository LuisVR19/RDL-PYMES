package tenancy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// TokenVerifier valida un JWT (firma, exp, aud, iss) y devuelve la identidad.
type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (Identity, error)
}

// ErrorWriter traduce los errores de tenancy a la respuesta HTTP (Problem Details en este servicio).
type ErrorWriter func(w http.ResponseWriter, r *http.Request, err error)

// Authenticate exige un Bearer token válido y deja la Identity en el contexto.
func Authenticate(v TokenVerifier, fail ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				fail(w, r, fmt.Errorf("%w: falta el header Authorization Bearer", ErrUnauthenticated))
				return
			}
			id, err := v.Verify(r.Context(), raw)
			if err != nil {
				fail(w, r, fmt.Errorf("%w: %w", ErrUnauthenticated, err))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// RequireOrganization construye el TenantContext. La organización sale SOLO del org_id del token verificado,
// y además se revalida la membresía en la base: suspender a un miembro surte efecto sin esperar a que expire el JWT.
// Nada del body, la query, los headers ni la ruta participa en esta decisión.
func RequireOrganization(res MembershipResolver, fail ErrorWriter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFrom(r.Context())
			if !ok {
				fail(w, r, ErrUnauthenticated)
				return
			}
			if !id.HasOrganization() {
				fail(w, r, ErrNoActiveOrganization)
				return
			}
			m, err := res.ActiveMembership(r.Context(), id.Subject, id.OrganizationID)
			if err != nil {
				fail(w, r, err)
				return
			}
			t := NewContext(m.UserID, id.Subject, id.OrganizationID, m.Roles)
			next.ServeHTTP(w, r.WithContext(WithTenant(r.Context(), t)))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(h, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

// IsAuthError indica si err es un error de tenancy conocido (para mapearlo sin filtrar detalles internos).
func IsAuthError(err error) bool {
	return errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrNoActiveOrganization) || errors.Is(err, ErrNoMembership)
}
