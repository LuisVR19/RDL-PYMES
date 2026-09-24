package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
)

type customers struct{ q *db.Queries }

func (r customers) Create(ctx context.Context, c customer.Customer) (customer.Customer, error) {
	row, err := r.q.InsertCustomer(ctx, db.InsertCustomerParams{
		OrganizationID:         c.OrganizationID,
		IdentificationTypeCode: c.Identification.TypeCode,
		IdentificationNumber:   c.Identification.Number,
		LegalName:              c.LegalName,
		TradeName:              c.TradeName,
		Email:                  c.Email,
		Phone:                  c.Phone,
		Address:                c.Address,
		CreatedByUserID:        nullUUID(c.CreatedByUserID),
	})
	if isUniqueViolation(err, "customers_identification_uk") {
		return customer.Customer{}, app.ErrCustomerIdentificationTaken
	}
	if err != nil {
		return customer.Customer{}, fmt.Errorf("creando cliente: %w", err)
	}
	return toCustomer(db.GetCustomerRow(row)), nil
}

func (r customers) List(ctx context.Context, org uuid.UUID, q app.CustomerQuery) ([]customer.Customer, error) {
	params := db.ListCustomersParams{
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
	rows, err := r.q.ListCustomers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("listando clientes: %w", err)
	}
	out := make([]customer.Customer, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCustomer(db.GetCustomerRow(row)))
	}
	return out, nil
}

func (r customers) Get(ctx context.Context, org, id uuid.UUID) (customer.Customer, error) {
	row, err := r.q.GetCustomer(ctx, db.GetCustomerParams{OrganizationID: org, ID: id})
	return oneCustomer(row, err)
}

func (r customers) GetForUpdate(ctx context.Context, org, id uuid.UUID) (customer.Customer, error) {
	row, err := r.q.GetCustomerForUpdate(ctx, db.GetCustomerForUpdateParams{OrganizationID: org, ID: id})
	return oneCustomer(db.GetCustomerRow(row), err)
}

func (r customers) Update(ctx context.Context, c customer.Customer) (customer.Customer, error) {
	row, err := r.q.UpdateCustomer(ctx, db.UpdateCustomerParams{
		OrganizationID: c.OrganizationID, ID: c.ID, LegalName: c.LegalName, TradeName: c.TradeName,
		Email: c.Email, Phone: c.Phone, Address: c.Address, IsActive: c.IsActive,
	})
	return oneCustomer(db.GetCustomerRow(row), err)
}

func oneCustomer(row db.GetCustomerRow, err error) (customer.Customer, error) {
	if isNoRows(err) {
		return customer.Customer{}, app.ErrNotFound
	}
	if err != nil {
		return customer.Customer{}, fmt.Errorf("leyendo cliente: %w", err)
	}
	return toCustomer(row), nil
}

// escapeLike neutraliza los comodines de LIKE: buscar "50%" no debe coincidir con todo.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func toCustomer(r db.GetCustomerRow) customer.Customer {
	return customer.Customer{
		ID:              r.ID,
		OrganizationID:  r.OrganizationID,
		Identification:  customer.Identification{TypeCode: r.IdentificationTypeCode, Number: r.IdentificationNumber},
		LegalName:       r.LegalName,
		TradeName:       r.TradeName,
		Email:           r.Email,
		Phone:           r.Phone,
		Address:         r.Address,
		IsActive:        r.IsActive,
		CreatedByUserID: r.CreatedByUserID.UUID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
