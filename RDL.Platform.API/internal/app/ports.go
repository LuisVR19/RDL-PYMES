// Package app contiene los casos de uso (un struct por caso) y los puertos que necesitan.
// Los puertos se definen aquí, del lado del consumidor; los adapters los implementan.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// TxManager es la unidad de trabajo: cada función corre en una transacción con la sesión RLS ya fijada,
// y todo lo que escribe (entidad, audit, outbox) se confirma o se descarta junto.
type TxManager interface {
	// WithinUserTx: operaciones del propio usuario, sin organización activa. userID puede ser uuid.Nil
	// antes de que el usuario exista en core.users.
	WithinUserTx(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context, tx Tx) error) error
	// WithinTenantTx: operaciones dentro de la organización activa del TenantContext.
	WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Tx expone los repositorios ligados a la transacción en curso.
type Tx interface {
	Users() UserRepository
	Memberships() MembershipRepository
	Members() MemberRepository
	Invitations() InvitationRepository
	Branches() BranchRepository
	Organizations() OrganizationRepository
	Idempotency() IdempotencyStore
	Audit() AuditRecorder
}

type UserRepository interface {
	FindBySubject(ctx context.Context, subject string) (user.User, error)
	// CreateIfAbsent inserta el usuario si no existe otro con el mismo sujeto y devuelve el registro vigente.
	// created indica si esta llamada lo creó (para auditar solo una vez ante llamadas concurrentes).
	CreateIfAbsent(ctx context.Context, u user.User) (stored user.User, created bool, err error)
	SetActiveOrganization(ctx context.Context, userID, organizationID uuid.UUID) error
}

type MembershipRepository interface {
	ListActiveForUser(ctx context.Context, userID uuid.UUID) ([]membership.Summary, error)
	// IsActiveMember: membresía activa en una organización activa. Lee bajo la sesión del propio usuario.
	IsActiveMember(ctx context.Context, userID, organizationID uuid.UUID) (bool, error)
	// Add crea la membresía activa y devuelve su id.
	Add(ctx context.Context, organizationID, userID uuid.UUID) (uuid.UUID, error)
	AssignRole(ctx context.Context, organizationID, membershipID uuid.UUID, role membership.Role, grantedBy uuid.UUID) error
}

// MemberRepository administra los miembros de la organización de la sesión (políticas *_tenant).
type MemberRepository interface {
	List(ctx context.Context, organizationID uuid.UUID, q MemberQuery) ([]membership.Member, error)
	// GetForUpdate bloquea la membresía del usuario; ErrNotFound si no es miembro de esta organización.
	GetForUpdate(ctx context.Context, organizationID, userID uuid.UUID) (membership.Member, error)
	CountActiveOwners(ctx context.Context, organizationID uuid.UUID) (int, error)
	// ReplaceRoles deja exactamente esos roles en la membresía (V1: uno).
	ReplaceRoles(ctx context.Context, organizationID, membershipID uuid.UUID, roles []membership.Role, grantedBy uuid.UUID) error
	SetStatus(ctx context.Context, organizationID, membershipID uuid.UUID, status membership.Status) error
	// FindByUser devuelve la membresía (cualquier estado) o ErrNotFound.
	FindByUser(ctx context.Context, organizationID, userID uuid.UUID) (membershipID uuid.UUID, status membership.Status, err error)
}

type InvitationRepository interface {
	// Create devuelve ErrConflict si ya hay una invitación pendiente para ese email en la organización.
	Create(ctx context.Context, inv invitation.Invitation, tokenHash string) (invitation.Invitation, error)
	List(ctx context.Context, organizationID uuid.UUID, q InvitationQuery) ([]invitation.Invitation, error)
	GetForUpdate(ctx context.Context, organizationID, id uuid.UUID) (invitation.Invitation, error)
	// FindByTokenHash busca bajo la sesión del invitado: solo ve invitaciones dirigidas a su email.
	FindByTokenHash(ctx context.Context, tokenHash string) (invitation.Invitation, error)
	GetByTokenHashForUpdate(ctx context.Context, organizationID uuid.UUID, tokenHash string) (invitation.Invitation, error)
	Revoke(ctx context.Context, organizationID, id uuid.UUID) error
	MarkAccepted(ctx context.Context, organizationID, id, userID uuid.UUID) error
	// ExpireStalePending marca como vencida la pendiente vencida de ese email, para poder reinvitar.
	ExpireStalePending(ctx context.Context, organizationID uuid.UUID, email string) error
	IsActiveMemberEmail(ctx context.Context, organizationID uuid.UUID, email string) (bool, error)
}

