package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
)

type invitations struct{ q *db.Queries }

func (r invitations) Create(ctx context.Context, inv invitation.Invitation, tokenHash string) (invitation.Invitation, error) {
	row, err := r.q.InsertInvitation(ctx, db.InsertInvitationParams{
		OrganizationID: inv.OrganizationID, Email: inv.Email, RoleCode: string(inv.Role),
		TokenHash: tokenHash, ExpiresAt: inv.ExpiresAt, InvitedByUserID: inv.InvitedByUserID,
	})
	if isUniqueViolation(err, "invitations_pending_email_uk") {
		return invitation.Invitation{}, fmt.Errorf("%w: ya hay una invitación pendiente para ese email", app.ErrConflict)
	}
	if err != nil {
		return invitation.Invitation{}, fmt.Errorf("creando invitación: %w", err)
	}
	inv.ID, inv.CreatedAt = row.ID, row.CreatedAt
	return inv, nil
}

func (r invitations) List(ctx context.Context, org uuid.UUID, q app.InvitationQuery) ([]invitation.Invitation, error) {
	params := db.ListInvitationsParams{
		OrganizationID: org,
		Status:         pgtype.Text{String: string(q.Status), Valid: q.Status != ""},
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListInvitations(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando invitaciones: %w", err)
	}
	out := make([]invitation.Invitation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toInvitation(db.GetInvitationForUpdateRow(row)))
	}
	return out, nil
}

func (r invitations) GetForUpdate(ctx context.Context, org, id uuid.UUID) (invitation.Invitation, error) {
	row, err := r.q.GetInvitationForUpdate(ctx, db.GetInvitationForUpdateParams{OrganizationID: org, ID: id})
	return oneInvitation(row, err)
}

func (r invitations) FindByTokenHash(ctx context.Context, hash string) (invitation.Invitation, error) {
	row, err := r.q.FindInvitationByTokenHash(ctx, hash)
	return oneInvitation(db.GetInvitationForUpdateRow(row), err)
}

func (r invitations) GetByTokenHashForUpdate(ctx context.Context, org uuid.UUID, hash string) (invitation.Invitation, error) {
	row, err := r.q.GetInvitationByTokenHashForUpdate(ctx, db.GetInvitationByTokenHashForUpdateParams{OrganizationID: org, TokenHash: hash})
	return oneInvitation(db.GetInvitationForUpdateRow(row), err)
}

func (r invitations) Revoke(ctx context.Context, org, id uuid.UUID) error {
	if err := r.q.RevokeInvitation(ctx, db.RevokeInvitationParams{OrganizationID: org, ID: id}); err != nil {
		return fmt.Errorf("revocando invitación: %w", err)
	}
	return nil
}

func (r invitations) MarkAccepted(ctx context.Context, org, id, userID uuid.UUID) error {
	if err := r.q.AcceptInvitation(ctx, db.AcceptInvitationParams{OrganizationID: org, ID: id, UserID: nullUUID(userID)}); err != nil {
		return fmt.Errorf("aceptando invitación: %w", err)
	}
	return nil
}

func (r invitations) ExpireStalePending(ctx context.Context, org uuid.UUID, email string) error {
	if err := r.q.ExpirePendingInvitationForEmail(ctx, db.ExpirePendingInvitationForEmailParams{OrganizationID: org, Email: email}); err != nil {
		return fmt.Errorf("venciendo invitación anterior: %w", err)
	}
	return nil
}

func (r invitations) IsActiveMemberEmail(ctx context.Context, org uuid.UUID, email string) (bool, error) {
	ok, err := r.q.IsActiveMemberEmail(ctx, db.IsActiveMemberEmailParams{OrganizationID: org, Email: email})
	if err != nil {
		return false, fmt.Errorf("verificando miembro por email: %w", err)
	}
	return ok, nil
}

func oneInvitation(row db.GetInvitationForUpdateRow, err error) (invitation.Invitation, error) {
	if isNoRows(err) {
		return invitation.Invitation{}, app.ErrNotFound
	}
	if err != nil {
		return invitation.Invitation{}, fmt.Errorf("leyendo invitación: %w", err)
	}
	return toInvitation(row), nil
}

func toInvitation(r db.GetInvitationForUpdateRow) invitation.Invitation {
	inv := invitation.Invitation{
		ID: r.ID, OrganizationID: r.OrganizationID, Email: r.Email, Role: membership.Role(r.RoleCode),
		Status: invitation.Status(r.Status), ExpiresAt: r.ExpiresAt, InvitedByUserID: r.InvitedByUserID, CreatedAt: r.CreatedAt,
	}
	if r.AcceptedByUserID.Valid {
		inv.AcceptedByUserID = r.AcceptedByUserID.UUID
	}
	if r.AcceptedAt.Valid {
		inv.AcceptedAt = r.AcceptedAt.Time
	}
	return inv
}
