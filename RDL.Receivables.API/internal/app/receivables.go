package app

import (
	"context"
	"fmt"
	"time"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/pkg/tenancy"
)

// ListReceivables devuelve una página de cuentas por cobrar de la organización activa, de la más nueva a la más
// vieja (owner, admin, collector, accountant, read_only).
type ListReceivables struct {
	tx  TxManager
	now func() time.Time
}

func NewListReceivables(tx TxManager) *ListReceivables {
	return &ListReceivables{tx: tx, now: time.Now}
}

type ReceivablePage struct {
	Items []ReceivableView
	Next  *PageCursor // nil = no hay más páginas
}

func (uc *ListReceivables) Execute(ctx context.Context, t tenancy.Context, q ReceivableQuery) (ReceivablePage, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return ReceivablePage{}, err
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageSize
	}
	q.Limit = min(q.Limit, MaxPageSize)

	var page ReceivablePage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if q.Overdue != nil {
			// "Vencida" depende del día de negocio de la organización, no del día en UTC.
			tz, err := tx.Organizations().Timezone(ctx, t.OrganizationID())
			if err != nil {
				return err
			}
			loc, err := time.LoadLocation(tz)
			if err != nil {
				return fmt.Errorf("zona horaria de la organización %q: %w", tz, err)
			}
			q.Today = civil.Today(uc.now(), loc)
		}
		// Se pide uno de más para saber si existe una página siguiente sin contar toda la tabla.
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Receivables().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		if len(items) > q.Limit {
			items = items[:q.Limit]
			last := items[len(items)-1]
			page.Next = &PageCursor{At: last.CreatedAt, ID: last.ID}
		}
		page.Items = items
		return nil
	})
	return page, err
}
