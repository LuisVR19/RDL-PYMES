package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/pkg/tenancy"
)

// CreateBranch crea una sucursal en la organización activa (owner, admin). Idempotente por Idempotency-Key.
type CreateBranch struct{ tx TxManager }

func NewCreateBranch(tx TxManager) *CreateBranch { return &CreateBranch{tx: tx} }

type CreateBranchResult struct {
	Branch   branch.Branch
	Replayed bool
}

func (uc *CreateBranch) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, in branch.NewInput) (CreateBranchResult, error) {
	if err := authorize(t, membership.PermBranchesManage); err != nil {
		return CreateBranchResult{}, err
	}
	candidate, err := branch.New(t.OrganizationID(), in)
	if err != nil {
		return CreateBranchResult{}, err
	}
	hash, err := requestHash(in)
	if err != nil {
		return CreateBranchResult{}, err
	}

	var res CreateBranchResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		prev, err := tx.Idempotency().Claim(ctx, t.OrganizationID(), idempotencyKey, hash, IdempotencyTTL)
		if err != nil {
			return err
		}
		if prev != nil {
			if prev.RequestHash != hash {
				return ErrIdempotencyKeyReused
			}
			id, err := uuid.Parse(prev.Result["branchId"])
			if err != nil {
				return fmt.Errorf("resultado idempotente corrupto: %w", err)
			}
			b, err := tx.Branches().Get(ctx, t.OrganizationID(), id)
			res = CreateBranchResult{Branch: b, Replayed: true}
			return err
		}

		b, err := tx.Branches().Create(ctx, candidate)
		if err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "branch.created", EntityType: "branch", EntityID: b.ID, After: branchAudit(b),
		}); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"branchId": b.ID.String()},
		}); err != nil {
			return err
		}
		res = CreateBranchResult{Branch: b}
		return nil
	})
	return res, err
}

// ListBranches y GetBranch: lectura para todos los roles.
type ListBranches struct{ tx TxManager }

func NewListBranches(tx TxManager) *ListBranches { return &ListBranches{tx: tx} }

type BranchPage struct {
	Items []branch.Branch
	Next  *PageCursor
}

func (uc *ListBranches) Execute(ctx context.Context, t tenancy.Context, q BranchQuery) (BranchPage, error) {
	if err := authorize(t, membership.PermBranchesRead); err != nil {
		return BranchPage{}, err
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageSize
	}
	q.Limit = min(q.Limit, MaxPageSize)

	var page BranchPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Branches().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		if len(items) > q.Limit {
			items = items[:q.Limit]
			last := items[len(items)-1]
			page.Next = &PageCursor{At: last.CreatedAt, ID: last.ID}
		}
		page.Items = items
		return nil
	})
	return page, err
}

type GetBranch struct{ tx TxManager }

func NewGetBranch(tx TxManager) *GetBranch { return &GetBranch{tx: tx} }

func (uc *GetBranch) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (branch.Branch, error) {
	if err := authorize(t, membership.PermBranchesRead); err != nil {
		return branch.Branch{}, err
	}
	var b branch.Branch
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		b, err = tx.Branches().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return b, err
}

// UpdateBranch edita o desactiva (baja lógica) una sucursal (owner, admin).
type UpdateBranch struct{ tx TxManager }

func NewUpdateBranch(tx TxManager) *UpdateBranch { return &UpdateBranch{tx: tx} }

func (uc *UpdateBranch) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID, p branch.Patch) (branch.Branch, error) {
	if err := authorize(t, membership.PermBranchesManage); err != nil {
		return branch.Branch{}, err
	}
	var out branch.Branch
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		current, err := tx.Branches().GetForUpdate(ctx, t.OrganizationID(), id)
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
		if out, err = tx.Branches().Update(ctx, next); err != nil {
			return err
		}
		action := "branch.updated"
		switch {
		case current.IsActive && !next.IsActive:
			action = "branch.deactivated"
		case !current.IsActive && next.IsActive:
			action = "branch.reactivated"
		}
		before, after := diff(branchAudit(current), branchAudit(next))
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: action, EntityType: "branch", EntityID: id, Before: before, After: after,
		})
	})
	return out, err
}

func branchAudit(b branch.Branch) map[string]any {
	return map[string]any{
		"code": b.Code, "name": b.Name, "address": b.Address, "phone": b.Phone, "email": b.Email, "isActive": b.IsActive,
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
