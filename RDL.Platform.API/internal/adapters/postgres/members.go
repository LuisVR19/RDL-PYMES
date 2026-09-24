package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/membership"
)

type members struct{ q *db.Queries }

func (r members) List(ctx context.Context, org uuid.UUID, q app.MemberQuery) ([]membership.Member, error) {
	params := db.ListMembersParams{
		OrganizationID: org,
		Status:         pgtype.Text{String: string(q.Status), Valid: q.Status != ""},
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.After != nil {
		params.AfterJoinedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListMembers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando miembros: %w", err)
	}
	out := make([]membership.Member, 0, len(rows))
	for _, row := range rows {
		out = append(out, membership.Member{
			MembershipID: row.MembershipID, UserID: row.UserID, Email: row.Email, FullName: row.FullName,
			Status: membership.Status(row.Status), Roles: toRoles(row.Roles), JoinedAt: row.JoinedAt,
		})
	}
	return out, nil
}

func (r members) GetForUpdate(ctx context.Context, org, userID uuid.UUID) (membership.Member, error) {
	row, err := r.q.GetMemberForUpdate(ctx, db.GetMemberForUpdateParams{OrganizationID: org, UserID: userID})
	if isNoRows(err) {
		return membership.Member{}, app.ErrNotFound
	}
	if err != nil {
		return membership.Member{}, fmt.Errorf("leyendo miembro: %w", err)
	}
	return membership.Member{
		MembershipID: row.MembershipID, UserID: row.UserID, Subject: row.ExternalSubject, Email: row.Email,
		FullName: row.FullName, Status: membership.Status(row.Status), Roles: toRoles(row.Roles), JoinedAt: row.JoinedAt,
	}, nil
}

func (r members) CountActiveOwners(ctx context.Context, org uuid.UUID) (int, error) {
	n, err := r.q.CountActiveOwners(ctx, org)
	if err != nil {
		return 0, fmt.Errorf("contando owners: %w", err)
	}
	return int(n), nil
}

func (r members) ReplaceRoles(ctx context.Context, org, membershipID uuid.UUID, roles []membership.Role, grantedBy uuid.UUID) error {
	if err := r.q.DeleteMembershipRoles(ctx, db.DeleteMembershipRolesParams{OrganizationID: org, OrganizationUserID: membershipID}); err != nil {
		return fmt.Errorf("quitando roles: %w", err)
	}
	for _, role := range roles {
		if err := r.q.InsertMembershipRole(ctx, db.InsertMembershipRoleParams{
			OrganizationID: org, OrganizationUserID: membershipID, RoleCode: string(role), GrantedByUserID: nullUUID(grantedBy),
		}); err != nil {
			return fmt.Errorf("asignando rol: %w", err)
		}
	}
	return nil
}

func (r members) SetStatus(ctx context.Context, org, membershipID uuid.UUID, status membership.Status) error {
	if err := r.q.UpdateMembershipStatus(ctx, db.UpdateMembershipStatusParams{
		OrganizationID: org, MembershipID: membershipID, Status: string(status),
	}); err != nil {
		return fmt.Errorf("cambiando estado de la membresía: %w", err)
	}
	return nil
}

func toRoles(codes []string) []membership.Role {
	out := make([]membership.Role, 0, len(codes))
	for _, c := range codes {
		out = append(out, membership.Role(c))
	}
	return out
}

func (r members) FindByUser(ctx context.Context, org, userID uuid.UUID) (uuid.UUID, membership.Status, error) {
	row, err := r.q.FindMembershipByUser(ctx, db.FindMembershipByUserParams{OrganizationID: org, UserID: userID})
	if isNoRows(err) {
		return uuid.Nil, "", app.ErrNotFound
	}
	if err != nil {
		return uuid.Nil, "", fmt.Errorf("buscando membresía: %w", err)
	}
	return row.ID, membership.Status(row.Status), nil
}
