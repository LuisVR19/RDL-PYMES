package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

// ledger implementa app.Ledger. Carga los agregados desde las filas y nunca escribe saldo ni estado.
type ledger struct{ q *db.Queries }

func (l ledger) LockPayment(ctx context.Context, org, id uuid.UUID) (*payment.Payment, error) {
	row, err := l.q.LockPayment(ctx, db.LockPaymentParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return nil, app.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("bloqueando el pago: %w", err)
	}
	amt, err := decimal.NewFromString(row.Amount)
	if err != nil {
		return nil, fmt.Errorf("monto del pago %s: %w", id, err)
	}
	rows, err := l.q.ListPaymentApplications(ctx, db.ListPaymentApplicationsParams{OrganizationID: org, PaymentID: id})
	if err != nil {
		return nil, fmt.Errorf("cargando las aplicaciones del pago: %w", err)
	}
	apps := make([]payment.Application, 0, len(rows))
	for _, a := range rows {
		v, err := decimal.NewFromString(a.Amount)
		if err != nil {
			return nil, fmt.Errorf("monto de la aplicación %s: %w", a.ID, err)
		}
		apps = append(apps, payment.Application{
			ID: a.ID, ReceivableID: a.ReceivableID, Amount: v, AppliedAt: a.AppliedAt,
			ReversedAt: tsOrZero(a.ReversedAt), ReversalReason: a.ReversalReason.String,
		})
	}
	return payment.Rehydrate(
		payment.Data{ID: row.ID, CustomerID: row.CustomerID, Currency: row.CurrencyCode, Amount: amt},
		payment.Status(row.Status), row.VoidReason.String, tsOrZero(row.VoidedAt), apps)
}

func (l ledger) LockReceivable(ctx context.Context, org, id uuid.UUID) (*receivable.Receivable, app.ReceivableMeta, error) {
	row, err := l.q.LockReceivable(ctx, db.LockReceivableParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return nil, app.ReceivableMeta{}, app.ErrNotFound
	}
	if err != nil {
		return nil, app.ReceivableMeta{}, fmt.Errorf("bloqueando la cuenta: %w", err)
	}
	original, err := decimal.NewFromString(row.OriginalAmount)
	if err != nil {
		return nil, app.ReceivableMeta{}, fmt.Errorf("monto original de la cuenta %s: %w", id, err)
	}
	adjRows, err := l.q.ListReceivableAdjustments(ctx, db.ListReceivableAdjustmentsParams{OrganizationID: org, ReceivableID: id})
	if err != nil {
		return nil, app.ReceivableMeta{}, fmt.Errorf("cargando los ajustes: %w", err)
	}
	adjs := make([]receivable.Adjustment, 0, len(adjRows))
	for _, a := range adjRows {
		v, err := decimal.NewFromString(a.Amount)
		if err != nil {
			return nil, app.ReceivableMeta{}, fmt.Errorf("monto del ajuste %s: %w", a.ID, err)
		}
		adjs = append(adjs, receivable.Adjustment{
			ID: a.ID, Type: receivable.AdjustmentType(a.AdjustmentType), Amount: v,
			SourceDocumentID: a.SourceDocumentID.UUID, SourceEventID: a.SourceEventID.UUID,
		})
	}
	appRows, err := l.q.ListReceivableApplications(ctx, db.ListReceivableApplicationsParams{OrganizationID: org, ReceivableID: id})
	if err != nil {
		return nil, app.ReceivableMeta{}, fmt.Errorf("cargando las aplicaciones: %w", err)
	}
	apps := make([]receivable.Application, 0, len(appRows))
	for _, a := range appRows {
		v, err := decimal.NewFromString(a.Amount)
		if err != nil {
			return nil, app.ReceivableMeta{}, fmt.Errorf("monto de la aplicación %s: %w", a.ID, err)
		}
		apps = append(apps, receivable.Application{
			ID: a.ID, PaymentID: a.PaymentID, Amount: v, AppliedAt: a.AppliedAt,
			ReversedAt: tsOrZero(a.ReversedAt), ReversalReason: a.ReversalReason.String,
		})
	}
	r, err := receivable.Rehydrate(row.ID, receivable.Invoice{
		ID: row.SourceInvoiceID, CustomerID: row.CustomerID, Currency: row.CurrencyCode, Original: original,
		IssuedOn: civil.FromTime(row.IssuedOn.Time), DueOn: civil.FromTime(row.DueOn.Time),
	}, adjs, apps)
	if err != nil {
		return nil, app.ReceivableMeta{}, err
	}
	// La base y el agregado deben coincidir antes de tocar nada (misma fórmula, 00004).
	balance, err := decimal.NewFromString(row.BalanceAmount)
	if err != nil {
		return nil, app.ReceivableMeta{}, fmt.Errorf("saldo de la cuenta %s: %w", id, err)
	}
	if !balance.Equal(r.Balance()) || receivable.Status(row.Status) != r.Status() {
		return nil, app.ReceivableMeta{}, fmt.Errorf("%w: cuenta %s, base %s %s, agregado %s %s", app.ErrInconsistentState,
			id, balance, row.Status, r.Balance(), r.Status())
	}
	return r, app.ReceivableMeta{DocumentNumber: row.DocumentNumber}, nil
}

