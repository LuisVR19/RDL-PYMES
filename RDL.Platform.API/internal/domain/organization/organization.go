// Package organization modela al tenant del SaaS (core.organizations).
// Solo guarda identificación básica: la configuración fiscal pertenece a E-Invoice (prompt P3, fuera de alcance).
package organization

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const DefaultTimezone = "America/Costa_Rica"

type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusClosed    Status = "closed"
)

// ErrInvalid agrupa los errores de validación de negocio; FieldError indica el campo.
var ErrInvalid = errors.New("organization: datos inválidos")

type FieldError struct {
	Field, Message string
}

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

type Organization struct {
	ID                     uuid.UUID
	LegalName              string
	TradeName              string
	IdentificationTypeCode string
	IdentificationNumber   string
	Email                  string
	Phone                  string
	Timezone               string
	DefaultCurrencyCode    string
	Status                 Status
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type NewInput struct {
	LegalName              string
	TradeName              string
	IdentificationTypeCode string
	IdentificationNumber   string
	Email                  string
	Phone                  string
	Timezone               string
}

// New valida y normaliza los datos de alta. TODO(contracts): validar identificationTypeCode contra el
// catálogo de tipos de identificación y el formato del número por tipo cuando el contrato lo defina;
// hoy solo se exige que no estén vacíos (no se inventan reglas fiscales).
func New(id uuid.UUID, in NewInput) (Organization, error) {
	o := Organization{
		ID:                     id,
		LegalName:              strings.TrimSpace(in.LegalName),
		TradeName:              strings.TrimSpace(in.TradeName),
		IdentificationTypeCode: strings.TrimSpace(in.IdentificationTypeCode),
		IdentificationNumber:   strings.TrimSpace(in.IdentificationNumber),
		Email:                  strings.TrimSpace(in.Email),
		Phone:                  strings.TrimSpace(in.Phone),
		Timezone:               strings.TrimSpace(in.Timezone),
		Status:                 StatusActive,
	}
	if o.Timezone == "" {
		o.Timezone = DefaultTimezone
	}
	if err := o.validate(); err != nil {
		return Organization{}, err
	}
	return o, nil
}

// Patch son los cambios permitidos por PATCH /organizations/current. nil = no cambia.
// Nombre legal e identificación no se editan aquí: son datos de identidad del contribuyente.
type Patch struct {
	TradeName *string
	Email     *string
	Phone     *string
	Timezone  *string
}

func (p Patch) Empty() bool {
	return p.TradeName == nil && p.Email == nil && p.Phone == nil && p.Timezone == nil
}

// Apply devuelve la organización con los cambios aplicados y validados.
func (o Organization) Apply(p Patch) (Organization, error) {
	next := o
	if p.TradeName != nil {
		next.TradeName = strings.TrimSpace(*p.TradeName)
	}
	if p.Email != nil {
		next.Email = strings.TrimSpace(*p.Email)
	}
	if p.Phone != nil {
		next.Phone = strings.TrimSpace(*p.Phone)
	}
	if p.Timezone != nil {
		next.Timezone = strings.TrimSpace(*p.Timezone)
	}
	if err := next.validate(); err != nil {
		return Organization{}, err
	}
	return next, nil
}

func (o Organization) validate() error {
	var errs []error
	check := func(ok bool, field, msg string) {
		if !ok {
			errs = append(errs, FieldError{Field: field, Message: msg})
		}
	}
	check(o.LegalName != "" && utf8.RuneCountInString(o.LegalName) <= 200, "legalName", "es obligatorio y admite hasta 200 caracteres")
	check(utf8.RuneCountInString(o.TradeName) <= 200, "tradeName", "admite hasta 200 caracteres")
	check(o.IdentificationTypeCode != "" && len(o.IdentificationTypeCode) <= 10, "identificationTypeCode", "es obligatorio y admite hasta 10 caracteres")
	check(o.IdentificationNumber != "" && len(o.IdentificationNumber) <= 30 && !strings.ContainsAny(o.IdentificationNumber, " \t"),
		"identificationNumber", "es obligatorio, sin espacios y de hasta 30 caracteres")
	check(validEmail(o.Email), "email", "debe ser un email válido")
	check(len(o.Phone) <= 30, "phone", "admite hasta 30 caracteres")
	_, tzErr := time.LoadLocation(o.Timezone)
	check(o.Timezone != "" && o.Timezone != "Local" && tzErr == nil, "timezone", "debe ser una zona horaria IANA (ej. America/Costa_Rica)")
	return errors.Join(errs...)
}

func validEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && len(s) <= 254
}
