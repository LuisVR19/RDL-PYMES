package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
)

type invoices struct{ q *db.Queries }

func (r invoices) Create(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	row, err := r.q.InsertInvoice(ctx, db.InsertInvoiceParams{
		OrganizationID: inv.OrganizationID, DocumentType: string(inv.DocumentType), BranchID: optUUID(inv.BranchID),
		CustomerID: inv.CustomerID, SaleConditionCode: inv.SaleConditionCode, CreditTermDays: optInt(inv.CreditTermDays),
		CurrencyCode: inv.Currency.String(), ExchangeRate: inv.ExchangeRate.String(), Notes: inv.Notes,
		CreatedByUserID: inv.CreatedByUserID, Subtotal: inv.Totals.Subtotal.String(), Discount: inv.Totals.Discount.String(),
		Tax: inv.Totals.Tax.String(), Exoneration: inv.Totals.Exoneration.String(), Total: inv.Totals.Total.String(),
	})
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("creando documento: %w", err)
	}
	inv.ID, inv.CreatedAt, inv.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	if err := r.insertLines(ctx, inv); err != nil {
		return invoice.Invoice{}, err
	}
	return inv, nil
}

func (r invoices) SaveDraft(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	updated, err := r.q.UpdateInvoiceDraft(ctx, db.UpdateInvoiceDraftParams{
		OrganizationID: inv.OrganizationID, ID: inv.ID, BranchID: optUUID(inv.BranchID), CustomerID: inv.CustomerID,
		SaleConditionCode: inv.SaleConditionCode, CreditTermDays: optInt(inv.CreditTermDays),
		CurrencyCode: inv.Currency.String(), ExchangeRate: inv.ExchangeRate.String(), Notes: inv.Notes,
		Subtotal: inv.Totals.Subtotal.String(), Discount: inv.Totals.Discount.String(), Tax: inv.Totals.Tax.String(),
		Exoneration: inv.Totals.Exoneration.String(), Total: inv.Totals.Total.String(),
	})
	if isNoRows(err) {
		return invoice.Invoice{}, invoice.ErrNotDraft
	}
	if err != nil {
		return invoice.Invoice{}, guardErr("actualizando borrador", err)
	}
	inv.UpdatedAt = updated
	if err := r.q.DeleteInvoiceLines(ctx, db.DeleteInvoiceLinesParams{OrganizationID: inv.OrganizationID, InvoiceID: inv.ID}); err != nil {
		return invoice.Invoice{}, guardErr("reemplazando líneas", err)
	}
	if err := r.insertLines(ctx, inv); err != nil {
		return invoice.Invoice{}, err
	}
	return inv, nil
}

func (r invoices) MarkIssued(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	if inv.IssuedAt == nil || inv.IssuedByUserID == nil {
		return invoice.Invoice{}, fmt.Errorf("emitiendo %s: faltan fecha o emisor", inv.ID)
	}
	c := inv.Customer
	updated, err := r.q.IssueInvoice(ctx, db.IssueInvoiceParams{
		OrganizationID: inv.OrganizationID, ID: inv.ID, Number: inv.Number, IssuedAt: *inv.IssuedAt,
		IssuedByUserID: *inv.IssuedByUserID, DueDate: inv.DueDate,
		CustomerIdentificationTypeCode: c.IdentificationTypeCode, CustomerIdentificationNumber: c.IdentificationNumber,
		CustomerLegalName: c.LegalName, CustomerEmail: c.Email, CustomerPhone: c.Phone, CustomerAddress: c.Address,
	})
	if isNoRows(err) {
		return invoice.Invoice{}, invoice.ErrNotDraft
	}
	if isUniqueViolation(err, "invoices_number_uk") {
		// La secuencia se bloquea antes de asignar: solo pasa si alguien configuró a mano un número ya usado.
		return invoice.Invoice{}, fmt.Errorf("el número %s ya existe: %w", inv.Number, err)
	}
	if err != nil {
		return invoice.Invoice{}, guardErr("emitiendo documento", err)
	}
	inv.UpdatedAt = updated
	return inv, nil
}

func (r invoices) AddStatusChange(ctx context.Context, org, id uuid.UUID, c app.StatusChange) error {
	params := db.InsertStatusChangeParams{
		OrganizationID: org, InvoiceID: id, ToStatus: string(c.To), Reason: c.Reason, ChangedAt: c.ChangedAt,
		ChangedByUserID: optUUID(c.ChangedByUserID),
	}
	if c.From != "" {
		params.FromStatus = pgtype.Text{String: string(c.From), Valid: true}
	}
	if err := r.q.InsertStatusChange(ctx, params); err != nil {
		return fmt.Errorf("registrando historial de estados: %w", err)
	}
	return nil
}

