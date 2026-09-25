package downstream

import (
	"context"
	"net/url"

	"rdl/portal-gateway/internal/domain/view"
)

// Los DTO de este archivo son la forma de `openapi/bff-internal.yaml` del repo de contratos. Se decodifican
// campo por campo y NO se recalcula nada: los montos llegan como string decimal y así se publican.

// SummarySource dice de dónde sale el resumen de factura de la vista transversal.
type SummarySource string

const (
	// SummaryInternal es la ruta del contrato: GET /internal/v1/invoices/{id}/summary.
	SummaryInternal SummarySource = "internal"
	// SummaryPublic la deriva del detalle público. Respaldo para una Billing anterior a la ruta interna:
	// trae la factura completa con sus líneas, así que pesa más.
	SummaryPublic SummarySource = "public"
)

// BillingReader lee de Billing el resumen de un documento comercial. Es la fuente PRINCIPAL de la vista.
type BillingReader struct {
	Client *Client
	Source SummarySource
}

type invoiceSummaryDTO struct {
	ID                 string `json:"id"`
	DocumentType       string `json:"documentType"`
	Number             string `json:"number"`
	Status             string `json:"status"`
	RequiresCorrection bool   `json:"requiresCorrection"`
	CustomerLegalName  string `json:"customerLegalName"`
	Currency           string `json:"currency"`
	Total              string `json:"total"`
}

// invoiceDTO es solo la parte del detalle público que la vista usa. El resto (líneas, impuestos, snapshot
// completo) se ignora a propósito: el Portal Gateway no lo necesita y no lo va a reinterpretar.
type invoiceDTO struct {
	ID                 string `json:"id"`
	DocumentType       string `json:"documentType"`
	Number             string `json:"number"`
	Status             string `json:"status"`
	RequiresCorrection bool   `json:"requiresCorrection"`
	Currency           string `json:"currency"`
	Total              string `json:"total"`
	CustomerSnapshot   struct {
		LegalName string `json:"legalName"`
	} `json:"customerSnapshot"`
}

func (b *BillingReader) InvoiceSummary(ctx context.Context, invoiceID string) (view.InvoiceSummary, error) {
	id := url.PathEscape(invoiceID)
	if b.Source == SummaryPublic {
		var dto invoiceDTO
		if err := b.Client.getJSON(ctx, "/v1/invoices/"+id, nil, &dto); err != nil {
			return view.InvoiceSummary{}, err
		}
		return view.InvoiceSummary{
			ID:                 dto.ID,
			DocumentType:       dto.DocumentType,
			Number:             dto.Number,
			Status:             dto.Status,
			RequiresCorrection: dto.RequiresCorrection,
			// El snapshot existe solo desde la emisión: en borrador queda vacío, y así se publica.
			CustomerLegalName: dto.CustomerSnapshot.LegalName,
			Currency:          dto.Currency,
			Total:             dto.Total,
		}, nil
	}

	var dto invoiceSummaryDTO
	if err := b.Client.getJSON(ctx, "/internal/v1/invoices/"+id+"/summary", nil, &dto); err != nil {
		return view.InvoiceSummary{}, err
	}
	return view.InvoiceSummary{
		ID:                 dto.ID,
		DocumentType:       dto.DocumentType,
		Number:             dto.Number,
		Status:             dto.Status,
		RequiresCorrection: dto.RequiresCorrection,
		CustomerLegalName:  dto.CustomerLegalName,
		Currency:           dto.Currency,
		Total:              dto.Total,
	}, nil
}

// FiscalReader lee de E-Invoice el estado del documento electrónico de una factura.
// TODO(P5): confirmar la ruta y los campos contra RDL.EInvoice.API cuando exista.
type FiscalReader struct {
	Client *Client
}

type fiscalStatusDTO struct {
	ElectronicDocumentID  string `json:"electronicDocumentId"`
	Status                string `json:"status"`
	HaciendaStatusMessage string `json:"haciendaStatusMessage"`
}

func (f *FiscalReader) FiscalStatusBySource(ctx context.Context, sourceDocumentID string) (view.FiscalStatus, error) {
	var dto fiscalStatusDTO
	path := "/internal/v1/electronic-documents/by-source/" + url.PathEscape(sourceDocumentID)
	if err := f.Client.getJSON(ctx, path, nil, &dto); err != nil {
		return view.FiscalStatus{}, err
	}
	return view.FiscalStatus{
		ElectronicDocumentID:  dto.ElectronicDocumentID,
		Status:                dto.Status,
		HaciendaStatusMessage: dto.HaciendaStatusMessage,
	}, nil
}

// ReceivablesReader lee de Receivables el saldo de la cuenta por cobrar de una factura.
// TODO(P6): confirmar la ruta y los campos contra RDL.Receivables.API cuando exista.
type ReceivablesReader struct {
	Client *Client
}

type balanceDTO struct {
	ReceivableID  string `json:"receivableId"`
	Status        string `json:"status"`
	Currency      string `json:"currency"`
	BalanceAmount string `json:"balanceAmount"`
	DueOn         string `json:"dueOn"`
}

