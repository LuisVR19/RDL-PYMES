// Package postgres implementa los puertos de persistencia sobre pgx.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool crea el pool para el rol de aplicación.
//
// Supavisor en modo transacción asigna una conexión física distinta en cada transacción, así que los
// prepared statements con nombre (el modo por defecto de pgx) fallan o chocan entre clientes.
// QueryExecModeExec usa el protocolo extendido con statement sin nombre: sigue enviando los parámetros
// por separado del SQL (sin interpolación en el cliente) y no deja estado en la conexión.
// Ver docs/decisiones/0004-pooler-y-pgx.md.
func NewPool(ctx context.Context, url, password string, maxConns int32, statementTimeout time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Sin %w: el error de parseo puede incluir la URL con la contraseña.
		return nil, fmt.Errorf("DATABASE_URL inválida")
	}
	if password != "" {
		cfg.ConnConfig.Password = password
	}
	cfg.MaxConns = maxConns
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.ConnConfig.StatementCacheCapacity = 0
	cfg.ConnConfig.DescriptionCacheCapacity = 0
	// statement_timeout como parámetro de arranque: aplica a toda la sesión física, también con el pooler.
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(statementTimeout.Milliseconds(), 10)
	cfg.ConnConfig.RuntimeParams["application_name"] = "platform-api"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creando pool: %w", err)
	}
	return pool, nil
}

// PingChecker verifica que la base responde para /readyz.
type PingChecker struct{ Pool *pgxpool.Pool }

func (PingChecker) Name() string { return "database" }

func (c PingChecker) Check(ctx context.Context) error { return c.Pool.Ping(ctx) }
