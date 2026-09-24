// Package correlation propaga el X-Correlation-Id entre HTTP, logs, trazas y audit events.
// Es infraestructura reutilizable (candidata a building-blocks): no conoce el dominio.
package correlation

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const Header = "X-Correlation-Id"

type ctxKey struct{}

// FromContext devuelve el correlation id del request. Nunca vacío si pasó por Middleware.
func FromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}

func WithID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Middleware lee X-Correlation-Id o genera uno. Solo acepta UUID porque audit.audit_events.correlation_id es uuid:
// un valor arbitrario del cliente se reemplaza en lugar de propagarse.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.Header.Get(Header))
		if err != nil || id == uuid.Nil {
			id = uuid.New()
		}
		w.Header().Set(Header, id.String())
		trace.SpanFromContext(r.Context()).SetAttributes(attribute.String("correlation_id", id.String()))
		next.ServeHTTP(w, r.WithContext(WithID(r.Context(), id)))
	})
}
