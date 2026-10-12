package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rdl/einvoice-api/internal/adapters/postgres/db"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/document"
)

type documentStore struct {
	q   *db.Queries
	org uuid.UUID
}

func (s documentStore) ExistsBySource(ctx context.Context, sourceType document.SourceType, sourceDocumentID uuid.UUID) (bool, error) {
	ok, err := s.q.ExistsDocumentBySource(ctx, db.ExistsDocumentBySourceParams{
		OrganizationID: s.org, SourceType: string(sourceType), SourceDocumentID: sourceDocumentID,
	})
	if err != nil {
		return false, fmt.Errorf("buscando el documento por origen: %w", err)
	}
	return ok, nil
}

// Create escribe el documento, sus líneas, impuestos, medios de pago y la primera fila del historial. Corre en la
// transacción del evento: si algo falla, el rollback se lleva también el inbox.
func (s documentStore) Create(ctx context.Context, d document.Document) error {
	if d.OrganizationID != s.org {
		return fmt.Errorf("postgres: documento de la organización %s en la sesión de %s", d.OrganizationID, s.org)
	}
	p := db.InsertElectronicDocumentParams{
		ID: d.ID, OrganizationID: d.OrganizationID, SourceType: string(d.SourceType),
		SourceDocumentID: d.SourceDocumentID, SourceDocumentNumber: d.SourceDocumentNumber, SourceEventID: d.SourceEventID,
		DocumentTypeCode: d.TypeCode(), IssuedAt: d.IssuedAt.UTC(),
		IssuerIdentificationTypeCode: d.Issuer.IdentificationTypeCode, IssuerIdentificationNumber: d.Issuer.IdentificationNumber,
		IssuerLegalName: d.Issuer.LegalName, IssuerTradeName: text(d.Issuer.TradeName),
		IssuerEconomicActivityCode: text(d.Issuer.EconomicActivityCode), IssuerEmail: text(d.Issuer.Email),
		IssuerPhone: text(d.Issuer.Phone), IssuerProvinceCode: text(d.Issuer.ProvinceCode),
		IssuerCantonCode: text(d.Issuer.CantonCode), IssuerDistrictCode: text(d.Issuer.DistrictCode),
		IssuerAddressDetails:           text(d.Issuer.Address),
		ReceiverIdentificationTypeCode: text(d.Receiver.IdentificationTypeCode),
		ReceiverIdentificationNumber:   text(d.Receiver.IdentificationNumber),
		ReceiverLegalName:              text(d.Receiver.LegalName), ReceiverEmail: text(d.Receiver.Email),
		ReceiverPhone: text(d.Receiver.Phone), ReceiverAddress: text(d.Receiver.Address),
		SaleConditionCode: d.SaleConditionCode, CurrencyCode: d.CurrencyCode, ExchangeRate: d.ExchangeRate.String(),
		SubtotalAmount: d.Subtotal.String(), DiscountAmount: d.Discount.String(), TaxAmount: d.Tax.String(),
		ExonerationAmount: d.Exoneration.String(), TotalAmount: d.Total.String(),
	}
	if d.CreditTermDays != nil {
		p.CreditTermDays = pgtype.Int4{Int32: int32(min(*d.CreditTermDays, 1<<30)), Valid: true} // #nosec G115 -- acotado
	}
	if err := s.q.InsertElectronicDocument(ctx, p); err != nil {
		return fmt.Errorf("guardando el documento electrónico: %w", err)
	}
	for _, l := range d.Lines {
		lineID := uuid.New()
		err := s.q.InsertDocumentLine(ctx, db.InsertDocumentLineParams{
			ID: lineID, OrganizationID: d.OrganizationID, ElectronicDocumentID: d.ID,
			LineNumber: int32(min(l.Number, 1<<30)), // #nosec G115 -- acotado (máximo 1000 líneas)
			CabysCode:  l.CabysCode, ProductCode: text(l.ProductCode), Description: l.Description,
			UnitOfMeasureCode: l.UnitOfMeasureCode, Quantity: l.Quantity.String(), UnitPrice: l.UnitPrice.String(),
			GrossAmount: l.GrossAmount.String(), DiscountAmount: l.Discount.String(), DiscountReason: text(l.DiscountReason),
			SubtotalAmount: l.Subtotal.String(), TaxAmount: l.Tax.String(), TotalAmount: l.Total.String(),
		})
		if err != nil {
			return fmt.Errorf("guardando la línea %d: %w", l.Number, err)
		}
		for _, tax := range l.Taxes {
			tp := db.InsertLineTaxParams{
				OrganizationID: d.OrganizationID, ElectronicDocumentLineID: lineID, TaxTypeCode: tax.TaxTypeCode,
				TaxRateCode: text(tax.TaxRateCode), Rate: tax.Rate.String(), TaxableBase: tax.TaxableBase.String(),
				TaxAmount: tax.Amount.String(),
			}
			if e := tax.Exoneration; e != nil {
				tp.ExonerationDocumentTypeCode = text(e.DocumentTypeCode)
				tp.ExonerationDocumentNumber = text(e.DocumentNumber)
				tp.ExonerationInstitution = text(e.Institution)
				tp.ExonerationIssuedAt = pgtype.Timestamptz{Time: e.IssuedAt.UTC(), Valid: true}
				tp.ExonerationPercentage = decimalText(&e.Rate)
				tp.ExonerationAmount = decimalText(&e.Amount)
			}
			if err := s.q.InsertLineTax(ctx, tp); err != nil {
				return fmt.Errorf("guardando el impuesto %s de la línea %d: %w", tax.TaxTypeCode, l.Number, err)
			}
		}
	}
	for i, pm := range d.PaymentMethods {
		err := s.q.InsertDocumentPaymentMethod(ctx, db.InsertDocumentPaymentMethodParams{
			OrganizationID: d.OrganizationID, ElectronicDocumentID: d.ID,
			Position:          int16(i + 1), // #nosec G115 -- a lo sumo 4 (document.MaxPaymentMethods)
			PaymentMethodCode: pm.Code, Amount: decimalText(pm.Amount),
		})
		if err != nil {
			return fmt.Errorf("guardando el medio de pago %s: %w", pm.Code, err)
		}
	}
	err := s.q.InsertStatusHistory(ctx, db.InsertStatusHistoryParams{
		OrganizationID: d.OrganizationID, ElectronicDocumentID: d.ID,
		ToStatus: string(document.StatusProcessing), Reason: text("creado desde " + string(d.SourceType)),
	})
	if err != nil {
		return fmt.Errorf("guardando el historial del documento: %w", err)
	}
	return nil
}

