package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
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

type getReceivable interface {
	Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (app.ReceivableDetail, error)
}

type getBalanceByInvoice interface {
	Execute(ctx context.Context, t tenancy.Context, invoiceID uuid.UUID) (app.ReceivableView, error)
}

type getAging interface {
	Execute(ctx context.Context, t tenancy.Context, q app.AgingQuery) (app.AgingReport, error)
}

type ReceivableHandlers struct {
	List      listReceivables
	Get       getReceivable
	ByInvoice getBalanceByInvoice
	Aging     getAging
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

// receivableDetailResponse: la cuenta con su historial (aplicaciones vigentes y revertidas, ajustes).
type receivableDetailResponse struct {
	receivableResponse
	Applications []applicationResponse `json:"applications"`
	Adjustments  []adjustmentResponse  `json:"adjustments"`
}

type adjustmentResponse struct {
	ID               uuid.UUID  `json:"id"`
	AdjustmentType   string     `json:"adjustmentType"`
	Amount           string     `json:"amount"`
	SourceDocumentID *uuid.UUID `json:"sourceDocumentId,omitempty"`
	Reason           string     `json:"reason"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// balanceResponse sigue RDL.Contracts/openapi/bff-internal.yaml (getBalanceByInvoice).
type balanceResponse struct {
	ReceivableID  uuid.UUID `json:"receivableId"`
	Status        string    `json:"status"`
	Currency      string    `json:"currency"`
	BalanceAmount string    `json:"balanceAmount"`
	DueOn         string    `json:"dueOn"`
}

type agingResponse struct {
	Bucket   string `json:"bucket"`
	Currency string `json:"currency"`
	Balance  string `json:"balance"`
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

	// Literal antes que {id}: el mux de Go elige el patrón más específico, "aging" nunca se toma como id.
	rt.handle("GET /v1/receivables/aging", func(w http.ResponseWriter, r *http.Request) {
		q, err := parseAgingQuery(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		rep, err := h.Aging.Execute(r.Context(), t, q)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]agingResponse, 0, len(rep.Buckets))
		for _, b := range rep.Buckets {
			out = append(out, agingResponse{Bucket: string(b.Bucket), Currency: b.Currency, Balance: b.Balance.String()})
		}
		w.Header().Set("X-Aging-As-Of", rep.AsOf.String())
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("GET /v1/receivables/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "id")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		d, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := receivableDetailResponse{
			receivableResponse: toReceivableResponse(d.ReceivableView),
			Applications:       make([]applicationResponse, 0, len(d.Applications)),
			Adjustments:        make([]adjustmentResponse, 0, len(d.Adjustments)),
		}
		for _, a := range d.Applications {
			out.Applications = append(out.Applications, toApplicationResponse(a))
		}
		for _, a := range d.Adjustments {
			adj := adjustmentResponse{ID: a.ID, AdjustmentType: string(a.Type), Amount: a.Amount, Reason: a.Reason, CreatedAt: a.CreatedAt.UTC()}
			if a.SourceDocumentID != uuid.Nil {
				id := a.SourceDocumentID
				adj.SourceDocumentID = &id
			}
			out.Adjustments = append(out.Adjustments, adj)
		}
		writeJSON(w, http.StatusOK, out)
	})
}

// registerInternal: endpoints para el BFF. Mismo JWT del usuario y TenantContext que /v1 (R10).
func (h *ReceivableHandlers) registerInternal(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /internal/v1/receivables/by-invoice/{invoiceId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathID(r, "invoiceId")
		if !ok {
			fail(w, r, app.ErrNotFound)
			return
		}
		t, _ := tenancy.From(r.Context())
		v, err := h.ByInvoice.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, balanceResponse{
			ReceivableID: v.ID, Status: string(v.Status), Currency: v.Currency, BalanceAmount: v.BalanceAmount, DueOn: v.DueOn.String(),
		})
	})
}

func parseAgingQuery(r *http.Request) (app.AgingQuery, error) {
	var f fields
	var q app.AgingQuery
	values := r.URL.Query()
	if v := values.Get("asOf"); v != "" {
		q.AsOf = f.date("asOf", v)
	}
	if v := values.Get("currency"); v != "" {
		if len(v) != 3 || strings.ToUpper(v) != v {
			f.add("currency", "debe ser un código ISO 4217 en mayúsculas")
		}
		q.Currency = v
	}
	return q, f.err()
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
