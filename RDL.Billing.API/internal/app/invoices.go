package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// HeaderInput es el encabezado que llega en POST. ExchangeRate nil = 1 si la moneda es la local (obligatorio si no).
type HeaderInput struct {
	DocumentType      invoice.DocumentType
	CustomerID        uuid.UUID
	BranchID          *uuid.UUID
	SaleConditionCode string
	CreditTermDays    *int
	Currency          money.Currency
	ExchangeRate      *money.ExchangeRate
	Notes             string
}

// LineRequest es una línea tal como la pide el usuario: producto, cantidad y, opcionalmente, precio y descuento.
// El resto (CABYS, descripción, unidad, impuestos y tasas) sale del producto y del catálogo, nunca del cliente.
type LineRequest struct {
	ProductID      uuid.UUID
	Quantity       money.Quantity
	UnitPrice      *money.Amount // nil = el precio del producto (si está en la moneda del documento)
	Discount       *money.Amount // nil = sin descuento
	DiscountReason string
}

// --- crear ---

// CreateInvoiceDraft crea un borrador con líneas opcionales (owner, admin, biller). Idempotente por Idempotency-Key.
type CreateInvoiceDraft struct{ tx TxManager }

func NewCreateInvoiceDraft(tx TxManager) *CreateInvoiceDraft { return &CreateInvoiceDraft{tx: tx} }

type CreateInvoiceResult struct {
	Invoice  invoice.Invoice
	Replayed bool
}

func (uc *CreateInvoiceDraft) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, h HeaderInput, lines []LineRequest) (CreateInvoiceResult, error) {
	if err := authorize(t, permission.InvoicesManage); err != nil {
		return CreateInvoiceResult{}, err
	}
	hash, err := requestHash(map[string]any{"header": h, "lines": lines})
	if err != nil {
		return CreateInvoiceResult{}, err
	}
	var res CreateInvoiceResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		prevID, err := replayed(ctx, tx, t.OrganizationID(), idempotencyKey, hash, "invoiceId")
		if err != nil {
			return err
		}
		if prevID != uuid.Nil {
			inv, err := tx.Invoices().Get(ctx, t.OrganizationID(), prevID)
			res = CreateInvoiceResult{Invoice: inv, Replayed: true}
			return err
		}

		b := builder{tx: tx, org: t.OrganizationID()}
		header, err := b.header(ctx, h)
		if err != nil {
			return err
		}
		draft, err := invoice.NewDraft(t.OrganizationID(), t.UserID(), header)
		if err != nil {
			return err
		}
		if draft, err = b.withLines(ctx, draft, lines); err != nil {
			return err
		}
		inv, err := tx.Invoices().Create(ctx, draft)
		if err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invoice.created", EntityType: "invoice", EntityID: inv.ID, After: invoiceAudit(inv),
		}); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"invoiceId": inv.ID.String()},
		}); err != nil {
			return err
		}
		res = CreateInvoiceResult{Invoice: inv}
		return nil
	})
	return res, err
}

// --- editar encabezado y líneas ---

// UpdateInvoiceDraft edita el encabezado de un borrador y, si vienen, reemplaza sus líneas (owner, admin, biller).
type UpdateInvoiceDraft struct{ tx TxManager }

func NewUpdateInvoiceDraft(tx TxManager) *UpdateInvoiceDraft { return &UpdateInvoiceDraft{tx: tx} }

func (uc *UpdateInvoiceDraft) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p invoice.HeaderPatch, lines *[]LineRequest) (invoice.Invoice, error) {
	if err := authorize(t, permission.InvoicesManage); err != nil {
		return invoice.Invoice{}, err
	}
	var out invoice.Invoice
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		current, err := tx.Invoices().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		next, err := current.ApplyHeader(p)
		if err != nil {
			return err
		}
		b := builder{tx: tx, org: t.OrganizationID()}
		if err := b.checkHeader(ctx, current.Header, next.Header); err != nil {
			return err
		}
		switch {
		case lines != nil:
			if next, err = b.withLines(ctx, next, *lines); err != nil {
				return err
			}
		case next.Currency != current.Currency && len(current.Lines) > 0:
			// Los precios de las líneas están en la moneda anterior: no se convierten solos.
			return invoice.FieldError{Field: "lines", Message: "al cambiar la moneda envíe las líneas de nuevo con sus precios"}
		}
		if out, err = tx.Invoices().SaveDraft(ctx, next); err != nil {
			return err
		}
		before, after := diff(invoiceAudit(current), invoiceAudit(out))
		if len(after) == 0 {
			return nil
		}
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invoice.updated", EntityType: "invoice", EntityID: id, Before: before, After: after,
		})
	})
	return out, err
}

// ReplaceInvoiceLines reemplaza las líneas del borrador y recalcula (PUT /v1/invoices/{id}/lines).
type ReplaceInvoiceLines struct{ update *UpdateInvoiceDraft }

func NewReplaceInvoiceLines(tx TxManager) *ReplaceInvoiceLines {
	return &ReplaceInvoiceLines{update: NewUpdateInvoiceDraft(tx)}
}

