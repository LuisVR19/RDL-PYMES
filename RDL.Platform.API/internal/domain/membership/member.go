package membership

import (
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrLastOwner: el cambio dejaría a la organización sin ningún owner activo.
	ErrLastOwner = errors.New("membership: la organización debe conservar al menos un owner activo")
	// ErrOwnerRequired: solo un owner puede modificar a otro owner o asignar el rol owner (evita que un admin escale).
	ErrOwnerRequired = errors.New("membership: solo un owner puede gestionar owners")
	ErrInvalidRole   = errors.New("membership: rol inválido")
	ErrInvalidStatus = errors.New("membership: estado inválido")
	ErrEmptyChange   = errors.New("membership: no se indicó ningún cambio")
)

// Member es un miembro visto desde la administración de la organización.
type Member struct {
	MembershipID uuid.UUID
	UserID       uuid.UUID
	Subject      string
	Email        string
	FullName     string
	Status       Status
	Roles        []Role
	JoinedAt     time.Time
}

func (m Member) IsActiveOwner() bool {
	return m.Status == StatusActive && slices.Contains(m.Roles, RoleOwner)
}

func (s Status) Valid() bool { return s == StatusActive || s == StatusSuspended }

// Change describe lo que se quiere modificar. nil = no cambia.
type Change struct {
	Role   *Role
	Status *Status
}

// PlanChange aplica las reglas de negocio y devuelve el miembro resultante.
// V1: un solo rol por membresía, así que asignar un rol reemplaza los anteriores.
// activeOwners es el número actual de owners activos en la organización (incluido el target si lo es).
func PlanChange(actorRoles []Role, target Member, c Change, activeOwners int) (Member, error) {
	if c.Role == nil && c.Status == nil {
		return Member{}, ErrEmptyChange
	}
	if c.Role != nil && !c.Role.Valid() {
		return Member{}, ErrInvalidRole
	}
	if c.Status != nil && !c.Status.Valid() {
		return Member{}, ErrInvalidStatus
	}

	next := target
	next.Roles = slices.Clone(target.Roles)
	if c.Role != nil {
		next.Roles = []Role{*c.Role}
	}
	if c.Status != nil {
		next.Status = *c.Status
	}

	touchesOwner := slices.Contains(target.Roles, RoleOwner) || slices.Contains(next.Roles, RoleOwner)
	if touchesOwner && !slices.Contains(actorRoles, RoleOwner) && !sameMember(target, next) {
		return Member{}, ErrOwnerRequired
	}
	if target.IsActiveOwner() && !next.IsActiveOwner() && activeOwners <= 1 {
		return Member{}, ErrLastOwner
	}
	return next, nil
}

func sameMember(a, b Member) bool {
	return a.Status == b.Status && slices.Equal(a.Roles, b.Roles)
}

// Unchanged indica si el plan no modifica nada (para no escribir ni auditar en vano).
func Unchanged(before, after Member) bool { return sameMember(before, after) }
