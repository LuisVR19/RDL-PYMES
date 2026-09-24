package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/pkg/tenancy"
)

type (
	createInvoice interface {
		Execute(ctx context.Context, t tenancy.Context, key string, h app.HeaderInput, lines []app.LineRequest) (app.CreateInvoiceResult, error)
	}
	listInvoices interface {
		Execute(ctx context.Context, t tenancy.Context, q app.InvoiceQuery) (app.InvoicePage, error)
	}
	getInvoice interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (invoice.Invoice, error)
	}
	updateInvoice interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p invoice.HeaderPatch, lines *[]app.LineRequest) (invoice.Invoice, error)
	}
	replaceInvoiceLines interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, lines []app.LineRequest) (invoice.Invoice, error)
	}
	discardInvoice interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) error
	}
	invoiceHistory interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) ([]app.StatusChange, error)
	}
	issueInvoice interface {
		Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID) (app.IssueResult, error)
	}
)

type InvoiceHandlers struct {
	Create       createInvoice
	List         listInvoices
	Get          getInvoice
	Update       updateInvoice
	ReplaceLines replaceInvoiceLines
	Discard      discardInvoice
	History      invoiceHistory
	Issue        issueInvoice
}

// --- requests (InvoiceDraftInput e InvoiceLineInput del contrato) ---

type invoiceLineRequest struct {
	ProductID      *uuid.UUID `json:"productId" validate:"required"`
	Quantity       *string    `json:"quantity" validate:"required"`
	UnitPrice      *string    `json:"unitPrice"`
	Discount       *string    `json:"discount"`
	DiscountReason string     `json:"discountReason"`
}

type createInvoiceRequest struct {
	DocumentType        string               `json:"documentType" validate:"required"`
	CustomerID          *uuid.UUID           `json:"customerId" validate:"required"`
	BranchID            *uuid.UUID           `json:"branchId"`
	ReferencedInvoiceID *uuid.UUID           `json:"referencedInvoiceId"`
	ReferenceReason     string               `json:"referenceReason"`
	SaleConditionCode   string               `json:"saleConditionCode"`
	CreditTermDays      *int                 `json:"creditTermDays"`
	Currency            *string              `json:"currency" validate:"required"`
	ExchangeRate        *string              `json:"exchangeRate"`
	Notes               string               `json:"notes"`
	Lines               []invoiceLineRequest `json:"lines" validate:"dive"`
}

// updateInvoiceRequest: campo ausente = no cambia; branchId y creditTermDays en null los quitan. Un
// InvoiceDraftInput completo (el cuerpo del contrato) también es válido; lines, si viene, reemplaza las líneas.
type updateInvoiceRequest struct {
	DocumentType        *string               `json:"documentType"`
	CustomerID          *uuid.UUID            `json:"customerId"`
	BranchID            optional[uuid.UUID]   `json:"branchId"`
	ReferencedInvoiceID *uuid.UUID            `json:"referencedInvoiceId"`
	ReferenceReason     *string               `json:"referenceReason"`
	SaleConditionCode   *string               `json:"saleConditionCode"`
	CreditTermDays      optional[int]         `json:"creditTermDays"`
	Currency            *string               `json:"currency"`
	ExchangeRate        *string               `json:"exchangeRate"`
	Notes               *string               `json:"notes"`
	Lines               *[]invoiceLineRequest `json:"lines" validate:"omitnil,dive"`
}

// linesBody envuelve el arreglo de PUT /lines para validarlo; con nombre, los errores salen como lines[i].campo.
type linesBody struct {
	Lines []invoiceLineRequest `json:"lines" validate:"dive"`
}

// optional distingue un campo ausente (Set false) de uno en null (Set true, Value nil).
type optional[T any] struct {
	Set   bool
	Value *T
}

