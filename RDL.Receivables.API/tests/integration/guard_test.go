//go:build integration

// Package integration prueba contra la base de dev con el login receivables_api (nunca superusuario ni
// producción). Cada test corre en una transacción que siempre hace rollback: no deja datos.
//
//	go test -count=1 -tags=integration ./tests/integration/...
package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/receivables-api/internal/adapters/postgres"
	"rdl/receivables-api/internal/platform/config"
)

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if err := config.LoadDotEnv("../../.env"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("sin configuración de base: %v", err)
	}
	p, err := postgres.NewPool(t.Context(), cfg.DB.URL, cfg.DB.Password, 2, cfg.DB.StatementTimeout)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// rollbackTx corre fn en una transacción con la sesión del tenant y la descarta siempre.
func rollbackTx(t *testing.T, p *pgxpool.Pool, org uuid.UUID, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := t.Context()
	tx, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true)", org.String()); err != nil {
		t.Fatal(err)
	}
	fn(ctx, tx)
}

// ADR 0001 §4.2 y migración 00003: aunque la app fije el GUC receivables.recalculating, un UPDATE directo del saldo
// o del estado se rechaza. Solo el recálculo desde un trigger (profundidad > 1) puede escribirlos.
func TestGuardRejectsDirectBalanceUpdate(t *testing.T) {
	p := pool(t)
	var org, id uuid.UUID
	err := p.QueryRow(t.Context(), `select organization_id, id from receivables.receivables limit 1`).Scan(&org, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Sin sesión RLS la tabla se ve vacía; se busca con la organización de los ejemplos de contratos.
		org = uuid.MustParse("304b6c9e-d2cd-56ab-ac15-919e708ec232")
		rollbackTx(t, p, org, func(ctx context.Context, tx pgx.Tx) {
			err = tx.QueryRow(ctx, `select id from receivables.receivables where organization_id = $1 limit 1`, org).Scan(&id)
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		t.Skip("no hay cuentas en dev: correr antes cmd/replay con los ejemplos de InvoiceIssued")
	}
	if err != nil {
		t.Fatal(err)
	}

	for name, update := range map[string]string{
		"saldo":  `update receivables.receivables set balance_amount = 0 where organization_id = $1 and id = $2`,
		"estado": `update receivables.receivables set status = 'paid' where organization_id = $1 and id = $2`,
	} {
		t.Run(name, func(t *testing.T) {
			rollbackTx(t, p, org, func(ctx context.Context, tx pgx.Tx) {
				if _, err := tx.Exec(ctx, "select set_config('receivables.recalculating', 'on', true)"); err != nil {
					t.Fatal(err)
				}
				_, err := tx.Exec(ctx, update, org, id)
				var pgErr *pgconn.PgError
				if !errors.As(err, &pgErr) || pgErr.Code != "23001" { // restrict_violation
					t.Fatalf("se esperaba restrict_violation, se obtuvo %v", err)
				}
			})
		})
	}
}
