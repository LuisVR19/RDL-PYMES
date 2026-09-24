package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/pkg/tenancy"
)

type (
	createOrganization interface {
		Execute(ctx context.Context, id tenancy.Identity, idempotencyKey string, in organization.NewInput) (app.CreateOrganizationResult, error)
	}
	getCurrentOrganization interface {
		Execute(ctx context.Context, t tenancy.Context) (organization.Organization, error)
	}
	updateCurrentOrganization interface {
		Execute(ctx context.Context, t tenancy.Context, p organization.Patch) (organization.Organization, error)
	}
)

type OrganizationHandlers struct {
	Create createOrganization
	Get    getCurrentOrganization
	Update updateCurrentOrganization
}

type organizationResponse struct {
	ID                     uuid.UUID `json:"id"`
	LegalName              string    `json:"legalName"`
	TradeName              string    `json:"tradeName,omitempty"`
	IdentificationTypeCode string    `json:"identificationTypeCode"`
	IdentificationNumber   string    `json:"identificationNumber"`
	Email                  string    `json:"email"`
	Phone                  string    `json:"phone,omitempty"`
	Timezone               string    `json:"timezone"`
	DefaultCurrencyCode    string    `json:"defaultCurrencyCode"`
	Status                 string    `json:"status"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type createOrganizationRequest struct {
	LegalName              string `json:"legalName" validate:"required,max=200"`
	TradeName              string `json:"tradeName" validate:"max=200"`
	IdentificationTypeCode string `json:"identificationTypeCode" validate:"required,max=10"`
	IdentificationNumber   string `json:"identificationNumber" validate:"required,max=30"`
	Email                  string `json:"email" validate:"required,email,max=254"`
	Phone                  string `json:"phone" validate:"max=30"`
	Timezone               string `json:"timezone" validate:"max=64"`
}

// updateOrganizationRequest: campo ausente = no cambia; "" en tradeName o phone lo borra.
type updateOrganizationRequest struct {
	TradeName *string `json:"tradeName" validate:"omitnil,max=200"`
	Email     *string `json:"email" validate:"omitnil,email,max=254"`
	Phone     *string `json:"phone" validate:"omitnil,max=30"`
	Timezone  *string `json:"timezone" validate:"omitnil,min=1,max=64"`
}

const idempotencyHeader = "Idempotency-Key"

var errIdempotencyKeyMissing = errors.New("falta el header Idempotency-Key")

// idempotencyKey exige el header en los comandos POST: 1 a 255 caracteres ASCII visibles.
func idempotencyKey(r *http.Request) (string, error) {
	k := strings.TrimSpace(r.Header.Get(idempotencyHeader))
	if k == "" || len(k) > 255 {
		return "", errIdempotencyKeyMissing
	}
	for _, c := range k {
		if c < 0x21 || c > 0x7e {
			return "", errIdempotencyKeyMissing
		}
	}
	return k, nil
}

func (h *OrganizationHandlers) registerAuthenticated(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("POST /v1/organizations", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createOrganizationRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		id, _ := tenancy.IdentityFrom(r.Context())
		res, err := h.Create.Execute(r.Context(), id, key, organization.NewInput{
			LegalName: req.LegalName, TradeName: req.TradeName,
			IdentificationTypeCode: req.IdentificationTypeCode, IdentificationNumber: req.IdentificationNumber,
			Email: req.Email, Phone: req.Phone, Timezone: req.Timezone,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		// Misma clave y mismo cuerpo → misma respuesta (201), con el estado actual del recurso.
		writeJSON(w, http.StatusCreated, toOrganizationResponse(res.Organization))
	})
}

func (h *OrganizationHandlers) registerCurrent(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/organizations/current", func(w http.ResponseWriter, r *http.Request) {
		t, _ := tenancy.From(r.Context())
		o, err := h.Get.Execute(r.Context(), t)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toOrganizationResponse(o))
	})

	rt.handle("PATCH /v1/organizations/current", func(w http.ResponseWriter, r *http.Request) {
		var req updateOrganizationRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		o, err := h.Update.Execute(r.Context(), t, organization.Patch{
			TradeName: req.TradeName, Email: req.Email, Phone: req.Phone, Timezone: req.Timezone,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toOrganizationResponse(o))
	})
}

func toOrganizationResponse(o organization.Organization) organizationResponse {
	return organizationResponse{
		ID: o.ID, LegalName: o.LegalName, TradeName: o.TradeName,
		IdentificationTypeCode: o.IdentificationTypeCode, IdentificationNumber: o.IdentificationNumber,
		Email: o.Email, Phone: o.Phone, Timezone: o.Timezone, DefaultCurrencyCode: o.DefaultCurrencyCode,
		Status: string(o.Status), CreatedAt: o.CreatedAt.UTC(), UpdatedAt: o.UpdatedAt.UTC(),
	}
}
