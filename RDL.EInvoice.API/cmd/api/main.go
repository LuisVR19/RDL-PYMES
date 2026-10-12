// Command api es el composition root de la E-Invoice API (servicio fiscal): configuración, wiring y arranque.
// Sin lógica de negocio.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // la imagen distroless no trae zoneinfo: FechaEmision usa la hora de Costa Rica

	"rdl/einvoice-api/internal/adapters/auth"
	httpadapter "rdl/einvoice-api/internal/adapters/http"
	"rdl/einvoice-api/internal/adapters/postgres"
	"rdl/einvoice-api/internal/platform/config"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/internal/platform/logger"
	"rdl/einvoice-api/internal/platform/telemetry"
	"rdl/einvoice-api/internal/wiring"
	"rdl/einvoice-api/pkg/tenancy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "einvoice-api:", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.Log.Level, cfg.ServiceName, cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, cfg.ServiceName, cfg.Env, cfg.OTel.Endpoint)
	if err != nil {
		return fmt.Errorf("telemetría: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg.DB.URL, cfg.DB.Password, cfg.DB.MaxConns, cfg.DB.StatementTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()

	verifier, err := auth.NewVerifier(ctx, log, cfg.Auth.JWKSURL, cfg.Auth.Issuer, cfg.Auth.Audience, cfg.Auth.JWKSRefresh)
	if err != nil {
		return err
	}
	memberships := tenancy.NewCachedResolver(postgres.NewMembershipResolver(pool), cfg.Auth.MembershipCacheTTL)
	txm, err := wiring.TxManager(pool)
	if err != nil {
		return err
	}

	secretStore, err := wiring.SecretStore(cfg.Fiscal.SecretsDir, cfg.Fiscal.SecretsMasterKey)
	if err != nil {
		return err
	}
	if secretStore == nil {
		log.Warn("sin SECRETS_MASTER_KEY: las credenciales de Hacienda y el certificado no se pueden configurar (503)")
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	checks := health.New(log, 3*time.Second,
		postgres.PingChecker{Pool: pool},
		health.CheckFunc{CheckName: "jwks", Fn: func(ctx context.Context) error {
			return reachable(ctx, httpClient, cfg.Auth.JWKSURL)
		}},
	)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           httpadapter.NewRouter(wiring.Deps(log, checks, verifier, memberships, txm, secretStore)),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("einvoice-api escuchando", slog.String("addr", cfg.HTTP.Addr),
			slog.String("hacienda_environment", cfg.Fiscal.HaciendaEnvironment),
			slog.Bool("provider_identification_set", cfg.Fiscal.ProviderIdentification != ""))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("apagando einvoice-api")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return errors.Join(srv.Shutdown(shutdownCtx), shutdownTelemetry(shutdownCtx))
}

// reachable es el check de readiness del JWKS hasta que el adapter de auth exponga el estado de su caché.
func reachable(ctx context.Context, c *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS respondió %d", resp.StatusCode)
	}
	return nil
}