func (r invoices) DeleteDraft(ctx context.Context, org, id uuid.UUID) error {
	n, err := r.q.DeleteInvoiceDraft(ctx, db.DeleteInvoiceDraftParams{OrganizationID: org, ID: id})
	if err != nil {
		return guardErr("descartando borrador", err)
	}
	if n == 0 {
		return invoice.ErrNotDraft
	}
	return nil
}

func (r invoices) insertLines(ctx context.Context, inv invoice.Invoice) error {
	for _, l := range inv.Lines {
		lineID, err := r.q.InsertInvoiceLine(ctx, db.InsertInvoiceLineParams{
			OrganizationID: inv.OrganizationID, InvoiceID: inv.ID, LineNumber: int32(l.Number), // #nosec G115 -- ≤ invoice.MaxLines
			ProductID: optUUID(l.ProductID), ProductCode: l.ProductCode, CabysCode: l.CabysCode,
			Description: l.Description, UnitOfMeasureCode: l.UnitOfMeasureCode, IsService: l.IsService,
			Quantity: l.Quantity.String(), UnitPrice: l.UnitPrice.String(), Discount: l.Discount.String(),
			DiscountReason: l.DiscountReason, Subtotal: l.Subtotal.String(), Tax: l.Tax.String(), Total: l.Total.String(),
		})
		if err != nil {
			return guardErr("guardando línea", err)
		}
		for _, t := range l.Taxes {
			if err := r.q.InsertInvoiceLineTax(ctx, db.InsertInvoiceLineTaxParams{
				OrganizationID: inv.OrganizationID, InvoiceLineID: lineID, TaxTypeCode: t.TypeCode, TaxRateCode: t.RateCode,
				Rate: t.Rate.String(), TaxableBase: t.TaxableBase.String(), TaxAmount: t.Amount.String(),
			}); err != nil {
				return guardErr("guardando impuesto de línea", err)
			}
		}
	}
	return nil
}

func (r invoices) Get(ctx context.Context, org, id uuid.UUID) (invoice.Invoice, error) {
	row, err := r.q.GetInvoice(ctx, db.GetInvoiceParams{OrganizationID: org, ID: id})
	return r.one(ctx, row, err)
}

func (r invoices) GetForUpdate(ctx context.Context, org, id uuid.UUID) (invoice.Invoice, error) {
	row, err := r.q.GetInvoiceForUpdate(ctx, db.GetInvoiceForUpdateParams{OrganizationID: org, ID: id})
	return r.one(ctx, db.GetInvoiceRow(row), err)
}

func (r invoices) one(ctx context.Context, row db.GetInvoiceRow, err error) (invoice.Invoice, error) {
	if isNoRows(err) {
		return invoice.Invoice{}, app.ErrNotFound
	}
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("leyendo documento: %w", err)
	}
	inv, err := toInvoice(row)
	if err != nil {
		return invoice.Invoice{}, err
	}
	all := []invoice.Invoice{inv}
	if err := r.loadLines(ctx, row.OrganizationID, all); err != nil {
		return invoice.Invoice{}, err
	}
	return all[0], nil
}

