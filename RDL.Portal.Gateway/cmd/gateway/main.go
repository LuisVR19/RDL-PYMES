// Command gateway es el composition root del Portal Gateway: configuración, wiring y arranque.
// Sin lógica de negocio y sin credenciales de base de datos: este servicio no tiene base.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"rdl/portal-gateway/internal/adapters/auth"
	httpadapter "rdl/portal-gateway/internal/adapters/http"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/platform/config"
	"rdl/portal-gateway/internal/platform/logger"
	"rdl/portal-gateway/internal/platform/telemetry"
	"rdl/portal-gateway/internal/wiring"
)

// printRoutes imprime la tabla y sale. La superficie pública del gateway se revisa leyendo esto, no el código.
var printRoutes = flag.Bool("routes", false, "imprime la tabla de rutas y termina")

func main() {
	flag.Parse()
	if *printRoutes {
		for _, r := range httpadapter.RegisteredRoutes() {
			fmt.Println(r)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "portal-gateway:", err)
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

	verifier, err := auth.NewVerifier(ctx, log, cfg.Auth.JWKSURL, cfg.Auth.Issuer, cfg.Auth.Audience, cfg.Auth.JWKSRefresh)
	if err != nil {
		return err
	}

	clients := wiring.Clients(cfg, log)
	checks := wiring.HealthChecks(cfg, log)
	handler, err := httpadapter.NewRouter(wiring.Deps(cfg, log, verifier, clients, checks))
	if err != nil {
		return err
	}
	logUpstreams(log, cfg)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("portal-gateway escuchando",
			slog.String("addr", cfg.HTTP.Addr),
			slog.Int("rutas", len(httpadapter.RegisteredRoutes())))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("apagando portal-gateway")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	// TODO(P7 · incremento 6): cerrar aquí, ordenadamente, las conexiones de notificaciones en tiempo real.
	return errors.Join(srv.Shutdown(shutdownCtx), shutdownTelemetry(shutdownCtx))
}

// logUpstreams deja constancia al arrancar de qué APIs están conectadas y cuáles todavía no: es la primera
// pregunta cuando una pantalla del portal responde 503.
func logUpstreams(log *slog.Logger, cfg config.Config) {
	for _, s := range routes.Services() {
		up := cfg.Upstreams.Get(s)
		if up.Configured() {
			log.Info("API destino conectada",
				slog.String("upstream", string(s)),
				slog.String("url", up.URL),
				slog.Duration("timeout", up.Timeout))
			continue
		}
		log.Warn("API destino sin configurar: sus rutas responden 503 y sus partes de una vista degradan",
			slog.String("upstream", string(s)))
	}
}
