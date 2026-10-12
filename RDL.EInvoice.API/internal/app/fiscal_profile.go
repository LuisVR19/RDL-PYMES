package app

import (
	"context"

	"rdl/einvoice-api/internal/domain/permission"
	"rdl/einvoice-api/pkg/tenancy"
)

// GetFiscalProfile es getFiscalProfile (owner, admin, accountant). Sin perfil configurado → ErrNotFound (404 en
// openapi/fiscal.yaml); fiscal-profile-missing (409) queda para las operaciones que lo exigen (emitir, firmar).
type GetFiscalProfile struct{ txm TxManager }

func NewGetFiscalProfile(txm TxManager) *GetFiscalProfile { return &GetFiscalProfile{txm: txm} }

func (uc *GetFiscalProfile) Execute(ctx context.Context, t tenancy.Context) (FiscalProfile, error) {
	if err := authorize(t, permission.FiscalProfileRead); err != nil {
		return FiscalProfile{}, err
	}
	var p FiscalProfile
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		p, err = tx.FiscalProfiles().Get(ctx, t.OrganizationID())
		return err
	})
	return p, err
}
