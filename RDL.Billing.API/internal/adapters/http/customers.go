package http

import (
	"context"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/pkg/tenancy"
)

type (
	createCustomer interface {
		Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in customer.NewInput) (app.CreateCustomerResult, error)
	}
	listCustomers interface {
		Execute(ctx context.Context, t tenancy.Context, q app.CustomerQuery) (app.CustomerPage, error)
	}
	getCustomer interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (customer.Customer, error)
	}
	updateCustomer interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p customer.Patch) (customer.Customer, error)
	}
)

type CustomerHandlers struct {
	Create createCustomer
	List   listCustomers
	Get    getCustomer
	Update updateCustomer
}

// Representación de openapi/billing.yaml (Customer, CustomerInput, CustomerPatch) del repo de contratos.
// Los límites de cada campo los valida el dominio: aquí solo la forma del JSON.
type identificationDTO struct {
	TypeCode string `json:"typeCode"`
	Number   string `json:"number"`
}

type customerResponse struct {
	ID             uuid.UUID         `json:"id"`
	Identification identificationDTO `json:"identification"`
	LegalName      string            `json:"legalName"`
	TradeName      string            `json:"tradeName,omitempty"`
	Email          string            `json:"email,omitempty"`
	Phone          string            `json:"phone,omitempty"`
	Address        string            `json:"address,omitempty"`
	IsActive       bool              `json:"isActive"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

type customerPageResponse struct {
	Items      []customerResponse `json:"items"`
	NextCursor *string            `json:"nextCursor"`
}

type createCustomerRequest struct {
	Identification *identificationDTO `json:"identification" validate:"required"`
	LegalName      string             `json:"legalName"`
	TradeName      string             `json:"tradeName"`
	Email          string             `json:"email"`
	Phone          string             `json:"phone"`
	Address        string             `json:"address"`
}

// updateCustomerRequest: campo ausente = no cambia; "" en tradeName, email, phone o address lo borra.
type updateCustomerRequest struct {
	LegalName *string `json:"legalName"`
	TradeName *string `json:"tradeName"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
	Address   *string `json:"address"`
	IsActive  *bool   `json:"isActive"`
}

const maxSearchLength = 100

func (h *CustomerHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/customers", func(w http.ResponseWriter, r *http.Request) {
		limit, after, fields := parsePage(r)
		q := app.CustomerQuery{Limit: limit, After: after, Search: r.URL.Query().Get("q")}
		if utf8.RuneCountInString(q.Search) > maxSearchLength {
			fields = append(fields, problem.FieldError{Field: "q", Message: "admite hasta 100 caracteres"})
		}
		q.Active = parseBoolParam(r, "active", &fields)
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
		out := customerPageResponse{Items: make([]customerResponse, 0, len(page.Items))}
		for _, c := range page.Items {
			out.Items = append(out.Items, toCustomerResponse(c))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/customers", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createCustomerRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, customer.NewInput{
			IdentificationTypeCode: req.Identification.TypeCode, IdentificationNumber: req.Identification.Number,
			LegalName: req.LegalName, TradeName: req.TradeName, Email: req.Email, Phone: req.Phone, Address: req.Address,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, toCustomerResponse(res.Customer))
	})

	rt.handle("GET /v1/customers/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		c, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toCustomerResponse(c))
	})

	rt.handle("PATCH /v1/customers/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req updateCustomerRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		c, err := h.Update.Execute(r.Context(), t, id, customer.Patch{
			LegalName: req.LegalName, TradeName: req.TradeName, Email: req.Email, Phone: req.Phone,
			Address: req.Address, IsActive: req.IsActive,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toCustomerResponse(c))
	})
}

func toCustomerResponse(c customer.Customer) customerResponse {
	return customerResponse{
		ID:             c.ID,
		Identification: identificationDTO{TypeCode: c.Identification.TypeCode, Number: c.Identification.Number},
		LegalName:      c.LegalName,
		TradeName:      c.TradeName,
		Email:          c.Email,
		Phone:          c.Phone,
		Address:        c.Address,
		IsActive:       c.IsActive,
		CreatedAt:      c.CreatedAt.UTC(),
		UpdatedAt:      c.UpdatedAt.UTC(),
	}
}
