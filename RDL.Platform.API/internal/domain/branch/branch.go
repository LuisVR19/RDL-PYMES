// Package branch modela las sucursales de una organización (core.branches).
// La configuración fiscal de establecimientos y terminales pertenece a E-Invoice (fiscal.establishments).
package branch

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ErrInvalid agrupa los errores de validación; FieldError indica el campo.
var ErrInvalid = errors.New("branch: datos inválidos")

type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

var codePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,20}$`)

type Branch struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Code           string
	Name           string
	Address        string
	Phone          string
	Email          string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type NewInput struct {
	Code, Name, Address, Phone, Email string
}

func New(organizationID uuid.UUID, in NewInput) (Branch, error) {
	b := Branch{
		OrganizationID: organizationID,
		Code:           strings.TrimSpace(in.Code),
		Name:           strings.TrimSpace(in.Name),
		Address:        strings.TrimSpace(in.Address),
		Phone:          strings.TrimSpace(in.Phone),
		Email:          strings.TrimSpace(in.Email),
		IsActive:       true,
	}
	if err := b.validate(); err != nil {
		return Branch{}, err
	}
	return b, nil
}

// Patch son los cambios de PATCH. nil = no cambia; "" en address, phone o email lo borra.
// El código no se edita: otros sistemas pueden referenciarlo.
type Patch struct {
	Name     *string
	Address  *string
	Phone    *string
	Email    *string
	IsActive *bool
}

func (b Branch) Apply(p Patch) (Branch, error) {
	next := b
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&next.Name, p.Name)
	set(&next.Address, p.Address)
	set(&next.Phone, p.Phone)
	set(&next.Email, p.Email)
	if p.IsActive != nil {
		next.IsActive = *p.IsActive
	}
	if err := next.validate(); err != nil {
		return Branch{}, err
	}
	return next, nil
}

func (b Branch) validate() error {
	var errs []error
	check := func(ok bool, field, msg string) {
		if !ok {
			errs = append(errs, FieldError{Field: field, Message: msg})
		}
	}
	check(codePattern.MatchString(b.Code), "code", "es obligatorio: 1 a 20 letras, números, guion o guion bajo")
	check(b.Name != "" && utf8.RuneCountInString(b.Name) <= 150, "name", "es obligatorio y admite hasta 150 caracteres")
	check(utf8.RuneCountInString(b.Address) <= 500, "address", "admite hasta 500 caracteres")
	check(len(b.Phone) <= 30, "phone", "admite hasta 30 caracteres")
	if b.Email != "" {
		a, err := mail.ParseAddress(b.Email)
		check(err == nil && a.Address == b.Email && len(b.Email) <= 254, "email", "debe ser un email válido")
	}
	return errors.Join(errs...)
}
