package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/receivables-api/internal/adapters/postgres/db"
	"rdl/receivables-api/internal/app"
)

// WithinServiceTx es la transacción del consumidor: solo app.current_organization_id (el organizationId del
// evento), sin usuario. Inbox, efecto y auditoría se confirman o se descartan juntos.
func (m *TxManager) WithinServiceTx(ctx context.Context, org uuid.UUID, fn func(context.Context, app.EventTx) error) error {
	if org == uuid.Nil {
		return errors.New("postgres: evento sin organización")
	}
	return inTx(ctx, m.pool, pgx.TxOptions{}, session{organizationID: org}, func(q *db.Queries, _ pgx.Tx) error {
		return fn(ctx, eventTx{q: q, validator: m.validator})
	})
}

type eventTx struct {
	q         *db.Queries
	validator *contracts.Validator
}

func (t eventTx) Inbox() app.Inbox                     { return inbox{q: t.q} }
func (t eventTx) ReceivableStore() app.ReceivableStore { return receivableStore{q: t.q} }
func (t eventTx) Ledger() app.Ledger                   { return ledger{q: t.q} }
func (t eventTx) Audit() app.AuditRecorder             { return audit{q: t.q} }
func (t eventTx) Outbox() app.Outbox                   { return outbox(t) }

type inbox struct{ q *db.Queries }

func (i inbox) Claim(ctx context.Context, ref app.EventRef) (bool, error) {
	n, err := i.q.ClaimInboxMessage(ctx, db.ClaimInboxMessageParams{
		EventID: ref.ID, EventType: ref.Type, OrganizationID: ref.OrganizationID,
	})
	if err != nil {
		return false, fmt.Errorf("registrando el evento en el inbox: %w", err)
	}
	return n == 1, nil
}

func (i inbox) MarkProcessed(ctx context.Context, ref app.EventRef) error {
	if err := i.q.MarkInboxProcessed(ctx, ref.ID); err != nil {
		return fmt.Errorf("marcando el evento como procesado: %w", err)
	}
	return nil
}

// DeadLetters escribe integration.dead_letters en una transacción propia: la del efecto ya hizo rollback.
type DeadLetters struct{ pool *pgxpool.Pool }

func NewDeadLetters(pool *pgxpool.Pool) *DeadLetters { return &DeadLetters{pool: pool} }

func (d *DeadLetters) Record(ctx context.Context, dl app.DeadLetter) error {
	// La política dead_letters_service filtra por servicio; la organización se fija igual si se conoce.
	s := session{organizationID: dl.Ref.OrganizationID}
	return inTx(ctx, d.pool, pgx.TxOptions{}, s, func(q *db.Queries, _ pgx.Tx) error {
		return q.InsertDeadLetter(ctx, db.InsertDeadLetterParams{
			EventID:        dl.Ref.ID,
			EventType:      dl.Ref.Type,
			OrganizationID: nullUUID(dl.Ref.OrganizationID),
			Payload:        jsonPayload(dl.Payload),
			ErrorMessage:   dl.Reason,
			Attempts:       int32(min(dl.Attempts, 1<<30)), // #nosec G115 -- acotado arriba
		})
	})
}

// jsonPayload: payload es jsonb NOT NULL. Un mensaje que ni siquiera es JSON se guarda como string dentro de un
// objeto, para no perderlo.
func jsonPayload(raw []byte) string {
	if json.Valid(raw) {
		return string(raw)
	}
	wrapped, _ := json.Marshal(map[string]string{"unparseable": string(raw)})
	return string(wrapped)
}

func nullUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: id != uuid.Nil}
}
