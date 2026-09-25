package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
)

// Ledger carga, bloquea y persiste los agregados (ADR 0006). Nunca escribe saldo ni estado: los recalcula la base
// con sus triggers, y el caso de uso compara el resultado con el agregado (verify).
//
// Orden de bloqueo: primero los pagos (por id), después las cuentas (por id), el mismo que usan los triggers.
// Lo respeta lockAll; nadie llama a LockPayment después de LockReceivable en la misma transacción.
type Ledger interface {
	// LockPayment bloquea el pago (FOR UPDATE) y lo carga con todas sus aplicaciones. ErrNotFound si no existe en
	// la organización.
	LockPayment(ctx context.Context, organizationID, id uuid.UUID) (*payment.Payment, error)
	// LockReceivable bloquea la cuenta, la carga con sus ajustes y aplicaciones, y verifica que el saldo y el estado
	// derivados por el agregado coincidan con los de la base. ErrNotFound si no existe en la organización.
	LockReceivable(ctx context.Context, organizationID, id uuid.UUID) (*receivable.Receivable, ReceivableMeta, error)
	// ActivePaymentIDs son los pagos con aplicaciones vigentes en la cuenta, sin bloquear y ordenados por id: se
	// bloquean antes que la cuenta.
	ActivePaymentIDs(ctx context.Context, organizationID, receivableID uuid.UUID) ([]uuid.UUID, error)
	// FindApplication devuelve a qué pago y cuenta pertenece una aplicación. ErrNotFound si no existe.
	FindApplication(ctx context.Context, organizationID, id uuid.UUID) (paymentID, receivableID uuid.UUID, err error)

	InsertPayment(ctx context.Context, organizationID uuid.UUID, p NewPayment) error
	InsertApplication(ctx context.Context, organizationID uuid.UUID, a NewApplication) error
	// ReverseApplication marca la aplicación como revertida (nunca la borra). by = uuid.Nil si la revierte el
	// servicio (nota de crédito, anulación de factura).
	ReverseApplication(ctx context.Context, organizationID, id uuid.UUID, reason string, at time.Time, by uuid.UUID) error
	VoidPayment(ctx context.Context, organizationID, id uuid.UUID, reason string, at time.Time) error
	InsertAdjustment(ctx context.Context, organizationID, receivableID uuid.UUID, adj receivable.Adjustment, reason string) error
	// ReceivableState es lo que dejaron los triggers después de escribir.
	ReceivableState(ctx context.Context, organizationID, id uuid.UUID) (ReceivableState, error)
}

// ReceivableMeta son los datos de la cuenta que no están en el agregado y hacen falta para ReceivableSettled.
type ReceivableMeta struct {
	DocumentNumber string
}

type ReceivableState struct {
	Balance   decimal.Decimal
	Status    receivable.Status
	SettledAt *time.Time
}

// NewPayment es el pago recién registrado más los datos que no usa el agregado.
type NewPayment struct {
	Payment           *payment.Payment
	ReceivedOn        civil.Date
	ExchangeRate      decimal.Decimal
	PaymentMethodCode string
	Reference         string
	Notes             string
	ReceivedBy        uuid.UUID
}

type NewApplication struct {
	ID           uuid.UUID
	PaymentID    uuid.UUID
	ReceivableID uuid.UUID
	Amount       decimal.Decimal
	AppliedAt    time.Time
	AppliedBy    uuid.UUID
}

// Outbox escribe en integration.outbox_messages, en la misma transacción que el cambio. El adapter serializa con
// los DTOs de contratos y valida contra el schema antes de insertar.
type Outbox interface {
	PaymentReceived(ctx context.Context, e PaymentReceivedEvent) error
	ReceivableSettled(ctx context.Context, e ReceivableSettledEvent) error
}

// EventMeta es el sobre de un evento que produce Receivables.
type EventMeta struct {
	OrganizationID uuid.UUID
	CorrelationID  uuid.UUID
	OccurredAt     time.Time
}

type PaymentReceivedEvent struct {
	EventMeta
	PaymentID         uuid.UUID
	CustomerID        uuid.UUID
	ReceivedOn        civil.Date
	Amount            decimal.Decimal
	Currency          string
	ExchangeRate      decimal.Decimal
	PaymentMethodCode string
	Reference         string
	ReceivedByUserID  uuid.UUID
	Applications      []PaymentReceivedApplication
}

type PaymentReceivedApplication struct {
	ApplicationID   uuid.UUID
	ReceivableID    uuid.UUID
	SourceInvoiceID uuid.UUID
	Amount          decimal.Decimal
}

