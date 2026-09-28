package app

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
	"rdl/receivables-api/pkg/tenancy"
)

// PaymentInput es POST /v1/payments ya parseado (montos en decimal, fechas de negocio).
type PaymentInput struct {
	CustomerID        uuid.UUID
	ReceivedOn        civil.Date
	Amount            decimal.Decimal
	Currency          string
	ExchangeRate      decimal.Decimal // cero = 1
	PaymentMethodCode string
	Reference         string
	Notes             string
	Applications      []ApplicationInput
}

type ApplicationInput struct {
	ReceivableID uuid.UUID
	Amount       decimal.Decimal
}

// CommandResult es la respuesta de un comando idempotente: el recurso releído y si fue una repetición.
type CommandResult[T any] struct {
	Value    T
	Replayed bool
}

// CreatePayment registra un pago (posted) y, opcionalmente, sus aplicaciones (owner, admin, collector). Emite
// PaymentReceived y un ReceivableSettled por cada cuenta que llegue a cero, todo en la misma transacción.
type CreatePayment struct {
	tx    TxManager
	now   func() time.Time
	newID func() uuid.UUID
}

func NewCreatePayment(tx TxManager) *CreatePayment {
	return &CreatePayment{tx: tx, now: utcNow, newID: uuid.New}
}

