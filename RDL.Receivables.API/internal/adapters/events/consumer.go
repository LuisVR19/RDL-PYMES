package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/correlation"
)

// Source entrega los mensajes del bus. Run llama a handle por cada mensaje y lo da por consumido (ack) solo si
// handle devuelve nil; si no, lo vuelve a entregar más tarde (entrega al menos una vez, sin orden garantizado).
// Run termina cuando ctx se cancela, después de que vuelve el handle en curso.
type Source interface {
	Run(ctx context.Context, handle func(context.Context, []byte) error) error
}

// NoSource es la fuente mientras no exista el transporte: no entrega nada y espera el apagado.
// TODO(P2): transporte de eventos (docs/ESTADO.md). Hasta entonces los eventos entran con cmd/replay.
type NoSource struct{}

func (NoSource) Run(ctx context.Context, _ func(context.Context, []byte) error) error {
	<-ctx.Done()
	return nil
}

// Processor es app.ProcessEvent visto desde el runner.
type Processor interface {
	Process(ctx context.Context, payload []byte) (app.Result, error)
}

// Consumer une la fuente con el procesador: logs, métricas y el tiempo máximo de cada mensaje.
type Consumer struct {
	source    Source
	processor Processor
	log       *slog.Logger
	timeout   time.Duration
	now       func() time.Time

	messages metric.Int64Counter
	attempts metric.Int64Histogram
	lag      metric.Float64Histogram
}

func NewConsumer(source Source, processor Processor, log *slog.Logger, messageTimeout time.Duration) *Consumer {
	meter := otel.Meter("rdl/receivables-api/consumer")
	c := &Consumer{source: source, processor: processor, log: log, timeout: messageTimeout, now: time.Now}
	// Los instrumentos del SDK no fallan con nombres válidos; con el provider no-op tampoco.
	c.messages, _ = meter.Int64Counter("receivables.consumer.messages",
		metric.WithDescription("Mensajes consumidos por resultado: processed, duplicate, dead_lettered, failed"))
	c.attempts, _ = meter.Int64Histogram("receivables.consumer.attempts",
		metric.WithDescription("Intentos por mensaje (más de 1 = hubo reintentos)"))
	c.lag, _ = meter.Float64Histogram("receivables.consumer.lag", metric.WithUnit("s"),
		metric.WithDescription("Antigüedad del evento (occurredAt) al terminar de procesarlo"))
	return c
}

// Run consume hasta que ctx se cancela. El mensaje en curso termina aunque llegue la señal de apagado (con su propio
// tiempo máximo): cortarlo a la mitad solo obligaría a reprocesarlo.
func (c *Consumer) Run(ctx context.Context) error {
	return c.source.Run(ctx, c.Handle)
}

// Handle procesa un mensaje. Lo usan la fuente y cmd/replay.
func (c *Consumer) Handle(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
	defer cancel()

	res, err := c.processor.Process(ctx, payload)
	ref := res.Ref
	if ref.CorrelationID != uuid.Nil {
		ctx = correlation.WithID(ctx, ref.CorrelationID)
	}
	outcome := string(res.Outcome)
	if err != nil {
		outcome = "failed"
	}
	attrs := metric.WithAttributes(attribute.String("event_type", ref.Type), attribute.String("outcome", outcome))
	c.messages.Add(ctx, 1, attrs)
	c.attempts.Record(ctx, int64(res.Attempts), attrs)
	if !ref.OccurredAt.IsZero() {
		c.lag.Record(ctx, c.now().Sub(ref.OccurredAt).Seconds(), attrs)
	}

	logAttrs := []any{
		slog.String("event_id", ref.ID.String()), slog.String("event_type", ref.Type),
		slog.String("organization_id", ref.OrganizationID.String()), slog.String("outcome", outcome),
		slog.Int("attempts", res.Attempts),
	}
	switch {
	case err != nil:
		c.log.ErrorContext(ctx, "evento sin consumir: se volverá a entregar", append(logAttrs, slog.Any("error", err))...)
	case res.Outcome == app.OutcomeDeadLettered:
		c.log.WarnContext(ctx, "evento a dead letter", append(logAttrs, slog.Any("error", res.Err))...)
	default:
		c.log.InfoContext(ctx, "evento consumido", logAttrs...)
	}
	return err
}
