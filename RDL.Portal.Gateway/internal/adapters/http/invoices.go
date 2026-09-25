package http

import (
	"log/slog"
	"net/http"

	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/domain/view"
)

// InvoiceListHandler publica el listado de documentos enriquecido (pantalla 12).
type InvoiceListHandler struct {
	Service *app.InvoiceListService
	Log     *slog.Logger
}

type invoicePageOutDTO struct {
	Items      []invoiceListItemDTO `json:"items"`
	NextCursor *string              `json:"nextCursor"`
}

type invoiceListItemDTO struct {
	Invoice    invoiceRowDTO  `json:"invoice"`
	Fiscal     fiscalPartDTO  `json:"fiscal"`
	Receivable balancePartDTO `json:"receivable"`
}

type invoiceRowDTO struct {
	ID                 string `json:"id"`
	DocumentType       string `json:"documentType"`
	Number             string `json:"number,omitempty"`
	Status             string `json:"status"`
	RequiresCorrection bool   `json:"requiresCorrection"`
	CustomerID         string `json:"customerId"`
	CustomerLegalName  string `json:"customerLegalName,omitempty"`
	IssuedAt           string `json:"issuedAt,omitempty"`
	DueDate            string `json:"dueDate,omitempty"`
	Currency           string `json:"currency"`
	Total              string `json:"total"`
	CreatedAt          string `json:"createdAt"`
}

func (h *InvoiceListHandler) List(w http.ResponseWriter, r *http.Request) {
	page, err := h.Service.ListInvoices(r.Context(), listQuery(r))
	if err != nil {
		writePrimaryError(w, r, h.Log, err, "no se pudo armar el listado de documentos")
		return
	}
	writeJSON(w, http.StatusOK, toInvoicePageDTO(page))
}

// listQuery copia solo los filtros declarados en la tabla. Lo que no está en la lista no viaja: tampoco una
// organización, aunque el cliente la mande.
func listQuery(r *http.Request) map[string][]string {
	in := r.URL.Query()
	out := map[string][]string{}
	for _, name := range routes.InvoiceListParams {
		if v, ok := in[name]; ok {
			out[name] = v
		}
	}
	return out
}

func toInvoicePageDTO(p view.InvoicePage) invoicePageOutDTO {
	out := invoicePageOutDTO{Items: make([]invoiceListItemDTO, 0, len(p.Items))}
	if p.NextCursor != "" {
		c := p.NextCursor
		out.NextCursor = &c
	}
	for _, it := range p.Items {
		inv := it.Invoice
		out.Items = append(out.Items, invoiceListItemDTO{
			Invoice: invoiceRowDTO{
				ID: inv.ID, DocumentType: inv.DocumentType, Number: inv.Number, Status: inv.Status,
				RequiresCorrection: inv.RequiresCorrection, CustomerID: inv.CustomerID,
				CustomerLegalName: inv.CustomerLegalName, IssuedAt: inv.IssuedAt, DueDate: inv.DueDate,
				Currency: inv.Currency, Total: inv.Total, CreatedAt: inv.CreatedAt,
			},
			Fiscal:     toFiscalPartDTO(it.Fiscal),
			Receivable: toBalancePartDTO(it.Receivable),
		})
	}
	return out
}
