package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/pkg/tenancy"
)

type createFollowUp interface {
	Execute(ctx context.Context, t tenancy.Context, key string, receivableID uuid.UUID, in app.FollowUpInput) (app.CommandResult[app.FollowUpView], error)
}

type listFollowUps interface {
	Execute(ctx context.Context, t tenancy.Context, receivableID uuid.UUID) ([]app.FollowUpView, error)
}

type createPromise interface {
	Execute(ctx context.Context, t tenancy.Context, key string, receivableID uuid.UUID, in app.PromiseInput) (app.CommandResult[app.PromiseView], error)
}

type listPromises interface {
	Execute(ctx context.Context, t tenancy.Context, receivableID uuid.UUID) ([]app.PromiseView, error)
}

type closePromise interface {
	Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, to collection.PromiseStatus) (app.CommandResult[app.PromiseView], error)
}

type CollectionHandlers struct {
	CreateFollowUp createFollowUp
	ListFollowUps  listFollowUps
	CreatePromise  createPromise
	ListPromises   listPromises
	ClosePromise   closePromise
}

type followUpRequest struct {
	FollowupType string `json:"followupType"`
	Notes        string `json:"notes"`
	NextActionOn string `json:"nextActionOn"`
}

type followUpResponse struct {
	ID                uuid.UUID  `json:"id"`
	ReceivableID      uuid.UUID  `json:"receivableId"`
	FollowupType      string     `json:"followupType"`
	Notes             string     `json:"notes"`
	NextActionOn      string     `json:"nextActionOn,omitempty"`
	PerformedAt       time.Time  `json:"performedAt"`
	PerformedByUserID *uuid.UUID `json:"performedByUserId,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type promiseRequest struct {
	PromisedAmount string `json:"promisedAmount"`
	PromisedOn     string `json:"promisedOn"`
	FollowupID     string `json:"followupId"`
}

type promiseStatusRequest struct {
	Status string `json:"status"`
}

type promiseResponse struct {
	ID             uuid.UUID  `json:"id"`
	ReceivableID   uuid.UUID  `json:"receivableId"`
	FollowupID     *uuid.UUID `json:"followupId,omitempty"`
	PromisedAmount string     `json:"promisedAmount"`
	PromisedOn     string     `json:"promisedOn"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (h *CollectionHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/receivables/{id}/follow-ups", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		items, err := h.ListFollowUps.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]followUpResponse, 0, len(items))
		for _, f := range items {
			out = append(out, toFollowUpResponse(f))
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/receivables/{id}/follow-ups", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req followUpRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var f fields
		in := app.FollowUpInput{Type: collection.FollowUpType(req.FollowupType), Notes: f.text("notes", req.Notes, 1, 2000)}
		if !in.Type.Valid() {
			f.add("followupType", "debe ser uno de: call email visit message note")
		}
		if req.NextActionOn != "" {
			d := f.date("nextActionOn", req.NextActionOn)
			in.NextActionOn = &d
		}
		if err := f.err(); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.CreateFollowUp.Execute(r.Context(), t, key, id, in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toFollowUpResponse(res.Value))
	})

	rt.handle("GET /v1/receivables/{id}/promises", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		items, err := h.ListPromises.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]promiseResponse, 0, len(items))
		for _, p := range items {
			out = append(out, toPromiseResponse(p))
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/receivables/{id}/promises", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req promiseRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var f fields
		in := app.PromiseInput{Amount: f.money("promisedAmount", req.PromisedAmount), PromisedOn: f.date("promisedOn", req.PromisedOn)}
		if req.FollowupID != "" {
			in.FollowUpID = f.uuid("followupId", req.FollowupID)
		}
		if err := f.err(); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.CreatePromise.Execute(r.Context(), t, key, id, in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toPromiseResponse(res.Value))
	})

	rt.handle("POST /v1/payment-promises/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req promiseStatusRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		to := collection.PromiseStatus(req.Status)
		if to != collection.PromiseKept && to != collection.PromiseBroken && to != collection.PromiseCancelled {
			var f fields
			f.add("status", "debe ser uno de: kept broken cancelled")
			fail(w, r, f.err())
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.ClosePromise.Execute(r.Context(), t, key, id, to)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toPromiseResponse(res.Value))
	})
}

func toFollowUpResponse(f app.FollowUpView) followUpResponse {
	out := followUpResponse{
		ID: f.ID, ReceivableID: f.ReceivableID, FollowupType: string(f.Type), Notes: f.Notes, PerformedAt: f.PerformedAt,
		CreatedAt: f.CreatedAt,
	}
	if f.NextActionOn != nil {
		out.NextActionOn = f.NextActionOn.String()
	}
	if f.PerformedBy != uuid.Nil {
		by := f.PerformedBy
		out.PerformedByUserID = &by
	}
	return out
}

func toPromiseResponse(p app.PromiseView) promiseResponse {
	out := promiseResponse{
		ID: p.ID, ReceivableID: p.ReceivableID, PromisedAmount: p.PromisedAmount.String(), PromisedOn: p.PromisedOn.String(),
		Status: string(p.Status), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if p.FollowUpID != uuid.Nil {
		id := p.FollowUpID
		out.FollowupID = &id
	}
	return out
}
