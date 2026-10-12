package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/adapters/postgres/db"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/taxpayer"
)

type fiscalConfig struct{ q *db.Queries }

// UpsertProfile guarda el perfil y deja la actividad económica indicada como la única principal (o ninguna).
func (s fiscalConfig) UpsertProfile(ctx context.Context, org uuid.UUID, p taxpayer.Profile) (bool, error) {
	params := db.UpsertTaxpayerProfileParams{
		OrganizationID: org, IdentificationTypeCode: p.IdentificationTypeCode, IdentificationNumber: p.IdentificationNumber,
		LegalName: p.LegalName, TradeName: text(p.TradeName), Email: p.Email, Phone: text(p.Phone),
		Environment: string(p.Environment),
	}
	if l := p.Location; l != nil {
		params.ProvinceCode, params.CantonCode, params.DistrictCode = text(l.ProvinceCode), text(l.CantonCode), text(l.DistrictCode)
		params.AddressDetails = text(l.AddressDetails)
	}
	created, err := s.q.UpsertTaxpayerProfile(ctx, params)
	if err != nil {
		return false, fmt.Errorf("guardando el perfil fiscal: %w", err)
	}
	if err := s.q.ClearPrimaryActivity(ctx, org); err != nil {
		return false, fmt.Errorf("actualizando la actividad principal: %w", err)
	}
	if p.EconomicActivityCode != "" {
		if err := s.q.SetPrimaryActivity(ctx, db.SetPrimaryActivityParams{OrganizationID: org, ActivityCode: p.EconomicActivityCode}); err != nil {
			return false, fmt.Errorf("guardando la actividad principal: %w", err)
		}
	}
	return created, nil
}

func (s fiscalConfig) IdentificationTypeExists(ctx context.Context, code string) (bool, error) {
	ok, err := s.q.IdentificationTypeExists(ctx, code)
	if err != nil {
		return false, fmt.Errorf("consultando el catálogo de identificación: %w", err)
	}
	return ok, nil
}

func (s fiscalConfig) DistrictExists(ctx context.Context, province, canton, district string) (bool, error) {
	ok, err := s.q.DistrictExists(ctx, db.DistrictExistsParams{ProvinceCode: province, CantonCode: canton, DistrictCode: district})
	if err != nil {
		return false, fmt.Errorf("consultando el catálogo de ubicación: %w", err)
	}
	return ok, nil
}

func (s fiscalConfig) EconomicActivityExists(ctx context.Context, code string) (bool, error) {
	ok, err := s.q.EconomicActivityExists(ctx, code)
	if err != nil {
		return false, fmt.Errorf("consultando el catálogo de actividades: %w", err)
	}
	return ok, nil
}

func (s fiscalConfig) SetHaciendaCredentials(ctx context.Context, org, userRef, passRef uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := s.q.SetHaciendaCredentialRefs(ctx, db.SetHaciendaCredentialRefsParams{
		OrganizationID: org, UsernameSecretID: nullUUID(userRef), PasswordSecretID: nullUUID(passRef),
	})
	if isNoRows(err) {
		return uuid.Nil, uuid.Nil, app.ErrProfileMissing
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("guardando las referencias de las credenciales: %w", err)
	}
	return row.PrevUsernameSecretID.UUID, row.PrevPasswordSecretID.UUID, nil
}

// ReplaceCertificate desactiva el activo y registra el nuevo en la misma transacción (índice único parcial
// certificates_one_active_uk). storage_path es la referencia al SecretStore, no una ruta de disco legible.
func (s fiscalConfig) ReplaceCertificate(ctx context.Context, c app.CertificateRecord) error {
	if err := s.q.DeactivateCertificates(ctx, c.OrganizationID); err != nil {
		return fmt.Errorf("desactivando el certificado anterior: %w", err)
	}
	err := s.q.InsertCertificate(ctx, db.InsertCertificateParams{
		ID: c.ID, OrganizationID: c.OrganizationID, StoragePath: "secret://" + c.FileRef.String(), PinSecretID: c.PINRef,
		Subject: text(c.Info.Subject), Issuer: text(c.Info.Issuer), SerialNumber: text(c.Info.SerialNumber),
		ValidFrom: c.Info.NotBefore.UTC(), ValidTo: c.Info.NotAfter.UTC(), UploadedByUserID: nullUUID(c.UploadedBy),
	})
	if err != nil {
		return fmt.Errorf("registrando el certificado: %w", err)
	}
	return nil
}
