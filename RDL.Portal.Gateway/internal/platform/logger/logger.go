// Package logger configura slog en JSON con los campos transversales de la arquitectura (8.2):
// service, correlationId, organizationId y trace_id, tomados del context.Context de cada registro.
package logger

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"rdl/portal-gateway/pkg/correlation"
)

// ContextAttrs extrae atributos del contexto. Permite que capas externas (tenancy) aporten campos
// sin que este paquete dependa de ellas.
type ContextAttrs func(ctx context.Context) []slog.Attr

func New(w io.Writer, level, service, env string, extra ...ContextAttrs) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(level))
	base := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	extractors := append([]ContextAttrs{correlationAttrs, traceAttrs}, extra...)
	return slog.New(&contextHandler{Handler: base, extractors: extractors}).
		With(slog.String("service", service), slog.String("env", env))
}

type contextHandler struct {
	slog.Handler
	extractors []ContextAttrs
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, extract := range h.extractors {
		r.AddAttrs(extract(ctx)...)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithAttrs(attrs), extractors: h.extractors}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{Handler: h.Handler.WithGroup(name), extractors: h.extractors}
}

func correlationAttrs(ctx context.Context) []slog.Attr {
	if id, ok := correlation.FromContext(ctx); ok {
		return []slog.Attr{slog.String("correlationId", id.String())}
	}
	return nil
}

func traceAttrs(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String())}
}
