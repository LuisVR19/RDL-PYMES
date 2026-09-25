package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/tenancy"
)

// TxManager implementa app.TxManager: una transacción por operación con la sesión RLS fijada al inicio.
// WithinServiceTx (consumidor, solo organización) está en events.go.
// El validador de contratos es el del outbox: todo evento se valida contra su schema antes de escribirse.
type TxManager struct {
	pool      *pgxpool.Pool
	validator *contracts.Validator
}

func NewTxManager(pool *pgxpool.Pool, validator *contracts.Validator) *TxManager {
	return &TxManager{pool: pool, validator: validator}
}

func (m *TxManager) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, app.Tx) error) error {
	if t.OrganizationID() == uuid.Nil || t.UserID() == uuid.Nil {
		// Defensa en profundidad: una transacción de tenant sin organización dejaría RLS devolviendo vacío
		// y ocultaría un error de wiring.
		return errors.New("postgres: TenantContext incompleto")
	}
	s := session{organizationID: t.OrganizationID(), userID: t.UserID()}
	return inTx(ctx, m.pool, pgx.TxOptions{}, s, func(q *db.Queries, _ pgx.Tx) error {
		return fn(ctx, &tx{q: q, validator: m.validator})
	})
}

type tx struct {
	q         *db.Queries
	validator *contracts.Validator
}

func (t *tx) Organizations() app.OrganizationReader { return organizations{q: t.q} }
func (t *tx) Receivables() app.ReceivableReader     { return receivables{q: t.q} }
func (t *tx) Payments() app.PaymentReader           { return payments{q: t.q} }
func (t *tx) Ledger() app.Ledger                    { return ledger{q: t.q} }
func (t *tx) Collection() app.CollectionStore       { return collectionStore{q: t.q} }
func (t *tx) Idempotency() app.IdempotencyStore     { return idempotency{q: t.q} }
func (t *tx) Audit() app.AuditRecorder              { return audit{q: t.q} }
func (t *tx) Outbox() app.Outbox                    { return outbox{q: t.q, validator: t.validator} }