type BranchRepository interface {
	// Create devuelve ErrConflict si el código ya existe en la organización.
	Create(ctx context.Context, b branch.Branch) (branch.Branch, error)
	List(ctx context.Context, organizationID uuid.UUID, q BranchQuery) ([]branch.Branch, error)
	// Get y GetForUpdate devuelven ErrNotFound si la sucursal no existe o es de otra organización.
	Get(ctx context.Context, organizationID, id uuid.UUID) (branch.Branch, error)
	GetForUpdate(ctx context.Context, organizationID, id uuid.UUID) (branch.Branch, error)
	Update(ctx context.Context, b branch.Branch) (branch.Branch, error)
}

type BranchQuery struct {
	Active *bool // nil = todas
	After  *PageCursor
	Limit  int
}

type InvitationQuery struct {
	Status invitation.Status // vacío = todas
	After  *PageCursor
	Limit  int
}

// MemberQuery pide una página de miembros. After es el último elemento de la página anterior.
type MemberQuery struct {
	Status membership.Status // vacío = todos
	After  *PageCursor
	Limit  int
}

// PageCursor es la posición del último elemento de una página: (instante de orden, id) para desempatar.
type PageCursor struct {
	At time.Time
	ID uuid.UUID
}

// MembershipCache es la caché de revalidación de membresías del middleware de tenancy. Al cambiar un rol o
// suspender a alguien se invalida su entrada para que el cambio rija de inmediato en este nodo
// (los demás nodos lo verán en ≤ AUTH_MEMBERSHIP_CACHE_TTL).
type MembershipCache interface {
	Invalidate(subject string, organizationID uuid.UUID)
}

type OrganizationRepository interface {
	// Create devuelve ErrConflict si la identificación ya pertenece a otra organización.
	Create(ctx context.Context, o organization.Organization) (organization.Organization, error)
	// Get lee la organización de la sesión; ErrNotFound si RLS no la deja ver.
	Get(ctx context.Context, id uuid.UUID) (organization.Organization, error)
	// GetForUpdate bloquea la fila hasta el fin de la transacción (actualizaciones concurrentes).
	GetForUpdate(ctx context.Context, id uuid.UUID) (organization.Organization, error)
	Update(ctx context.Context, o organization.Organization) (organization.Organization, error)
}

// IdempotencyStore persiste las claves Idempotency-Key en integration.idempotency_keys, en la misma
// transacción que el comando: si el comando falla, la reserva desaparece con él.
type IdempotencyStore interface {
	// Claim reserva la clave. Si ya existía (y no venció) devuelve el registro previo en lugar de reservar.
	// Una petición concurrente con la misma clave espera a que la primera confirme y luego ve su registro.
	Claim(ctx context.Context, organizationID uuid.UUID, key, requestHash string, ttl time.Duration) (*IdempotencyRecord, error)
	Complete(ctx context.Context, organizationID uuid.UUID, key string, result IdempotencyRecord) error
}

// IdempotencyRecord guarda la referencia al resultado, no la respuesta HTTP: al repetir, el caso de uso
// relee el recurso y responde con su estado actual.
type IdempotencyRecord struct {
	RequestHash string
	Status      int
	Result      map[string]string
}

// AuditRecorder escribe en audit.audit_events dentro de la transacción del cambio. El correlationId, la IP y
// el user-agent los toma el adapter del contexto del request.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEvent) error
}

type AuditEvent struct {
	OrganizationID uuid.UUID // uuid.Nil para eventos sin organización (alta de usuario)
	ActorUserID    uuid.UUID
	Action         string
	EntityType     string
	EntityID       uuid.UUID
	Before         any
	After          any
	Reason         string
}
