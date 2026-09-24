package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/branch"
)

type branches struct{ q *db.Queries }

func (r branches) Create(ctx context.Context, b branch.Branch) (branch.Branch, error) {
	row, err := r.q.InsertBranch(ctx, db.InsertBranchParams{
		OrganizationID: b.OrganizationID, Code: b.Code, Name: b.Name, Address: b.Address, Phone: b.Phone, Email: b.Email,
	})
	if isUniqueViolation(err, "branches_org_code_uk") {
		return branch.Branch{}, fmt.Errorf("%w: ya existe una sucursal con el código %q", app.ErrConflict, b.Code)
	}
	if err != nil {
		return branch.Branch{}, fmt.Errorf("creando sucursal: %w", err)
	}
	return toBranch(db.GetBranchRow(row)), nil
}

func (r branches) List(ctx context.Context, org uuid.UUID, q app.BranchQuery) ([]branch.Branch, error) {
	params := db.ListBranchesParams{
		OrganizationID: org,
		PageSize:       int32(q.Limit), // #nosec G115 -- acotado por app.MaxPageSize
	}
	if q.Active != nil {
		params.IsActive = pgtype.Bool{Bool: *q.Active, Valid: true}
	}
	if q.After != nil {
		params.AfterCreatedAt = pgtype.Timestamptz{Time: q.After.At, Valid: true}
		params.AfterID = uuid.NullUUID{UUID: q.After.ID, Valid: true}
	}
	rows, err := r.q.ListBranches(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando sucursales: %w", err)
	}
	out := make([]branch.Branch, 0, len(rows))
	for _, row := range rows {
		out = append(out, toBranch(db.GetBranchRow(row)))
	}
	return out, nil
}

func (r branches) Get(ctx context.Context, org, id uuid.UUID) (branch.Branch, error) {
	row, err := r.q.GetBranch(ctx, db.GetBranchParams{OrganizationID: org, ID: id})
	return oneBranch(row, err)
}

func (r branches) GetForUpdate(ctx context.Context, org, id uuid.UUID) (branch.Branch, error) {
	row, err := r.q.GetBranchForUpdate(ctx, db.GetBranchForUpdateParams{OrganizationID: org, ID: id})
	return oneBranch(db.GetBranchRow(row), err)
}

func (r branches) Update(ctx context.Context, b branch.Branch) (branch.Branch, error) {
	row, err := r.q.UpdateBranch(ctx, db.UpdateBranchParams{
		OrganizationID: b.OrganizationID, ID: b.ID, Name: b.Name, Address: b.Address, Phone: b.Phone, Email: b.Email, IsActive: b.IsActive,
	})
	return oneBranch(db.GetBranchRow(row), err)
}

func oneBranch(row db.GetBranchRow, err error) (branch.Branch, error) {
	if isNoRows(err) {
		return branch.Branch{}, app.ErrNotFound
	}
	if err != nil {
		return branch.Branch{}, fmt.Errorf("leyendo sucursal: %w", err)
	}
	return toBranch(row), nil
}

func toBranch(r db.GetBranchRow) branch.Branch {
	return branch.Branch{
		ID: r.ID, OrganizationID: r.OrganizationID, Code: r.Code, Name: r.Name, Address: r.Address,
		Phone: r.Phone, Email: r.Email, IsActive: r.IsActive, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
