// Command api es el composition root de Platform API: configuración, wiring y arranque. Sin lógica de negocio.
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
	_ "time/tzdata" // la imagen distroless no trae zoneinfo: sin esto la validación de zona horaria fallaría

	"rdl/platform-api/internal/adapters/auth"
	httpadapter "rdl/platform-api/internal/adapters/http"
	"rdl/platform-api/internal/adapters/postgres"
	"rdl/platform-api/internal/platform/config"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/internal/platform/logger"
	"rdl/platform-api/internal/platform/telemetry"
	"rdl/platform-api/internal/wiring"
	"rdl/platform-api/pkg/tenancy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "platform-api:", err)
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
	txm := postgres.NewTxManager(pool)

	httpClient := &http.Client{Timeout: 5 * time.Second}
	checks := health.New(log, 3*time.Second,
		postgres.PingChecker{Pool: pool},
		health.CheckFunc{CheckName: "jwks", Fn: func(ctx context.Context) error {
			return reachable(ctx, httpClient, cfg.Auth.JWKSURL)
		}},
	)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           httpadapter.NewRouter(wiring.Deps(log, checks, verifier, memberships, txm)),
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("platform-api escuchando", slog.String("addr", cfg.HTTP.Addr))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("apagando platform-api")
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
