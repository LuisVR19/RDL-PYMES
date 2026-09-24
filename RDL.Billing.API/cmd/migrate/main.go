// Command migrate aplica las migraciones goose del schema billing.
//
// Uso: migrate <up|up-by-one|status|down> (MIGRATE_DATABASE_URL con el login billing_migrate, conexión directa o puerto 5432).
// Se niega a correr si la sesión no es billing_migrator: así ningún objeto queda a nombre de otro rol.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"

	"rdl/billing-api/internal/platform/config"
	"rdl/billing-api/migrations"
)

const (
	expectedRole = "billing_migrator"
	historyTable = "billing.goose_db_version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return errors.New("uso: migrate <up|up-by-one|status|down>")
	}
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		// Modo sesión (5432): el `set role billing_migrator` que fija ALTER ROLE debe durar toda la conexión.
		var err error
		url, err = config.PoolerURL(os.Getenv("SUPABASE_URL"), os.Getenv("DB_POOLER_HOST"), "billing_migrate", config.PoolerSessionPort)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	connCfg, err := pgx.ParseConfig(url)
	if err != nil {
		return errors.New("MIGRATE_DATABASE_URL inválida")
	}
	// MIGRATE_DB_PASSWORD evita tener que codificar caracteres especiales dentro de la URL.
	if pw := os.Getenv("MIGRATE_DB_PASSWORD"); pw != "" {
		connCfg.Password = pw
	}
	db := stdlib.OpenDB(*connCfg)
	defer func() { _ = db.Close() }()
	// Una sola conexión: el `set role` que ALTER ROLE fija al conectar debe valer para todo el proceso.
	db.SetMaxOpenConns(1)

	var role string
	if err := db.QueryRowContext(ctx, "select current_user").Scan(&role); err != nil {
		return fmt.Errorf("conectando: %w", err)
	}
	if role != expectedRole {
		return fmt.Errorf("la sesión corre como %q; se esperaba %q (revise ALTER ROLE ... SET role)", role, expectedRole)
	}

	store, err := database.NewStore(database.DialectPostgres, historyTable)
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider("", db, migrations.FS, goose.WithStore(store))
	if err != nil {
		return err
	}

	switch os.Args[1] {
	case "up":
		results, err := provider.Up(ctx)
		for _, r := range results {
			fmt.Println(r)
		}
		return err
	case "up-by-one":
		// Expand → migrate → contract: aplica solo la siguiente migración pendiente.
		r, err := provider.UpByOne(ctx)
		if r != nil {
			fmt.Println(r)
		}
		return err
	case "down":
		r, err := provider.Down(ctx)
		if r != nil {
			fmt.Println(r)
		}
		return err
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			applied := "pendiente"
			if s.State == goose.StateApplied {
				applied = "aplicada " + s.AppliedAt.UTC().Format(time.RFC3339)
			}
			fmt.Printf("%05d  %-45s %s\n", s.Source.Version, s.Source.Path, applied)
		}
		return nil
	default:
		return fmt.Errorf("comando desconocido %q", os.Args[1])
	}
}
