package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/platform-api/internal/adapters/postgres/db"
)

// session son las variables que leen shared.current_organization_id() y shared.current_user_id() en las políticas RLS.
type session struct {
	organizationID uuid.UUID
	userID         uuid.UUID
}

// inTx ejecuta fn en una transacción con las variables de sesión fijadas antes de cualquier consulta.
// set_config(..., true) equivale a SET LOCAL: el valor muere con la transacción, requisito con Supavisor
// en modo transacción, donde la siguiente transacción puede caer en otra conexión física (ADR 0003/0004).
func inTx(ctx context.Context, pool *pgxpool.Pool, opts pgx.TxOptions, s session, fn func(q *db.Queries, tx pgx.Tx) error) (err error) {
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return fmt.Errorf("iniciando transacción: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = setSession(ctx, tx, s); err != nil {
		return err
	}
	if err = fn(db.New(tx), tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("confirmando transacción: %w", err)
	}
	return nil
}

func setSession(ctx context.Context, tx pgx.Tx, s session) error {
	_, err := tx.Exec(ctx,
		"select set_config('app.current_organization_id', $1, true), set_config('app.current_user_id', $2, true)",
		uuidOrEmpty(s.organizationID), uuidOrEmpty(s.userID))
	if err != nil {
		return fmt.Errorf("fijando sesión RLS: %w", err)
	}
	return nil
}

// uuidOrEmpty: las funciones shared.current_* convierten la cadena vacía en NULL, que ninguna política acepta.
func uuidOrEmpty(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
