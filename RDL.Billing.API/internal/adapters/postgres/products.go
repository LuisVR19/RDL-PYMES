package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
)

type products struct{ q *db.Queries }

func (r products) Create(ctx context.Context, p product.Product) (product.Product, error) {
	row, err := r.q.InsertProduct(ctx, db.InsertProductParams{
		OrganizationID: p.OrganizationID, Code: p.Code, Description: p.Description, CabysCode: p.CabysCode,
		UnitOfMeasureCode: p.UnitOfMeasureCode, UnitPrice: p.UnitPrice.String(), CurrencyCode: p.Currency.String(),
		IsService: p.IsService, IsActive: p.IsActive,
	})
	if isUniqueViolation(err, "products_org_code_uk") {
		return product.Product{}, app.ErrProductCodeTaken
	}
	if err != nil {
		return product.Product{}, fmt.Errorf("creando producto: %w", err)
	}
	out, err := toProduct(db.GetProductRow(row))
	if err != nil {
		return product.Product{}, err
	}
	if out.Taxes, err = r.insertTaxes(ctx, out.OrganizationID, out.ID, p.Taxes); err != nil {
		return product.Product{}, err
	}
	return out, nil
}

func (r products) List(ctx context.Context, org uuid.UUID, q app.ProductQuery) ([]product.Product, error) {
	params := db.ListProductsParams{
		OrganizationID: org,
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.Active != nil {
		params.IsActive = pgtype.Bool{Bool: *q.Active, Valid: true}
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		params.Search = pgtype.Text{String: escapeLike(s), Valid: true}
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListProducts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando productos: %w", err)
	}
	out := make([]product.Product, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		p, err := toProduct(db.GetProductRow(row))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
		ids = append(ids, p.ID)
	}
	taxes, err := r.taxesOf(ctx, org, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Taxes = taxes[out[i].ID]
	}
	return out, nil
}

func (r products) Get(ctx context.Context, org, id uuid.UUID) (product.Product, error) {
	row, err := r.q.GetProduct(ctx, db.GetProductParams{OrganizationID: org, ID: id})
	return r.withTaxes(ctx, row, err)
}

func (r products) GetForUpdate(ctx context.Context, org, id uuid.UUID) (product.Product, error) {
	row, err := r.q.GetProductForUpdate(ctx, db.GetProductForUpdateParams{OrganizationID: org, ID: id})
	return r.withTaxes(ctx, db.GetProductRow(row), err)
}

// Update reescribe el producto y reemplaza sus impuestos (a lo sumo unos pocos por producto).
func (r products) Update(ctx context.Context, p product.Product) (product.Product, error) {
	row, err := r.q.UpdateProduct(ctx, db.UpdateProductParams{
		OrganizationID: p.OrganizationID, ID: p.ID, Code: p.Code, Description: p.Description, CabysCode: p.CabysCode,
		UnitOfMeasureCode: p.UnitOfMeasureCode, UnitPrice: p.UnitPrice.String(), CurrencyCode: p.Currency.String(),
		IsService: p.IsService, IsActive: p.IsActive,
	})
	if isUniqueViolation(err, "products_org_code_uk") {
		return product.Product{}, app.ErrProductCodeTaken
	}
	if isNoRows(err) {
		return product.Product{}, app.ErrNotFound
	}
	if err != nil {
		return product.Product{}, fmt.Errorf("actualizando producto: %w", err)
	}
	out, err := toProduct(db.GetProductRow(row))
	if err != nil {
		return product.Product{}, err
	}
	if err := r.q.DeleteProductTaxes(ctx, db.DeleteProductTaxesParams{OrganizationID: p.OrganizationID, ProductID: p.ID}); err != nil {
		return product.Product{}, fmt.Errorf("reemplazando impuestos del producto: %w", err)
	}
	if out.Taxes, err = r.insertTaxes(ctx, p.OrganizationID, p.ID, p.Taxes); err != nil {
		return product.Product{}, err
	}
	return out, nil
}

func (r products) insertTaxes(ctx context.Context, org, productID uuid.UUID, taxes []product.Tax) ([]product.Tax, error) {
	for _, t := range taxes {
		if err := r.q.InsertProductTax(ctx, db.InsertProductTaxParams{
			OrganizationID: org, ProductID: productID, TaxTypeCode: t.TypeCode, TaxRateCode: t.RateCode,
		}); err != nil {
			return nil, fmt.Errorf("guardando impuesto del producto: %w", err)
		}
	}
	return append([]product.Tax{}, taxes...), nil
}

func (r products) withTaxes(ctx context.Context, row db.GetProductRow, err error) (product.Product, error) {
	if isNoRows(err) {
		return product.Product{}, app.ErrNotFound
	}
	if err != nil {
		return product.Product{}, fmt.Errorf("leyendo producto: %w", err)
	}
	p, err := toProduct(row)
	if err != nil {
		return product.Product{}, err
	}
	taxes, err := r.taxesOf(ctx, p.OrganizationID, []uuid.UUID{p.ID})
	if err != nil {
		return product.Product{}, err
	}
	p.Taxes = taxes[p.ID]
	return p, nil
}

func (r products) taxesOf(ctx context.Context, org uuid.UUID, ids []uuid.UUID) (map[uuid.UUID][]product.Tax, error) {
	out := make(map[uuid.UUID][]product.Tax, len(ids))
	for _, id := range ids {
		out[id] = []product.Tax{}
	}
	if len(ids) == 0 {
		return out, nil
	}
	texts := make([]string, 0, len(ids))
	for _, id := range ids {
		texts = append(texts, id.String())
	}
	rows, err := r.q.ListProductTaxes(ctx, db.ListProductTaxesParams{OrganizationID: org, ProductIds: texts})
	if err != nil {
		return nil, fmt.Errorf("leyendo impuestos de productos: %w", err)
	}
	for _, row := range rows {
		out[row.ProductID] = append(out[row.ProductID], product.Tax{TypeCode: row.TaxTypeCode, RateCode: row.TaxRateCode})
	}
	return out, nil
}

// toProduct valida lo que llega de la base con los mismos tipos del contrato: un valor fuera de formato es un error
// interno (la base y el contrato dejaron de coincidir), no algo que se deba entregar.
func toProduct(r db.GetProductRow) (product.Product, error) {
	price, err := money.ParseAmount(r.UnitPrice)
	if err != nil {
		return product.Product{}, fmt.Errorf("precio del producto %s: %w", r.ID, err)
	}
	currency, err := money.ParseCurrency(r.CurrencyCode)
	if err != nil {
		return product.Product{}, fmt.Errorf("moneda del producto %s: %w", r.ID, err)
	}
	return product.Product{
		ID: r.ID, OrganizationID: r.OrganizationID, Code: r.Code, Description: r.Description,
		CabysCode: r.CabysCode, UnitOfMeasureCode: r.UnitOfMeasureCode,
		UnitPrice: money.CanonicalAmount(price), Currency: currency,
		IsService: r.IsService, IsActive: r.IsActive, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}
