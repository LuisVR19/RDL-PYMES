package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/pkg/tenancy"
)

type listReceivables interface {
	Execute(ctx context.Context, t tenancy.Context, q app.ReceivableQuery) (app.ReceivablePage, error)
}

type ReceivableHandlers struct {
	List listReceivables
}

// receivableResponse sigue el schema Receivable de RDL.Contracts/openapi/receivables.yaml.
type receivableResponse struct {
	ID                uuid.UUID  `json:"id"`
	SourceInvoiceID   uuid.UUID  `json:"sourceInvoiceId"`
	CustomerID        uuid.UUID  `json:"customerId"`
	CustomerLegalName string     `json:"customerLegalName"`
	DocumentNumber    string     `json:"documentNumber"`
	Currency          string     `json:"currency"`
	OriginalAmount    string     `json:"originalAmount"`
	BalanceAmount     string     `json:"balanceAmount"`
	IssuedOn          string     `json:"issuedOn"`
	DueOn             string     `json:"dueOn"`
	Status            string     `json:"status"`
	SettledAt         *time.Time `json:"settledAt,omitempty"`
}

type receivablePageResponse struct {
	Items      []receivableResponse `json:"items"`
	NextCursor *string              `json:"nextCursor"`
}

func (h *ReceivableHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/receivables", func(w http.ResponseWriter, r *http.Request) {
		q, err := parseReceivableQuery(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		page, err := h.List.Execute(r.Context(), t, q)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := receivablePageResponse{Items: make([]receivableResponse, 0, len(page.Items))}
		for _, v := range page.Items {
			out.Items = append(out.Items, toReceivableResponse(v))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func toReceivableResponse(v app.ReceivableView) receivableResponse {
	out := receivableResponse{
		ID: v.ID, SourceInvoiceID: v.SourceInvoiceID, CustomerID: v.CustomerID, CustomerLegalName: v.CustomerLegalName,
		DocumentNumber: v.DocumentNumber, Currency: v.Currency, OriginalAmount: v.OriginalAmount,
		BalanceAmount: v.BalanceAmount, IssuedOn: v.IssuedOn.String(), DueOn: v.DueOn.String(), Status: string(v.Status),
	}
	if v.SettledAt != nil {
		at := v.SettledAt.UTC()
		out.SettledAt = &at
	}
	return out
}

// parseReceivableQuery lee los filtros del contrato. Un organization_id en la query se ignora: el tenant sale
// solo del TenantContext.
func parseReceivableQuery(r *http.Request) (app.ReceivableQuery, error) {
	limit, after, fields := parsePage(r)
	q := app.ReceivableQuery{Limit: limit, After: after}
	values := r.URL.Query()
	if v := values.Get("status"); v != "" {
		q.Status = receivable.Status(v)
		if !q.Status.Valid() {
			fields = append(fields, problem.FieldError{Field: "status", Message: "debe ser uno de: open partially_paid paid cancelled"})
		}
	}
	if v := values.Get("customerId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil || id == uuid.Nil {
			fields = append(fields, problem.FieldError{Field: "customerId", Message: "debe ser un UUID"})
		}
		q.CustomerID = id
	}
	if v := values.Get("overdue"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil || (v != "true" && v != "false") {
			fields = append(fields, problem.FieldError{Field: "overdue", Message: "debe ser true o false"})
		}
		q.Overdue = &b
	}
	if len(fields) > 0 {
		return app.ReceivableQuery{}, validationError{fields: fields}
	}
	return q, nil
}
