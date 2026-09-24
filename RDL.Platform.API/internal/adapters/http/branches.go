package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/pkg/tenancy"
)

type (
	createBranch interface {
		Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in branch.NewInput) (app.CreateBranchResult, error)
	}
	listBranches interface {
		Execute(ctx context.Context, t tenancy.Context, q app.BranchQuery) (app.BranchPage, error)
	}
	getBranch interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (branch.Branch, error)
	}
	updateBranch interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p branch.Patch) (branch.Branch, error)
	}
)

type BranchHandlers struct {
	Create createBranch
	List   listBranches
	Get    getBranch
	Update updateBranch
}

type branchResponse struct {
	ID        uuid.UUID `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	Phone     string    `json:"phone,omitempty"`
	Email     string    `json:"email,omitempty"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type branchPageResponse struct {
	Items      []branchResponse `json:"items"`
	NextCursor *string          `json:"nextCursor"`
}

type createBranchRequest struct {
	Code    string `json:"code" validate:"required,max=20"`
	Name    string `json:"name" validate:"required,max=150"`
	Address string `json:"address" validate:"max=500"`
	Phone   string `json:"phone" validate:"max=30"`
	Email   string `json:"email" validate:"omitempty,email,max=254"`
}

type updateBranchRequest struct {
	Name     *string `json:"name" validate:"omitnil,min=1,max=150"`
	Address  *string `json:"address" validate:"omitnil,max=500"`
	Phone    *string `json:"phone" validate:"omitnil,max=30"`
	Email    *string `json:"email" validate:"omitnil,omitempty,email,max=254"`
	IsActive *bool   `json:"isActive"`
}

func (h *BranchHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/organizations/current/branches", func(w http.ResponseWriter, r *http.Request) {
		limit, after, fields := parsePage(r)
		q := app.BranchQuery{Limit: limit, After: after}
		if v := r.URL.Query().Get("active"); v != "" {
			active, err := strconv.ParseBool(v)
			if err != nil {
				fields = append(fields, problem.FieldError{Field: "active", Message: "debe ser true o false"})
			}
			q.Active = &active
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
		out := branchPageResponse{Items: make([]branchResponse, 0, len(page.Items))}
		for _, b := range page.Items {
			out.Items = append(out.Items, toBranchResponse(b))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/organizations/current/branches", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createBranchRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, branch.NewInput{
			Code: req.Code, Name: req.Name, Address: req.Address, Phone: req.Phone, Email: req.Email,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, toBranchResponse(res.Branch))
	})

	rt.handle("GET /v1/organizations/current/branches/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		b, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toBranchResponse(b))
	})

	rt.handle("PATCH /v1/organizations/current/branches/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			fail(w, r, app.ErrNotFound)
			return
		}
		var req updateBranchRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		b, err := h.Update.Execute(r.Context(), t, id, branch.Patch{
			Name: req.Name, Address: req.Address, Phone: req.Phone, Email: req.Email, IsActive: req.IsActive,
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toBranchResponse(b))
	})
}

func toBranchResponse(b branch.Branch) branchResponse {
	return branchResponse{
		ID: b.ID, Code: b.Code, Name: b.Name, Address: b.Address, Phone: b.Phone, Email: b.Email,
		IsActive: b.IsActive, CreatedAt: b.CreatedAt.UTC(), UpdatedAt: b.UpdatedAt.UTC(),
	}
}
