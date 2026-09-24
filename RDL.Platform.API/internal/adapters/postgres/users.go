package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"rdl/platform-api/internal/adapters/postgres/db"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/user"
)

type users struct{ q *db.Queries }

func (r users) FindBySubject(ctx context.Context, subject string) (user.User, error) {
	row, err := r.q.FindUserBySubject(ctx, subject)
	if isNoRows(err) {
		return user.User{}, app.ErrNotFound
	}
	if err != nil {
		return user.User{}, fmt.Errorf("buscando usuario: %w", err)
	}
	return toUser(row.ID, row.ExternalSubject, row.Email, row.FullName, row.Status, row.ActiveOrganizationID, row.CreatedAt), nil
}

func (r users) CreateIfAbsent(ctx context.Context, u user.User) (user.User, bool, error) {
	row, err := r.q.InsertUserIfAbsent(ctx, db.InsertUserIfAbsentParams{
		ExternalSubject: u.Subject, Email: u.Email, FullName: u.FullName,
	})
	if isNoRows(err) {
		// Otra transacción lo creó entre la búsqueda y el insert.
		existing, err := r.FindBySubject(ctx, u.Subject)
		return existing, false, err
	}
	if isUniqueViolation(err, "users_email_uk") {
		return user.User{}, false, fmt.Errorf("%w: el email ya pertenece a otra identidad", app.ErrConflict)
	}
	if err != nil {
		return user.User{}, false, fmt.Errorf("creando usuario: %w", err)
	}
	return toUser(row.ID, row.ExternalSubject, row.Email, row.FullName, row.Status, row.ActiveOrganizationID, row.CreatedAt), true, nil
}

func (r users) SetActiveOrganization(ctx context.Context, userID, organizationID uuid.UUID) error {
	n, err := r.q.SetUserActiveOrganization(ctx, db.SetUserActiveOrganizationParams{
		OrganizationID: uuid.NullUUID{UUID: organizationID, Valid: true}, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("guardando organización activa: %w", err)
	}
	if n == 0 {
		return app.ErrNotFound
	}
	return nil
}

func toUser(id uuid.UUID, subject, email, fullName, status string, activeOrg uuid.NullUUID, createdAt time.Time) user.User {
	u := user.User{ID: id, Subject: subject, Email: email, FullName: fullName, Status: user.Status(status), CreatedAt: createdAt}
	if activeOrg.Valid {
		u.ActiveOrganizationID = activeOrg.UUID
	}
	return u
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