func (uc *ReplaceInvoiceLines) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, lines []LineRequest) (invoice.Invoice, error) {
	return uc.update.Execute(ctx, t, id, invoice.HeaderPatch{}, &lines)
}

// DiscardInvoiceDraft borra un borrador (owner, admin, biller). Nunca un documento emitido (409).
type DiscardInvoiceDraft struct{ tx TxManager }

func NewDiscardInvoiceDraft(tx TxManager) *DiscardInvoiceDraft { return &DiscardInvoiceDraft{tx: tx} }

func (uc *DiscardInvoiceDraft) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) error {
	if err := authorize(t, permission.InvoicesManage); err != nil {
		return err
	}
	return uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		inv, err := tx.Invoices().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		if err := inv.CanDiscard(); err != nil {
			return err
		}
		if err := tx.Invoices().DeleteDraft(ctx, t.OrganizationID(), id); err != nil {
			return err
		}
		// El historial de estados registra transiciones; crear y descartar un borrador quedan aquí (informe 0001 §3.3).
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invoice.discarded", EntityType: "invoice", EntityID: id, Before: invoiceAudit(inv),
		})
	})
}

// --- leer ---

type ListInvoices struct{ tx TxManager }

func NewListInvoices(tx TxManager) *ListInvoices { return &ListInvoices{tx: tx} }

type InvoicePage struct {
	Items []invoice.Invoice
	Next  *PageCursor
}

func (uc *ListInvoices) Execute(ctx context.Context, t tenancy.Context, q InvoiceQuery) (InvoicePage, error) {
	if err := authorize(t, permission.InvoicesRead); err != nil {
		return InvoicePage{}, err
	}
	q.Limit = pageLimit(q.Limit)
	var page InvoicePage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if q.IssuedFrom != "" || q.IssuedTo != "" {
			settings, err := tx.Catalog().Organization(ctx, t.OrganizationID())
			if err != nil {
				return err
			}
			q.Timezone = settings.Timezone
		}
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Invoices().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		page.Items, page.Next = cutPage(items, q.Limit, func(i invoice.Invoice) PageCursor {
			return PageCursor{At: i.CreatedAt, ID: i.ID}
		})
		return nil
	})
	return page, err
}

type GetInvoice struct{ tx TxManager }

func NewGetInvoice(tx TxManager) *GetInvoice { return &GetInvoice{tx: tx} }

func (uc *GetInvoice) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (invoice.Invoice, error) {
	if err := authorize(t, permission.InvoicesRead); err != nil {
		return invoice.Invoice{}, err
	}
	var inv invoice.Invoice
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		inv, err = tx.Invoices().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return inv, err
}

type GetInvoiceHistory struct{ tx TxManager }

func NewGetInvoiceHistory(tx TxManager) *GetInvoiceHistory { return &GetInvoiceHistory{tx: tx} }

func (uc *GetInvoiceHistory) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) ([]StatusChange, error) {
	if err := authorize(t, permission.InvoicesRead); err != nil {
		return nil, err
	}
	var out []StatusChange
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		// Primero el documento: uno de otra organización es 404, no una lista vacía.
		if _, err := tx.Invoices().Get(ctx, t.OrganizationID(), id); err != nil {
			return err
		}
		var err error
		out, err = tx.Invoices().History(ctx, t.OrganizationID(), id)
		return err
	})
	return out, err
}

// --- armado del documento a partir de lo que pide el usuario ---

// builder valida cada id recibido contra la organización activa y arma el encabezado y las líneas con datos de la
// base (cliente, sucursal, productos, tasas), nunca con datos del cliente HTTP.
type builder struct {
	tx  Tx
	org uuid.UUID
}

func (b builder) header(ctx context.Context, in HeaderInput) (invoice.Header, error) {
	settings, err := b.tx.Catalog().Organization(ctx, b.org)
	if err != nil {
		return invoice.Header{}, err
	}
	h := invoice.Header{
		DocumentType: in.DocumentType, CustomerID: in.CustomerID, BranchID: in.BranchID,
		SaleConditionCode: in.SaleConditionCode, CreditTermDays: in.CreditTermDays, Currency: in.Currency, Notes: in.Notes,
	}
	switch {
	case in.ExchangeRate != nil:
		h.ExchangeRate = *in.ExchangeRate
	case in.Currency == settings.LocalCurrency:
		h.ExchangeRate = money.MustExchangeRateForTest("1")
	default:
		return invoice.Header{}, invoice.FieldError{Field: "exchangeRate",
			Message: "es obligatorio cuando la moneda no es la local (" + settings.LocalCurrency.String() + ")"}
	}
	if err := h.CheckExchangeRate(settings.LocalCurrency); err != nil {
		return invoice.Header{}, err
	}
	if err := b.checkCustomer(ctx, h.CustomerID); err != nil {
		return invoice.Header{}, err
	}
	return h, b.checkBranch(ctx, h.BranchID)
}