func (r invoices) List(ctx context.Context, org uuid.UUID, q app.InvoiceQuery) ([]invoice.Invoice, error) {
	params := db.ListInvoicesParams{
		OrganizationID: org, Timezone: q.Timezone,
		PageSize: int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if params.Timezone == "" {
		params.Timezone = "UTC"
	}
	if q.DocumentType != "" {
		params.DocumentType = pgtype.Text{String: string(q.DocumentType), Valid: true}
	}
	if q.Status != "" {
		params.Status = pgtype.Text{String: string(q.Status), Valid: true}
	}
	if q.CustomerID != nil {
		params.CustomerID = uuid.NullUUID{UUID: *q.CustomerID, Valid: true}
	}
	if q.RequiresCorrection != nil {
		params.RequiresCorrection = pgtype.Bool{Bool: *q.RequiresCorrection, Valid: true}
	}
	for dst, v := range map[*pgtype.Date]string{&params.IssuedFrom: q.IssuedFrom, &params.IssuedTo: q.IssuedTo} {
		if v == "" {
			continue
		}
		d, err := time.Parse(time.DateOnly, v)
		if err != nil {
			return nil, fmt.Errorf("fecha %q: %w", v, err)
		}
		*dst = pgtype.Date{Time: d, Valid: true}
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListInvoices(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando documentos: %w", err)
	}
	out := make([]invoice.Invoice, 0, len(rows))
	for _, row := range rows {
		inv, err := toInvoice(db.GetInvoiceRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, r.loadLines(ctx, org, out)
}

// loadLines trae las líneas y sus impuestos de varios documentos con dos consultas (no una por documento).
func (r invoices) loadLines(ctx context.Context, org uuid.UUID, docs []invoice.Invoice) error {
	if len(docs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(docs))
	for _, d := range docs {
		ids = append(ids, d.ID.String())
	}
	lineRows, err := r.q.ListInvoiceLines(ctx, db.ListInvoiceLinesParams{OrganizationID: org, InvoiceIds: ids})
	if err != nil {
		return fmt.Errorf("leyendo líneas: %w", err)
	}
	taxRows, err := r.q.ListInvoiceLineTaxes(ctx, db.ListInvoiceLineTaxesParams{OrganizationID: org, InvoiceIds: ids})
	if err != nil {
		return fmt.Errorf("leyendo impuestos de líneas: %w", err)
	}
	taxesByLine := map[uuid.UUID][]invoice.LineTax{}
	for _, t := range taxRows {
		lt, err := toLineTax(t)
		if err != nil {
			return err
		}
		taxesByLine[t.InvoiceLineID] = append(taxesByLine[t.InvoiceLineID], lt)
	}
	linesByDoc := map[uuid.UUID][]invoice.Line{}
	for _, l := range lineRows {
		line, err := toLine(l)
		if err != nil {
			return err
		}
		line.Taxes = taxesByLine[l.ID]
		if line.Taxes == nil {
			line.Taxes = []invoice.LineTax{}
		}
		linesByDoc[l.InvoiceID] = append(linesByDoc[l.InvoiceID], line)
	}
	for i := range docs {
		docs[i].Lines = linesByDoc[docs[i].ID]
		if docs[i].Lines == nil {
			docs[i].Lines = []invoice.Line{}
		}
	}
	return nil
}

func (r invoices) History(ctx context.Context, org, id uuid.UUID) ([]app.StatusChange, error) {
	rows, err := r.q.ListInvoiceStatusHistory(ctx, db.ListInvoiceStatusHistoryParams{OrganizationID: org, InvoiceID: id})
	if err != nil {
		return nil, fmt.Errorf("leyendo historial: %w", err)
	}
	out := make([]app.StatusChange, 0, len(rows))
	for _, row := range rows {
		c := app.StatusChange{
			From: invoice.Status(row.FromStatus.String), To: invoice.Status(row.ToStatus), Reason: row.Reason,
			ChangedAt: row.ChangedAt,
		}
		if row.ChangedByUserID.Valid {
			u := row.ChangedByUserID.UUID
			c.ChangedByUserID = &u
		}
		out = append(out, c)
	}
	return out, nil
}

// --- conversión de filas: cada monto se valida con los tipos del contrato ---

// parser junta el primer error de conversión: un valor de la base fuera de formato es un error interno.
type parser struct{ err error }

func (p *parser) amount(s, what string) money.Amount {
	a, err := money.ParseAmount(s)
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%s: %w", what, err)
	}
	return money.CanonicalAmount(orZero(a, err))
}

func orZero(a money.Amount, err error) money.Amount {
	if err != nil {
		return money.Zero
	}
	return a
}

func (p *parser) percentage(s, what string) money.Percentage {
	v, err := money.ParsePercentage(trimZerosPct(s))
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%s: %w", what, err)
	}
	return v
}

func (p *parser) taxRate(s, what string) money.TaxRate {
	v, err := money.ParseTaxRate(trimZerosPct(s))
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%s: %w", what, err)
	}
	return v
}

func (p *parser) quantity(s, what string) money.Quantity {
	v, err := money.ParseQuantity(trimZerosPct(s))
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%s: %w", what, err)
	}
	return v
}

// trimZerosPct deja porcentajes y cantidades en forma canónica ("13.0000" → "13"), como los montos.
func trimZerosPct(s string) string {
	a, err := money.ParseAmount(s)
	if err != nil {
		return s
	}
	return money.CanonicalAmount(a).String()
}

