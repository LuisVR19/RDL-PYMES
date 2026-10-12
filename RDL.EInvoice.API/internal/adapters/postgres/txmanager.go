package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/einvoice-api/internal/adapters/postgres/db"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/pkg/tenancy"
)

// TxManager implementa app.TxManager: una transacción por operación con la sesión RLS fijada al inicio.
// WithinServiceTx (consumidor y worker, solo organización) está en events.go.
// El validador de contratos es el del outbox (incremento 10): todo evento se valida contra su schema antes de escribirse.
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
	return inTx(ctx, m.pool, pgx.TxOptions{}, s, func(q *db.Queries, ptx pgx.Tx) error {
		return fn(ctx, &tx{q: q, tx: ptx})
	})
}

type tx struct {
	q  *db.Queries
	tx pgx.Tx
}

func (t *tx) FiscalProfiles() app.FiscalProfileReader { return fiscalProfiles{q: t.q} }
func (t *tx) FiscalStatuses() app.FiscalStatusReader  { return fiscalStatuses{q: t.q} }
func (t *tx) FiscalConfig() app.FiscalConfigStore     { return fiscalConfig{q: t.q} }
func (t *tx) Catalogs() app.CatalogReader             { return catalogs{tx: t.tx} }
func (t *tx) Audit() app.AuditRecorder                { return audit{q: t.q} }
