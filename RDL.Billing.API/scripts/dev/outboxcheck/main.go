// Command outboxcheck verifica, para el E2E, que la emisión de un documento dejó exactamente un InvoiceIssued en
// integration.outbox_messages y que su payload valida contra el schema del contrato. Solo lee, con el login
// billing_api del .env y la sesión RLS de la organización.
//
// Uso: go run ./scripts/dev/outboxcheck -org <organizationId> -invoice <invoiceId>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rdl/billing-api/internal/adapters/postgres"
	"rdl/billing-api/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "outboxcheck:", err)
		os.Exit(1)
	}
}

func run() error {
	orgFlag := flag.String("org", "", "organización del documento")
	invFlag := flag.String("invoice", "", "documento emitido")
	flag.Parse()
	org, err := uuid.Parse(*orgFlag)
	if err != nil {
		return errors.New("-org debe ser un UUID")
	}
	inv, err := uuid.Parse(*invFlag)
	if err != nil {
		return errors.New("-invoice debe ser un UUID")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, cfg.DB.URL, cfg.DB.Password, 1, 10*time.Second)
	if err != nil {
		return err
	}
	defer pool.Close()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "select set_config('app.current_organization_id', $1, true)", org.String()); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `select event_type, payload::text from integration.outbox_messages
	                            where organization_id = $1 and aggregate_id = $2`, org, inv)
	if err != nil {
		return err
	}
	defer rows.Close()
	var events []string
	var payload string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType, &payload); err != nil {
			return err
		}
		events = append(events, eventType)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(events) != 1 || events[0] != cevents.InvoiceIssuedType {
		return fmt.Errorf("se esperaba exactamente un InvoiceIssued, hay %v", events)
	}
	v, err := cevents.DefaultValidator()
	if err != nil {
		return err
	}
	if err := v.Validate(cevents.InvoiceIssuedSchemaV1, []byte(payload)); err != nil {
		return err
	}
	fmt.Println("InvoiceIssued valido")
	return nil
}
