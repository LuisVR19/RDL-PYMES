package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// IdempotencyTTL es cuánto se recuerda una Idempotency-Key.
const IdempotencyTTL = 24 * time.Hour

// organizationIDNamespace deriva ids deterministas para el alta idempotente (docs/decisiones/0001 §5.3).
var organizationIDNamespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("urn:rdl:platform:organization-create"))

// OrganizationIDFor calcula el id de la organización que creará este usuario con esta Idempotency-Key.
// Así la fila de idempotencia (que exige organization_id) puede guardarse antes de que la organización exista,
// y un reintento cae en la misma fila. Otro usuario con la misma clave obtiene otro id.
func OrganizationIDFor(userID uuid.UUID, idempotencyKey string) uuid.UUID {
	return uuid.NewSHA1(organizationIDNamespace, []byte(userID.String()+":"+idempotencyKey))
}

// CreateOrganization da de alta una organización y deja al creador como owner.
type CreateOrganization struct{ tx TxManager }

func NewCreateOrganization(tx TxManager) *CreateOrganization { return &CreateOrganization{tx: tx} }

type CreateOrganizationResult struct {
	Organization organization.Organization
	// Replayed indica que la clave ya se había usado con la misma petición: no se creó nada nuevo.
	Replayed bool
}

func (uc *CreateOrganization) Execute(ctx context.Context, id tenancy.Identity, idempotencyKey string, in organization.NewInput) (CreateOrganizationResult, error) {
	var creator user.User
	if err := uc.tx.WithinUserTx(ctx, uuid.Nil, func(ctx context.Context, tx Tx) error {
		var err error
		creator, err = provision(ctx, tx, id)
		return err
	}); err != nil {
		return CreateOrganizationResult{}, err
	}
	if !creator.IsActive() {
		return CreateOrganizationResult{}, user.ErrDisabled
	}

	orgID := OrganizationIDFor(creator.ID, idempotencyKey)
	candidate, err := organization.New(orgID, in)
	if err != nil {
		return CreateOrganizationResult{}, err
	}
	hash, err := requestHash(in)
	if err != nil {
		return CreateOrganizationResult{}, err
	}

	// La transacción de alta corre ya "dentro" de la organización nueva: las políticas RLS de
	// organization_users, roles, idempotencia y la lectura con RETURNING exigen esa organización en la sesión.
	founding := tenancy.NewContext(creator.ID, id.Subject, orgID, []string{string(membership.RoleOwner)})
	var res CreateOrganizationResult
	err = uc.tx.WithinTenantTx(ctx, founding, func(ctx context.Context, tx Tx) error {
		prev, err := tx.Idempotency().Claim(ctx, orgID, idempotencyKey, hash, IdempotencyTTL)
		if err != nil {
			return err
		}
		if prev != nil {
			if prev.RequestHash != hash {
				return ErrIdempotencyKeyReused
			}
			o, err := tx.Organizations().Get(ctx, orgID)
			res = CreateOrganizationResult{Organization: o, Replayed: true}
			return err
		}

		o, err := tx.Organizations().Create(ctx, candidate)
		if err != nil {
			return err
		}
		membershipID, err := tx.Memberships().Add(ctx, orgID, creator.ID)
		if err != nil {
			return err
		}
		if err := tx.Memberships().AssignRole(ctx, orgID, membershipID, membership.RoleOwner, creator.ID); err != nil {
			return err
		}
		// Primera organización del usuario: queda activa para que el próximo refresco del token ya la traiga.
		if creator.ActiveOrganizationID == uuid.Nil {
			if err := tx.Users().SetActiveOrganization(ctx, creator.ID, orgID); err != nil {
				return err
			}
		}

		for _, e := range []AuditEvent{
			{OrganizationID: orgID, ActorUserID: creator.ID, Action: "organization.created", EntityType: "organization", EntityID: orgID, After: organizationAudit(o)},
			{OrganizationID: orgID, ActorUserID: creator.ID, Action: "membership.created", EntityType: "membership", EntityID: membershipID,
				After: map[string]any{"userId": creator.ID, "roles": []membership.Role{membership.RoleOwner}}},
		} {
			if err := tx.Audit().Record(ctx, e); err != nil {
				return err
			}
		}
		// TODO(contracts): publicar OrganizationCreated/MemberAdded en el outbox cuando el catálogo de eventos
		// los defina. Hoy el catálogo (arquitectura 6.2) no tiene eventos de Platform: no se inventan.

		if err := tx.Idempotency().Complete(ctx, orgID, idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"organizationId": orgID.String()},
		}); err != nil {
			return err
		}
		res = CreateOrganizationResult{Organization: o}
		return nil
	})
	return res, err
}

// GetCurrentOrganization devuelve la organización activa. Cualquier rol puede leerla.
type GetCurrentOrganization struct{ tx TxManager }

func NewGetCurrentOrganization(tx TxManager) *GetCurrentOrganization {
	return &GetCurrentOrganization{tx: tx}
}

func (uc *GetCurrentOrganization) Execute(ctx context.Context, t tenancy.Context) (organization.Organization, error) {
	if err := authorize(t, membership.PermOrganizationRead); err != nil {
		return organization.Organization{}, err
	}
	var o organization.Organization
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		o, err = tx.Organizations().Get(ctx, t.OrganizationID())
		return err
	})
	return o, err
}

// UpdateCurrentOrganization modifica datos de contacto y presentación de la organización activa (owner, admin).
type UpdateCurrentOrganization struct{ tx TxManager }

func NewUpdateCurrentOrganization(tx TxManager) *UpdateCurrentOrganization {
	return &UpdateCurrentOrganization{tx: tx}
}

func (uc *UpdateCurrentOrganization) Execute(ctx context.Context, t tenancy.Context, p organization.Patch) (organization.Organization, error) {
	if err := authorize(t, membership.PermOrganizationUpdate); err != nil {
		return organization.Organization{}, err
	}
	var out organization.Organization
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		current, err := tx.Organizations().GetForUpdate(ctx, t.OrganizationID())
		if err != nil {
			return err
		}
		next, err := current.Apply(p)
		if err != nil {
			return err
		}
		if next == current {
			out = current
			return nil
		}
		out, err = tx.Organizations().Update(ctx, next)
		if err != nil {
			return err
		}
		before, after := diff(organizationAudit(current), organizationAudit(next))
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "organization.updated", EntityType: "organization", EntityID: t.OrganizationID(),
			Before: before, After: after,
		})
	})
	return out, err
}

// requestHash identifica el contenido de la petición para detectar una Idempotency-Key reutilizada con otro cuerpo.
// Se calcula sobre la entrada ya decodificada: el orden de los campos o los espacios del JSON no cuentan.
func requestHash(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("calculando hash de la petición: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func organizationAudit(o organization.Organization) map[string]any {
	return map[string]any{
		"legalName": o.LegalName, "tradeName": o.TradeName,
		"identificationTypeCode": o.IdentificationTypeCode, "identificationNumber": o.IdentificationNumber,
		"email": o.Email, "phone": o.Phone, "timezone": o.Timezone,
	}
}
