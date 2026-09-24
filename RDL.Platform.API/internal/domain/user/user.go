// Package user modela al usuario global de la plataforma (core.users), enlazado al proveedor de identidad.
package user

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IdentityProvider es el único proveedor en V1 (docs/decisiones/0001 §4, punto 4).
const IdentityProvider = "supabase"

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

var (
	ErrEmailRequired = errors.New("user: el proveedor de identidad no entregó un email válido")
	ErrDisabled      = errors.New("user: usuario deshabilitado")
)

type User struct {
	ID                   uuid.UUID
	Subject              string
	Email                string
	FullName             string
	Status               Status
	ActiveOrganizationID uuid.UUID // uuid.Nil si no eligió organización
	CreatedAt            time.Time
}

func (u User) IsActive() bool { return u.Status == StatusActive }

// NewFromIdentity crea el perfil inicial a partir de los claims verificados. full_name es obligatorio en la
// base: si el proveedor no lo trae se usa la parte local del email, que el usuario podrá cambiar después.
func NewFromIdentity(subject, email, fullName string) (User, error) {
	email = strings.TrimSpace(email)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return User{}, ErrEmailRequired
	}
	name := strings.TrimSpace(fullName)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	return User{Subject: subject, Email: email, FullName: name, Status: StatusActive}, nil
}
