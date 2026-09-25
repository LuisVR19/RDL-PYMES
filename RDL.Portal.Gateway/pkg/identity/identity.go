// Package identity lleva en el contexto al usuario verificado y su token original.
//
// A diferencia de `pkg/tenancy` de Platform y Billing, aquí NO hay TenantContext: el Portal Gateway no tiene
// base de datos, no revalida la membresía y no decide la organización. Guarda la identidad solo para registrar
// quién pidió qué, y el token tal como llegó para reenviarlo byte por byte a la API dueña, que es quien
// revalida la membresía contra `core`.
package identity

import (
	"context"

	"github.com/google/uuid"
)

// Identity es lo verificado del JWT. OrganizationID viene del claim org_id: aquí es solo un dato para el log,
// nunca una autorización y nunca un parámetro que se le agregue a una llamada.
type Identity struct {
	Subject        string
	Email          string
	FullName       string
	OrganizationID uuid.UUID

	// RawToken es el access token tal como llegó en el Authorization. Se reenvía sin modificar.
	RawToken string
}

type ctxKey struct{}

func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func From(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// Token devuelve el token del usuario para los clientes de salida. Sin identidad no hay token: los clientes
// fallan en lugar de improvisar credenciales propias.
func Token(ctx context.Context) (string, bool) {
	id, ok := From(ctx)
	if !ok || id.RawToken == "" {
		return "", false
	}
	return id.RawToken, true
}
