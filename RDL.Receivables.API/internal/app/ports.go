// Package app contiene los casos de uso. Orquesta dominio y puertos; no conoce HTTP ni SQL.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/pkg/tenancy"
)

// TxManager abre una transacción por operación con la sesión RLS fijada (app.current_organization_id y
// app.current_user_id). Todo lo que hace fn ocurre en esa transacción.
type TxManager interface {
	WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, Tx) error) error
}

// Tx expone los repositorios dentro de una transacción.
type Tx interface {
	Organizations() OrganizationReader
	Receivables() ReceivableReader
	Payments() PaymentReader
	Ledger() Ledger
	Collection() CollectionStore
	Idempotency() IdempotencyStore
	Audit() AuditRecorder
	Outbox() Outbox
}

// OrganizationReader lee de core lo que Receivables necesita de la organización activa (solo lectura, ADR 0002 §2).
type OrganizationReader interface {
	// Timezone devuelve core.organizations.timezone (nombre IANA) de la organización de la sesión.
	Timezone(ctx context.Context, organizationID uuid.UUID) (string, error)
}

type ReceivableReader interface {
	List(ctx context.Context, organizationID uuid.UUID, q ReceivableQuery) ([]ReceivableView, error)
	// Get y GetByInvoice devuelven ErrNotFound si la cuenta no existe en la organización (o es de otra).
	Get(ctx context.Context, organizationID, id uuid.UUID) (ReceivableDetail, error)
	GetByInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (ReceivableView, error)
	// AgingByDueDate suma el saldo cobrable por moneda y vencimiento; el tramo lo decide internal/domain/aging.
	AgingByDueDate(ctx context.Context, organizationID uuid.UUID, currency string) ([]AgingRow, error)
}

// ReceivableQuery pide una página de cuentas. After es el último elemento de la página anterior.
type ReceivableQuery struct {
	Status     receivable.Status // vacío = todas
	CustomerID uuid.UUID         // uuid.Nil = todos
	Overdue    *bool             // nil = sin filtro
	// Today es la fecha de negocio de la organización; solo se usa con Overdue.
	Today civil.Date
	After *PageCursor
	Limit int
}

// ReceivableView es la cuenta tal como la lista la API. Los montos son el string decimal del contrato
// (RDL.Contracts ADR 0002): la API no calcula con ellos, solo los muestra.
type ReceivableView struct {
	ID                uuid.UUID
	SourceInvoiceID   uuid.UUID
	CustomerID        uuid.UUID
	CustomerLegalName string
	DocumentNumber    string
	Currency          string
	OriginalAmount    string
	BalanceAmount     string
	IssuedOn          civil.Date
	DueOn             civil.Date
	Status            receivable.Status
	SettledAt         *time.Time
	CreatedAt         time.Time
}

// PageCursor es la posición del último elemento de una página: (instante de orden, id) para desempatar.
type PageCursor struct {
	At time.Time
	ID uuid.UUID
}

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)