type fiscalStatuses struct{ q *db.Queries }

func (r fiscalStatuses) BySource(ctx context.Context, org, sourceDocumentID uuid.UUID) (app.FiscalStatus, error) {
	row, err := r.q.GetFiscalStatusBySource(ctx, db.GetFiscalStatusBySourceParams{OrganizationID: org, SourceDocumentID: sourceDocumentID})
	if isNoRows(err) {
		return app.FiscalStatus{}, app.ErrNotFound
	}
	if err != nil {
		return app.FiscalStatus{}, fmt.Errorf("leyendo el estado fiscal: %w", err)
	}
	return app.FiscalStatus{
		SourceDocumentID: row.SourceDocumentID, ElectronicDocumentID: row.ID, Status: row.Status,
		HaciendaStatusMessage: row.HaciendaStatusMessage.String,
	}, nil
}

func (r fiscalStatuses) BySources(ctx context.Context, org uuid.UUID, ids []uuid.UUID) ([]app.FiscalStatus, error) {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	rows, err := r.q.ListFiscalStatusesBySource(ctx, db.ListFiscalStatusesBySourceParams{OrganizationID: org, SourceDocumentIds: strs})
	if err != nil {
		return nil, fmt.Errorf("leyendo estados fiscales por lote: %w", err)
	}
	out := make([]app.FiscalStatus, 0, len(rows))
	for _, row := range rows {
		out = append(out, app.FiscalStatus{
			SourceDocumentID: row.SourceDocumentID, ElectronicDocumentID: row.ID, Status: row.Status,
			HaciendaStatusMessage: row.HaciendaStatusMessage.String,
		})
	}
	return out, nil
}

// text: cadena vacía → NULL.
func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func decimalText(d *decimal.Decimal) pgtype.Text {
	if d == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: d.String(), Valid: true}
}
