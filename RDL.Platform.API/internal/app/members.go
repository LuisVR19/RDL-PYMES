package app

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/pkg/tenancy"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// ListMembers devuelve una página de miembros de la organización activa (owner, admin).
type ListMembers struct{ tx TxManager }

func NewListMembers(tx TxManager) *ListMembers { return &ListMembers{tx: tx} }

type MemberPage struct {
	Items []membership.Member
	Next  *PageCursor // nil = no hay más páginas
}

func (uc *ListMembers) Execute(ctx context.Context, t tenancy.Context, q MemberQuery) (MemberPage, error) {
	if err := authorize(t, membership.PermMembersRead); err != nil {
		return MemberPage{}, err
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageSize
	}
	q.Limit = min(q.Limit, MaxPageSize)

	var page MemberPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		// Se pide uno de más para saber si existe una página siguiente sin contar toda la tabla.
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Members().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		if len(items) > q.Limit {
			items = items[:q.Limit]
			last := items[len(items)-1]
			page.Next = &PageCursor{At: last.JoinedAt, ID: last.MembershipID}
		}
		page.Items = items
		return nil
	})
	return page, err
}

// UpdateMember cambia el rol o el estado (baja lógica) de un miembro de la organización activa (owner, admin).
type UpdateMember struct {
	tx    TxManager
	cache MembershipCache
}

func NewUpdateMember(tx TxManager, cache MembershipCache) *UpdateMember {
	return &UpdateMember{tx: tx, cache: cache}
}

func (uc *UpdateMember) Execute(ctx context.Context, t tenancy.Context, userID uuid.UUID, c membership.Change) (membership.Member, error) {
	if err := authorize(t, membership.PermMembersManage); err != nil {
		return membership.Member{}, err
	}
	actorRoles := make([]membership.Role, 0, len(t.Roles()))
	for _, r := range t.Roles() {
		actorRoles = append(actorRoles, membership.Role(r))
	}

	var before, after membership.Member
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		before, err = tx.Members().GetForUpdate(ctx, t.OrganizationID(), userID)
		if err != nil {
			return err
		}
		owners, err := tx.Members().CountActiveOwners(ctx, t.OrganizationID())
		if err != nil {
			return err
		}
		after, err = membership.PlanChange(actorRoles, before, c, owners)
		if err != nil {
			return err
		}
		if membership.Unchanged(before, after) {
			return nil
		}

		if c.Role != nil && !slices.Equal(before.Roles, after.Roles) {
			if err := tx.Members().ReplaceRoles(ctx, t.OrganizationID(), before.MembershipID, after.Roles, t.UserID()); err != nil {
				return err
			}
			if err := tx.Audit().Record(ctx, AuditEvent{
				OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
				Action: "membership.role_changed", EntityType: "membership", EntityID: before.MembershipID,
				Before: map[string]any{"userId": userID, "roles": before.Roles},
				After:  map[string]any{"userId": userID, "roles": after.Roles},
			}); err != nil {
				return err
			}
		}
		if before.Status != after.Status {
			if err := tx.Members().SetStatus(ctx, t.OrganizationID(), before.MembershipID, after.Status); err != nil {
				return err
			}
			action := "membership.reactivated"
			if after.Status == membership.StatusSuspended {
				action = "membership.suspended"
			}
			if err := tx.Audit().Record(ctx, AuditEvent{
				OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
				Action: action, EntityType: "membership", EntityID: before.MembershipID,
				Before: map[string]any{"userId": userID, "status": before.Status},
				After:  map[string]any{"userId": userID, "status": after.Status},
			}); err != nil {
				return err
			}
		}
		// TODO(contracts): evento de dominio de cambio de membresía cuando el catálogo lo defina.
		return nil
	})
	if err != nil {
		return membership.Member{}, err
	}
	if !membership.Unchanged(before, after) {
		uc.cache.Invalidate(before.Subject, t.OrganizationID())
	}
	return after, nil
}