type ReceivableSettledEvent struct {
	EventMeta
	ReceivableID    uuid.UUID
	SourceInvoiceID uuid.UUID
	CustomerID      uuid.UUID
	DocumentNumber  string
	Currency        string
	OriginalAmount  decimal.Decimal
	SettledAt       time.Time
}

// PaymentReader lee pagos para la API. ErrNotFound si no existe en la organización.
type PaymentReader interface {
	Get(ctx context.Context, organizationID, id uuid.UUID) (PaymentView, error)
	List(ctx context.Context, organizationID uuid.UUID, q PaymentQuery) ([]PaymentView, error)
	GetApplication(ctx context.Context, organizationID, id uuid.UUID) (ApplicationView, error)
}

type PaymentQuery struct {
	CustomerID uuid.UUID // uuid.Nil = todos
	After      *PageCursor
	Limit      int
}

// PaymentView: montos como el string decimal del contrato.
type PaymentView struct {
	ID                uuid.UUID
	CustomerID        uuid.UUID
	ReceivedOn        civil.Date
	Amount            string
	Currency          string
	ExchangeRate      string
	PaymentMethodCode string
	Reference         string
	Notes             string
	Status            payment.Status
	VoidReason        string
	VoidedAt          *time.Time
	ReceivedByUserID  uuid.UUID
	CreatedAt         time.Time
	Applications      []ApplicationView
}

type ApplicationView struct {
	ID             uuid.UUID
	PaymentID      uuid.UUID
	ReceivableID   uuid.UUID
	Amount         string
	AppliedAt      time.Time
	ReversedAt     *time.Time
	ReversalReason string
}

// ReceivableDetail es la cuenta con su historial: aplicaciones (vigentes y revertidas) y ajustes.
type ReceivableDetail struct {
	ReceivableView
	Applications []ApplicationView
	Adjustments  []AdjustmentView
}

type AdjustmentView struct {
	ID               uuid.UUID
	Type             receivable.AdjustmentType
	Amount           string
	SourceDocumentID uuid.UUID
	Reason           string
	CreatedAt        time.Time
}

type AgingRow struct {
	Currency string
	DueOn    civil.Date
	Balance  decimal.Decimal
}

// CollectionStore: seguimientos y promesas. ErrNotFound si no existen en la organización.
type CollectionStore interface {
	CreateFollowUp(ctx context.Context, organizationID uuid.UUID, f FollowUpView) error
	GetFollowUp(ctx context.Context, organizationID, id uuid.UUID) (FollowUpView, error)
	ListFollowUps(ctx context.Context, organizationID, receivableID uuid.UUID) ([]FollowUpView, error)
	CreatePromise(ctx context.Context, organizationID uuid.UUID, p PromiseView) error
	GetPromise(ctx context.Context, organizationID, id uuid.UUID) (PromiseView, error)
	LockPromise(ctx context.Context, organizationID, id uuid.UUID) (PromiseView, error)
	ListPromises(ctx context.Context, organizationID, receivableID uuid.UUID) ([]PromiseView, error)
	SetPromiseStatus(ctx context.Context, organizationID, id uuid.UUID, status collection.PromiseStatus) error
}

type FollowUpView struct {
	ID           uuid.UUID
	ReceivableID uuid.UUID
	Type         collection.FollowUpType
	Notes        string
	PerformedAt  time.Time
	PerformedBy  uuid.UUID
	NextActionOn *civil.Date
	CreatedAt    time.Time
}

type PromiseView struct {
	ID             uuid.UUID
	ReceivableID   uuid.UUID
	FollowUpID     uuid.UUID // uuid.Nil = sin seguimiento
	PromisedAmount decimal.Decimal
	PromisedOn     civil.Date
	Status         collection.PromiseStatus
	CreatedBy      uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// IdempotencyStore persiste las Idempotency-Key en integration.idempotency_keys, en la misma transacción que el
// comando: si el comando falla, la reserva desaparece con él. Origen: RDL.Platform.API.
type IdempotencyStore interface {
	// Claim reserva la clave. Si ya existía (y no venció) devuelve el registro previo en lugar de reservar. Una
	// petición concurrente con la misma clave espera a que la primera confirme y luego ve su registro.
	Claim(ctx context.Context, organizationID uuid.UUID, key, requestHash string, ttl time.Duration) (*IdempotencyRecord, error)
	Complete(ctx context.Context, organizationID uuid.UUID, key string, result IdempotencyRecord) error
}

// IdempotencyRecord guarda la referencia al resultado, no la respuesta HTTP: al repetir, el caso de uso relee el
// recurso y responde con su estado actual.
type IdempotencyRecord struct {
	RequestHash string
	Status      int
	Result      map[string]string
}
