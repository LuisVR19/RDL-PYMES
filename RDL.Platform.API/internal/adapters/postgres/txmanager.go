package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/pkg/tenancy"
)

// TxManager implementa app.TxManager: una transacción por operación con la sesión RLS fijada al inicio.
type TxManager struct{ pool *pgxpool.Pool }

func NewTxManager(pool *pgxpool.Pool) *TxManager { return &TxManager{pool: pool} }

func (m *TxManager) WithinUserTx(ctx context.Context, userID uuid.UUID, fn func(context.Context, app.Tx) error) error {
	return inTx(ctx, m.pool, pgx.TxOptions{}, session{userID: userID}, func(q *db.Queries, _ pgx.Tx) error {
		return fn(ctx, &tx{q: q})
	})
}

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

func (t *tx) Users() app.UserRepository                 { return users{q: t.q} }
func (t *tx) Memberships() app.MembershipRepository     { return memberships{q: t.q} }
func (t *tx) Members() app.MemberRepository             { return members{q: t.q} }
func (t *tx) Invitations() app.InvitationRepository     { return invitations{q: t.q} }
func (t *tx) Branches() app.BranchRepository            { return branches{q: t.q} }
func (t *tx) Organizations() app.OrganizationRepository { return organizations{q: t.q} }
func (t *tx) Idempotency() app.IdempotencyStore         { return idempotency{q: t.q} }
func (t *tx) Audit() app.AuditRecorder                  { return audit{q: t.q} }
