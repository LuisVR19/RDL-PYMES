package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/pkg/tenancy"
)

// MembershipResolver revalida contra core, en cada request (con la caché de tenancy delante), que el sujeto
// del token siga siendo miembro activo de una organización activa. Billing solo lee core: los GRANT están en
// docs/decisiones/0002-propuesta-database-platform.md.
type MembershipResolver struct{ pool *pgxpool.Pool }

func NewMembershipResolver(pool *pgxpool.Pool) *MembershipResolver {
	return &MembershipResolver{pool: pool}
}

func (r *MembershipResolver) ActiveMembership(ctx context.Context, subject string, org uuid.UUID) (tenancy.Membership, error) {
	var m tenancy.Membership
	err := inTx(ctx, r.pool, pgx.TxOptions{AccessMode: pgx.ReadOnly}, session{}, func(q *db.Queries, tx pgx.Tx) error {
		userID, err := q.FindActiveUserIDBySubject(ctx, subject)
		if isNoRows(err) {
			return tenancy.ErrNoMembership
		}
		if err != nil {
			return fmt.Errorf("buscando usuario: %w", err)
		}
		// La membresía se lee bajo RLS como el propio usuario: organization_users_own solo expone sus filas.
		if err := setSession(ctx, tx, session{userID: userID}); err != nil {
			return err
		}
		row, err := q.GetActiveMembership(ctx, db.GetActiveMembershipParams{OrganizationID: org, UserID: userID})
		if isNoRows(err) {
			return tenancy.ErrNoMembership
		}
		if err != nil {
			return fmt.Errorf("leyendo membresía: %w", err)
		}
		m = tenancy.Membership{UserID: row.UserID, Roles: row.Roles}
		return nil
	})
	return m, err
}
