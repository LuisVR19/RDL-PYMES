package app

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// CreateProduct crea un producto o servicio con sus impuestos (owner, admin, biller). Idempotente por Idempotency-Key.
type CreateProduct struct{ tx TxManager }

func NewCreateProduct(tx TxManager) *CreateProduct { return &CreateProduct{tx: tx} }

type CreateProductResult struct {
	Product  product.Product
	Replayed bool
}

func (uc *CreateProduct) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in product.NewInput) (CreateProductResult, error) {
	if err := authorize(t, permission.ProductsManage); err != nil {
		return CreateProductResult{}, err
	}
	candidate, err := product.New(t.OrganizationID(), in)
	if err != nil {
		return CreateProductResult{}, err
	}
	hash, err := requestHash(productAudit(candidate))
	if err != nil {
		return CreateProductResult{}, err
	}

	var res CreateProductResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		prevID, err := replayed(ctx, tx, t.OrganizationID(), idempotencyKey, hash, "productId")
		if err != nil {
			return err
		}
		if prevID != uuid.Nil {
			p, err := tx.Products().Get(ctx, t.OrganizationID(), prevID)
			res = CreateProductResult{Product: p, Replayed: true}
			return err
		}

		p, err := tx.Products().Create(ctx, candidate)
		if err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "product.created", EntityType: "product", EntityID: p.ID, After: productAudit(p),
		}); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"productId": p.ID.String()},
		}); err != nil {
			return err
		}
		res = CreateProductResult{Product: p}
		return nil
	})
	return res, err
}

// ListProducts pagina los productos de la organización activa. Lectura para todos los roles.
type ListProducts struct{ tx TxManager }

func NewListProducts(tx TxManager) *ListProducts { return &ListProducts{tx: tx} }

type ProductPage struct {
	Items []product.Product
	Next  *PageCursor
}

func (uc *ListProducts) Execute(ctx context.Context, t tenancy.Context, q ProductQuery) (ProductPage, error) {
	if err := authorize(t, permission.ProductsRead); err != nil {
		return ProductPage{}, err
	}
	q.Limit = pageLimit(q.Limit)

	var page ProductPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Products().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		page.Items, page.Next = cutPage(items, q.Limit, func(p product.Product) PageCursor {
			return PageCursor{At: p.CreatedAt, ID: p.ID}
		})
		return nil
	})
	return page, err
}

// GetProduct: detalle con impuestos, para todos los roles. De otra organización → ErrNotFound.
type GetProduct struct{ tx TxManager }

func NewGetProduct(tx TxManager) *GetProduct { return &GetProduct{tx: tx} }

func (uc *GetProduct) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (product.Product, error) {
	if err := authorize(t, permission.ProductsRead); err != nil {
		return product.Product{}, err
	}
	var p product.Product
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		p, err = tx.Products().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return p, err
}

// UpdateProduct edita o desactiva un producto (owner, admin, biller). Solo toca billing.products y product_taxes:
// las líneas de facturas guardan su propio snapshot del producto.
type UpdateProduct struct{ tx TxManager }

func NewUpdateProduct(tx TxManager) *UpdateProduct { return &UpdateProduct{tx: tx} }

func (uc *UpdateProduct) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, ch product.Patch) (product.Product, error) {
	if err := authorize(t, permission.ProductsManage); err != nil {
		return product.Product{}, err
	}
	var out product.Product
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		current, err := tx.Products().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		next, err := current.Apply(ch)
		if err != nil {
			return err
		}
		if next.Equal(current) {
			out = current
			return nil
		}
		if out, err = tx.Products().Update(ctx, next); err != nil {
			return err
		}
		action := "product.updated"
		switch {
		case current.IsActive && !next.IsActive:
			action = "product.deactivated"
		case !current.IsActive && next.IsActive:
			action = "product.reactivated"
		}
		before, after := diff(productAudit(current), productAudit(next))
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: action, EntityType: "product", EntityID: id, Before: before, After: after,
		})
	})
	return out, err
}

func productAudit(p product.Product) map[string]any {
	taxes := make([]string, 0, len(p.Taxes))
	for _, tx := range p.Taxes {
		taxes = append(taxes, tx.TypeCode+":"+tx.RateCode)
	}
	return map[string]any{
		"code": p.Code, "description": p.Description, "cabysCode": p.CabysCode,
		"unitOfMeasureCode": p.UnitOfMeasureCode, "unitPrice": p.UnitPrice.String(), "currency": p.Currency.String(),
		"isService": p.IsService, "isActive": p.IsActive,
		// Como texto: diff compara con != y un slice no es comparable.
		"taxes": strings.Join(taxes, ","),
	}
}
