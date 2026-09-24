package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/money"
)

// catalog lee core y fiscal con los GRANT de solo lectura de billing_app (docs/decisiones/0002).
type catalog struct{ q *db.Queries }

func (c catalog) Organization(ctx context.Context, org uuid.UUID) (app.OrganizationSettings, error) {
	row, err := c.q.GetOrganizationSettings(ctx, org)
	if isNoRows(err) {
		// La membresía ya se revalidó: si RLS no deja ver la organización, algo está mal configurado.
		return app.OrganizationSettings{}, fmt.Errorf("organización %s no visible en core.organizations", org)
	}
	if err != nil {
		return app.OrganizationSettings{}, fmt.Errorf("leyendo organización: %w", err)
	}
	currency, err := money.ParseCurrency(row.DefaultCurrencyCode)
	if err != nil {
		return app.OrganizationSettings{}, fmt.Errorf("moneda local de la organización: %w", err)
	}
	return app.OrganizationSettings{LocalCurrency: currency, Timezone: row.Timezone}, nil
}

func (c catalog) Branch(ctx context.Context, org, id uuid.UUID) (app.Branch, error) {
	row, err := c.q.GetBranch(ctx, db.GetBranchParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.Branch{}, app.ErrNotFound
	}
	if err != nil {
		return app.Branch{}, fmt.Errorf("leyendo sucursal: %w", err)
	}
	return app.Branch{ID: row.ID, IsActive: row.IsActive}, nil
}

func (c catalog) TaxRates(ctx context.Context, codes []string) (map[string]money.Percentage, error) {
	out := make(map[string]money.Percentage, len(codes))
	if len(codes) == 0 {
		return out, nil
	}
	rows, err := c.q.ListActiveTaxRates(ctx, codes)
	if err != nil {
		return nil, fmt.Errorf("leyendo tarifas de impuesto: %w", err)
	}
	for _, r := range rows {
		p, err := money.ParsePercentage(trimZerosPct(r.Rate))
		if err != nil {
			return nil, fmt.Errorf("tarifa %s del catálogo: %w", r.Code, err)
		}
		out[r.Code] = p
	}
	return out, nil
}
