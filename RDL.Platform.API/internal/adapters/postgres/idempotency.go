package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
)

type idempotency struct{ q *db.Queries }

func (s idempotency) Claim(ctx context.Context, org uuid.UUID, key, hash string, ttl time.Duration) (*app.IdempotencyRecord, error) {
	// Una clave vencida se puede reutilizar: se borra antes de intentar reservarla.
	if err := s.q.DeleteExpiredIdempotencyKey(ctx, db.DeleteExpiredIdempotencyKeyParams{OrganizationID: org, IdempotencyKey: key}); err != nil {
		return nil, fmt.Errorf("limpiando clave vencida: %w", err)
	}
	_, err := s.q.ClaimIdempotencyKey(ctx, db.ClaimIdempotencyKeyParams{
		OrganizationID: org, IdempotencyKey: key, RequestHash: hash, TtlSeconds: int64(ttl / time.Second),
	})
	if err == nil {
		return nil, nil // reservada por esta petición
	}
	if !isNoRows(err) {
		return nil, fmt.Errorf("reservando Idempotency-Key: %w", err)
	}

	row, err := s.q.GetIdempotencyKey(ctx, db.GetIdempotencyKeyParams{OrganizationID: org, IdempotencyKey: key})
	if err != nil {
		return nil, fmt.Errorf("leyendo Idempotency-Key: %w", err)
	}
	rec := &app.IdempotencyRecord{RequestHash: row.RequestHash, Status: int(row.ResponseStatus.Int32)}
	if len(row.ResponseBody) > 0 {
		if err := json.Unmarshal(row.ResponseBody, &rec.Result); err != nil {
			return nil, fmt.Errorf("leyendo resultado idempotente: %w", err)
		}
	}
	return rec, nil
}

func (s idempotency) Complete(ctx context.Context, org uuid.UUID, key string, r app.IdempotencyRecord) error {
	body, err := json.Marshal(r.Result)
	if err != nil {
		return fmt.Errorf("serializando resultado idempotente: %w", err)
	}
	status := pgtype.Int4{Int32: int32(r.Status), Valid: true} // #nosec G115 -- códigos HTTP (100-599)
	if err := s.q.CompleteIdempotencyKey(ctx, db.CompleteIdempotencyKeyParams{
		OrganizationID: org, IdempotencyKey: key, ResponseStatus: status, ResponseBody: string(body),
	}); err != nil {
		return fmt.Errorf("completando Idempotency-Key: %w", err)
	}
	return nil
}
