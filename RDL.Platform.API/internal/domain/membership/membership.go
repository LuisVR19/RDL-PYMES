// Package membership modela la pertenencia de un usuario a una organización y sus roles.
package membership

import (
	"time"

	"github.com/google/uuid"
)

// Role es un código del catálogo core.roles.
type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleBiller     Role = "biller"
	RoleCollector  Role = "collector"
	RoleAccountant Role = "accountant"
	RoleReadOnly   Role = "read_only"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
)

// Summary es la vista de una membresía propia: lo que el usuario necesita para elegir organización activa.
type Summary struct {
	OrganizationID     uuid.UUID
	LegalName          string
	TradeName          string
	OrganizationStatus string
	Status             Status
	Roles              []Role
	JoinedAt           time.Time
}
