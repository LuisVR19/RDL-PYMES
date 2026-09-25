package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"bitbucket.org/rdl/contracts/pkg/events/money"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/tenancy"
)

type createPayment interface {
	Execute(ctx context.Context, t tenancy.Context, key string, in app.PaymentInput) (app.CommandResult[app.PaymentView], error)
}

type listPayments interface {
	Execute(ctx context.Context, t tenancy.Context, q app.PaymentQuery) (app.PaymentPage, error)
}

type getPayment interface {
	Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (app.PaymentView, error)
}

type voidPayment interface {
	Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (app.CommandResult[app.PaymentView], error)
}

type applyPayment interface {
	Execute(ctx context.Context, t tenancy.Context, key string, in app.ApplyInput) (app.CommandResult[app.ApplicationView], error)
}

type reverseApplication interface {
	Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (app.CommandResult[app.ApplicationView], error)
}

type PaymentHandlers struct {
	Create  createPayment
	List    listPayments
	Get     getPayment
	Void    voidPayment
	Apply   applyPayment
	Reverse reverseApplication
}

// Peticiones y respuestas de RDL.Contracts/openapi/receivables.yaml (PaymentInput, Payment, PaymentApplication).
// Los montos son strings decimales: nunca pasan por float.

type paymentRequest struct {
	CustomerID        string `json:"customerId"`
	ReceivedOn        string `json:"receivedOn"`
	Amount            string `json:"amount"`
	Currency          string `json:"currency"`
	ExchangeRate      string `json:"exchangeRate"`
	PaymentMethodCode string `json:"paymentMethodCode"`
	Reference         string `json:"reference"`
	Notes             string `json:"notes"`
	Applications      []struct {
		ReceivableID string `json:"receivableId"`
		Amount       string `json:"amount"`
	} `json:"applications"`
}

type applyRequest struct {
	PaymentID    string `json:"paymentId"`
	ReceivableID string `json:"receivableId"`
	Amount       string `json:"amount"`
}

type reasonRequest struct {
	Reason string `json:"reason"`
}

type paymentResponse struct {
	ID                uuid.UUID             `json:"id"`
	CustomerID        uuid.UUID             `json:"customerId"`
	ReceivedOn        string                `json:"receivedOn"`
	Amount            string                `json:"amount"`
	Currency          string                `json:"currency"`
	ExchangeRate      string                `json:"exchangeRate"`
	PaymentMethodCode string                `json:"paymentMethodCode"`
	Reference         string                `json:"reference,omitempty"`
	Notes             string                `json:"notes,omitempty"`
	Status            string                `json:"status"`
	VoidReason        string                `json:"voidReason,omitempty"`
	VoidedAt          *time.Time            `json:"voidedAt,omitempty"`
	CreatedAt         time.Time             `json:"createdAt"`
	Applications      []applicationResponse `json:"applications"`
}

type applicationResponse struct {
	ID             uuid.UUID  `json:"id"`
	PaymentID      uuid.UUID  `json:"paymentId"`
	ReceivableID   uuid.UUID  `json:"receivableId"`
	Amount         string     `json:"amount"`
	AppliedAt      time.Time  `json:"appliedAt"`
	ReversedAt     *time.Time `json:"reversedAt,omitempty"`
	ReversalReason string     `json:"reversalReason,omitempty"`
}