func (r *ReceivablesReader) BalanceByInvoice(ctx context.Context, invoiceID string) (view.Balance, error) {
	var dto balanceDTO
	path := "/internal/v1/receivables/by-invoice/" + url.PathEscape(invoiceID)
	if err := r.Client.getJSON(ctx, path, nil, &dto); err != nil {
		return view.Balance{}, err
	}
	return view.Balance{
		ReceivableID:  dto.ReceivableID,
		Status:        dto.Status,
		Currency:      dto.Currency,
		BalanceAmount: dto.BalanceAmount,
		DueOn:         dto.DueOn,
	}, nil
}

// --- Listado de documentos (pantalla 12) ---

// invoicePageDTO es la parte de GET /v1/invoices de Billing que usa el listado. Las líneas se ignoran a
// propósito: la tabla no las muestra y el gateway no las reinterpreta.
type invoicePageDTO struct {
	Items []struct {
		ID                 string `json:"id"`
		DocumentType       string `json:"documentType"`
		Number             string `json:"number"`
		Status             string `json:"status"`
		RequiresCorrection bool   `json:"requiresCorrection"`
		CustomerID         string `json:"customerId"`
		CustomerSnapshot   struct {
			LegalName string `json:"legalName"`
		} `json:"customerSnapshot"`
		IssuedAt  string `json:"issuedAt"`
		DueDate   string `json:"dueDate"`
		Currency  string `json:"currency"`
		Total     string `json:"total"`
		CreatedAt string `json:"createdAt"`
	} `json:"items"`
	NextCursor string `json:"nextCursor"`
}

func (b *BillingReader) ListInvoices(ctx context.Context, query map[string][]string) (view.InvoiceRows, error) {
	var dto invoicePageDTO
	if err := b.Client.getJSON(ctx, "/v1/invoices", url.Values(query), &dto); err != nil {
		return view.InvoiceRows{}, err
	}
	rows := view.InvoiceRows{Rows: make([]view.InvoiceRow, 0, len(dto.Items)), NextCursor: dto.NextCursor}
	for _, it := range dto.Items {
		rows.Rows = append(rows.Rows, view.InvoiceRow{
			ID: it.ID, DocumentType: it.DocumentType, Number: it.Number, Status: it.Status,
			RequiresCorrection: it.RequiresCorrection, CustomerID: it.CustomerID,
			CustomerLegalName: it.CustomerSnapshot.LegalName, IssuedAt: it.IssuedAt, DueDate: it.DueDate,
			Currency: it.Currency, Total: it.Total, CreatedAt: it.CreatedAt,
		})
	}
	return rows, nil
}

// idBatchDTO es IdBatch de bff-internal.yaml.
type idBatchDTO struct {
	IDs []string `json:"ids"`
}

// FiscalStatusesBySource: POST /internal/v1/electronic-documents/by-source (getFiscalStatusesBySource).
// TODO(P5): confirmar contra RDL.EInvoice.API cuando exista; la ruta está propuesta, sin publicar, en contratos.
func (f *FiscalReader) FiscalStatusesBySource(ctx context.Context, ids []string) (map[string]view.FiscalStatus, error) {
	var dto struct {
		Items []struct {
			SourceDocumentID string `json:"sourceDocumentId"`
			fiscalStatusDTO
		} `json:"items"`
	}
	if err := f.Client.postReadJSON(ctx, "/internal/v1/electronic-documents/by-source", idBatchDTO{IDs: ids}, &dto); err != nil {
		return nil, err
	}
	out := make(map[string]view.FiscalStatus, len(dto.Items))
	for _, it := range dto.Items {
		out[it.SourceDocumentID] = view.FiscalStatus{
			ElectronicDocumentID:  it.ElectronicDocumentID,
			Status:                it.Status,
			HaciendaStatusMessage: it.HaciendaStatusMessage,
		}
	}
	return out, nil
}

// BalancesByInvoice: POST /internal/v1/receivables/by-invoice (getBalancesByInvoice).
// TODO(P6): confirmar contra RDL.Receivables.API cuando exista; la ruta está propuesta, sin publicar, en contratos.
func (r *ReceivablesReader) BalancesByInvoice(ctx context.Context, ids []string) (map[string]view.Balance, error) {
	var dto struct {
		Items []struct {
			InvoiceID string `json:"invoiceId"`
			balanceDTO
		} `json:"items"`
	}
	if err := r.Client.postReadJSON(ctx, "/internal/v1/receivables/by-invoice", idBatchDTO{IDs: ids}, &dto); err != nil {
		return nil, err
	}
	out := make(map[string]view.Balance, len(dto.Items))
	for _, it := range dto.Items {
		out[it.InvoiceID] = view.Balance{
			ReceivableID:  it.ReceivableID,
			Status:        it.Status,
			Currency:      it.Currency,
			BalanceAmount: it.BalanceAmount,
			DueOn:         it.DueOn,
		}
	}
	return out, nil
}
