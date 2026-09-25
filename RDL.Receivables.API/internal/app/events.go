package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
)

// Consumo de eventos (docs/PLAN.md §2.4, ADR 0005). Un mensaje pasa por HandleEvent una vez por intento; ProcessEvent
// decide los reintentos y la dead letter. El tenant de un evento sale SIEMPRE de su organizationId.

var (
	// ErrInvalidEvent: el payload no cumple su schema o no se puede leer. Permanente: dead letter sin reintentos.
	ErrInvalidEvent = errors.New("evento inválido")
	// ErrUnsupportedEvent: Receivables no consume ese tipo o versión. Permanente.
	ErrUnsupportedEvent = errors.New("evento sin consumidor en receivables")
	// ErrRejectedEvent: el evento es válido pero una regla de negocio impide aplicarlo (cuenta anulada, montos que
	// no cuadran, documento repetido con otro monto). Permanente.
	ErrRejectedEvent = errors.New("el evento no se puede aplicar")
)

// Todo error que no envuelve uno de estos es transitorio (base caída, serialización, deadlock, "la factura todavía
// no existe") y se reintenta.
func permanent(err error) bool {
	return errors.Is(err, ErrInvalidEvent) || errors.Is(err, ErrUnsupportedEvent) || errors.Is(err, ErrRejectedEvent)
}

// reject marca un error de dominio como permanente sin perder el original (errors.Is sigue funcionando).
func reject(err error) error { return fmt.Errorf("%w: %w", ErrRejectedEvent, err) }

// EventRef identifica un evento entrante: es lo que va al inbox, a la auditoría y a la dead letter.
type EventRef struct {
	ID             uuid.UUID
	Type           string
	Version        int
	OrganizationID uuid.UUID
	CorrelationID  uuid.UUID
	OccurredAt     time.Time
}

// IncomingEvent es un evento ya validado contra su schema y traducido a un comando (por ejemplo InvoiceIssued).
type IncomingEvent struct {
	Ref  EventRef
	Body any
}

// EventDecoder valida el payload contra los schemas de contratos y lo traduce a un comando.
type EventDecoder interface {
	Decode(payload []byte) (IncomingEvent, error)
	// Peek lee lo que pueda del sobre aunque el payload sea inválido, para la dead letter y los logs.
	Peek(payload []byte) EventRef
}

// EventTxManager abre la transacción de un evento con app.current_organization_id = organizationId del evento y
// sin usuario. Inbox, efecto y auditoría ocurren en esa transacción.
type EventTxManager interface {
	WithinServiceTx(ctx context.Context, organizationID uuid.UUID, fn func(context.Context, EventTx) error) error
}

type EventTx interface {
	Inbox() Inbox
	ReceivableStore() ReceivableStore
	Ledger() Ledger
	Audit() AuditRecorder
	Outbox() Outbox
}

// Inbox es integration.inbox_messages con consumer_service = 'receivables'.
type Inbox interface {
	// Claim registra el evento; false si ya estaba (duplicado: el efecto ya se aplicó en otra transacción).
	Claim(ctx context.Context, ref EventRef) (bool, error)
	MarkProcessed(ctx context.Context, ref EventRef) error
}

// DeadLetterStore escribe en integration.dead_letters en su propia transacción (la del efecto ya hizo rollback).
type DeadLetterStore interface {
	Record(ctx context.Context, d DeadLetter) error
}

type DeadLetter struct {
	Ref      EventRef
	Payload  []byte
	Reason   string
	Attempts int
}

// AuditRecorder escribe en audit.audit_events dentro de la transacción del cambio.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEvent) error
}

const (
	ActorUser    = "user"
	ActorService = "service"
)

type AuditEvent struct {
	OrganizationID uuid.UUID
	ActorType      string
	ActorUserID    uuid.UUID // uuid.Nil si el actor es el servicio
	Action         string
	EntityType     string
	EntityID       uuid.UUID
	CorrelationID  uuid.UUID
	Payload        map[string]any
}

// EventHandler aplica un tipo de evento. Un tipo nuevo se agrega registrando otro handler, sin tocar los existentes.
type EventHandler interface {
	Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error
}

type Outcome string

const (
	OutcomeProcessed    Outcome = "processed"
	OutcomeDuplicate    Outcome = "duplicate"
	OutcomeDeadLettered Outcome = "dead_lettered"
)

// HandleEvent es un intento de procesar un mensaje: decodifica, abre la transacción del tenant del evento, reclama
// el inbox (un duplicado termina aquí) y aplica el efecto con su auditoría.
type HandleEvent struct {
	decoder  EventDecoder
	tx       EventTxManager
	handlers map[string]EventHandler
}

