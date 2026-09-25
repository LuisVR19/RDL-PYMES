package http

import (
	"errors"
	"log/slog"
	"net/http"

	"rdl/portal-gateway/internal/adapters/downstream"
	"rdl/portal-gateway/internal/adapters/http/problem"
	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/view"
)

// OverviewHandler publica la vista transversal de una factura: el total de Billing, el estado de Hacienda
// y el saldo, en una sola respuesta (arquitectura 2.2).
type OverviewHandler struct {
	Service *app.OverviewService
	Log     *slog.Logger
}

// Los DTO llevan el JSON que promete api/openapi.yaml. El modelo de dominio no tiene etiquetas: cambiar la
// forma de la respuesta no puede arrastrar al dominio, ni al revés.

type overviewDTO struct {
	Invoice    invoiceSummaryDTO `json:"invoice"`
	Fiscal     fiscalPartDTO     `json:"fiscal"`
	Receivable balancePartDTO    `json:"receivable"`
}

type invoiceSummaryDTO struct {
	ID                 string `json:"id"`
	DocumentType       string `json:"documentType"`
	Number             string `json:"number,omitempty"`
	Status             string `json:"status"`
	RequiresCorrection bool   `json:"requiresCorrection"`
	CustomerLegalName  string `json:"customerLegalName,omitempty"`
	Currency           string `json:"currency"`
	Total              string `json:"total"`
}

// availability es siempre obligatorio; el dato solo viene cuando es "available". El portal decide qué mostrar
// mirando ese campo, no la ausencia del objeto.
type fiscalPartDTO struct {
	Availability string           `json:"availability"`
	Status       *fiscalStatusDTO `json:"status,omitempty"`
}

type fiscalStatusDTO struct {
	ElectronicDocumentID  string `json:"electronicDocumentId"`
	Status                string `json:"status"`
	HaciendaStatusMessage string `json:"haciendaStatusMessage,omitempty"`
}

type balancePartDTO struct {
	Availability string      `json:"availability"`
	Balance      *balanceDTO `json:"balance,omitempty"`
}

type balanceDTO struct {
	ReceivableID  string `json:"receivableId"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	BalanceAmount string `json:"balanceAmount"`
	DueOn         string `json:"dueOn"`
}

func (h *OverviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	overview, err := h.Service.InvoiceOverview(r.Context(), r.PathValue("id"))
	if err != nil {
		writePrimaryError(w, r, h.Log, err, "no se pudo armar la vista de la factura",
			slog.String("invoiceId", r.PathValue("id")))
		return
	}
	writeJSON(w, http.StatusOK, toOverviewDTO(overview))
}

// writePrimaryError responde cuando falla la fuente PRINCIPAL de una composición (Billing): si mandó Problem
// Details, el portal recibe el suyo tal cual; si no, uno del gateway que conserva lo que se sepa.
func writePrimaryError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error, msg string, attrs ...any) {
	var status *downstream.StatusError
	if errors.As(err, &status) && len(status.Problem) > 0 {
		w.Header().Set("Content-Type", problem.ContentType)
		w.WriteHeader(status.Status)
		if _, err := w.Write(status.Problem); err != nil {
			log.WarnContext(r.Context(), "no se pudo reenviar el problema de la API", slog.Any("error", err))
		}
		return
	}

	log.ErrorContext(r.Context(), msg, append(attrs, slog.Any("error", err))...)

	switch {
	case errors.As(err, &status):
		// La API respondió, pero sin Problem Details: se conserva su status y el `type` es del gateway.
		problem.Write(w, r, problem.New(status.Status, "upstream-error", "La API respondió con un error"))
	case errors.Is(err, app.ErrNotConfigured):
		problem.Write(w, r, problem.UpstreamNotConfigured)
	case errors.Is(err, app.ErrTimeout):
		problem.Write(w, r, problem.UpstreamTimeout)
	default:
		problem.Write(w, r, problem.UpstreamUnavailable)
	}
}

func toOverviewDTO(o view.InvoiceOverview) overviewDTO {
	dto := overviewDTO{
		Invoice: invoiceSummaryDTO{
			ID:                 o.Invoice.ID,
			DocumentType:       o.Invoice.DocumentType,
			Number:             o.Invoice.Number,
			Status:             o.Invoice.Status,
			RequiresCorrection: o.Invoice.RequiresCorrection,
			CustomerLegalName:  o.Invoice.CustomerLegalName,
			Currency:           o.Invoice.Currency,
			Total:              o.Invoice.Total,
		},
		Fiscal:     toFiscalPartDTO(o.Fiscal),
		Receivable: toBalancePartDTO(o.Receivable),
	}
	return dto
}

func toFiscalPartDTO(p view.FiscalPart) fiscalPartDTO {
	dto := fiscalPartDTO{Availability: string(p.Availability)}
	if s := p.Status; s != nil {
		dto.Status = &fiscalStatusDTO{
			ElectronicDocumentID:  s.ElectronicDocumentID,
			Status:                s.Status,
			HaciendaStatusMessage: s.HaciendaStatusMessage,
		}
	}
	return dto
}

func toBalancePartDTO(p view.BalancePart) balancePartDTO {
	dto := balancePartDTO{Availability: string(p.Availability)}
	if b := p.Balance; b != nil {
		dto.Balance = &balanceDTO{
			ReceivableID:  b.ReceivableID,
			Status:        b.Status,
			Currency:      b.Currency,
			BalanceAmount: b.BalanceAmount,
			DueOn:         b.DueOn,
		}
	}
	return dto
}
