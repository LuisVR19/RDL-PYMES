package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// GetMe devuelve el perfil del usuario autenticado y lo da de alta en core.users la primera vez.
// No hay trigger sobre auth.users (ese schema es de Supabase): el alta es perezosa (ADR 0001 §4, punto 4).
type GetMe struct{ tx TxManager }

func NewGetMe(tx TxManager) *GetMe { return &GetMe{tx: tx} }

func (uc *GetMe) Execute(ctx context.Context, id tenancy.Identity) (user.User, error) {
	var u user.User
	err := uc.tx.WithinUserTx(ctx, uuid.Nil, func(ctx context.Context, tx Tx) error {
		var err error
		u, err = provision(ctx, tx, id)
		return err
	})
	if err != nil {
		return user.User{}, err
	}
	if !u.IsActive() {
		return user.User{}, user.ErrDisabled
	}
	return u, nil
}

// provision busca al usuario por sujeto y lo crea si no existe, auditando el alta una sola vez.
func provision(ctx context.Context, tx Tx, id tenancy.Identity) (user.User, error) {
	u, err := tx.Users().FindBySubject(ctx, id.Subject)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return user.User{}, err
	}

	candidate, err := user.NewFromIdentity(id.Subject, id.Email, id.FullName)
	if err != nil {
		return user.User{}, err
	}
	u, created, err := tx.Users().CreateIfAbsent(ctx, candidate)
	if err != nil {
		return user.User{}, err
	}
	if created {
		if err := tx.Audit().Record(ctx, AuditEvent{
			ActorUserID: u.ID,
			Action:      "user.provisioned",
			EntityType:  "user",
			EntityID:    u.ID,
			After:       map[string]any{"email": u.Email, "identityProvider": user.IdentityProvider},
		}); err != nil {
			return user.User{}, fmt.Errorf("auditando alta de usuario: %w", err)
		}
	}
	return u, nil
}

// findUser busca al usuario por sujeto sin darlo de alta. Corre sin sesión de usuario: la búsqueda pasa por
// core.find_user_by_subject (security definer, ADR 0007) con el sub verificado del token.
func findUser(ctx context.Context, txm TxManager, subject string) (user.User, error) {
	var u user.User
	err := txm.WithinUserTx(ctx, uuid.Nil, func(ctx context.Context, tx Tx) error {
		var err error
		u, err = tx.Users().FindBySubject(ctx, subject)
		return err
	})
	if err != nil {
		return user.User{}, err
	}
	if !u.IsActive() {
		return user.User{}, user.ErrDisabled
	}
	return u, nil
}

// ListMyMemberships devuelve las organizaciones donde el usuario tiene membresía activa (caso contador: varias).
type ListMyMemberships struct{ tx TxManager }

func NewListMyMemberships(tx TxManager) *ListMyMemberships { return &ListMyMemberships{tx: tx} }

type MyMemberships struct {
	ActiveOrganizationID uuid.UUID
	Items                []membership.Summary
}

func (uc *ListMyMemberships) Execute(ctx context.Context, id tenancy.Identity) (MyMemberships, error) {
	u, err := findUser(ctx, uc.tx, id.Subject)
	if errors.Is(err, ErrNotFound) {
		return MyMemberships{Items: []membership.Summary{}}, nil // aún no dado de alta: no tiene membresías
	}
	if err != nil {
		return MyMemberships{}, err
	}

	out := MyMemberships{ActiveOrganizationID: u.ActiveOrganizationID}
	// Con la sesión del usuario: las políticas *_own solo exponen sus propias membresías.
	err = uc.tx.WithinUserTx(ctx, u.ID, func(ctx context.Context, tx Tx) error {
		items, err := tx.Memberships().ListActiveForUser(ctx, u.ID)
		out.Items = items
		return err
	})
	return out, err
}

// SelectActiveOrganization guarda la organización activa si el usuario es miembro activo de ella.
// El JWT vigente no cambia: el cliente debe refrescar la sesión para que el hook emita el nuevo org_id.
type SelectActiveOrganization struct{ tx TxManager }

func NewSelectActiveOrganization(tx TxManager) *SelectActiveOrganization {
	return &SelectActiveOrganization{tx: tx}
}

// Execute responde ErrNotFound tanto si la organización no existe como si el usuario no es miembro activo.
func (uc *SelectActiveOrganization) Execute(ctx context.Context, id tenancy.Identity, organizationID uuid.UUID) error {
	u, err := findUser(ctx, uc.tx, id.Subject)
	if err != nil {
		return err
	}
	return uc.tx.WithinUserTx(ctx, u.ID, func(ctx context.Context, tx Tx) error {
		ok, err := tx.Memberships().IsActiveMember(ctx, u.ID, organizationID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if u.ActiveOrganizationID == organizationID {
			return nil
		}
		if err := tx.Users().SetActiveOrganization(ctx, u.ID, organizationID); err != nil {
			return err
		}
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: organizationID,
			ActorUserID:    u.ID,
			Action:         "user.active_organization_selected",
			EntityType:     "user",
			EntityID:       u.ID,
			Before:         map[string]any{"activeOrganizationId": nullableID(u.ActiveOrganizationID)},
			After:          map[string]any{"activeOrganizationId": organizationID},
		})
	})
}

func nullableID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
