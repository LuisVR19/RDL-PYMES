package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/pkg/tenancy"
)

type (
	createInvitation interface {
		Execute(ctx context.Context, t tenancy.Context, idempotencyKey, email string, role membership.Role) (app.CreateInvitationResult, error)
	}
	listInvitations interface {
		Execute(ctx context.Context, t tenancy.Context, q app.InvitationQuery) (app.InvitationPage, error)
	}
	revokeInvitation interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) error
	}
	acceptInvitation interface {
		Execute(ctx context.Context, id tenancy.Identity, idempotencyKey, token string) (app.AcceptInvitationResult, error)
	}
)

type InvitationHandlers struct {
	Create createInvitation
	List   listInvitations
	Revoke revokeInvitation
	Accept acceptInvitation
}

type invitationResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
	// Token y AcceptPath solo aparecen al crearla: la base guarda únicamente el hash del token.
	Token      string `json:"token,omitempty"`
	AcceptPath string `json:"acceptPath,omitempty"`
}

type invitationPageResponse struct {
	Items      []invitationResponse `json:"items"`
	NextCursor *string              `json:"nextCursor"`
}

type createInvitationRequest struct {
	Email string `json:"email" validate:"required,email,max=254"`
	Role  string `json:"role" validate:"required,oneof=owner admin biller collector accountant read_only"`
}

type acceptInvitationResponse struct {
	OrganizationID             uuid.UUID `json:"organizationId"`
	Role                       string    `json:"role"`
	ActiveOrganizationSelected bool      `json:"activeOrganizationSelected"`
	// Para operar en la organización hay que elegirla (si no quedó activa) y refrescar la sesión.
	TokenRefreshRequired bool `json:"tokenRefreshRequired"`
}

func (h *InvitationHandlers) registerCurrent(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("POST /v1/organizations/current/invitations", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createInvitationRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, req.Email, membership.Role(req.Role))
		if err != nil {
			fail(w, r, err)
			return
		}
		out := toInvitationResponse(res.Invitation, time.Now())
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		} else {
			out.Token, out.AcceptPath = res.Token, "/v1/invitations/"+res.Token+"/accept"
		}
		writeJSON(w, http.StatusCreated, out)
	})

	rt.handle("GET /v1/organizations/current/invitations", func(w http.ResponseWriter, r *http.Request) {
		limit, after, fields := parsePage(r)
		q := app.InvitationQuery{Limit: limit, After: after}
		if v := r.URL.Query().Get("status"); v != "" {
			q.Status = invitation.Status(v)
			switch q.Status {
			case invitation.StatusPending, invitation.StatusAccepted, invitation.StatusRevoked, invitation.StatusExpired:
			default:
				fields = append(fields, problem.FieldError{Field: "status", Message: "debe ser pending, accepted, revoked o expired"})
			}
		}
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		page, err := h.List.Execute(r.Context(), t, q)
		if err != nil {
			fail(w, r, err)
			return
		}
		now := time.Now()
		out := invitationPageResponse{Items: make([]invitationResponse, 0, len(page.Items))}
		for _, inv := range page.Items {
			out.Items = append(out.Items, toInvitationResponse(inv, now))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("DELETE /v1/organizations/current/invitations/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		if err := h.Revoke.Execute(r.Context(), t, id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func (h *InvitationHandlers) registerAuthenticated(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("POST /v1/invitations/{token}/accept", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		id, _ := tenancy.IdentityFrom(r.Context())
		res, err := h.Accept.Execute(r.Context(), id, key, r.PathValue("token"))
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusOK, acceptInvitationResponse{
			OrganizationID: res.OrganizationID, Role: string(res.Role),
			ActiveOrganizationSelected: res.ActiveOrganizationSelected, TokenRefreshRequired: true,
		})
	})
}

func toInvitationResponse(inv invitation.Invitation, now time.Time) invitationResponse {
	return invitationResponse{
		ID: inv.ID, Email: inv.Email, Role: string(inv.Role), Status: string(inv.EffectiveStatus(now)),
		ExpiresAt: inv.ExpiresAt.UTC(), CreatedAt: inv.CreatedAt.UTC(),
	}
}
