package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/domain/permission"
	"rdl/einvoice-api/pkg/tenancy"
)

// FiscalStatus es el estado fiscal de un documento comercial para el BFF (FiscalStatus de
// RDL.Contracts/openapi/bff-internal.yaml).
type FiscalStatus struct {
	SourceDocumentID      uuid.UUID
	ElectronicDocumentID  uuid.UUID
	Status                string
	HaciendaStatusMessage string
}

// FiscalStatusReader lee el estado fiscal por documento comercial, en la organización de la sesión.
type FiscalStatusReader interface {
	BySource(ctx context.Context, organizationID, sourceDocumentID uuid.UUID) (FiscalStatus, error)
	BySources(ctx context.Context, organizationID uuid.UUID, sourceDocumentIDs []uuid.UUID) ([]FiscalStatus, error)
}

// MaxBatch es el tope de ids por llamada de las rutas por lote (IdBatch, convenciones §9).
const MaxBatch = 100

// GetFiscalStatusBySource es getFiscalStatusBySource (BFF, todos los roles). Sin documento → ErrNotFound.
type GetFiscalStatusBySource struct{ txm TxManager }

func NewGetFiscalStatusBySource(txm TxManager) *GetFiscalStatusBySource {
	return &GetFiscalStatusBySource{txm: txm}
}

func (uc *GetFiscalStatusBySource) Execute(ctx context.Context, t tenancy.Context, sourceDocumentID uuid.UUID) (FiscalStatus, error) {
	if err := authorize(t, permission.DocumentsRead); err != nil {
		return FiscalStatus{}, err
	}
	var s FiscalStatus
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		s, err = tx.FiscalStatuses().BySource(ctx, t.OrganizationID(), sourceDocumentID)
		return err
	})
	return s, err
}

// GetFiscalStatusesBySource es getFiscalStatusesBySource: de 1 a 100 ids distintos; omite los que no tienen
// documento o son de otra organización (no se distinguen).
type GetFiscalStatusesBySource struct{ txm TxManager }

func NewGetFiscalStatusesBySource(txm TxManager) *GetFiscalStatusesBySource {
	return &GetFiscalStatusesBySource{txm: txm}
}

func (uc *GetFiscalStatusesBySource) Execute(ctx context.Context, t tenancy.Context, ids []uuid.UUID) ([]FiscalStatus, error) {
	if err := authorize(t, permission.DocumentsRead); err != nil {
		return nil, err
	}
	if len(ids) == 0 || len(ids) > MaxBatch {
		return nil, ValidationError{Field: "ids", Message: fmt.Sprintf("debe traer de 1 a %d ids", MaxBatch)}
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			return nil, ValidationError{Field: fmt.Sprintf("ids[%d]", i), Message: "id repetido"}
		}
		seen[id] = true
	}
	var out []FiscalStatus
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = tx.FiscalStatuses().BySources(ctx, t.OrganizationID(), ids)
		return err
	})
	return out, err
}

// ValidationError es un campo inválido de una petición (422).
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Message }
