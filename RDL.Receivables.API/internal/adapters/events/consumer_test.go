package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"rdl/receivables-api/internal/app"
)

type fakeProcessor struct {
	err         error
	errAtStart  error
	hasDeadline bool
}

func (p *fakeProcessor) Process(ctx context.Context, _ []byte) (app.Result, error) {
	p.errAtStart = ctx.Err()
	_, p.hasDeadline = ctx.Deadline()
	return app.Result{Outcome: app.OutcomeProcessed, Attempts: 1}, p.err
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// El apagado no corta el mensaje en curso: su contexto no hereda la cancelación, solo el tiempo máximo propio.
func TestHandleDetachesFromShutdown(t *testing.T) {
	p := &fakeProcessor{}
	c := NewConsumer(NoSource{}, p, quietLog(), time.Minute)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := c.Handle(ctx, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if p.errAtStart != nil {
		t.Error("el mensaje recibió un contexto ya cancelado")
	}
	if !p.hasDeadline {
		t.Error("el mensaje debe tener tiempo máximo")
	}
}

func TestHandleReturnsErrorSoTheSourceRedelivers(t *testing.T) {
	c := NewConsumer(NoSource{}, &fakeProcessor{err: errors.New("dead letter caída")}, quietLog(), time.Minute)
	if err := c.Handle(t.Context(), []byte("{}")); err == nil {
		t.Error("se esperaba error")
	}
}

func TestNoSourceStopsOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- NewConsumer(NoSource{}, &fakeProcessor{}, quietLog(), time.Minute).Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run no terminó al cancelar")
	}
}
