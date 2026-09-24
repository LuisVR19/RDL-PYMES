package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

// Puertos de entrada que consumen estos handlers (definidos del lado del consumidor).
type (
	getMe interface {
		Execute(ctx context.Context, id tenancy.Identity) (user.User, error)
	}
	listMyMemberships interface {
		Execute(ctx context.Context, id tenancy.Identity) (app.MyMemberships, error)
	}
	selectActiveOrganization interface {
		Execute(ctx context.Context, id tenancy.Identity, organizationID uuid.UUID) error
	}
)

// MeHandlers atiende /v1/me*: operaciones del usuario autenticado que no requieren organización activa.
type MeHandlers struct {
	GetMe                    getMe
	ListMyMemberships        listMyMemberships
	SelectActiveOrganization selectActiveOrganization
}

type meResponse struct {
	ID                   string     `json:"id"`
	Email                string     `json:"email"`
	FullName             string     `json:"fullName"`
	Status               string     `json:"status"`
	ActiveOrganizationID *uuid.UUID `json:"activeOrganizationId"`
	CreatedAt            time.Time  `json:"createdAt"`
}

type membershipResponse struct {
	OrganizationID     uuid.UUID `json:"organizationId"`
	LegalName          string    `json:"legalName"`
	TradeName          string    `json:"tradeName,omitempty"`
	OrganizationStatus string    `json:"organizationStatus"`
	Status             string    `json:"status"`
	Roles              []string  `json:"roles"`
	JoinedAt           time.Time `json:"joinedAt"`
	IsActive           bool      `json:"isActive"`
}

type membershipsResponse struct {
	ActiveOrganizationID *uuid.UUID           `json:"activeOrganizationId"`
	Items                []membershipResponse `json:"items"`
}

type selectActiveOrganizationRequest struct {
	OrganizationID string `json:"organizationId" validate:"required,uuid"`
}

type selectActiveOrganizationResponse struct {
	ActiveOrganizationID uuid.UUID `json:"activeOrganizationId"`
	// El JWT vigente conserva el org_id anterior hasta que el cliente refresque la sesión.
	TokenRefreshRequired bool `json:"tokenRefreshRequired"`
}

func (h *MeHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/me", func(w http.ResponseWriter, r *http.Request) {
		id, _ := tenancy.IdentityFrom(r.Context())
		u, err := h.GetMe.Execute(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, meResponse{
			ID:                   u.ID.String(),
			Email:                u.Email,
			FullName:             u.FullName,
			Status:               string(u.Status),
			ActiveOrganizationID: optionalID(u.ActiveOrganizationID),
			CreatedAt:            u.CreatedAt.UTC(),
		})
	})

	rt.handle("GET /v1/me/memberships", func(w http.ResponseWriter, r *http.Request) {
		id, _ := tenancy.IdentityFrom(r.Context())
		res, err := h.ListMyMemberships.Execute(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := membershipsResponse{ActiveOrganizationID: optionalID(res.ActiveOrganizationID), Items: make([]membershipResponse, 0, len(res.Items))}
		for _, m := range res.Items {
			roles := make([]string, 0, len(m.Roles))
			for _, role := range m.Roles {
				roles = append(roles, string(role))
			}
			out.Items = append(out.Items, membershipResponse{
				OrganizationID:     m.OrganizationID,
				LegalName:          m.LegalName,
				TradeName:          m.TradeName,
				OrganizationStatus: m.OrganizationStatus,
				Status:             string(m.Status),
				Roles:              roles,
				JoinedAt:           m.JoinedAt.UTC(),
				IsActive:           m.OrganizationID == res.ActiveOrganizationID,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("PUT /v1/me/active-organization", func(w http.ResponseWriter, r *http.Request) {
		var req selectActiveOrganizationRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		orgID := uuid.MustParse(req.OrganizationID) // ya validado como UUID
		id, _ := tenancy.IdentityFrom(r.Context())
		if err := h.SelectActiveOrganization.Execute(r.Context(), id, orgID); err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, selectActiveOrganizationResponse{ActiveOrganizationID: orgID, TokenRefreshRequired: true})
	})
}

func optionalID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}
