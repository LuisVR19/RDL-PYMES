package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// CreateInvitation invita a una persona por email con un rol (owner, admin).
type CreateInvitation struct {
	tx  TxManager
	now func() time.Time
}

func NewCreateInvitation(tx TxManager) *CreateInvitation {
	return &CreateInvitation{tx: tx, now: time.Now}
}

type CreateInvitationResult struct {
	Invitation invitation.Invitation
	// Token en claro: solo se conoce al crearla. En una repetición idempotente viene vacío, porque la base
	// guarda únicamente su hash (docs/decisiones/0006-invitaciones.md).
	Token    string
	Replayed bool
}

func (uc *CreateInvitation) Execute(ctx context.Context, t tenancy.Context, idempotencyKey, email string, role membership.Role) (CreateInvitationResult, error) {
	if err := authorize(t, membership.PermInvitationsManage); err != nil {
		return CreateInvitationResult{}, err
	}
	if role == membership.RoleOwner && !slices.Contains(t.Roles(), string(membership.RoleOwner)) {
		return CreateInvitationResult{}, membership.ErrOwnerRequired
	}
	inv, token, tokenHash, err := invitation.New(t.OrganizationID(), email, role, t.UserID(), uc.now())
	if err != nil {
		return CreateInvitationResult{}, err
	}
	reqHash, err := requestHash(map[string]string{"email": inv.Email, "role": string(inv.Role)})
	if err != nil {
		return CreateInvitationResult{}, err
	}

	var res CreateInvitationResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		prev, err := tx.Idempotency().Claim(ctx, t.OrganizationID(), idempotencyKey, reqHash, IdempotencyTTL)
		if err != nil {
			return err
		}
		if prev != nil {
			if prev.RequestHash != reqHash {
				return ErrIdempotencyKeyReused
			}
			id, err := uuid.Parse(prev.Result["invitationId"])
			if err != nil {
				return fmt.Errorf("resultado idempotente corrupto: %w", err)
			}
			stored, err := tx.Invitations().GetForUpdate(ctx, t.OrganizationID(), id)
			res = CreateInvitationResult{Invitation: stored, Replayed: true}
			return err
		}

		member, err := tx.Invitations().IsActiveMemberEmail(ctx, t.OrganizationID(), inv.Email)
		if err != nil {
			return err
		}
		if member {
			return fmt.Errorf("%w: esa persona ya es miembro activo de la organización", ErrConflict)
		}
		if err := tx.Invitations().ExpireStalePending(ctx, t.OrganizationID(), inv.Email); err != nil {
			return err
		}
		created, err := tx.Invitations().Create(ctx, inv, tokenHash)
		if err != nil {
			return err
		}
		// El token nunca va a la auditoría ni a los logs.
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invitation.created", EntityType: "invitation", EntityID: created.ID,
			After: map[string]any{"email": created.Email, "role": created.Role, "expiresAt": created.ExpiresAt},
		}); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: reqHash, Status: http.StatusCreated, Result: map[string]string{"invitationId": created.ID.String()},
		}); err != nil {
			return err
		}
		res = CreateInvitationResult{Invitation: created, Token: token}
		return nil
	})
	// TODO(notificaciones): enviar el enlace por email cuando exista el servicio de notificaciones;
	// hoy la API devuelve el token para que el admin lo comparta.
	return res, err
}

// ListInvitations lista las invitaciones de la organización activa (owner, admin).
type ListInvitations struct{ tx TxManager }

func NewListInvitations(tx TxManager) *ListInvitations { return &ListInvitations{tx: tx} }

type InvitationPage struct {
	Items []invitation.Invitation
	Next  *PageCursor
}

func (uc *ListInvitations) Execute(ctx context.Context, t tenancy.Context, q InvitationQuery) (InvitationPage, error) {
	if err := authorize(t, membership.PermInvitationsManage); err != nil {
		return InvitationPage{}, err
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageSize
	}
	q.Limit = min(q.Limit, MaxPageSize)

	var page InvitationPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Invitations().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		if len(items) > q.Limit {
			items = items[:q.Limit]
			last := items[len(items)-1]
			page.Next = &PageCursor{At: last.CreatedAt, ID: last.ID}
		}
		page.Items = items
		return nil
	})
	return page, err
}

// RevokeInvitation anula una invitación pendiente (owner, admin). Revocar una ya revocada o vencida no hace nada.
type RevokeInvitation struct {
	tx  TxManager
	now func() time.Time
}

func NewRevokeInvitation(tx TxManager) *RevokeInvitation {
	return &RevokeInvitation{tx: tx, now: time.Now}
}

func (uc *RevokeInvitation) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) error {
	if err := authorize(t, membership.PermInvitationsManage); err != nil {
		return err
	}
	return uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		inv, err := tx.Invitations().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		switch inv.EffectiveStatus(uc.now()) {
		case invitation.StatusRevoked, invitation.StatusExpired:
			return nil
		case invitation.StatusAccepted:
			return invitation.ErrNotPending
		}
		if err := tx.Invitations().Revoke(ctx, t.OrganizationID(), id); err != nil {
			return err
		}
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invitation.revoked", EntityType: "invitation", EntityID: id,
			Before: map[string]any{"status": invitation.StatusPending},
			After:  map[string]any{"status": invitation.StatusRevoked},
		})
	})
}

