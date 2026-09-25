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
	"rdl/receivables-api/internal/domain/collection"
)

type collectionStore struct{ q *db.Queries }

func (s collectionStore) CreateFollowUp(ctx context.Context, org uuid.UUID, f app.FollowUpView) error {
	params := db.InsertFollowUpParams{
		ID: f.ID, OrganizationID: org, ReceivableID: f.ReceivableID, FollowupType: string(f.Type), Notes: f.Notes,
		PerformedAt: f.PerformedAt, PerformedByUserID: nullUUID(f.PerformedBy),
	}
	if f.NextActionOn != nil {
		params.NextActionOn = pgtype.Date{Time: f.NextActionOn.Time(), Valid: true}
	}
	if err := s.q.InsertFollowUp(ctx, params); err != nil {
		return fmt.Errorf("registrando el seguimiento: %w", err)
	}
	return nil
}

func (s collectionStore) GetFollowUp(ctx context.Context, org, id uuid.UUID) (app.FollowUpView, error) {
	row, err := s.q.GetFollowUp(ctx, db.GetFollowUpParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.FollowUpView{}, app.ErrNotFound
	}
	if err != nil {
		return app.FollowUpView{}, fmt.Errorf("leyendo el seguimiento: %w", err)
	}
	return followUpView(db.ListFollowUpsRow(row)), nil
}

func (s collectionStore) ListFollowUps(ctx context.Context, org, receivableID uuid.UUID) ([]app.FollowUpView, error) {
	rows, err := s.q.ListFollowUps(ctx, db.ListFollowUpsParams{OrganizationID: org, ReceivableID: receivableID})
	if err != nil {
		return nil, fmt.Errorf("listando seguimientos: %w", err)
	}
	out := make([]app.FollowUpView, 0, len(rows))
	for _, r := range rows {
		out = append(out, followUpView(r))
	}
	return out, nil
}

func followUpView(r db.ListFollowUpsRow) app.FollowUpView {
	v := app.FollowUpView{
		ID: r.ID, ReceivableID: r.ReceivableID, Type: collection.FollowUpType(r.FollowupType), Notes: r.Notes,
		PerformedAt: r.PerformedAt.UTC(), PerformedBy: r.PerformedByUserID.UUID, CreatedAt: r.CreatedAt.UTC(),
	}
	if r.NextActionOn.Valid {
		d := civil.FromTime(r.NextActionOn.Time)
		v.NextActionOn = &d
	}
	return v
}

func (s collectionStore) CreatePromise(ctx context.Context, org uuid.UUID, p app.PromiseView) error {
	err := s.q.InsertPromise(ctx, db.InsertPromiseParams{
		ID: p.ID, OrganizationID: org, ReceivableID: p.ReceivableID, FollowupID: nullUUID(p.FollowUpID),
		PromisedAmount: p.PromisedAmount.String(), PromisedOn: pgtype.Date{Time: p.PromisedOn.Time(), Valid: true},
		CreatedByUserID: nullUUID(p.CreatedBy),
	})
	if err != nil {
		return fmt.Errorf("registrando la promesa: %w", err)
	}
	return nil
}

func (s collectionStore) GetPromise(ctx context.Context, org, id uuid.UUID) (app.PromiseView, error) {
	row, err := s.q.GetPromise(ctx, db.GetPromiseParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.PromiseView{}, app.ErrNotFound
	}
	if err != nil {
		return app.PromiseView{}, fmt.Errorf("leyendo la promesa: %w", err)
	}
	return promiseView(db.ListPromisesRow(row))
}

func (s collectionStore) LockPromise(ctx context.Context, org, id uuid.UUID) (app.PromiseView, error) {
	row, err := s.q.LockPromise(ctx, db.LockPromiseParams{OrganizationID: org, ID: id})
	if isNoRows(err) {
		return app.PromiseView{}, app.ErrNotFound
	}
	if err != nil {
		return app.PromiseView{}, fmt.Errorf("bloqueando la promesa: %w", err)
	}
	return promiseView(db.ListPromisesRow(row))
}

func (s collectionStore) ListPromises(ctx context.Context, org, receivableID uuid.UUID) ([]app.PromiseView, error) {
	rows, err := s.q.ListPromises(ctx, db.ListPromisesParams{OrganizationID: org, ReceivableID: receivableID})
	if err != nil {
		return nil, fmt.Errorf("listando promesas: %w", err)
	}
	out := make([]app.PromiseView, 0, len(rows))
	for _, r := range rows {
		v, err := promiseView(r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s collectionStore) SetPromiseStatus(ctx context.Context, org, id uuid.UUID, status collection.PromiseStatus) error {
	n, err := s.q.UpdatePromiseStatus(ctx, db.UpdatePromiseStatusParams{Status: string(status), OrganizationID: org, ID: id})
	if err != nil {
		return fmt.Errorf("cerrando la promesa: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("cerrando la promesa %s: %w", id, errNotUpdated)
	}
	return nil
}

func promiseView(r db.ListPromisesRow) (app.PromiseView, error) {
	amt, err := decimal.NewFromString(r.PromisedAmount)
	if err != nil {
		return app.PromiseView{}, fmt.Errorf("monto de la promesa %s: %w", r.ID, err)
	}
	return app.PromiseView{
		ID: r.ID, ReceivableID: r.ReceivableID, FollowUpID: r.FollowupID.UUID, PromisedAmount: amt,
		PromisedOn: civil.FromTime(r.PromisedOn.Time), Status: collection.PromiseStatus(r.Status),
		CreatedBy: r.CreatedByUserID.UUID, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
	}, nil
}
