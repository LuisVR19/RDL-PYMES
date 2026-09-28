package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
)

type receivableStore struct{ q *db.Queries }

func (s receivableStore) FindByInvoice(ctx context.Context, org, invoiceID uuid.UUID) (app.ReceivableRecord, bool, error) {
	row, err := s.q.FindReceivableByInvoice(ctx, db.FindReceivableByInvoiceParams{OrganizationID: org, SourceInvoiceID: invoiceID})
	if isNoRows(err) {
		return app.ReceivableRecord{}, false, nil
	}
	if err != nil {
		return app.ReceivableRecord{}, false, fmt.Errorf("buscando la cuenta de la factura: %w", err)
	}
	original, err := decimal.NewFromString(row.OriginalAmount)
	if err != nil {
		return app.ReceivableRecord{}, false, fmt.Errorf("monto original de la cuenta %s: %w", row.ID, err)
	}
	return app.ReceivableRecord{ID: row.ID, CustomerID: row.CustomerID, Currency: row.CurrencyCode, Original: original}, true, nil
}

// Create inserta la cuenta. Saldo y estado los fija receivables_guard en el alta (saldo = original, open).
func (s receivableStore) Create(ctx context.Context, org uuid.UUID, n app.NewReceivable) error {
	r := n.Receivable
	inv := r.Invoice()
	err := s.q.InsertReceivable(ctx, db.InsertReceivableParams{
		ID:                           r.ID(),
		OrganizationID:               org,
		SourceInvoiceID:              inv.ID,
		SourceEventID:                n.SourceEventID,
		CustomerID:                   inv.CustomerID,
		CustomerIdentificationNumber: n.CustomerIdentification,
		CustomerLegalName:            n.CustomerLegalName,
		DocumentNumber:               n.InvoiceNumber,
		CurrencyCode:                 inv.Currency,
		OriginalAmount:               inv.Original.String(),
		IssuedOn:                     pgtype.Date{Time: inv.IssuedOn.Time(), Valid: true},
		DueOn:                        pgtype.Date{Time: inv.DueOn.Time(), Valid: true},
		SaleConditionCode:            n.SaleConditionCode,
	})
	if err != nil {
		// Una violación de unicidad (otro evento de la misma factura confirmó antes) se reintenta: en el siguiente
		// intento FindByInvoice la ve y decide si es un reenvío o un conflicto.
		return fmt.Errorf("creando la cuenta por cobrar: %w", err)
	}
	return nil
}