func toInvoice(r db.GetInvoiceRow) (invoice.Invoice, error) {
	var p parser
	currency, err := money.ParseCurrency(r.CurrencyCode)
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("moneda del documento %s: %w", r.ID, err)
	}
	rate, err := money.ParseExchangeRate(trimZerosPct(r.ExchangeRate))
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("tipo de cambio del documento %s: %w", r.ID, err)
	}
	inv := invoice.Invoice{
		ID: r.ID, OrganizationID: r.OrganizationID,
		Header: invoice.Header{
			DocumentType: invoice.DocumentType(r.DocumentType), CustomerID: r.CustomerID, BranchID: ptrUUID(r.BranchID),
			SaleConditionCode: r.SaleConditionCode, CreditTermDays: ptrInt(r.CreditTermDays),
			Currency: currency, ExchangeRate: rate, Notes: r.Notes,
		},
		Number: r.Number, Status: invoice.Status(r.Status), DueDate: r.DueDate,
		Customer: invoice.CustomerSnapshot{
			IdentificationTypeCode: r.CustomerIdentificationTypeCode.String, IdentificationNumber: r.CustomerIdentificationNumber.String,
			LegalName: r.CustomerLegalName.String, Email: r.CustomerEmail.String, Phone: r.CustomerPhone.String,
			Address: r.CustomerAddress.String,
		},
		Totals: invoice.Totals{
			Subtotal: p.amount(r.Subtotal, "subtotal"), Discount: p.amount(r.Discount, "descuento"),
			Tax: p.amount(r.Tax, "impuesto"), Exoneration: p.amount(r.Exoneration, "exoneración"), Total: p.amount(r.Total, "total"),
		},
		ReferencedInvoiceID: ptrUUID(r.ReferencedInvoiceID), ReferenceReason: r.ReferenceReason,
		RequiresCorrection: r.RequiresCorrection, FiscalRejectionReason: r.FiscalRejectionReason,
		CreatedByUserID: r.CreatedByUserID, IssuedByUserID: ptrUUID(r.IssuedByUserID),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if r.IssuedAt.Valid {
		t := r.IssuedAt.Time
		inv.IssuedAt = &t
	}
	if p.err != nil {
		return invoice.Invoice{}, fmt.Errorf("documento %s: %w", r.ID, p.err)
	}
	return inv, nil
}

func toLine(r db.ListInvoiceLinesRow) (invoice.Line, error) {
	var p parser
	l := invoice.Line{
		Number: int(r.LineNumber), ProductID: ptrUUID(r.ProductID), ProductCode: r.ProductCode, CabysCode: r.CabysCode,
		Description: r.Description, UnitOfMeasureCode: r.UnitOfMeasureCode, IsService: r.IsService,
		Quantity: p.quantity(r.Quantity, "cantidad"), UnitPrice: p.amount(r.UnitPrice, "precio"),
		Discount: p.amount(r.Discount, "descuento"), DiscountReason: r.DiscountReason,
		Subtotal: p.amount(r.Subtotal, "subtotal"), Tax: p.amount(r.Tax, "impuesto"), Total: p.amount(r.Total, "total"),
	}
	if p.err != nil {
		return invoice.Line{}, fmt.Errorf("línea %s: %w", r.ID, p.err)
	}
	return l, nil
}

func toLineTax(r db.ListInvoiceLineTaxesRow) (invoice.LineTax, error) {
	var p parser
	t := invoice.LineTax{
		TypeCode: r.TaxTypeCode, RateCode: r.TaxRateCode, Rate: p.percentage(r.Rate, "tarifa"),
		TaxableBase: p.amount(r.TaxableBase, "base imponible"), Amount: p.amount(r.TaxAmount, "impuesto"),
	}
	if r.ExonerationRate != "" {
		t.Exoneration = &invoice.Exoneration{
			DocumentTypeCode: r.ExonerationDocumentTypeCode.String, DocumentNumber: r.ExonerationDocumentNumber.String,
			Institution: r.ExonerationInstitution.String, IssuedAt: r.ExonerationIssuedAt.Time,
			ExoneratedRate: p.taxRate(r.ExonerationRate, "tarifa exonerada"),
			Amount:         p.amount(r.ExonerationAmount, "exoneración"),
		}
	}
	if p.err != nil {
		return invoice.LineTax{}, fmt.Errorf("impuesto de línea %s: %w", r.InvoiceLineID, p.err)
	}
	return t, nil
}

// guardErr traduce el rechazo de invoices_guard / guard_draft_children (restrict_violation): alguien emitió el
// documento entre la lectura y la escritura. Con GetForUpdate no debería pasar; la base es la última defensa.
func guardErr(what string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23001" {
		return invoice.ErrNotDraft
	}
	return fmt.Errorf("%s: %w", what, err)
}

func optUUID(id *uuid.UUID) uuid.NullUUID {
	if id == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *id, Valid: true}
}

func ptrUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	id := n.UUID
	return &id
}

func optInt(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true} // #nosec G115 -- acotado por el dominio (≤ 3650)
}

func ptrInt(v pgtype.Int4) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}
