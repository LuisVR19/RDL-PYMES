package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/pkg/tenancy"
)

// SavepointTxManager es SOLO para pruebas contra la base (tests/isolation, pruebas de integración): corre cada caso de
// uso real en un savepoint de una transacción externa que la prueba revierte al final. Ejecuta exactamente el mismo
// código que TxManager (sesión RLS incluida) sin dejar datos: audit.audit_events es append-only y un evento de prueba
// en el outbox lo publicaría el worker. Nunca se usa en cmd/api.
type SavepointTxManager struct{ Outer pgx.Tx }

func (m SavepointTxManager) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, app.Tx) error) error {
	if t.OrganizationID() == uuid.Nil || t.UserID() == uuid.Nil {
		return errors.New("postgres: TenantContext incompleto")
	}
	sp, err := m.Outer.Begin(ctx)
	if err != nil {
		return err
	}
	if err := setSession(ctx, sp, session{organizationID: t.OrganizationID(), userID: t.UserID()}); err != nil {
		_ = sp.Rollback(ctx)
		return err
	}
	if err := fn(ctx, &tx{q: db.New(sp)}); err != nil {
		_ = sp.Rollback(ctx)
		return err
	}
	return sp.Commit(ctx)
}