func (o *optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// --- responses (Invoice del contrato; las líneas son DocumentLine v1) ---

type customerSnapshotDTO struct {
	CustomerID     uuid.UUID         `json:"customerId"`
	Identification identificationDTO `json:"identification"`
	LegalName      string            `json:"legalName"`
	Email          string            `json:"email,omitempty"`
	Phone          string            `json:"phone,omitempty"`
	Address        string            `json:"address,omitempty"`
}

type lineTaxDTO struct {
	TaxTypeCode string          `json:"taxTypeCode"`
	TaxRateCode string          `json:"taxRateCode,omitempty"`
	Rate        string          `json:"rate"`
	TaxableBase string          `json:"taxableBase"`
	Amount      string          `json:"amount"`
	Exoneration *exonerationDTO `json:"exoneration,omitempty"`
}

type exonerationDTO struct {
	DocumentTypeCode string    `json:"documentTypeCode"`
	DocumentNumber   string    `json:"documentNumber"`
	Institution      string    `json:"institution"`
	IssuedAt         time.Time `json:"issuedAt"`
	ExoneratedRate   string    `json:"exoneratedRate"`
	Amount           string    `json:"amount"`
}

type invoiceLineDTO struct {
	LineNumber        int          `json:"lineNumber"`
	ProductID         *uuid.UUID   `json:"productId,omitempty"`
	ProductCode       string       `json:"productCode,omitempty"`
	CabysCode         string       `json:"cabysCode"`
	Description       string       `json:"description"`
	UnitOfMeasureCode string       `json:"unitOfMeasureCode"`
	IsService         bool         `json:"isService"`
	Quantity          string       `json:"quantity"`
	UnitPrice         string       `json:"unitPrice"`
	Discount          string       `json:"discount"`
	DiscountReason    string       `json:"discountReason,omitempty"`
	Subtotal          string       `json:"subtotal"`
	Tax               string       `json:"tax"`
	Total             string       `json:"total"`
	Taxes             []lineTaxDTO `json:"taxes"`
}

type invoiceResponse struct {
	ID                    uuid.UUID            `json:"id"`
	DocumentType          string               `json:"documentType"`
	Number                *string              `json:"number"`
	Status                string               `json:"status"`
	RequiresCorrection    bool                 `json:"requiresCorrection"`
	FiscalRejectionReason string               `json:"fiscalRejectionReason,omitempty"`
	CustomerID            uuid.UUID            `json:"customerId"`
	CustomerSnapshot      *customerSnapshotDTO `json:"customerSnapshot,omitempty"`
	BranchID              *uuid.UUID           `json:"branchId,omitempty"`
	SaleConditionCode     string               `json:"saleConditionCode"`
	CreditTermDays        *int                 `json:"creditTermDays,omitempty"`
	IssuedAt              *time.Time           `json:"issuedAt,omitempty"`
	DueDate               string               `json:"dueDate,omitempty"`
	Currency              string               `json:"currency"`
	ExchangeRate          string               `json:"exchangeRate"`
	Notes                 string               `json:"notes,omitempty"`
	Lines                 []invoiceLineDTO     `json:"lines"`
	Subtotal              string               `json:"subtotal"`
	Discount              string               `json:"discount"`
	Tax                   string               `json:"tax"`
	Exoneration           string               `json:"exoneration"`
	Total                 string               `json:"total"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

type invoicePageResponse struct {
	Items      []invoiceResponse `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

type statusChangeDTO struct {
	FromStatus      string     `json:"fromStatus,omitempty"`
	ToStatus        string     `json:"toStatus"`
	Reason          string     `json:"reason,omitempty"`
	ChangedByUserID *uuid.UUID `json:"changedByUserId,omitempty"`
	ChangedAt       time.Time  `json:"changedAt"`
}

func (h *InvoiceHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/invoices", func(w http.ResponseWriter, r *http.Request) {
		q, err := parseInvoiceQuery(r)
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
		out := invoicePageResponse{Items: make([]invoiceResponse, 0, len(page.Items))}
		for _, inv := range page.Items {
			out.Items = append(out.Items, toInvoiceResponse(inv))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/invoices", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createInvoiceRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var fields []problem.FieldError
		if req.ReferencedInvoiceID != nil || req.ReferenceReason != "" {
			fields = append(fields, problem.FieldError{Field: "referencedInvoiceId", Message: "solo aplica a notas de crédito y débito (F5)"})
		}
		currency := parseCurrencyField(req.Currency, "currency", &fields)
		var rate *money.ExchangeRate
		if req.ExchangeRate != nil {
			rate = parseExchangeRateField(*req.ExchangeRate, &fields)
		}
		lines := parseLines(req.Lines, &fields)
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, app.HeaderInput{
			DocumentType: invoice.DocumentType(req.DocumentType), CustomerID: *req.CustomerID, BranchID: req.BranchID,
			SaleConditionCode: req.SaleConditionCode, CreditTermDays: req.CreditTermDays, Currency: *currency,
			ExchangeRate: rate, Notes: req.Notes,
		}, lines)
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, toInvoiceResponse(res.Invoice))
	})

	rt.handle("GET /v1/invoices/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		inv, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toInvoiceResponse(inv))
	})

	rt.handle("PATCH /v1/invoices/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req updateInvoiceRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var fields []problem.FieldError
		// El tipo no cambia: una nota es otro documento. Mandar el mismo tipo (cuerpo completo del contrato) es válido.
		if req.DocumentType != nil && invoice.DocumentType(*req.DocumentType) != invoice.TypeInvoice {
			fields = append(fields, problem.FieldError{Field: "documentType", Message: "no se puede cambiar"})
		}
		if req.ReferencedInvoiceID != nil || req.ReferenceReason != nil && *req.ReferenceReason != "" {
			fields = append(fields, problem.FieldError{Field: "referencedInvoiceId", Message: "solo aplica a notas de crédito y débito (F5)"})
		}
		p := invoice.HeaderPatch{
			CustomerID: req.CustomerID, SaleConditionCode: req.SaleConditionCode, Notes: req.Notes,
			Currency: parseCurrencyField(req.Currency, "currency", &fields),
		}
		if req.BranchID.Set {
			p.BranchID = &req.BranchID.Value
		}
		if req.CreditTermDays.Set {
			p.CreditTermDays = &req.CreditTermDays.Value
		}
		if req.ExchangeRate != nil {
			p.ExchangeRate = parseExchangeRateField(*req.ExchangeRate, &fields)
		}
		var lines *[]app.LineRequest
		if req.Lines != nil {
			parsed := parseLines(*req.Lines, &fields)
			lines = &parsed
		}
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		inv, err := h.Update.Execute(r.Context(), t, id, p, lines)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toInvoiceResponse(inv))
	})

	rt.handle("PUT /v1/invoices/{id}/lines", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		// El contrato define el cuerpo como un arreglo con al menos una línea.
		var req []invoiceLineRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		if err := validateStruct(&linesBody{Lines: req}); err != nil {
			fail(w, r, err)
			return
		}
		var fields []problem.FieldError
		if len(req) == 0 {
			fields = append(fields, problem.FieldError{Field: "lines", Message: "debe tener al menos una línea"})
		}
		lines := parseLines(req, &fields)
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		inv, err := h.ReplaceLines.Execute(r.Context(), t, id, lines)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toInvoiceResponse(inv))
	})

	rt.handle("DELETE /v1/invoices/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		if err := h.Discard.Execute(r.Context(), t, id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Emisión: draft → issued. Exige Idempotency-Key; el reintento con la misma clave responde el mismo 201.
	rt.handle("POST /v1/invoices/{id}/issue", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Issue.Execute(r.Context(), t, key, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, toInvoiceResponse(res.Invoice))
	})

	rt.handle("GET /v1/invoices/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		changes, err := h.History.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]statusChangeDTO, 0, len(changes))
		for _, c := range changes {
			out = append(out, statusChangeDTO{
				FromStatus: string(c.From), ToStatus: string(c.To), Reason: c.Reason,
				ChangedByUserID: c.ChangedByUserID, ChangedAt: c.ChangedAt.UTC(),
			})
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func parseInvoiceQuery(r *http.Request) (app.InvoiceQuery, error) {
	limit, after, fields := parsePage(r)
	values := r.URL.Query()
	q := app.InvoiceQuery{Limit: limit, After: after}
	if v := values.Get("documentType"); v != "" {
		switch invoice.DocumentType(v) {
		case invoice.TypeInvoice, invoice.TypeCreditNote, invoice.TypeDebitNote:
			q.DocumentType = invoice.DocumentType(v)
		default:
			fields = append(fields, problem.FieldError{Field: "documentType", Message: "debe ser invoice, credit_note o debit_note"})
		}
	}
	if v := values.Get("status"); v != "" {
		switch invoice.Status(v) {
		case invoice.StatusDraft, invoice.StatusIssued, invoice.StatusCancelled:
			q.Status = invoice.Status(v)
		default:
			fields = append(fields, problem.FieldError{Field: "status", Message: "debe ser draft, issued o cancelled"})
		}
	}
	if v := values.Get("customerId"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			fields = append(fields, problem.FieldError{Field: "customerId", Message: "debe ser un UUID"})
		}
		q.CustomerID = &id
	}
	q.RequiresCorrection = parseBoolParam(r, "requiresCorrection", &fields)
	for name, dst := range map[string]*string{"issuedFrom": &q.IssuedFrom, "issuedTo": &q.IssuedTo} {
		if v := values.Get(name); v != "" {
			if _, err := time.Parse(time.DateOnly, v); err != nil {
				fields = append(fields, problem.FieldError{Field: name, Message: "debe ser una fecha YYYY-MM-DD"})
				continue
			}
			*dst = v
		}
	}
	if len(fields) > 0 {
		return app.InvoiceQuery{}, validationError{fields: fields}
	}
	return q, nil
}

func parseLines(in []invoiceLineRequest, fields *[]problem.FieldError) []app.LineRequest {
	out := make([]app.LineRequest, 0, len(in))
	for i, l := range in {
		name := func(f string) string { return fmt.Sprintf("lines[%d].%s", i, f) }
		lr := app.LineRequest{ProductID: *l.ProductID, DiscountReason: l.DiscountReason}
		q, err := money.ParseQuantity(*l.Quantity)
		if err != nil {
			*fields = append(*fields, problem.FieldError{Field: name("quantity"),
				Message: "debe ser una cantidad decimal como string mayor que cero, con hasta 3 decimales"})
		}
		lr.Quantity = q
		lr.UnitPrice = parseAmountField(l.UnitPrice, name("unitPrice"), fields)
		lr.Discount = parseAmountField(l.Discount, name("discount"), fields)
		out = append(out, lr)
	}
	return out
}

func parseExchangeRateField(v string, fields *[]problem.FieldError) *money.ExchangeRate {
	rate, err := money.ParseExchangeRate(v)
	if err != nil {
		*fields = append(*fields, problem.FieldError{Field: "exchangeRate",
			Message: "debe ser un decimal como string mayor que cero, con hasta 5 decimales"})
		return nil
	}
	return &rate
}

func toInvoiceResponse(inv invoice.Invoice) invoiceResponse {
	out := invoiceResponse{
		ID: inv.ID, DocumentType: string(inv.DocumentType), Status: string(inv.Status),
		RequiresCorrection: inv.RequiresCorrection, FiscalRejectionReason: inv.FiscalRejectionReason,
		CustomerID: inv.CustomerID, BranchID: inv.BranchID, SaleConditionCode: inv.SaleConditionCode,
		CreditTermDays: inv.CreditTermDays, DueDate: inv.DueDate, Currency: inv.Currency.String(),
		ExchangeRate: inv.ExchangeRate.String(), Notes: inv.Notes, Lines: make([]invoiceLineDTO, 0, len(inv.Lines)),
		Subtotal: inv.Totals.Subtotal.String(), Discount: inv.Totals.Discount.String(), Tax: inv.Totals.Tax.String(),
		Exoneration: inv.Totals.Exoneration.String(), Total: inv.Totals.Total.String(),
		CreatedAt: inv.CreatedAt.UTC(), UpdatedAt: inv.UpdatedAt.UTC(),
	}
	if inv.Number != "" {
		n := inv.Number
		out.Number = &n
	}
	if inv.IssuedAt != nil {
		at := inv.IssuedAt.UTC()
		out.IssuedAt = &at
	}
	// El snapshot existe desde la emisión; en borrador el cliente se identifica solo por customerId.
	if inv.Customer.LegalName != "" {
		c := inv.Customer
		out.CustomerSnapshot = &customerSnapshotDTO{
			CustomerID: inv.CustomerID, Identification: identificationDTO{TypeCode: c.IdentificationTypeCode, Number: c.IdentificationNumber},
			LegalName: c.LegalName, Email: c.Email, Phone: c.Phone, Address: c.Address,
		}
	}
	for _, l := range inv.Lines {
		dto := invoiceLineDTO{
			LineNumber: l.Number, ProductID: l.ProductID, ProductCode: l.ProductCode, CabysCode: l.CabysCode,
			Description: l.Description, UnitOfMeasureCode: l.UnitOfMeasureCode, IsService: l.IsService,
			Quantity: l.Quantity.String(), UnitPrice: l.UnitPrice.String(), Discount: l.Discount.String(),
			DiscountReason: l.DiscountReason, Subtotal: l.Subtotal.String(), Tax: l.Tax.String(), Total: l.Total.String(),
			Taxes: make([]lineTaxDTO, 0, len(l.Taxes)),
		}
		for _, t := range l.Taxes {
			td := lineTaxDTO{TaxTypeCode: t.TypeCode, TaxRateCode: t.RateCode, Rate: t.Rate.String(),
				TaxableBase: t.TaxableBase.String(), Amount: t.Amount.String()}
			if x := t.Exoneration; x != nil {
				td.Exoneration = &exonerationDTO{DocumentTypeCode: x.DocumentTypeCode, DocumentNumber: x.DocumentNumber,
					Institution: x.Institution, IssuedAt: x.IssuedAt.UTC(), ExoneratedRate: x.ExoneratedRate.String(),
					Amount: x.Amount.String()}
			}
			dto.Taxes = append(dto.Taxes, td)
		}
		out.Lines = append(out.Lines, dto)
	}
	return out
}
