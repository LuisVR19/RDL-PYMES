package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
)

type receivables struct{ q *db.Queries }

func (r receivables) List(ctx context.Context, org uuid.UUID, q app.ReceivableQuery) ([]app.ReceivableView, error) {
	params := db.ListReceivablesParams{
		OrganizationID: org,
		Status:         pgtype.Text{String: string(q.Status), Valid: q.Status != ""},
		CustomerID:     uuid.NullUUID{UUID: q.CustomerID, Valid: q.CustomerID != uuid.Nil},
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.Overdue != nil {
		params.Overdue = pgtype.Bool{Bool: *q.Overdue, Valid: true}
		params.Today = pgtype.Date{Time: q.Today.Time(), Valid: true}
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListReceivables(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando cuentas por cobrar: %w", err)
	}
	out := make([]app.ReceivableView, 0, len(rows))
	for _, row := range rows {
		v := app.ReceivableView{
			ID:                row.ID,
			SourceInvoiceID:   row.SourceInvoiceID,
			CustomerID:        row.CustomerID,
			CustomerLegalName: row.CustomerLegalName,
			DocumentNumber:    row.DocumentNumber,
			Currency:          row.CurrencyCode,
			OriginalAmount:    row.OriginalAmount,
			BalanceAmount:     row.BalanceAmount,
			IssuedOn:          civil.FromTime(row.IssuedOn.Time),
			DueOn:             civil.FromTime(row.DueOn.Time),
			Status:            receivable.Status(row.Status),
			CreatedAt:         row.CreatedAt,
		}
		if row.SettledAt.Valid {
			at := row.SettledAt.Time
			v.SettledAt = &at
		}
		out = append(out, v)
	}
	return out, nil
}
