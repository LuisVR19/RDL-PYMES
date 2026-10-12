// Package app contiene los casos de uso de fiscal y los puertos que implementan los adapters.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"rdl/einvoice-api/pkg/tenancy"
)

// TxManager abre una transacción con la sesión RLS de la organización y el usuario del TenantContext.
type TxManager interface {
	WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, Tx) error) error
}

// Tx expone los puertos de persistencia dentro de una transacción. Crece con cada incremento.
type Tx interface {
	FiscalProfiles() FiscalProfileReader
	FiscalStatuses() FiscalStatusReader
	FiscalConfig() FiscalConfigStore
	Catalogs() CatalogReader
	Audit() AuditRecorder
}

// FiscalProfileReader lee el perfil de contribuyente. Devuelve ErrNotFound si la organización no lo configuró.
type FiscalProfileReader interface {
	Get(ctx context.Context, organizationID uuid.UUID) (FiscalProfile, error)
}

// FiscalProfile es la vista del perfil de contribuyente (fiscal.taxpayer_profiles + actividad principal + certificado
// activo). Nunca trae secretos: de las credenciales de Hacienda solo dice si están configuradas.
type FiscalProfile struct {
	Environment            string
	IdentificationTypeCode string
	IdentificationNumber   string
	LegalName              string
	TradeName              string
	Email                  string
	Phone                  string
	ProvinceCode           string
	CantonCode             string
	DistrictCode           string
	AddressDetails         string
	EconomicActivityCode   string
	HaciendaCredentialsSet bool
	CertificateValidTo     *time.Time
	UpdatedAt              time.Time
}