// checkHeader revalida lo que cambió en un PATCH: el cliente, la sucursal y el tipo de cambio.
func (b builder) checkHeader(ctx context.Context, before, after invoice.Header) error {
	if after.CustomerID != before.CustomerID {
		if err := b.checkCustomer(ctx, after.CustomerID); err != nil {
			return err
		}
	}
	if !sameBranch(before.BranchID, after.BranchID) {
		if err := b.checkBranch(ctx, after.BranchID); err != nil {
			return err
		}
	}
	if after.Currency != before.Currency || after.ExchangeRate != before.ExchangeRate {
		settings, err := b.tx.Catalog().Organization(ctx, b.org)
		if err != nil {
			return err
		}
		return after.CheckExchangeRate(settings.LocalCurrency)
	}
	return nil
}

func (b builder) checkCustomer(ctx context.Context, id uuid.UUID) error {
	c, err := b.tx.Customers().Get(ctx, b.org, id)
	if err != nil {
		return err // ErrNotFound: de otra organización se comporta como inexistente
	}
	if !c.IsActive {
		return ErrCustomerInactive
	}
	return nil
}

func (b builder) checkBranch(ctx context.Context, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	br, err := b.tx.Catalog().Branch(ctx, b.org, *id)
	if err != nil {
		return err
	}
	if !br.IsActive {
		return invoice.FieldError{Field: "branchId", Message: "la sucursal está desactivada"}
	}
	return nil
}

func (b builder) withLines(ctx context.Context, inv invoice.Invoice, lines []LineRequest) (invoice.Invoice, error) {
	drafts := make([]invoice.LineDraft, 0, len(lines))
	var errs []error
	rates := map[string]money.Percentage{}
	for i, l := range lines {
		field := func(name string) string { return fmt.Sprintf("lines[%d].%s", i, name) }
		p, err := b.tx.Products().Get(ctx, b.org, l.ProductID)
		if err != nil {
			return invoice.Invoice{}, err // ErrNotFound: producto inexistente o de otra organización
		}
		if !p.IsActive {
			errs = append(errs, invoice.FieldError{Field: field("productId"), Message: "el producto está desactivado"})
			continue
		}
		price := p.UnitPrice
		switch {
		case l.UnitPrice != nil:
			price = *l.UnitPrice
		case p.Currency != inv.Currency:
			errs = append(errs, invoice.FieldError{Field: field("unitPrice"), Message: fmt.Sprintf(
				"el producto tiene precio en %s y el documento es en %s: indique el precio", p.Currency, inv.Currency)})
			continue
		}
		discount := money.Zero
		if l.Discount != nil {
			discount = *l.Discount
		}
		missing := slices.DeleteFunc(taxCodes(p.Taxes), func(code string) bool { _, ok := rates[code]; return ok })
		if len(missing) > 0 {
			found, err := b.tx.Catalog().TaxRates(ctx, missing)
			if err != nil {
				return invoice.Invoice{}, err
			}
			for code, r := range found {
				rates[code] = r
			}
		}
		d := invoice.LineDraft{
			ProductID: &p.ID, ProductCode: p.Code, CabysCode: p.CabysCode, Description: p.Description,
			UnitOfMeasureCode: p.UnitOfMeasureCode, IsService: p.IsService, Quantity: l.Quantity,
			UnitPrice: price, Discount: discount, DiscountReason: l.DiscountReason,
		}
		for _, tax := range p.Taxes {
			rate, ok := rates[tax.RateCode]
			if !ok {
				// TODO(fiscal): el catálogo fiscal.tax_rates está vacío en dev; fiscal lo carga.
				errs = append(errs, invoice.FieldError{Field: field("productId"), Message: fmt.Sprintf(
					"la tarifa %q del producto no está vigente en el catálogo fiscal", tax.RateCode)})
				continue
			}
			d.Taxes = append(d.Taxes, invoice.TaxDraft{TypeCode: tax.TypeCode, RateCode: tax.RateCode, Rate: rate})
		}
		drafts = append(drafts, d)
	}
	if err := errors.Join(errs...); err != nil {
		return invoice.Invoice{}, err
	}
	return inv.ReplaceLines(drafts)
}

// taxCodes devuelve los códigos de tarifa de los impuestos del producto.
func taxCodes(taxes []product.Tax) []string {
	out := make([]string, 0, len(taxes))
	for _, t := range taxes {
		out = append(out, t.RateCode)
	}
	return out
}

func sameBranch(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func invoiceAudit(inv invoice.Invoice) map[string]any {
	branch := ""
	if inv.BranchID != nil {
		branch = inv.BranchID.String()
	}
	credit := ""
	if inv.CreditTermDays != nil {
		credit = fmt.Sprint(*inv.CreditTermDays)
	}
	return map[string]any{
		"documentType": string(inv.DocumentType), "status": string(inv.Status), "customerId": inv.CustomerID.String(),
		"branchId": branch, "saleConditionCode": inv.SaleConditionCode, "creditTermDays": credit,
		"currency": inv.Currency.String(), "exchangeRate": inv.ExchangeRate.String(), "notes": inv.Notes,
		"lines": len(inv.Lines), "subtotal": inv.Totals.Subtotal.String(), "tax": inv.Totals.Tax.String(),
		"total": inv.Totals.Total.String(),
	}
}
