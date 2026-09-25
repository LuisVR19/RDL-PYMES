// Command replay es el adapter de desarrollo del consumidor: lee eventos de un archivo JSON y los pasa por el mismo
// procesamiento que cmd/consumer (validación, inbox, efecto, auditoría, reintentos y dead letter). Solo dev y local.
//
//	go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json
//	go run ./cmd/replay -file evento.json
//
// Acepta un evento suelto, un arreglo de eventos o un archivo de ejemplos de contratos (-set elige los válidos, los
// inválidos, que se arman aplicando su patch, o todos). Reproducir el mismo archivo dos veces no duplica nada: son los mismos eventId.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	eventsadapter "rdl/receivables-api/internal/adapters/events"
	"rdl/receivables-api/internal/adapters/postgres"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/platform/config"
	"rdl/receivables-api/internal/platform/examples"
	"rdl/receivables-api/internal/platform/logger"
	"rdl/receivables-api/internal/wiring"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "receivables-replay:", err)
		os.Exit(1)
	}
}

func run() error {
	file := flag.String("file", "", "archivo con uno o varios eventos (obligatorio)")
	set := flag.String("set", "valid", "en un archivo de ejemplos de contratos: valid, invalid o all")
	attempts := flag.Int("attempts", 3, "intentos por evento ante errores transitorios")
	flag.Parse()
	if *file == "" {
		flag.Usage()
		return errors.New("falta -file")
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	messages, err := split(raw, *set)
	if err != nil {
		return err
	}

	if err := config.LoadDotEnv(".env"); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.Env == "prod" || cfg.Env == "production" {
		return errors.New("replay es solo para dev y local")
	}
	log := logger.New(os.Stderr, cfg.Log.Level, cfg.ServiceName+"-replay", cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.NewPool(ctx, cfg.DB.URL, cfg.DB.Password, cfg.DB.MaxConns, cfg.DB.StatementTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()

	policy := app.DefaultRetryPolicy()
	policy.Attempts, policy.Max = max(*attempts, 1), 4*time.Second
	txm, err := wiring.TxManager(pool)
	if err != nil {
		return err
	}
	processor, err := wiring.EventProcessor(pool, txm, policy)
	if err != nil {
		return err
	}
	consumer := eventsadapter.NewConsumer(nil, processor, log, cfg.Consumer.MessageTimeout)

	failed := 0
	for i, m := range messages {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := consumer.Handle(ctx, m); err != nil {
			failed++
			fmt.Printf("%d: sin consumir: %v\n", i, err)
		}
	}
	fmt.Printf("%d eventos, %d sin consumir (el resultado de cada uno está en el log)\n", len(messages), failed)
	if failed > 0 {
		return fmt.Errorf("%d eventos sin consumir", failed)
	}
	return nil
}

// split separa los mensajes del archivo sin interpretarlos: cada uno pasa crudo por el decodificador.
func split(raw []byte, set string) ([]json.RawMessage, error) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	if !examples.IsExamplesFile(raw) {
		return []json.RawMessage{raw}, nil
	}
	valid, invalid, err := examples.Parse(raw)
	if err != nil {
		return nil, err
	}
	var bad []json.RawMessage
	for _, inv := range invalid {
		bad = append(bad, inv.Payload)
	}
	switch set {
	case "valid":
		return valid, nil
	case "invalid":
		return bad, nil
	case "all":
		return append(valid, bad...), nil
	}
	return nil, fmt.Errorf("-set %q: se esperaba valid, invalid o all", set)
}
