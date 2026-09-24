// Package invitation modela la invitación de una persona (por email) a una organización con un rol.
package invitation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
)

// TTL es la vigencia de una invitación.
const TTL = 7 * 24 * time.Hour

const tokenPrefix = "inv_"

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRevoked  Status = "revoked"
	StatusExpired  Status = "expired"
)

var (
	ErrInvalidEmail = errors.New("invitation: email inválido")
	ErrExpired      = errors.New("invitation: la invitación venció")
	ErrNotPending   = errors.New("invitation: la invitación ya no está pendiente")
	// ErrEmailMismatch: la invitación es para otra persona. Hacia afuera se responde como inexistente.
	ErrEmailMismatch = errors.New("invitation: la invitación es para otro email")
)

type Invitation struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	Email            string
	Role             membership.Role
	Status           Status
	ExpiresAt        time.Time
	InvitedByUserID  uuid.UUID
	AcceptedByUserID uuid.UUID
	AcceptedAt       time.Time
	CreatedAt        time.Time
}

// New prepara una invitación pendiente. El token en claro se devuelve una sola vez; solo su hash se guarda.
func New(organizationID uuid.UUID, email string, role membership.Role, invitedBy uuid.UUID, now time.Time) (inv Invitation, token, tokenHash string, err error) {
	email, err = NormalizeEmail(email)
	if err != nil {
		return Invitation{}, "", "", err
	}
	if !role.Valid() {
		return Invitation{}, "", "", membership.ErrInvalidRole
	}
	token, err = newToken()
	if err != nil {
		return Invitation{}, "", "", err
	}
	return Invitation{
		OrganizationID:  organizationID,
		Email:           email,
		Role:            role,
		Status:          StatusPending,
		ExpiresAt:       now.Add(TTL),
		InvitedByUserID: invitedBy,
	}, token, HashToken(token), nil
}

// NormalizeEmail valida el email y lo pasa a minúsculas: la unicidad en la base es por lower(email).
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken es lo que se guarda y se busca (shared.sha256_hex: 64 hex en minúscula).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LooksLikeToken descarta de entrada valores que no pueden ser un token (evita consultas inútiles).
func LooksLikeToken(s string) bool {
	return strings.HasPrefix(s, tokenPrefix) && len(s) > len(tokenPrefix)+40 && len(s) < 100
}

// EffectiveStatus trata como vencida una invitación pendiente cuya fecha pasó, aunque la fila siga en pending.
func (i Invitation) EffectiveStatus(now time.Time) Status {
	if i.Status == StatusPending && !now.Before(i.ExpiresAt) {
		return StatusExpired
	}
	return i.Status
}

// CheckAcceptable valida que el usuario con ese email pueda aceptarla ahora.
func (i Invitation) CheckAcceptable(email string, now time.Time) error {
	if !strings.EqualFold(strings.TrimSpace(email), i.Email) {
		return ErrEmailMismatch
	}
	switch i.EffectiveStatus(now) {
	case StatusPending:
		return nil
	case StatusExpired:
		return ErrExpired
	default:
		return ErrNotPending
	}
}
