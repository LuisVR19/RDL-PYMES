package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/organization"
)

type organizations struct{ q *db.Queries }

func (r organizations) Create(ctx context.Context, o organization.Organization) (organization.Organization, error) {
	row, err := r.q.InsertOrganization(ctx, db.InsertOrganizationParams{
		ID: o.ID, LegalName: o.LegalName, TradeName: o.TradeName,
		IdentificationTypeCode: o.IdentificationTypeCode, IdentificationNumber: o.IdentificationNumber,
		Email: o.Email, Phone: o.Phone, Timezone: o.Timezone,
	})
	if isUniqueViolation(err, "organizations_identification_uk") {
		return organization.Organization{}, fmt.Errorf("%w: ya existe una organización con esa identificación", app.ErrConflict)
	}
	if err != nil {
		return organization.Organization{}, fmt.Errorf("creando organización: %w", err)
	}
	// Las filas generadas por sqlc para cada consulta tienen los mismos campos: se convierten al mismo tipo.
	return toOrganization(db.GetOrganizationRow(row)), nil
}

func (r organizations) Get(ctx context.Context, id uuid.UUID) (organization.Organization, error) {
	row, err := r.q.GetOrganization(ctx, id)
	if isNoRows(err) {
		return organization.Organization{}, app.ErrNotFound
	}
	if err != nil {
		return organization.Organization{}, fmt.Errorf("leyendo organización: %w", err)
	}
	return toOrganization(row), nil
}

func (r organizations) GetForUpdate(ctx context.Context, id uuid.UUID) (organization.Organization, error) {
	row, err := r.q.GetOrganizationForUpdate(ctx, id)
	if isNoRows(err) {
		return organization.Organization{}, app.ErrNotFound
	}
	if err != nil {
		return organization.Organization{}, fmt.Errorf("bloqueando organización: %w", err)
	}
	return toOrganization(db.GetOrganizationRow(row)), nil
}

func (r organizations) Update(ctx context.Context, o organization.Organization) (organization.Organization, error) {
	row, err := r.q.UpdateOrganization(ctx, db.UpdateOrganizationParams{
		ID: o.ID, TradeName: o.TradeName, Email: o.Email, Phone: o.Phone, Timezone: o.Timezone,
	})
	if isNoRows(err) {
		return organization.Organization{}, app.ErrNotFound
	}
	if err != nil {
		return organization.Organization{}, fmt.Errorf("actualizando organización: %w", err)
	}
	return toOrganization(db.GetOrganizationRow(row)), nil
}

func toOrganization(r db.GetOrganizationRow) organization.Organization {
	return organization.Organization{
		ID: r.ID, LegalName: r.LegalName, TradeName: r.TradeName,
		IdentificationTypeCode: r.IdentificationTypeCode, IdentificationNumber: r.IdentificationNumber,
		Email: r.Email, Phone: r.Phone, Timezone: r.Timezone, DefaultCurrencyCode: r.DefaultCurrencyCode,
		Status: organization.Status(r.Status), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
