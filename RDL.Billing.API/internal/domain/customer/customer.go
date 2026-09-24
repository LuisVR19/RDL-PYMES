// Package customer modela los clientes de una organización (billing.customers). Cambiar un cliente nunca altera
// una factura emitida: la factura guarda su propio snapshot al emitirse (arquitectura 4.4).
package customer

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ErrInvalid agrupa los errores de validación; FieldError indica el campo con su nombre JSON.
var ErrInvalid = errors.New("customer: datos inválidos")

type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

// Identification es tipo + número. TODO(fiscal): catálogo de tipos (fiscal.identification_types, vacío) y formato
// del número por tipo según Hacienda (D9); por ahora solo longitud, como schemas/common/identification.json.
type Identification struct {
	TypeCode string
	Number   string
}

type Customer struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Identification Identification
	LegalName      string
	TradeName      string
	Email          string
	Phone          string
	// Address es billing.customers.address_details. TODO(fiscal): provincia, cantón y distrito si los exige el comprobante.
	Address         string
	IsActive        bool
	CreatedByUserID uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type NewInput struct {
	IdentificationTypeCode, IdentificationNumber string
	LegalName, TradeName, Email, Phone, Address  string
}

func New(organizationID, createdBy uuid.UUID, in NewInput) (Customer, error) {
	c := Customer{
		OrganizationID: organizationID,
		Identification: Identification{
			TypeCode: strings.TrimSpace(in.IdentificationTypeCode),
			Number:   strings.TrimSpace(in.IdentificationNumber),
		},
		LegalName:       strings.TrimSpace(in.LegalName),
		TradeName:       strings.TrimSpace(in.TradeName),
		Email:           strings.TrimSpace(in.Email),
		Phone:           strings.TrimSpace(in.Phone),
		Address:         strings.TrimSpace(in.Address),
		IsActive:        true,
		CreatedByUserID: createdBy,
	}
	if err := c.validate(); err != nil {
		return Customer{}, err
	}
	return c, nil
}

// Patch son los cambios de PATCH. nil = no cambia; "" en tradeName, email, phone o address lo borra.
// La identificación no se edita (CustomerPatch del contrato): un cliente con otra identificación es otro cliente.
type Patch struct {
	LegalName *string
	TradeName *string
	Email     *string
	Phone     *string
	Address   *string
	IsActive  *bool
}

func (c Customer) Apply(p Patch) (Customer, error) {
	next := c
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&next.LegalName, p.LegalName)
	set(&next.TradeName, p.TradeName)
	set(&next.Email, p.Email)
	set(&next.Phone, p.Phone)
	set(&next.Address, p.Address)
	if p.IsActive != nil {
		next.IsActive = *p.IsActive
	}
	if err := next.validate(); err != nil {
		return Customer{}, err
	}
	return next, nil
}

// Límites de openapi/billing.yaml y schemas/common/*.json del contrato. La base usa text sin límite.
func (c Customer) validate() error {
	var errs []error
	check := func(ok bool, field, msg string) {
		if !ok {
			errs = append(errs, FieldError{Field: field, Message: msg})
		}
	}
	runes := utf8.RuneCountInString
	check(c.Identification.TypeCode != "" && runes(c.Identification.TypeCode) <= 10,
		"identification.typeCode", "es obligatorio y admite hasta 10 caracteres")
	check(c.Identification.Number != "" && runes(c.Identification.Number) <= 30,
		"identification.number", "es obligatorio y admite hasta 30 caracteres")
	check(c.LegalName != "" && runes(c.LegalName) <= 200, "legalName", "es obligatorio y admite hasta 200 caracteres")
	check(runes(c.TradeName) <= 200, "tradeName", "admite hasta 200 caracteres")
	check(runes(c.Phone) <= 30, "phone", "admite hasta 30 caracteres")
	check(runes(c.Address) <= 500, "address", "admite hasta 500 caracteres")
	if c.Email != "" {
		a, err := mail.ParseAddress(c.Email)
		check(err == nil && a.Address == c.Email && len(c.Email) <= 254, "email", "debe ser un email válido")
	}
	return errors.Join(errs...)
}
