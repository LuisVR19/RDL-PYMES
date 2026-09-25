package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

// Lecturas para la API. Las filas de sqlc con la misma forma se convierten entre sí (misma estructura), así cada
// vista tiene un solo mapeo.

func (r receivables) Get(ctx context.Context, org, id uuid.UUID) (app.ReceivableDetail, error) {
	row, err := r.q.GetReceivable(ctx, db.GetReceivableParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.ReceivableDetail{}, app.ErrNotFound
	}
	if err != nil {
		return app.ReceivableDetail{}, fmt.Errorf("leyendo la cuenta: %w", err)
	}
	d := app.ReceivableDetail{ReceivableView: receivableView(db.ListReceivablesRow(row))}
	apps, err := r.q.ListApplicationViewsForReceivable(ctx, db.ListApplicationViewsForReceivableParams{OrganizationID: org, ReceivableID: id})
	if err != nil {
		return app.ReceivableDetail{}, fmt.Errorf("leyendo las aplicaciones de la cuenta: %w", err)
	}
	for _, a := range apps {
		d.Applications = append(d.Applications, applicationView(db.GetApplicationViewRow(a)))
	}
	adjs, err := r.q.ListAdjustmentViews(ctx, db.ListAdjustmentViewsParams{OrganizationID: org, ReceivableID: id})
	if err != nil {
		return app.ReceivableDetail{}, fmt.Errorf("leyendo los ajustes de la cuenta: %w", err)
	}
	for _, a := range adjs {
		d.Adjustments = append(d.Adjustments, app.AdjustmentView{
			ID: a.ID, Type: receivable.AdjustmentType(a.AdjustmentType), Amount: a.Amount,
			SourceDocumentID: a.SourceDocumentID.UUID, Reason: a.Reason, CreatedAt: a.CreatedAt,
		})
	}
	return d, nil
}

func (r receivables) GetByInvoice(ctx context.Context, org, invoiceID uuid.UUID) (app.ReceivableView, error) {
	row, err := r.q.GetReceivableByInvoice(ctx, db.GetReceivableByInvoiceParams{OrganizationID: org, SourceInvoiceID: invoiceID})
	if isNoRows(err) {
		return app.ReceivableView{}, app.ErrNotFound
	}
	if err != nil {
		return app.ReceivableView{}, fmt.Errorf("leyendo la cuenta de la factura: %w", err)
	}
	return receivableView(db.ListReceivablesRow(row)), nil
}

func (r receivables) AgingByDueDate(ctx context.Context, org uuid.UUID, currency string) ([]app.AgingRow, error) {
	rows, err := r.q.AgingByDueDate(ctx, db.AgingByDueDateParams{OrganizationID: org, Currency: text(currency)})
	if err != nil {
		return nil, fmt.Errorf("calculando el aging: %w", err)
	}
	out := make([]app.AgingRow, 0, len(rows))
	for _, row := range rows {
		b, err := decimal.NewFromString(row.Balance)
		if err != nil {
			return nil, fmt.Errorf("saldo del aging: %w", err)
		}
		out = append(out, app.AgingRow{Currency: row.CurrencyCode, DueOn: civil.FromTime(row.DueOn.Time), Balance: b})
	}
	return out, nil
}

func receivableView(row db.ListReceivablesRow) app.ReceivableView {
	return app.ReceivableView{
		ID: row.ID, SourceInvoiceID: row.SourceInvoiceID, CustomerID: row.CustomerID,
		CustomerLegalName: row.CustomerLegalName, DocumentNumber: row.DocumentNumber, Currency: row.CurrencyCode,
		OriginalAmount: row.OriginalAmount, BalanceAmount: row.BalanceAmount,
		IssuedOn: civil.FromTime(row.IssuedOn.Time), DueOn: civil.FromTime(row.DueOn.Time),
		Status: receivable.Status(row.Status), SettledAt: tsPtr(row.SettledAt), CreatedAt: row.CreatedAt,
	}
}

func applicationView(a db.GetApplicationViewRow) app.ApplicationView {
	return app.ApplicationView{
		ID: a.ID, PaymentID: a.PaymentID, ReceivableID: a.ReceivableID, Amount: a.Amount, AppliedAt: a.AppliedAt.UTC(),
		ReversedAt: tsPtr(a.ReversedAt), ReversalReason: a.ReversalReason.String,
	}
}

type payments struct{ q *db.Queries }

func (p payments) Get(ctx context.Context, org, id uuid.UUID) (app.PaymentView, error) {
	row, err := p.q.GetPayment(ctx, db.GetPaymentParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.PaymentView{}, app.ErrNotFound
	}
	if err != nil {
		return app.PaymentView{}, fmt.Errorf("leyendo el pago: %w", err)
	}
	views, err := p.withApplications(ctx, org, []db.ListPaymentsRow{db.ListPaymentsRow(row)})
	if err != nil {
		return app.PaymentView{}, err
	}
	return views[0], nil
}

func (p payments) List(ctx context.Context, org uuid.UUID, q app.PaymentQuery) ([]app.PaymentView, error) {
	params := db.ListPaymentsParams{
		OrganizationID: org,
		CustomerID:     nullUUID(q.CustomerID),
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := p.q.ListPayments(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando pagos: %w", err)
	}
	return p.withApplications(ctx, org, rows)
}

func (p payments) GetApplication(ctx context.Context, org, id uuid.UUID) (app.ApplicationView, error) {
	row, err := p.q.GetApplicationView(ctx, db.GetApplicationViewParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.ApplicationView{}, app.ErrNotFound
	}
	if err != nil {
		return app.ApplicationView{}, fmt.Errorf("leyendo la aplicación: %w", err)
	}
	return applicationView(row), nil
}

// withApplications carga las aplicaciones de toda la página en una sola consulta.
func (p payments) withApplications(ctx context.Context, org uuid.UUID, rows []db.ListPaymentsRow) ([]app.PaymentView, error) {
	out := make([]app.PaymentView, 0, len(rows))
	ids := make([]string, 0, len(rows))
	index := map[uuid.UUID]int{}
	for _, row := range rows {
		index[row.ID] = len(out)
		ids = append(ids, row.ID.String())
		out = append(out, app.PaymentView{
			ID: row.ID, CustomerID: row.CustomerID, ReceivedOn: civil.FromTime(row.ReceivedOn.Time), Amount: row.Amount,
			Currency: row.CurrencyCode, ExchangeRate: row.ExchangeRate, PaymentMethodCode: row.PaymentMethodCode,
			Reference: row.Reference.String, Notes: row.Notes.String, Status: payment.Status(row.Status),
			VoidReason: row.VoidReason.String, VoidedAt: tsPtr(row.VoidedAt), ReceivedByUserID: row.ReceivedByUserID.UUID,
			CreatedAt: row.CreatedAt.UTC(), Applications: []app.ApplicationView{},
		})
	}
	if len(ids) == 0 {
		return out, nil
	}
	apps, err := p.q.ListApplicationsForPayments(ctx, db.ListApplicationsForPaymentsParams{OrganizationID: org, PaymentIds: ids})
	if err != nil {
		return nil, fmt.Errorf("leyendo las aplicaciones de los pagos: %w", err)
	}
	for _, a := range apps {
		i := index[a.PaymentID]
		out[i].Applications = append(out[i].Applications, applicationView(db.GetApplicationViewRow(a)))
	}
	return out, nil
}
