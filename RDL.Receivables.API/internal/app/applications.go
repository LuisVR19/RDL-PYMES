package app

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
	"rdl/receivables-api/pkg/tenancy"
)

// ApplyPayment aplica (parte de) un pago ya registrado a una cuenta (owner, admin, collector). Si la cuenta llega a
// cero, emite ReceivableSettled en la misma transacción.
type ApplyPayment struct {
	tx    TxManager
	now   func() time.Time
	newID func() uuid.UUID
}

func NewApplyPayment(tx TxManager) *ApplyPayment {
	return &ApplyPayment{tx: tx, now: utcNow, newID: uuid.New}
}

type ApplyInput struct {
	PaymentID    uuid.UUID
	ReceivableID uuid.UUID
	Amount       decimal.Decimal
}

func (uc *ApplyPayment) Execute(ctx context.Context, t tenancy.Context, key string, in ApplyInput) (CommandResult[ApplicationView], error) {
	if err := authorize(t, permission.PaymentsWrite); err != nil {
		return CommandResult[ApplicationView]{}, err
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()
	var res CommandResult[ApplicationView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, in,
			func(r map[string]string) error { return readApplication(ctx, tx, org, r, &res.Value) },
			func() (int, map[string]string, error) {
				l := tx.Ledger()
				lk, err := lockAll(ctx, l, org, []uuid.UUID{in.PaymentID}, []uuid.UUID{in.ReceivableID})
				if err != nil {
					return 0, nil, err
				}
				p, r := lk.payments[in.PaymentID], lk.receivables[in.ReceivableID]
				id := uc.newID()
				change, err := settlement.Apply(p, r, id, in.Amount, now)
				if err != nil {
					return 0, nil, err
				}
				if err := l.InsertApplication(ctx, org, NewApplication{
					ID: id, PaymentID: p.ID(), ReceivableID: r.ID(), Amount: in.Amount, AppliedAt: now, AppliedBy: t.UserID(),
				}); err != nil {
					return 0, nil, err
				}
				if err := settle(ctx, l, tx.Outbox(), org, r, lk.meta[r.ID()], change, cid, now); err != nil {
					return 0, nil, err
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "payment_application.created", EntityType: "payment_application", EntityID: id,
					Payload: map[string]any{"after": applicationAudit(p, r, id, in.Amount, change)},
				}); err != nil {
					return 0, nil, err
				}
				result := map[string]string{"applicationId": id.String()}
				return http.StatusCreated, result, readApplication(ctx, tx, org, result, &res.Value)
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

// ReversePaymentApplication revierte una aplicación con motivo (owner, admin): el pago recupera el monto y la cuenta
// vuelve exactamente al saldo de antes. No emite eventos (no hay "aplicación revertida" en el catálogo).
type ReversePaymentApplication struct {
	tx  TxManager
	now func() time.Time
}

func NewReversePaymentApplication(tx TxManager) *ReversePaymentApplication {
	return &ReversePaymentApplication{tx: tx, now: utcNow}
}

func (uc *ReversePaymentApplication) Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (CommandResult[ApplicationView], error) {
	if err := authorize(t, permission.PaymentsReverse); err != nil {
		return CommandResult[ApplicationView]{}, err
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()
	request := map[string]string{"applicationId": id.String(), "reason": reason}
	var res CommandResult[ApplicationView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, request,
			func(r map[string]string) error { return readApplication(ctx, tx, org, r, &res.Value) },
			func() (int, map[string]string, error) {
				l := tx.Ledger()
				paymentID, receivableID, err := l.FindApplication(ctx, org, id)
				if err != nil {
					return 0, nil, err
				}
				lk, err := lockAll(ctx, l, org, []uuid.UUID{paymentID}, []uuid.UUID{receivableID})
				if err != nil {
					return 0, nil, err
				}
				p, r := lk.payments[paymentID], lk.receivables[receivableID]
				before := r.Balance()
				change, err := settlement.Reverse(p, r, id, reason, now)
				if err != nil {
					return 0, nil, err
				}
				if err := l.ReverseApplication(ctx, org, id, reason, now, t.UserID()); err != nil {
					return 0, nil, err
				}
				if err := settle(ctx, l, tx.Outbox(), org, r, lk.meta[r.ID()], change, cid, now); err != nil {
					return 0, nil, err
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "payment_application.reversed", EntityType: "payment_application", EntityID: id,
					Payload: map[string]any{
						"reason": reason,
						"before": map[string]any{"receivableBalance": before.String(), "status": change.From},
						"after":  map[string]any{"receivableBalance": r.Balance().String(), "status": change.To, "paymentAvailable": p.Available().String()},
					},
				}); err != nil {
					return 0, nil, err
				}
				result := map[string]string{"applicationId": id.String()}
				return http.StatusOK, result, readApplication(ctx, tx, org, result, &res.Value)
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

// VoidPayment anula un pago con motivo (owner, admin): revierte todas sus aplicaciones vigentes y lo deja voided.
// No emite eventos: el catálogo no tiene "pago anulado" (R7). TODO(contratos): evento de pago anulado.
type VoidPayment struct {
	tx  TxManager
	now func() time.Time
}

func NewVoidPayment(tx TxManager) *VoidPayment { return &VoidPayment{tx: tx, now: utcNow} }

func (uc *VoidPayment) Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, reason string) (CommandResult[PaymentView], error) {
	if err := authorize(t, permission.PaymentsReverse); err != nil {
		return CommandResult[PaymentView]{}, err
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()
	request := map[string]string{"paymentId": id.String(), "reason": reason}
	var res CommandResult[PaymentView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, request,
			func(map[string]string) error {
				var err error
				res.Value, err = tx.Payments().Get(ctx, org, id)
				return err
			},
			func() (int, map[string]string, error) {
				l := tx.Ledger()
				p, err := l.LockPayment(ctx, org, id)
				if err != nil {
					return 0, nil, err
				}
				// El pago ya está bloqueado: sus cuentas se bloquean después, en orden de id.
				lk, err := lockAll(ctx, l, org, nil, p.ActiveReceivableIDs())
				if err != nil {
					return 0, nil, err
				}
				active := activeApplications(p)
				effects, err := settlement.VoidPayment(p, lk.receivables, reason, now)
				if err != nil {
					return 0, nil, err
				}
				for _, a := range active {
					if err := l.ReverseApplication(ctx, org, a.ID, reason, now, t.UserID()); err != nil {
						return 0, nil, err
					}
				}
				if err := l.VoidPayment(ctx, org, id, reason, now); err != nil {
					return 0, nil, err
				}
				reversed := make([]map[string]any, 0, len(effects))
				for i, e := range effects {
					r := lk.receivables[e.ReceivableID]
					if err := settle(ctx, l, tx.Outbox(), org, r, lk.meta[r.ID()], e.Change, cid, now); err != nil {
						return 0, nil, err
					}
					reversed = append(reversed, map[string]any{
						"applicationId": active[i].ID, "receivableId": r.ID(), "amount": active[i].Amount.String(),
						"receivableStatus": e.Change.To, "receivableBalance": r.Balance().String(),
					})
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "payment.voided", EntityType: "payment", EntityID: id,
					Payload: map[string]any{"reason": reason, "reversedApplications": reversed},
				}); err != nil {
					return 0, nil, err
				}
				res.Value, err = tx.Payments().Get(ctx, org, id)
				return http.StatusOK, map[string]string{"paymentId": id.String()}, err
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

// activeApplications en el mismo orden en que settlement.VoidPayment las revierte (por cuenta).
func activeApplications(p *payment.Payment) []payment.Application {
	var out []payment.Application
	for _, a := range p.Applications() {
		if a.Active() {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b payment.Application) int {
		return cmp.Compare(a.ReceivableID.String(), b.ReceivableID.String())
	})
	return out
}

func readApplication(ctx context.Context, tx Tx, org uuid.UUID, result map[string]string, dst *ApplicationView) error {
	id, err := resultID(result, "applicationId")
	if err != nil {
		return err
	}
	*dst, err = tx.Payments().GetApplication(ctx, org, id)
	return err
}

func applicationAudit(p *payment.Payment, r *receivable.Receivable, id uuid.UUID, amt decimal.Decimal, change receivable.Change) map[string]any {
	return map[string]any{
		"applicationId": id, "paymentId": p.ID(), "receivableId": r.ID(), "amount": amt.String(),
		"paymentAvailable": p.Available().String(), "receivableBalance": r.Balance().String(),
		"receivableStatus": change.To,
	}
}
