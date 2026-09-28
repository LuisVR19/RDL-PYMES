package app

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/aging"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/pkg/tenancy"
)

// GetReceivable devuelve la cuenta con sus aplicaciones y ajustes (lectura).
type GetReceivable struct{ tx TxManager }

func NewGetReceivable(tx TxManager) *GetReceivable { return &GetReceivable{tx: tx} }

func (uc *GetReceivable) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (ReceivableDetail, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return ReceivableDetail{}, err
	}
	var d ReceivableDetail
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		d, err = tx.Receivables().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return d, err
}

// GetBalanceByInvoice es el endpoint interno del BFF: saldo de la cuenta de una factura. Mismo JWT del usuario y
// roles de lectura más biller (R10), porque la vista de facturación muestra el saldo.
type GetBalanceByInvoice struct{ tx TxManager }

func NewGetBalanceByInvoice(tx TxManager) *GetBalanceByInvoice { return &GetBalanceByInvoice{tx: tx} }

func (uc *GetBalanceByInvoice) Execute(ctx context.Context, t tenancy.Context, invoiceID uuid.UUID) (ReceivableView, error) {
	if err := authorize(t, permission.InvoiceBalanceRead); err != nil {
		return ReceivableView{}, err
	}
	var v ReceivableView
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		v, err = tx.Receivables().GetByInvoice(ctx, t.OrganizationID(), invoiceID)
		return err
	})
	return v, err
}

// MaxBalanceBatch es el tope de ids por llamada de GetBalancesByInvoice (IdBatch de bff-internal.yaml: el `limit`
// máximo de una página).
const MaxBalanceBatch = 100

// GetBalancesByInvoice es la versión por lote de GetBalanceByInvoice, para los listados del BFF (sin N+1). Mismos
// permisos (R10). La forma del lote (1 a 100 ids distintos) la valida el handler.
type GetBalancesByInvoice struct{ tx TxManager }

func NewGetBalancesByInvoice(tx TxManager) *GetBalancesByInvoice {
	return &GetBalancesByInvoice{tx: tx}
}

func (uc *GetBalancesByInvoice) Execute(ctx context.Context, t tenancy.Context, invoiceIDs []uuid.UUID) ([]ReceivableView, error) {
	if err := authorize(t, permission.InvoiceBalanceRead); err != nil {
		return nil, err
	}
	var out []ReceivableView
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = tx.Receivables().ListByInvoices(ctx, t.OrganizationID(), invoiceIDs)
		return err
	})
	return out, err
}

// GetAging agrupa el saldo cobrable por moneda y tramo (R6) a la fecha asOf, que por defecto es hoy en la zona de la
// organización. Sin conversión entre monedas. Los saldos son los actuales: un asOf pasado reclasifica por
// vencimiento, no reconstruye el saldo que había ese día.
type GetAging struct {
	tx  TxManager
	now func() time.Time
}

func NewGetAging(tx TxManager) *GetAging { return &GetAging{tx: tx, now: time.Now} }

type AgingQuery struct {
	AsOf     civil.Date // cero = hoy en la zona de la organización
	Currency string     // vacío = todas
}

type AgingReport struct {
	AsOf    civil.Date
	Buckets []AgingBucket
}

type AgingBucket struct {
	Bucket   aging.Bucket
	Currency string
	Balance  decimal.Decimal
}

func (uc *GetAging) Execute(ctx context.Context, t tenancy.Context, q AgingQuery) (AgingReport, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return AgingReport{}, err
	}
	var rep AgingReport
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		asOf := q.AsOf
		if asOf.IsZero() {
			var err error
			if asOf, err = today(ctx, tx.Organizations(), t.OrganizationID(), uc.now()); err != nil {
				return err
			}
		}
		rows, err := tx.Receivables().AgingByDueDate(ctx, t.OrganizationID(), q.Currency)
		if err != nil {
			return err
		}
		rep = buildAging(asOf, rows)
		return nil
	})
	return rep, err
}

// buildAging suma con decimal exacto por moneda y tramo; cada moneda presente trae sus cinco tramos (en cero si no
// hay saldo), en el orden de aging.Buckets.
func buildAging(asOf civil.Date, rows []AgingRow) AgingReport {
	sums := map[string]map[aging.Bucket]decimal.Decimal{}
	var currencies []string
	for _, r := range rows {
		if sums[r.Currency] == nil {
			sums[r.Currency] = map[aging.Bucket]decimal.Decimal{}
			currencies = append(currencies, r.Currency)
		}
		b := aging.For(r.DueOn, asOf)
		sums[r.Currency][b] = sums[r.Currency][b].Add(r.Balance)
	}
	slices.Sort(currencies)
	rep := AgingReport{AsOf: asOf, Buckets: []AgingBucket{}}
	for _, c := range currencies {
		for _, b := range aging.Buckets {
			rep.Buckets = append(rep.Buckets, AgingBucket{Bucket: b, Currency: c, Balance: sums[c][b]})
		}
	}
	return rep
}
