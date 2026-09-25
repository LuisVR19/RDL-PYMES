// Command consumer consume los eventos de Billing (InvoiceIssued; CreditNoteIssued, DebitNoteIssued e
// InvoiceCancelled en el incremento 8). Es un proceso aparte de la API para escalar y apagarse por separado
// (ADR 0005). Sirve solo /healthz y /readyz. Sin lógica de negocio: config, wiring y ciclo de vida.
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
	_ "time/tzdata"

	eventsadapter "rdl/receivables-api/internal/adapters/events"
	"rdl/receivables-api/internal/adapters/postgres"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/platform/config"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/internal/platform/logger"
	"rdl/receivables-api/internal/platform/telemetry"
	"rdl/receivables-api/internal/wiring"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "receivables-consumer:", err)
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
	service := cfg.ServiceName + "-consumer"
	log := logger.New(os.Stdout, cfg.Log.Level, service, cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTelemetry, err := telemetry.Setup(ctx, service, cfg.Env, cfg.OTel.Endpoint)
	if err != nil {
		return fmt.Errorf("telemetría: %w", err)
	}
	pool, err := postgres.NewPool(ctx, cfg.DB.URL, cfg.DB.Password, cfg.DB.MaxConns, cfg.DB.StatementTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()

	txm, err := wiring.TxManager(pool)
	if err != nil {
		return err
	}
	processor, err := wiring.EventProcessor(pool, txm, app.DefaultRetryPolicy())
	if err != nil {
		return err
	}
	// TODO(P2): reemplazar NoSource por el adapter del transporte de eventos.
	consumer := eventsadapter.NewConsumer(eventsadapter.NoSource{}, processor, log, cfg.Consumer.MessageTimeout)

	checks := health.New(log, 3*time.Second, postgres.PingChecker{Pool: pool})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", checks.Live)
	mux.HandleFunc("GET /readyz", checks.Ready)
	srv := &http.Server{
		Addr:              cfg.Consumer.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	consumeErr := make(chan error, 1)
	go func() { consumeErr <- consumer.Run(ctx) }()
	log.Info("receivables-consumer iniciado", slog.String("health_addr", cfg.Consumer.HTTPAddr))

	var runErr error
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = err
		}
		stop()
		<-consumeErr
	case runErr = <-consumeErr:
	case <-ctx.Done():
		log.Info("apagando receivables-consumer: se termina el mensaje en curso")
		// Run vuelve cuando termina el mensaje en curso; el tiempo máximo lo acota el propio mensaje.
		runErr = <-consumeErr
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return errors.Join(runErr, srv.Shutdown(shutdownCtx), shutdownTelemetry(shutdownCtx))
}
