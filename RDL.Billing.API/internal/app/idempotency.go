package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// IdempotencyTTL: vigencia de una Idempotency-Key (convenciones §7).
const IdempotencyTTL = 24 * time.Hour

// requestHash identifica el contenido del comando ya normalizado por el dominio, no los bytes del body: dos
// reintentos con distinto espaciado o con campos en otro orden son la misma petición.
func requestHash(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("calculando hash de la petición: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// replayed resuelve la reserva de una Idempotency-Key. Devuelve el id guardado en resultKey si la clave ya se usó
// con la misma petición, uuid.Nil si esta petición la acaba de reservar, o ErrIdempotencyKeyReused.
func replayed(ctx context.Context, tx Tx, org uuid.UUID, key, hash, resultKey string) (uuid.UUID, error) {
	prev, err := tx.Idempotency().Claim(ctx, org, key, hash, IdempotencyTTL)
	if err != nil || prev == nil {
		return uuid.Nil, err
	}
	if prev.RequestHash != hash {
		return uuid.Nil, ErrIdempotencyKeyReused
	}
	id, err := uuid.Parse(prev.Result[resultKey])
	if err != nil {
		return uuid.Nil, fmt.Errorf("resultado idempotente corrupto: %w", err)
	}
	return id, nil
}