func NewHandleEvent(decoder EventDecoder, tx EventTxManager, handlers map[string]EventHandler) *HandleEvent {
	return &HandleEvent{decoder: decoder, tx: tx, handlers: handlers}
}

func (uc *HandleEvent) Handle(ctx context.Context, payload []byte) (Outcome, error) {
	ev, err := uc.decoder.Decode(payload)
	if err != nil {
		return "", err
	}
	h, ok := uc.handlers[ev.Ref.Type]
	if !ok {
		return "", fmt.Errorf("%w: %s v%d", ErrUnsupportedEvent, ev.Ref.Type, ev.Ref.Version)
	}
	if ev.Ref.OrganizationID == uuid.Nil {
		return "", fmt.Errorf("%w: sin organizationId", ErrInvalidEvent)
	}
	outcome := OutcomeProcessed
	err = uc.tx.WithinServiceTx(ctx, ev.Ref.OrganizationID, func(ctx context.Context, tx EventTx) error {
		claimed, err := tx.Inbox().Claim(ctx, ev.Ref)
		if err != nil {
			return err
		}
		if !claimed {
			outcome = OutcomeDuplicate
			return nil
		}
		if err := h.Handle(ctx, tx, ev); err != nil {
			return err
		}
		return tx.Inbox().MarkProcessed(ctx, ev.Ref)
	})
	if err != nil {
		return "", err
	}
	return outcome, nil
}

// RetryPolicy: intentos totales y backoff exponencial con jitter (docs/PLAN.md §2.4: 6 intentos, de 1 s a 32 s).
type RetryPolicy struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
	// Jitter recibe la espera nominal y devuelve la real; nil = sin jitter.
	Jitter func(time.Duration) time.Duration
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{Attempts: 6, Base: time.Second, Max: 32 * time.Second, Jitter: EqualJitter}
}

// EqualJitter espera entre la mitad y el total de d, para que los consumidores no reintenten todos a la vez.
func EqualJitter(d time.Duration) time.Duration {
	half := d / 2
	return half + rand.N(half+1) //nolint:gosec // G404: el jitter no necesita azar criptográfico
}

func (p RetryPolicy) delay(attempt int) time.Duration {
	d := p.Base << (attempt - 1) // #nosec G115 -- attempt está acotado por Attempts
	if d > p.Max || d <= 0 {
		d = p.Max
	}
	if p.Jitter != nil {
		d = p.Jitter(d)
	}
	return d
}

// Result es lo que pasó con un mensaje.
type Result struct {
	Ref      EventRef
	Outcome  Outcome
	Attempts int
	// Err es la causa de la dead letter (informativa: el mensaje igual se da por consumido).
	Err error
}

// ProcessEvent reintenta los errores transitorios y manda a dead letter los permanentes y los que agotan los
// intentos. Devuelve error solo si el mensaje NO debe darse por consumido: el contexto se canceló antes de terminar
// o no se pudo escribir la dead letter. En ese caso la fuente lo vuelve a entregar y el inbox evita el efecto doble.
type ProcessEvent struct {
	handle  *HandleEvent
	decoder EventDecoder
	dead    DeadLetterStore
	policy  RetryPolicy
	sleep   func(context.Context, time.Duration) error
}

func NewProcessEvent(handle *HandleEvent, decoder EventDecoder, dead DeadLetterStore, policy RetryPolicy) *ProcessEvent {
	return &ProcessEvent{handle: handle, decoder: decoder, dead: dead, policy: policy, sleep: sleepCtx}
}

func (p *ProcessEvent) Process(ctx context.Context, payload []byte) (Result, error) {
	res := Result{Ref: p.decoder.Peek(payload)}
	attempts := max(p.policy.Attempts, 1)
	var lastErr error
	for res.Attempts = 1; ; res.Attempts++ {
		outcome, err := p.handle.Handle(ctx, payload)
		if err == nil {
			res.Outcome = outcome
			return res, nil
		}
		lastErr = err
		if permanent(err) || res.Attempts >= attempts {
			break
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		if err := p.sleep(ctx, p.policy.delay(res.Attempts)); err != nil {
			return res, err
		}
	}
	res.Outcome, res.Err = OutcomeDeadLettered, lastErr
	d := DeadLetter{Ref: res.Ref, Payload: payload, Reason: lastErr.Error(), Attempts: res.Attempts}
	if err := p.dead.Record(context.WithoutCancel(ctx), d); err != nil {
		return res, fmt.Errorf("escribiendo dead letter: %w (causa: %w)", err, lastErr)
	}
	return res, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
