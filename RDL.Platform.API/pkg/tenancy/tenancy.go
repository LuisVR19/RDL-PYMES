// Package tenancy construye el TenantContext a partir de la identidad verificada y la membresía vigente.
// Es infraestructura reutilizable (candidata a building-blocks): no conoce HTTP de ningún handler concreto
// ni reglas de negocio más allá de "quién es el usuario y en qué organización actúa".
package tenancy

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
)

var (
	// ErrUnauthenticated: no hay token o no es válido (firma, exp, aud, iss).
	ErrUnauthenticated = errors.New("tenancy: identidad no verificada")
	// ErrNoActiveOrganization: el token no trae org_id (el usuario no eligió organización o el hook no la emitió).
	ErrNoActiveOrganization = errors.New("tenancy: sin organización activa")
	// ErrNoMembership: el token trae org_id pero la membresía ya no está activa (o la organización no lo está).
	ErrNoMembership = errors.New("tenancy: sin membresía activa en la organización")
)

// Identity es lo que sabemos del usuario tras verificar el JWT. OrganizationID viene del claim org_id que
// emite el hook de Auth: es una pista, no una autorización (ver RequireOrganization).
type Identity struct {
	Subject        string
	Email          string
	FullName       string
	OrganizationID uuid.UUID
}

func (i Identity) HasOrganization() bool { return i.OrganizationID != uuid.Nil }

// Context es el TenantContext: inmutable una vez construido.
type Context struct {
	userID         uuid.UUID
	subject        string
	organizationID uuid.UUID
	roles          []string
}

func NewContext(userID uuid.UUID, subject string, organizationID uuid.UUID, roles []string) Context {
	return Context{userID: userID, subject: subject, organizationID: organizationID, roles: slices.Clone(roles)}
}

func (c Context) UserID() uuid.UUID         { return c.userID }
func (c Context) Subject() string           { return c.subject }
func (c Context) OrganizationID() uuid.UUID { return c.organizationID }
func (c Context) Roles() []string           { return slices.Clone(c.roles) }

type identityKey struct{}
type tenantKey struct{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}

func WithTenant(ctx context.Context, t Context) context.Context {
	return context.WithValue(ctx, tenantKey{}, t)
}

// From devuelve el TenantContext del request. Los casos de uso que operan dentro de una organización
// deben fallar si no existe: nunca derivan la organización de otra fuente.
func From(ctx context.Context) (Context, bool) {
	t, ok := ctx.Value(tenantKey{}).(Context)
	return t, ok
}
