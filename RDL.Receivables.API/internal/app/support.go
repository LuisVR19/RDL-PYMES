package app

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/pkg/correlation"
)

var (
	// ErrIdempotencyKeyReused: la misma Idempotency-Key llegó con un cuerpo distinto (422).
	ErrIdempotencyKeyReused = errors.New("la Idempotency-Key ya se usó con otra petición")
	// ErrInconsistentState: el saldo o el estado que dejó la base no es el que calculó el agregado. Nunca debería
	// pasar (la fórmula es la misma, ADR 0001 §4.1): se hace rollback y se responde 500 sin detalle.
	ErrInconsistentState = errors.New("la base y el agregado no coinciden en el saldo de la cuenta")
)

// IdempotencyTTL: una clave se puede reutilizar para otra operación después de 24 h (problems/receivables.yaml).
const IdempotencyTTL = 24 * time.Hour

// ValidationError es un dato de la petición que solo el caso de uso puede juzgar (por ejemplo, una fecha futura
// respecto del día de negocio de la organización). HTTP lo responde como 422 con errors[].
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Message }

// correlationID es el del request (middleware) o, fuera de HTTP, uno nuevo: todo audit y todo evento lo lleva.
func correlationID(ctx context.Context) uuid.UUID {
	if id, ok := correlation.FromContext(ctx); ok {
		return id
	}
	return uuid.New()
}

func requestHash(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("calculando hash de la petición: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// idempotent ejecuta run una sola vez por Idempotency-Key. Si la clave ya se usó con la misma petición, llama a
// replay con el resultado guardado (el recurso se relee y se responde con su estado actual); con otra petición,
// ErrIdempotencyKeyReused. run devuelve el status HTTP y la referencia al resultado.
func idempotent(ctx context.Context, tx Tx, org uuid.UUID, key string, request any,
	replay func(result map[string]string) error,
	run func() (status int, result map[string]string, err error),
) (replayed bool, err error) {
	hash, err := requestHash(request)
	if err != nil {
		return false, err
	}
	prev, err := tx.Idempotency().Claim(ctx, org, key, hash, IdempotencyTTL)
	if err != nil {
		return false, err
	}
	if prev != nil {
		if prev.RequestHash != hash {
			return false, ErrIdempotencyKeyReused
		}
		return true, replay(prev.Result)
	}
	status, result, err := run()
	if err != nil {
		return false, err
	}
	return false, tx.Idempotency().Complete(ctx, org, key, IdempotencyRecord{RequestHash: hash, Status: status, Result: result})
}

func resultID(result map[string]string, key string) (uuid.UUID, error) {
	id, err := uuid.Parse(result[key])
	if err != nil {
		return uuid.Nil, fmt.Errorf("resultado idempotente corrupto (%s): %w", key, err)
	}
	return id, nil
}

// locked son los agregados de una operación, ya bloqueados en orden estable.
type locked struct {
	payments    map[uuid.UUID]*payment.Payment
	receivables map[uuid.UUID]*receivable.Receivable
	meta        map[uuid.UUID]ReceivableMeta
}

// lockAll bloquea primero los pagos y después las cuentas, cada grupo ordenado por id: el mismo orden que los
// triggers (pago → cuenta). Dos operaciones concurrentes sobre las mismas filas esperan en vez de entrar en
// deadlock (ADR 0006).
func lockAll(ctx context.Context, l Ledger, org uuid.UUID, paymentIDs, receivableIDs []uuid.UUID) (locked, error) {
	out := locked{
		payments:    map[uuid.UUID]*payment.Payment{},
		receivables: map[uuid.UUID]*receivable.Receivable{},
		meta:        map[uuid.UUID]ReceivableMeta{},
	}
	for _, id := range sortedUnique(paymentIDs) {
		p, err := l.LockPayment(ctx, org, id)
		if err != nil {
			return locked{}, err
		}
		out.payments[id] = p
	}
	for _, id := range sortedUnique(receivableIDs) {
		r, meta, err := l.LockReceivable(ctx, org, id)
		if err != nil {
			return locked{}, err
		}
		out.receivables[id], out.meta[id] = r, meta
	}
	return out, nil
}

func sortedUnique(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return cmp.Compare(a.String(), b.String()) })
	return slices.Compact(out)
}

// settle compara lo que dejó la base con el agregado y, si la cuenta acaba de pasar a paid, publica
// ReceivableSettled con el settled_at que fijó la base. Se llama después de escribir, en la misma transacción.
func settle(ctx context.Context, l Ledger, out Outbox, org uuid.UUID, r *receivable.Receivable, meta ReceivableMeta,
	change receivable.Change, cid uuid.UUID, now time.Time,
) error {
	state, err := l.ReceivableState(ctx, org, r.ID())
	if err != nil {
		return err
	}
	if !state.Balance.Equal(r.Balance()) || state.Status != r.Status() {
		return fmt.Errorf("%w: cuenta %s, agregado %s %s, base %s %s", ErrInconsistentState, r.ID(),
			r.Balance(), r.Status(), state.Balance, state.Status)
	}
	if !change.Settled() {
		return nil
	}
	settledAt := now
	if state.SettledAt != nil {
		settledAt = *state.SettledAt
	}
	inv := r.Invoice()
	return out.ReceivableSettled(ctx, ReceivableSettledEvent{
		EventMeta:    EventMeta{OrganizationID: org, CorrelationID: cid, OccurredAt: settledAt},
		ReceivableID: r.ID(), SourceInvoiceID: inv.ID, CustomerID: inv.CustomerID, DocumentNumber: meta.DocumentNumber,
		Currency: inv.Currency, OriginalAmount: inv.Original, SettledAt: settledAt,
	})
}

// today es la fecha de negocio de la organización (core.organizations.timezone), nunca la de UTC.
func today(ctx context.Context, orgs OrganizationReader, org uuid.UUID, now time.Time) (civil.Date, error) {
	tz, err := orgs.Timezone(ctx, org)
	if err != nil {
		return civil.Date{}, err
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return civil.Date{}, fmt.Errorf("zona horaria de la organización %q: %w", tz, err)
	}
	return civil.Today(now, loc), nil
}

// utcNow va truncado a microsegundos, la precisión de timestamptz: el instante del agregado es el mismo que se relee.
func utcNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
