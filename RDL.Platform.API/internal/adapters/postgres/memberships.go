package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/membership"
)

type memberships struct{ q *db.Queries }

func (r memberships) ListActiveForUser(ctx context.Context, userID uuid.UUID) ([]membership.Summary, error) {
	rows, err := r.q.ListActiveMembershipsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listando membresías: %w", err)
	}
	out := make([]membership.Summary, 0, len(rows))
	for _, row := range rows {
		roles := make([]membership.Role, 0, len(row.Roles))
		for _, r := range row.Roles {
			roles = append(roles, membership.Role(r))
		}
		out = append(out, membership.Summary{
			OrganizationID:     row.OrganizationID,
			LegalName:          row.LegalName,
			TradeName:          row.TradeName,
			OrganizationStatus: row.OrganizationStatus,
			Status:             membership.Status(row.Status),
			Roles:              roles,
			JoinedAt:           row.JoinedAt,
		})
	}
	return out, nil
}

func (r memberships) IsActiveMember(ctx context.Context, userID, organizationID uuid.UUID) (bool, error) {
	_, err := r.q.GetActiveMembership(ctx, db.GetActiveMembershipParams{OrganizationID: organizationID, UserID: userID})
	if isNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verificando membresía: %w", err)
	}
	return true, nil
}

func (r memberships) Add(ctx context.Context, organizationID, userID uuid.UUID) (uuid.UUID, error) {
	id, err := r.q.InsertMembership(ctx, db.InsertMembershipParams{OrganizationID: organizationID, UserID: userID})
	if isUniqueViolation(err, "organization_users_org_user_uk") {
		return uuid.Nil, fmt.Errorf("%w: el usuario ya es miembro de la organización", app.ErrConflict)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("creando membresía: %w", err)
	}
	return id, nil
}

func (r memberships) AssignRole(ctx context.Context, organizationID, membershipID uuid.UUID, role membership.Role, grantedBy uuid.UUID) error {
	if err := r.q.InsertMembershipRole(ctx, db.InsertMembershipRoleParams{
		OrganizationID: organizationID, OrganizationUserID: membershipID, RoleCode: string(role), GrantedByUserID: nullUUID(grantedBy),
	}); err != nil {
		return fmt.Errorf("asignando rol: %w", err)
	}
	return nil
}