type paymentPageResponse struct {
	Items      []paymentResponse `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

func (h *PaymentHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("POST /v1/payments", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req paymentRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		in, err := parsePayment(req)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toPaymentResponse(res.Value))
	})

	rt.handle("GET /v1/payments", func(w http.ResponseWriter, r *http.Request) {
		limit, after, fieldErrs := parsePage(r)
		f := fields{errs: fieldErrs}
		q := app.PaymentQuery{Limit: limit, After: after}
		if v := r.URL.Query().Get("customerId"); v != "" {
			q.CustomerID = f.uuid("customerId", v)
		}
		if err := f.err(); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		page, err := h.List.Execute(r.Context(), t, q)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := paymentPageResponse{Items: make([]paymentResponse, 0, len(page.Items))}
		for _, p := range page.Items {
			out.Items = append(out.Items, toPaymentResponse(p))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("GET /v1/payments/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		p, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toPaymentResponse(p))
	})

	rt.handle("POST /v1/payments/{id}/void", func(w http.ResponseWriter, r *http.Request) {
		id, key, reason, err := parseReasonCommand(w, r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Void.Execute(r.Context(), t, key, id, reason)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toPaymentResponse(res.Value))
	})

	rt.handle("POST /v1/payment-applications", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req applyRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var f fields
		in := app.ApplyInput{
			PaymentID: f.uuid("paymentId", req.PaymentID), ReceivableID: f.uuid("receivableId", req.ReceivableID),
			Amount: f.money("amount", req.Amount),
		}
		if err := f.err(); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Apply.Execute(r.Context(), t, key, in)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, toApplicationResponse(res.Value))
	})

	rt.handle("POST /v1/payment-applications/{id}/reverse", func(w http.ResponseWriter, r *http.Request) {
		id, key, reason, err := parseReasonCommand(w, r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Reverse.Execute(r.Context(), t, key, id, reason)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toApplicationResponse(res.Value))
	})
}

// parseReasonCommand: {id} de la ruta, Idempotency-Key y {"reason": "..."} de 1 a 500 caracteres.
func parseReasonCommand(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, string, error) {
	id, ok := pathID(r, "id")
	if !ok {
		return uuid.Nil, "", "", app.ErrNotFound
	}
	key, err := idempotencyKey(r)
	if err != nil {
		return uuid.Nil, "", "", err
	}
	var req reasonRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return uuid.Nil, "", "", err
	}
	var f fields
	reason := f.text("reason", req.Reason, 1, 500)
	return id, key, reason, f.err()
}

func parsePayment(req paymentRequest) (app.PaymentInput, error) {
	var f fields
	in := app.PaymentInput{
		CustomerID:        f.uuid("customerId", req.CustomerID),
		ReceivedOn:        f.date("receivedOn", req.ReceivedOn),
		Amount:            f.money("amount", req.Amount),
		Currency:          req.Currency,
		PaymentMethodCode: f.text("paymentMethodCode", req.PaymentMethodCode, 1, 10),
		Reference:         f.text("reference", req.Reference, 0, 100),
		Notes:             f.text("notes", req.Notes, 0, 1000),
	}
	if _, err := money.ParseCurrency(req.Currency); err != nil {
		f.add("currency", "debe ser un código ISO 4217 en mayúsculas")
	}
	if req.ExchangeRate != "" {
		if _, err := money.ParseExchangeRate(req.ExchangeRate); err != nil {
			f.add("exchangeRate", "debe ser un decimal en texto mayor que cero, con hasta 5 decimales")
		} else {
			in.ExchangeRate = decimal.RequireFromString(req.ExchangeRate)
		}
	}
	if len(req.Applications) > 100 {
		f.add("applications", "admite hasta 100 aplicaciones por pago")
	}
	for i, a := range req.Applications {
		prefix := "applications[" + strconv.Itoa(i) + "]."
		in.Applications = append(in.Applications, app.ApplicationInput{
			ReceivableID: f.uuid(prefix+"receivableId", a.ReceivableID),
			Amount:       f.money(prefix+"amount", a.Amount),
		})
	}
	return in, f.err()
}

func toPaymentResponse(p app.PaymentView) paymentResponse {
	out := paymentResponse{
		ID: p.ID, CustomerID: p.CustomerID, ReceivedOn: p.ReceivedOn.String(), Amount: p.Amount, Currency: p.Currency,
		ExchangeRate: p.ExchangeRate, PaymentMethodCode: p.PaymentMethodCode, Reference: p.Reference, Notes: p.Notes,
		Status: string(p.Status), VoidReason: p.VoidReason, VoidedAt: p.VoidedAt, CreatedAt: p.CreatedAt,
		Applications: make([]applicationResponse, 0, len(p.Applications)),
	}
	for _, a := range p.Applications {
		out.Applications = append(out.Applications, toApplicationResponse(a))
	}
	return out
}

func toApplicationResponse(a app.ApplicationView) applicationResponse {
	return applicationResponse{
		ID: a.ID, PaymentID: a.PaymentID, ReceivableID: a.ReceivableID, Amount: a.Amount, AppliedAt: a.AppliedAt,
		ReversedAt: a.ReversedAt, ReversalReason: a.ReversalReason,
	}
}
