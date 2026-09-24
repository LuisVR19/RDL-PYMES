// Package product modela los productos y servicios de una organización (billing.products) y sus impuestos
// (billing.product_taxes). Cambiar un producto nunca altera una factura emitida: la línea guarda su snapshot.
package product

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/money"
)

// ErrInvalid agrupa los errores de validación; FieldError indica el campo con su nombre JSON.
var ErrInvalid = errors.New("product: datos inválidos")

type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

// Formatos de schemas/common del contrato. Solo formato: los catálogos (fiscal.cabys_items, tax_types, tax_rates,
// units_of_measure) son de fiscal y hoy están vacíos. TODO(fiscal): validar contra el catálogo cuando exista.
var (
	cabysPattern      = regexp.MustCompile(`^[0-9]{13}$`)
	fiscalCodePattern = regexp.MustCompile(`^[0-9A-Za-z._-]{1,20}$`)
)

// Tax es un impuesto que aplica al producto: tipo y tarifa por código. La tasa (porcentaje) no vive aquí: sale del
// catálogo fiscal.tax_rates al armar la línea (informe 0001 §4.2, pendiente de aprobación).
type Tax struct {
	TypeCode string
	RateCode string
}

type Product struct {
	ID                uuid.UUID
	OrganizationID    uuid.UUID
	Code              string
	Description       string
	CabysCode         string
	UnitOfMeasureCode string
	UnitPrice         money.Amount
	Currency          money.Currency
	IsService         bool
	IsActive          bool
	// Taxes van ordenados por TypeCode: a lo sumo uno por tipo (product_taxes_product_tax_uk).
	Taxes     []Tax
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NewInput struct {
	Code, Description, CabysCode, UnitOfMeasureCode string
	UnitPrice                                       money.Amount
	Currency                                        money.Currency
	IsService                                       bool
	// IsActive nil = activo.
	IsActive *bool
	Taxes    []Tax
}

func New(organizationID uuid.UUID, in NewInput) (Product, error) {
	p := Product{
		OrganizationID:    organizationID,
		Code:              strings.TrimSpace(in.Code),
		Description:       strings.TrimSpace(in.Description),
		CabysCode:         strings.TrimSpace(in.CabysCode),
		UnitOfMeasureCode: strings.TrimSpace(in.UnitOfMeasureCode),
		UnitPrice:         money.CanonicalAmount(in.UnitPrice),
		Currency:          in.Currency,
		IsService:         in.IsService,
		IsActive:          in.IsActive == nil || *in.IsActive,
		Taxes:             normalizeTaxes(in.Taxes),
	}
	if err := p.validate(); err != nil {
		return Product{}, err
	}
	return p, nil
}

// Patch son los cambios de PATCH. nil = no cambia. Taxes no nil reemplaza la lista completa (vacía = sin impuestos).
// Un cuerpo ProductInput completo (el del contrato) es un Patch con todos los campos.
type Patch struct {
	Code              *string
	Description       *string
	CabysCode         *string
	UnitOfMeasureCode *string
	UnitPrice         *money.Amount
	Currency          *money.Currency
	IsService         *bool
	IsActive          *bool
	Taxes             *[]Tax
}

func (p Product) Apply(ch Patch) (Product, error) {
	next := p
	next.Taxes = slices.Clone(p.Taxes)
	set := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&next.Code, ch.Code)
	set(&next.Description, ch.Description)
	set(&next.CabysCode, ch.CabysCode)
	set(&next.UnitOfMeasureCode, ch.UnitOfMeasureCode)
	if ch.UnitPrice != nil {
		next.UnitPrice = money.CanonicalAmount(*ch.UnitPrice)
	}
	if ch.Currency != nil {
		next.Currency = *ch.Currency
	}
	if ch.IsService != nil {
		next.IsService = *ch.IsService
	}
	if ch.IsActive != nil {
		next.IsActive = *ch.IsActive
	}
	if ch.Taxes != nil {
		next.Taxes = normalizeTaxes(*ch.Taxes)
	}
	if err := next.validate(); err != nil {
		return Product{}, err
	}
	return next, nil
}

// Equal compara todo lo editable (Taxes es un slice: Product no es comparable con ==).
func (p Product) Equal(o Product) bool {
	a, b := p, o
	return slices.Equal(a.Taxes, b.Taxes) && a.ID == b.ID && a.OrganizationID == b.OrganizationID && a.Code == b.Code && a.Description == b.Description &&
		a.CabysCode == b.CabysCode && a.UnitOfMeasureCode == b.UnitOfMeasureCode &&
		a.UnitPrice.String() == b.UnitPrice.String() && a.Currency.String() == b.Currency.String() &&
		a.IsService == b.IsService && a.IsActive == b.IsActive
}

func normalizeTaxes(in []Tax) []Tax {
	out := make([]Tax, 0, len(in))
	for _, t := range in {
		out = append(out, Tax{TypeCode: strings.TrimSpace(t.TypeCode), RateCode: strings.TrimSpace(t.RateCode)})
	}
	// Orden estable por tipo: la misma lista en otro orden es el mismo producto (idempotencia, PATCH sin cambios).
	slices.SortStableFunc(out, func(a, b Tax) int { return strings.Compare(a.TypeCode, b.TypeCode) })
	return out
}

// Límites de openapi/billing.yaml y schemas/common/*.json del contrato. La base usa text sin límite.
func (p Product) validate() error {
	var errs []error
	check := func(ok bool, field, msg string) {
		if !ok {
			errs = append(errs, FieldError{Field: field, Message: msg})
		}
	}
	runes := utf8.RuneCountInString
	check(p.Code != "" && runes(p.Code) <= 50, "code", "es obligatorio y admite hasta 50 caracteres")
	check(p.Description != "" && runes(p.Description) <= 200, "description", "es obligatoria y admite hasta 200 caracteres")
	check(cabysPattern.MatchString(p.CabysCode), "cabysCode", "debe tener 13 dígitos")
	check(fiscalCodePattern.MatchString(p.UnitOfMeasureCode), "unitOfMeasureCode", "debe ser un código de 1 a 20 letras, dígitos, punto, guion o guion bajo")
	check(p.Currency.String() != "", "currency", "es obligatoria")
	seen := map[string]bool{}
	for i, t := range p.Taxes {
		field := fmt.Sprintf("taxes[%d]", i)
		check(fiscalCodePattern.MatchString(t.TypeCode), field+".taxTypeCode", "debe ser un código fiscal válido")
		check(fiscalCodePattern.MatchString(t.RateCode), field+".taxRateCode", "debe ser un código fiscal válido")
		check(!seen[t.TypeCode], field+".taxTypeCode", "el tipo de impuesto está repetido: a lo sumo uno por tipo")
		seen[t.TypeCode] = true
	}
	return errors.Join(errs...)
}
