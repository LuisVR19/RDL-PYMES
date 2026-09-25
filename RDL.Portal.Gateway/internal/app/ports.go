// Package app son los casos de uso del Portal Gateway: qué API llamar, cómo juntar y cómo degradar.
// No hay reglas de negocio: ningún `if` sobre un estado, un rol o un monto decide nada aquí.
package app

import (
	"context"
	"errors"

	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/domain/view"
)

// Errores que los adapters traducen desde la respuesta de cada API. Son los únicos que los casos de uso
// interpretan: cualquier otro se trata como "no se pudo consultar".
var (
	// ErrNotFound: la API dueña respondió 404. Para una parte secundaria significa que todavía no existe
	// (un borrador no tiene documento electrónico ni cuenta por cobrar), no que algo falló.
	ErrNotFound = errors.New("app: la API dueña respondió que no existe")
	// ErrUnavailable: no se pudo consultar (caída, timeout, 5xx, respuesta ilegible).
	ErrUnavailable = errors.New("app: la API dueña no se pudo consultar")
	// ErrTimeout acompaña a ErrUnavailable cuando la causa fue el reloj: el paso directo lo traduce a 504
	// en lugar de 502. Para la degradación de una composición da lo mismo: la parte no se pudo consultar.
	ErrTimeout = errors.New("app: la API dueña no respondió a tiempo")
	// ErrNotConfigured: el servicio todavía no tiene URL en este ambiente (E-Invoice y Receivables hoy).
	ErrNotConfigured = errors.New("app: el servicio no está configurado en este ambiente")
	// ErrUnauthenticated: no hay token del usuario en el contexto. Nunca se sustituye por uno de servicio.
	ErrUnauthenticated = errors.New("app: sin token del usuario")
)

// Puertos, definidos del lado del consumidor. Cada uno es lo mínimo que una vista necesita de una API;
// los implementa `internal/adapters/downstream` y los casos de uso se prueban con fakes.

// InvoiceSummaryReader lee de Billing el resumen de una factura o nota: es la fuente PRINCIPAL de la vista.
type InvoiceSummaryReader interface {
	InvoiceSummary(ctx context.Context, invoiceID string) (view.InvoiceSummary, error)
}

// FiscalStatusReader lee de E-Invoice el estado del documento electrónico de esa factura.
type FiscalStatusReader interface {
	FiscalStatusBySource(ctx context.Context, sourceDocumentID string) (view.FiscalStatus, error)
}

// BalanceReader lee de Receivables el saldo de la cuenta por cobrar de esa factura.
type BalanceReader interface {
	BalanceByInvoice(ctx context.Context, invoiceID string) (view.Balance, error)
}

// availabilityOf traduce el resultado de una parte SECUNDARIA a su disponibilidad. Es la regla de degradación
// del criterio 2: que una API secundaria falle no puede tumbar la vista entera.
func availabilityOf(err error) view.Availability {
	switch {
	case err == nil:
		return view.Available
	case errors.Is(err, ErrNotFound):
		return view.Absent
	default:
		return view.Unavailable
	}
}

// degradedService nombra al servicio que no respondió, para el log y las métricas.
type degradedService struct {
	Service routes.Service
	Err     error
}
