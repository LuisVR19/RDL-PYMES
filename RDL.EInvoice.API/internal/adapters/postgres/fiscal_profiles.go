package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/adapters/postgres/db"
	"rdl/einvoice-api/internal/app"
)

type fiscalProfiles struct{ q *db.Queries }

func (r fiscalProfiles) Get(ctx context.Context, organizationID uuid.UUID) (app.FiscalProfile, error) {
	row, err := r.q.GetTaxpayerProfile(ctx, organizationID)
	if isNoRows(err) {
		return app.FiscalProfile{}, app.ErrNotFound
	}
	if err != nil {
		return app.FiscalProfile{}, fmt.Errorf("leyendo el perfil fiscal: %w", err)
	}
	p := app.FiscalProfile{
		Environment:            row.Environment,
		IdentificationTypeCode: row.IdentificationTypeCode,
		IdentificationNumber:   row.IdentificationNumber,
		LegalName:              row.LegalName,
		TradeName:              row.TradeName.String,
		Email:                  row.Email,
		Phone:                  row.Phone.String,
		ProvinceCode:           row.ProvinceCode.String,
		CantonCode:             row.CantonCode.String,
		DistrictCode:           row.DistrictCode.String,
		AddressDetails:         row.AddressDetails.String,
		EconomicActivityCode:   row.PrimaryActivityCode.String,
		HaciendaCredentialsSet: row.HaciendaCredentialsSet,
		UpdatedAt:              row.UpdatedAt.UTC(),
	}
	if row.CertificateValidTo.Valid {
		v := row.CertificateValidTo.Time.UTC()
		p.CertificateValidTo = &v
	}
	return p, nil
}
