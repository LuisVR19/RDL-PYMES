package http

import (
	"context"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

type (
	createProduct interface {
		Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in product.NewInput) (app.CreateProductResult, error)
	}
	listProducts interface {
		Execute(ctx context.Context, t tenancy.Context, q app.ProductQuery) (app.ProductPage, error)
	}
	getProduct interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (product.Product, error)
	}
	updateProduct interface {
		Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p product.Patch) (product.Product, error)
	}
)

type ProductHandlers struct {
	Create createProduct
	List   listProducts
	Get    getProduct
	Update updateProduct
}

// Representación de openapi/billing.yaml (Product, ProductInput) del repo de contratos.
type productTaxDTO struct {
	TaxTypeCode string `json:"taxTypeCode"`
	TaxRateCode string `json:"taxRateCode"`
}

type productResponse struct {
	ID                uuid.UUID       `json:"id"`
	Code              string          `json:"code"`
	Description       string          `json:"description"`
	CabysCode         string          `json:"cabysCode"`
	UnitOfMeasureCode string          `json:"unitOfMeasureCode"`
	UnitPrice         string          `json:"unitPrice"`
	Currency          string          `json:"currency"`
	IsService         bool            `json:"isService"`
	IsActive          bool            `json:"isActive"`
	Taxes             []productTaxDTO `json:"taxes"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

type productPageResponse struct {
	Items      []productResponse `json:"items"`
	NextCursor *string           `json:"nextCursor"`
}

// Los montos llegan como string (convenciones §3): un JSON number es un 400. Se parsean aquí para responder 422 con
// el nombre del campo si el formato no es el del contrato.
type createProductRequest struct {
	Code              string          `json:"code"`
	Description       string          `json:"description"`
	CabysCode         string          `json:"cabysCode"`
	UnitOfMeasureCode string          `json:"unitOfMeasureCode"`
	UnitPrice         *string         `json:"unitPrice" validate:"required"`
	Currency          *string         `json:"currency" validate:"required"`
	IsService         *bool           `json:"isService" validate:"required"`
	IsActive          *bool           `json:"isActive"`
	Taxes             []productTaxDTO `json:"taxes"`
}

// updateProductRequest: campo ausente = no cambia; taxes presente reemplaza la lista. Un ProductInput completo
// (el cuerpo que define el contrato para este PATCH) también es válido.
type updateProductRequest struct {
	Code              *string          `json:"code"`
	Description       *string          `json:"description"`
	CabysCode         *string          `json:"cabysCode"`
	UnitOfMeasureCode *string          `json:"unitOfMeasureCode"`
	UnitPrice         *string          `json:"unitPrice"`
	Currency          *string          `json:"currency"`
	IsService         *bool            `json:"isService"`
	IsActive          *bool            `json:"isActive"`
	Taxes             *[]productTaxDTO `json:"taxes"`
}

func (h *ProductHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/products", func(w http.ResponseWriter, r *http.Request) {
		limit, after, fields := parsePage(r)
		q := app.ProductQuery{Limit: limit, After: after, Search: r.URL.Query().Get("q")}
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
		out := productPageResponse{Items: make([]productResponse, 0, len(page.Items))}
		for _, p := range page.Items {
			out.Items = append(out.Items, toProductResponse(p))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("POST /v1/products", func(w http.ResponseWriter, r *http.Request) {
		key, err := idempotencyKey(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req createProductRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var fields []problem.FieldError
		price := parseAmountField(req.UnitPrice, "unitPrice", &fields)
		currency := parseCurrencyField(req.Currency, "currency", &fields)
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		res, err := h.Create.Execute(r.Context(), t, key, product.NewInput{
			Code: req.Code, Description: req.Description, CabysCode: req.CabysCode,
			UnitOfMeasureCode: req.UnitOfMeasureCode, UnitPrice: *price, Currency: *currency,
			IsService: *req.IsService, IsActive: req.IsActive, Taxes: toTaxes(req.Taxes),
		})
		if err != nil {
			fail(w, r, err)
			return
		}
		if res.Replayed {
			w.Header().Set("Idempotent-Replayed", "true")
		}
		writeJSON(w, http.StatusCreated, toProductResponse(res.Product))
	})

	rt.handle("GET /v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		p, err := h.Get.Execute(r.Context(), t, id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toProductResponse(p))
	})

	rt.handle("PATCH /v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var req updateProductRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var fields []problem.FieldError
		ch := product.Patch{
			Code: req.Code, Description: req.Description, CabysCode: req.CabysCode,
			UnitOfMeasureCode: req.UnitOfMeasureCode, IsService: req.IsService, IsActive: req.IsActive,
			UnitPrice: parseAmountField(req.UnitPrice, "unitPrice", &fields),
			Currency:  parseCurrencyField(req.Currency, "currency", &fields),
		}
		if req.Taxes != nil {
			taxes := toTaxes(*req.Taxes)
			ch.Taxes = &taxes
		}
		if len(fields) > 0 {
			fail(w, r, validationError{fields: fields})
			return
		}
		t, _ := tenancy.From(r.Context())
		p, err := h.Update.Execute(r.Context(), t, id, ch)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toProductResponse(p))
	})
}

// parseAmountField devuelve nil si el campo no vino; si vino con formato inválido, agrega el error del campo.
func parseAmountField(v *string, field string, fields *[]problem.FieldError) *money.Amount {
	if v == nil {
		return nil
	}
	a, err := money.ParseAmount(*v)
	if err != nil {
		*fields = append(*fields, problem.FieldError{Field: field,
			Message: "debe ser un monto decimal como string: hasta 13 enteros y 5 decimales, sin signo ni separador de miles"})
		return nil
	}
	return &a
}

func parseCurrencyField(v *string, field string, fields *[]problem.FieldError) *money.Currency {
	if v == nil {
		return nil
	}
	c, err := money.ParseCurrency(*v)
	if err != nil {
		*fields = append(*fields, problem.FieldError{Field: field, Message: "debe ser un código ISO 4217 en mayúsculas"})
		return nil
	}
	return &c
}

func toTaxes(in []productTaxDTO) []product.Tax {
	out := make([]product.Tax, 0, len(in))
	for _, t := range in {
		out = append(out, product.Tax{TypeCode: t.TaxTypeCode, RateCode: t.TaxRateCode})
	}
	return out
}

func toProductResponse(p product.Product) productResponse {
	taxes := make([]productTaxDTO, 0, len(p.Taxes))
	for _, t := range p.Taxes {
		taxes = append(taxes, productTaxDTO{TaxTypeCode: t.TypeCode, TaxRateCode: t.RateCode})
	}
	return productResponse{
		ID: p.ID, Code: p.Code, Description: p.Description, CabysCode: p.CabysCode,
		UnitOfMeasureCode: p.UnitOfMeasureCode, UnitPrice: p.UnitPrice.String(), Currency: p.Currency.String(),
		IsService: p.IsService, IsActive: p.IsActive, Taxes: taxes,
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.UpdatedAt.UTC(),
	}
}