// AcceptInvitation une al usuario autenticado a la organización de la invitación. Puede ya pertenecer a otras
// organizaciones (caso contador). La autoridad para operar en esa organización sale del token de un solo uso
// más el email verificado del usuario, nunca de un id enviado por el cliente.
type AcceptInvitation struct {
	tx  TxManager
	now func() time.Time
}

func NewAcceptInvitation(tx TxManager) *AcceptInvitation {
	return &AcceptInvitation{tx: tx, now: time.Now}
}

type AcceptInvitationResult struct {
	OrganizationID uuid.UUID
	Role           membership.Role
	// ActiveOrganizationSelected indica que era la primera organización del usuario y quedó activa.
	ActiveOrganizationSelected bool
	Replayed                   bool
}

func (uc *AcceptInvitation) Execute(ctx context.Context, id tenancy.Identity, idempotencyKey, token string) (AcceptInvitationResult, error) {
	if !invitation.LooksLikeToken(token) {
		return AcceptInvitationResult{}, ErrNotFound
	}
	var u user.User
	if err := uc.tx.WithinUserTx(ctx, uuid.Nil, func(ctx context.Context, tx Tx) error {
		var err error
		u, err = provision(ctx, tx, id)
		return err
	}); err != nil {
		return AcceptInvitationResult{}, err
	}
	if !u.IsActive() {
		return AcceptInvitationResult{}, user.ErrDisabled
	}

	tokenHash := invitation.HashToken(token)
	// 1) Con la sesión del invitado: RLS solo le deja ver invitaciones a su email. Así se descubre la organización.
	var found invitation.Invitation
	if err := uc.tx.WithinUserTx(ctx, u.ID, func(ctx context.Context, tx Tx) error {
		var err error
		found, err = tx.Invitations().FindByTokenHash(ctx, tokenHash)
		return err
	}); err != nil {
		return AcceptInvitationResult{}, err
	}

	// 2) Dentro de esa organización: se revalida todo bajo bloqueo y se crea la membresía de forma atómica.
	joining := tenancy.NewContext(u.ID, id.Subject, found.OrganizationID, nil)
	var res AcceptInvitationResult
	err := uc.tx.WithinTenantTx(ctx, joining, func(ctx context.Context, tx Tx) error {
		org := found.OrganizationID
		prev, err := tx.Idempotency().Claim(ctx, org, idempotencyKey, tokenHash, IdempotencyTTL)
		if err != nil {
			return err
		}
		if prev != nil {
			if prev.RequestHash != tokenHash {
				return ErrIdempotencyKeyReused
			}
			res = AcceptInvitationResult{OrganizationID: org, Role: membership.Role(prev.Result["role"]), Replayed: true}
			return nil
		}

		inv, err := tx.Invitations().GetByTokenHashForUpdate(ctx, org, tokenHash)
		if err != nil {
			return err
		}
		if err := inv.CheckAcceptable(u.Email, uc.now()); err != nil {
			if errors.Is(err, invitation.ErrEmailMismatch) {
				return ErrNotFound
			}
			return err
		}

		action := "membership.created"
		membershipID, status, err := tx.Members().FindByUser(ctx, org, u.ID)
		switch {
		case errors.Is(err, ErrNotFound):
			if membershipID, err = tx.Memberships().Add(ctx, org, u.ID); err != nil {
				return err
			}
			if err := tx.Memberships().AssignRole(ctx, org, membershipID, inv.Role, inv.InvitedByUserID); err != nil {
				return err
			}
		case err != nil:
			return err
		case status == membership.StatusActive:
			return fmt.Errorf("%w: ya es miembro activo de esta organización", ErrConflict)
		default:
			// Membresía suspendida: la invitación la reactiva con el rol invitado.
			action = "membership.reactivated"
			if err := tx.Members().SetStatus(ctx, org, membershipID, membership.StatusActive); err != nil {
				return err
			}
			if err := tx.Members().ReplaceRoles(ctx, org, membershipID, []membership.Role{inv.Role}, inv.InvitedByUserID); err != nil {
				return err
			}
		}
		if err := tx.Invitations().MarkAccepted(ctx, org, inv.ID, u.ID); err != nil {
			return err
		}
		selected := false
		if u.ActiveOrganizationID == uuid.Nil {
			if err := tx.Users().SetActiveOrganization(ctx, u.ID, org); err != nil {
				return err
			}
			selected = true
		}

		for _, e := range []AuditEvent{
			{OrganizationID: org, ActorUserID: u.ID, Action: "invitation.accepted", EntityType: "invitation", EntityID: inv.ID,
				Before: map[string]any{"status": invitation.StatusPending}, After: map[string]any{"status": invitation.StatusAccepted}},
			{OrganizationID: org, ActorUserID: u.ID, Action: action, EntityType: "membership", EntityID: membershipID,
				After: map[string]any{"userId": u.ID, "roles": []membership.Role{inv.Role}, "invitationId": inv.ID}},
		} {
			if err := tx.Audit().Record(ctx, e); err != nil {
				return err
			}
		}
		if err := tx.Idempotency().Complete(ctx, org, idempotencyKey, IdempotencyRecord{
			RequestHash: tokenHash, Status: http.StatusOK,
			Result: map[string]string{"organizationId": org.String(), "role": string(inv.Role)},
		}); err != nil {
			return err
		}
		res = AcceptInvitationResult{OrganizationID: org, Role: inv.Role, ActiveOrganizationSelected: selected}
		return nil
	})
	return res, err
}
