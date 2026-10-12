package http

import (
	"context"
	"net/http"
	"strconv"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/pkg/tenancy"
)

// CatalogHandlers son /v1/catalogs (searchCabys y getFiscalCatalog de openapi/fiscal.yaml).
type CatalogHandlers struct {
	List interface {
		Execute(ctx context.Context, t tenancy.Context, catalog string, f app.CatalogFilter) ([]app.CatalogEntry, error)
	}
	Cabys interface {
		Execute(ctx context.Context, t tenancy.Context, q string, limit int) ([]app.CabysEntry, error)
	}
}

type catalogEntryResponse struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	Rate               string `json:"rate,omitempty"`
	RequiresCreditTerm *bool  `json:"requiresCreditTerm,omitempty"`
}

type cabysEntryResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	TaxRate     string `json:"taxRate,omitempty"`
	TaxLabel    string `json:"taxLabel,omitempty"`
}

func (h *CatalogHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/catalogs/cabys", func(w http.ResponseWriter, r *http.Request) {
		limit := 0
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				fail(w, r, app.ValidationError{Field: "limit", Message: "debe ser un entero"})
				return
			}
			limit = n
		}
		t, _ := tenancy.From(r.Context())
		items, err := h.Cabys.Execute(r.Context(), t, r.URL.Query().Get("q"), limit)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]cabysEntryResponse, 0, len(items))
		for _, it := range items {
			out = append(out, cabysEntryResponse(it))
		}
		writeJSON(w, http.StatusOK, out)
	})
	rt.handle("GET /v1/catalogs/{catalog}", func(w http.ResponseWriter, r *http.Request) {
		t, _ := tenancy.From(r.Context())
		f := app.CatalogFilter{ProvinceCode: r.URL.Query().Get("provinceCode"), CantonCode: r.URL.Query().Get("cantonCode")}
		entries, err := h.List.Execute(r.Context(), t, r.PathValue("catalog"), f)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := make([]catalogEntryResponse, 0, len(entries))
		for _, e := range entries {
			out = append(out, catalogEntryResponse(e))
		}
		writeJSON(w, http.StatusOK, out)
	})
}
