package app

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/pkg/tenancy"
)

// CreateCustomer crea un cliente en la organización activa (owner, admin, biller). Idempotente por Idempotency-Key.
type CreateCustomer struct{ tx TxManager }

func NewCreateCustomer(tx TxManager) *CreateCustomer { return &CreateCustomer{tx: tx} }

type CreateCustomerResult struct {
	Customer customer.Customer
	Replayed bool
}

func (uc *CreateCustomer) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in customer.NewInput) (CreateCustomerResult, error) {
	if err := authorize(t, permission.CustomersManage); err != nil {
		return CreateCustomerResult{}, err
	}
	candidate, err := customer.New(t.OrganizationID(), t.UserID(), in)
	if err != nil {
		return CreateCustomerResult{}, err
	}
	hash, err := requestHash(customerAudit(candidate))
	if err != nil {
		return CreateCustomerResult{}, err
	}

	var res CreateCustomerResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		prevID, err := replayed(ctx, tx, t.OrganizationID(), idempotencyKey, hash, "customerId")
		if err != nil {
			return err
		}
		if prevID != uuid.Nil {
			c, err := tx.Customers().Get(ctx, t.OrganizationID(), prevID)
			res = CreateCustomerResult{Customer: c, Replayed: true}
			return err
		}

		c, err := tx.Customers().Create(ctx, candidate)
		if err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "customer.created", EntityType: "customer", EntityID: c.ID, After: customerAudit(c),
		}); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"customerId": c.ID.String()},
		}); err != nil {
			return err
		}
		res = CreateCustomerResult{Customer: c}
		return nil
	})
	return res, err
}

// ListCustomers pagina los clientes de la organización activa. Lectura para todos los roles.
type ListCustomers struct{ tx TxManager }

func NewListCustomers(tx TxManager) *ListCustomers { return &ListCustomers{tx: tx} }

type CustomerPage struct {
	Items []customer.Customer
	Next  *PageCursor
}

func (uc *ListCustomers) Execute(ctx context.Context, t tenancy.Context, q CustomerQuery) (CustomerPage, error) {
	if err := authorize(t, permission.CustomersRead); err != nil {
		return CustomerPage{}, err
	}
	q.Limit = pageLimit(q.Limit)

	var page CustomerPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Customers().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		page.Items, page.Next = cutPage(items, q.Limit, func(c customer.Customer) PageCursor {
			return PageCursor{At: c.CreatedAt, ID: c.ID}
		})
		return nil
	})
	return page, err
}

// GetCustomer: detalle, para todos los roles. De otra organización → ErrNotFound.
type GetCustomer struct{ tx TxManager }

func NewGetCustomer(tx TxManager) *GetCustomer { return &GetCustomer{tx: tx} }

func (uc *GetCustomer) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (customer.Customer, error) {
	if err := authorize(t, permission.CustomersRead); err != nil {
		return customer.Customer{}, err
	}
	var c customer.Customer
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		c, err = tx.Customers().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return c, err
}

// UpdateCustomer edita o desactiva (baja lógica) un cliente (owner, admin, biller). Solo toca billing.customers:
// las facturas emitidas conservan su snapshot y los borradores lo toman al emitirse.
type UpdateCustomer struct{ tx TxManager }

func NewUpdateCustomer(tx TxManager) *UpdateCustomer { return &UpdateCustomer{tx: tx} }

func (uc *UpdateCustomer) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p customer.Patch) (customer.Customer, error) {
	if err := authorize(t, permission.CustomersManage); err != nil {
		return customer.Customer{}, err
	}
	var out customer.Customer
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		current, err := tx.Customers().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		next, err := current.Apply(p)
		if err != nil {
			return err
		}
		if next == current {
			out = current
			return nil
		}
		if out, err = tx.Customers().Update(ctx, next); err != nil {
			return err
		}
		action := "customer.updated"
		switch {
		case current.IsActive && !next.IsActive:
			action = "customer.deactivated"
		case !current.IsActive && next.IsActive:
			action = "customer.reactivated"
		}
		before, after := diff(customerAudit(current), customerAudit(next))
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: action, EntityType: "customer", EntityID: id, Before: before, After: after,
		})
	})
	return out, err
}

func customerAudit(c customer.Customer) map[string]any {
	return map[string]any{
		"identificationTypeCode": c.Identification.TypeCode, "identificationNumber": c.Identification.Number,
		"legalName": c.LegalName, "tradeName": c.TradeName, "email": c.Email, "phone": c.Phone,
		"address": c.Address, "isActive": c.IsActive,
	}
}

// diff deja en la auditoría solo los campos que cambiaron.
func diff(a, b map[string]any) (before, after map[string]any) {
	before, after = map[string]any{}, map[string]any{}
	for k, v := range a {
		if b[k] != v {
			before[k], after[k] = v, b[k]
		}
	}
	return before, after
}
