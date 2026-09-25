package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/tenancy"
)

// TxManager implementa app.TxManager: una transacción por operación con la sesión RLS fijada al inicio.
// WithinServiceTx (consumidor, solo organización) llega en el incremento 4.
type TxManager struct{ pool *pgxpool.Pool }

func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

func (m *TxManager) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, app.Tx) error) error {
	if t.OrganizationID() == uuid.Nil || t.UserID() == uuid.Nil {
		// Defensa en profundidad: una transacción de tenant sin organización dejaría RLS devolviendo vacío
		// y ocultaría un error de wiring.
		return errors.New("postgres: TenantContext incompleto")
	}
	s := session{organizationID: t.OrganizationID(), userID: t.UserID()}
	return inTx(ctx, m.pool, pgx.TxOptions{}, s, func(q *db.Queries, _ pgx.Tx) error {
		return fn(ctx, &tx{q: q})
	})
}

type tx struct{ q *db.Queries }

func (t *tx) Organizations() app.OrganizationReader { return organizations{q: t.q} }
func (t *tx) Receivables() app.ReceivableReader     { return receivables{q: t.q} }