func (l ledger) ActivePaymentIDs(ctx context.Context, org, receivableID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := l.q.ListActivePaymentIDsForReceivable(ctx, db.ListActivePaymentIDsForReceivableParams{
		OrganizationID: org, ReceivableID: receivableID,
	})
	if err != nil {
		return nil, fmt.Errorf("leyendo los pagos de la cuenta: %w", err)
	}
	return ids, nil
}

func (l ledger) FindApplication(ctx context.Context, org, id uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := l.q.GetApplication(ctx, db.GetApplicationParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return uuid.Nil, uuid.Nil, app.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("buscando la aplicación: %w", err)
	}
	return row.PaymentID, row.ReceivableID, nil
}

func (l ledger) InsertPayment(ctx context.Context, org uuid.UUID, n app.NewPayment) error {
	p := n.Payment
	err := l.q.InsertPayment(ctx, db.InsertPaymentParams{
		ID: p.ID(), OrganizationID: org, CustomerID: p.CustomerID(),
		ReceivedOn: pgtype.Date{Time: n.ReceivedOn.Time(), Valid: true}, Amount: p.Amount().String(),
		CurrencyCode: p.Currency(), ExchangeRate: n.ExchangeRate.String(), PaymentMethodCode: n.PaymentMethodCode,
		Reference: text(n.Reference), Notes: text(n.Notes), ReceivedByUserID: nullUUID(n.ReceivedBy),
	})
	if err != nil {
		return fmt.Errorf("registrando el pago: %w", err)
	}
	return nil
}

func (l ledger) InsertApplication(ctx context.Context, org uuid.UUID, a app.NewApplication) error {
	err := l.q.InsertApplication(ctx, db.InsertApplicationParams{
		ID: a.ID, OrganizationID: org, PaymentID: a.PaymentID, ReceivableID: a.ReceivableID,
		Amount: a.Amount.String(), AppliedAt: a.AppliedAt, AppliedByUserID: nullUUID(a.AppliedBy),
	})
	if err != nil {
		return fmt.Errorf("registrando la aplicación: %w", err)
	}
	return nil
}

var errNotUpdated = errors.New("la fila no estaba en el estado esperado")

func (l ledger) ReverseApplication(ctx context.Context, org, id uuid.UUID, reason string, at time.Time, by uuid.UUID) error {
	n, err := l.q.ReverseApplication(ctx, db.ReverseApplicationParams{
		ReversedAt: pgtype.Timestamptz{Time: at, Valid: true}, ReversedByUserID: nullUUID(by),
		ReversalReason: pgtype.Text{String: reason, Valid: true}, OrganizationID: org, ID: id,
	})
	if err != nil {
		return fmt.Errorf("revirtiendo la aplicación: %w", err)
	}
	if n != 1 {
		// El agregado ya validó bajo bloqueo; si la fila no cambió, algo escribió sin bloquear.
		return fmt.Errorf("revirtiendo la aplicación %s: %w", id, errNotUpdated)
	}
	return nil
}

func (l ledger) VoidPayment(ctx context.Context, org, id uuid.UUID, reason string, at time.Time) error {
	n, err := l.q.VoidPayment(ctx, db.VoidPaymentParams{
		VoidReason: pgtype.Text{String: reason, Valid: true}, VoidedAt: pgtype.Timestamptz{Time: at, Valid: true},
		OrganizationID: org, ID: id,
	})
	if err != nil {
		return fmt.Errorf("anulando el pago: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("anulando el pago %s: %w", id, errNotUpdated)
	}
	return nil
}

func (l ledger) InsertAdjustment(ctx context.Context, org, receivableID uuid.UUID, adj receivable.Adjustment, reason string) error {
	err := l.q.InsertAdjustment(ctx, db.InsertAdjustmentParams{
		ID: adj.ID, OrganizationID: org, ReceivableID: receivableID, AdjustmentType: string(adj.Type),
		SourceDocumentID: nullUUID(adj.SourceDocumentID), SourceEventID: nullUUID(adj.SourceEventID),
		Amount: adj.Amount.String(), Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("registrando el ajuste: %w", err)
	}
	return nil
}

func (l ledger) ReceivableState(ctx context.Context, org, id uuid.UUID) (app.ReceivableState, error) {
	row, err := l.q.GetReceivableState(ctx, db.GetReceivableStateParams{OrganizationID: org, ID: id})
	if err != nil {
		return app.ReceivableState{}, fmt.Errorf("leyendo el saldo de la cuenta: %w", err)
	}
	balance, err := decimal.NewFromString(row.BalanceAmount)
	if err != nil {
		return app.ReceivableState{}, fmt.Errorf("saldo de la cuenta %s: %w", id, err)
	}
	return app.ReceivableState{Balance: balance, Status: receivable.Status(row.Status), SettledAt: tsPtr(row.SettledAt)}, nil
}

func tsOrZero(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

func tsPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC()
	return &v
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