func (uc *CreatePayment) Execute(ctx context.Context, t tenancy.Context, key string, in PaymentInput) (CommandResult[PaymentView], error) {
	if err := authorize(t, permission.PaymentsWrite); err != nil {
		return CommandResult[PaymentView]{}, err
	}
	if in.ExchangeRate.IsZero() {
		in.ExchangeRate = decimal.New(1, 0)
	}
	if !in.ExchangeRate.IsPositive() {
		return CommandResult[PaymentView]{}, ValidationError{Field: "exchangeRate", Message: "debe ser mayor que cero"}
	}
	seen := map[uuid.UUID]bool{}
	for _, a := range in.Applications {
		if seen[a.ReceivableID] {
			return CommandResult[PaymentView]{}, ValidationError{Field: "applications",
				Message: "una cuenta aparece dos veces: sume los montos en una sola aplicación"}
		}
		seen[a.ReceivableID] = true
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()

	var res CommandResult[PaymentView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		// receivedOn es una fecha de negocio: se compara con el "hoy" de la organización, no con el de UTC.
		day, err := today(ctx, tx.Organizations(), org, now)
		if err != nil {
			return err
		}
		if day.Before(in.ReceivedOn) {
			return ValidationError{Field: "receivedOn", Message: "no puede ser posterior a hoy (" + day.String() + ")"}
		}
		replayed, err := idempotent(ctx, tx, org, key, in,
			func(r map[string]string) error {
				id, err := resultID(r, "paymentId")
				if err != nil {
					return err
				}
				res.Value, err = tx.Payments().Get(ctx, org, id)
				return err
			},
			func() (int, map[string]string, error) {
				id, err := uc.create(ctx, tx, t, in, cid, now)
				if err != nil {
					return 0, nil, err
				}
				res.Value, err = tx.Payments().Get(ctx, org, id)
				return http.StatusCreated, map[string]string{"paymentId": id.String()}, err
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

func (uc *CreatePayment) create(ctx context.Context, tx Tx, t tenancy.Context, in PaymentInput, cid uuid.UUID, now time.Time) (uuid.UUID, error) {
	org := t.OrganizationID()
	p, err := payment.New(payment.Data{ID: uc.newID(), CustomerID: in.CustomerID, Currency: in.Currency, Amount: in.Amount})
	if err != nil {
		return uuid.Nil, err
	}
	l := tx.Ledger()
	if err := l.InsertPayment(ctx, org, NewPayment{
		Payment: p, ReceivedOn: in.ReceivedOn, ExchangeRate: in.ExchangeRate, PaymentMethodCode: in.PaymentMethodCode,
		Reference: in.Reference, Notes: in.Notes, ReceivedBy: t.UserID(),
	}); err != nil {
		return uuid.Nil, err
	}
	// El pago es nuevo: nadie más lo ve, así que solo hace falta bloquear las cuentas (en orden de id).
	ids := make([]uuid.UUID, 0, len(in.Applications))
	for _, a := range in.Applications {
		ids = append(ids, a.ReceivableID)
	}
	lk, err := lockAll(ctx, l, org, nil, ids)
	if err != nil {
		return uuid.Nil, err
	}

	changes := map[uuid.UUID]receivable.Change{}
	event := PaymentReceivedEvent{
		EventMeta: EventMeta{OrganizationID: org, CorrelationID: cid, OccurredAt: now},
		PaymentID: p.ID(), CustomerID: in.CustomerID, ReceivedOn: in.ReceivedOn, Amount: in.Amount,
		Currency: in.Currency, ExchangeRate: in.ExchangeRate, PaymentMethodCode: in.PaymentMethodCode,
		Reference: in.Reference, ReceivedByUserID: t.UserID(),
	}
	audited := make([]map[string]any, 0, len(in.Applications))
	for _, a := range in.Applications {
		r := lk.receivables[a.ReceivableID]
		appID := uc.newID()
		change, err := settlement.Apply(p, r, appID, a.Amount, now)
		if err != nil {
			return uuid.Nil, err
		}
		if err := l.InsertApplication(ctx, org, NewApplication{
			ID: appID, PaymentID: p.ID(), ReceivableID: r.ID(), Amount: a.Amount, AppliedAt: now, AppliedBy: t.UserID(),
		}); err != nil {
			return uuid.Nil, err
		}
		changes[r.ID()] = change
		event.Applications = append(event.Applications, PaymentReceivedApplication{
			ApplicationID: appID, ReceivableID: r.ID(), SourceInvoiceID: r.Invoice().ID, Amount: a.Amount,
		})
		audited = append(audited, map[string]any{
			"applicationId": appID, "receivableId": r.ID(), "amount": a.Amount.String(),
			"status": change.To, "balance": r.Balance().String(),
		})
	}
	for _, id := range sortedUnique(ids) {
		if err := settle(ctx, l, tx.Outbox(), org, lk.receivables[id], lk.meta[id], changes[id], cid, now); err != nil {
			return uuid.Nil, err
		}
	}
	if err := tx.Outbox().PaymentReceived(ctx, event); err != nil {
		return uuid.Nil, err
	}
	return p.ID(), tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
		Action: "payment.created", EntityType: "payment", EntityID: p.ID(),
		Payload: map[string]any{"after": map[string]any{
			"customerId": in.CustomerID, "receivedOn": in.ReceivedOn.String(), "amount": in.Amount.String(),
			"currency": in.Currency, "exchangeRate": in.ExchangeRate.String(), "paymentMethodCode": in.PaymentMethodCode,
			"reference": in.Reference, "applications": audited,
		}},
	})
}

// ListPayments y GetPayment: lectura (owner, admin, collector, accountant, read_only).
type ListPayments struct{ tx TxManager }

func NewListPayments(tx TxManager) *ListPayments { return &ListPayments{tx: tx} }

type PaymentPage struct {
	Items []PaymentView
	Next  *PageCursor
}

func (uc *ListPayments) Execute(ctx context.Context, t tenancy.Context, q PaymentQuery) (PaymentPage, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return PaymentPage{}, err
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageSize
	}
	q.Limit = min(q.Limit, MaxPageSize)
	var page PaymentPage
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		probe := q
		probe.Limit = q.Limit + 1
		items, err := tx.Payments().List(ctx, t.OrganizationID(), probe)
		if err != nil {
			return err
		}
		if len(items) > q.Limit {
			items = items[:q.Limit]
			last := items[len(items)-1]
			page.Next = &PageCursor{At: last.CreatedAt, ID: last.ID}
		}
		page.Items = items
		return nil
	})
	return page, err
}

type GetPayment struct{ tx TxManager }

func NewGetPayment(tx TxManager) *GetPayment { return &GetPayment{tx: tx} }

func (uc *GetPayment) Execute(ctx context.Context, t tenancy.Context, id uuid.UUID) (PaymentView, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return PaymentView{}, err
	}
	var v PaymentView
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		v, err = tx.Payments().Get(ctx, t.OrganizationID(), id)
		return err
	})
	return v, err
}
