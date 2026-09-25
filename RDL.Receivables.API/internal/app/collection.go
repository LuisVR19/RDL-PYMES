package app

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/pkg/tenancy"
)

// Seguimientos y promesas de pago (owner, admin, collector registran; los roles de lectura los ven). Son historial
// auditable: no se editan ni se borran; una promesa solo se cierra (R8).

type FollowUpInput struct {
	Type         collection.FollowUpType
	Notes        string
	NextActionOn *civil.Date
}

type CreateFollowUp struct {
	tx    TxManager
	now   func() time.Time
	newID func() uuid.UUID
}

func NewCreateFollowUp(tx TxManager) *CreateFollowUp {
	return &CreateFollowUp{tx: tx, now: utcNow, newID: uuid.New}
}

func (uc *CreateFollowUp) Execute(ctx context.Context, t tenancy.Context, key string, receivableID uuid.UUID, in FollowUpInput) (CommandResult[FollowUpView], error) {
	if err := authorize(t, permission.PaymentsWrite); err != nil {
		return CommandResult[FollowUpView]{}, err
	}
	if err := collection.ValidateFollowUp(in.Type, in.Notes); err != nil {
		return CommandResult[FollowUpView]{}, err
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()
	request := map[string]any{"receivableId": receivableID, "input": in}
	var res CommandResult[FollowUpView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, request,
			func(r map[string]string) error {
				id, err := resultID(r, "followUpId")
				if err != nil {
					return err
				}
				res.Value, err = tx.Collection().GetFollowUp(ctx, org, id)
				return err
			},
			func() (int, map[string]string, error) {
				// La cuenta tiene que existir en la organización (404 si es de otra).
				if _, err := tx.Receivables().Get(ctx, org, receivableID); err != nil {
					return 0, nil, err
				}
				f := FollowUpView{
					ID: uc.newID(), ReceivableID: receivableID, Type: in.Type, Notes: in.Notes, PerformedAt: now,
					PerformedBy: t.UserID(), NextActionOn: in.NextActionOn,
				}
				if err := tx.Collection().CreateFollowUp(ctx, org, f); err != nil {
					return 0, nil, err
				}
				after := map[string]any{"receivableId": receivableID, "type": in.Type, "notes": in.Notes}
				if in.NextActionOn != nil {
					after["nextActionOn"] = in.NextActionOn.String()
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "collection_followup.created", EntityType: "collection_followup", EntityID: f.ID,
					Payload: map[string]any{"after": after},
				}); err != nil {
					return 0, nil, err
				}
				var err error
				res.Value, err = tx.Collection().GetFollowUp(ctx, org, f.ID)
				return http.StatusCreated, map[string]string{"followUpId": f.ID.String()}, err
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

type ListFollowUps struct{ tx TxManager }

func NewListFollowUps(tx TxManager) *ListFollowUps { return &ListFollowUps{tx: tx} }

func (uc *ListFollowUps) Execute(ctx context.Context, t tenancy.Context, receivableID uuid.UUID) ([]FollowUpView, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return nil, err
	}
	var out []FollowUpView
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if _, err := tx.Receivables().Get(ctx, t.OrganizationID(), receivableID); err != nil {
			return err
		}
		var err error
		out, err = tx.Collection().ListFollowUps(ctx, t.OrganizationID(), receivableID)
		return err
	})
	return out, err
}

type PromiseInput struct {
	Amount     decimal.Decimal
	PromisedOn civil.Date
	FollowUpID uuid.UUID // uuid.Nil = sin seguimiento
}

type CreatePromise struct {
	tx    TxManager
	now   func() time.Time
	newID func() uuid.UUID
}

func NewCreatePromise(tx TxManager) *CreatePromise {
	return &CreatePromise{tx: tx, now: utcNow, newID: uuid.New}
}

func (uc *CreatePromise) Execute(ctx context.Context, t tenancy.Context, key string, receivableID uuid.UUID, in PromiseInput) (CommandResult[PromiseView], error) {
	if err := authorize(t, permission.PaymentsWrite); err != nil {
		return CommandResult[PromiseView]{}, err
	}
	org, cid, now := t.OrganizationID(), correlationID(ctx), uc.now()
	request := map[string]any{"receivableId": receivableID, "input": in}
	var res CommandResult[PromiseView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, request,
			func(r map[string]string) error {
				id, err := resultID(r, "promiseId")
				if err != nil {
					return err
				}
				res.Value, err = tx.Collection().GetPromise(ctx, org, id)
				return err
			},
			func() (int, map[string]string, error) {
				rec, err := tx.Receivables().Get(ctx, org, receivableID)
				if err != nil {
					return 0, nil, err
				}
				if in.FollowUpID != uuid.Nil {
					f, err := tx.Collection().GetFollowUp(ctx, org, in.FollowUpID)
					if err != nil {
						return 0, nil, err
					}
					if f.ReceivableID != receivableID {
						return 0, nil, ValidationError{Field: "followupId", Message: "el seguimiento es de otra cuenta"}
					}
				}
				day, err := today(ctx, tx.Organizations(), org, now)
				if err != nil {
					return 0, nil, err
				}
				balance, err := decimal.NewFromString(rec.BalanceAmount)
				if err != nil {
					return 0, nil, err
				}
				if err := collection.ValidatePromise(rec.Status, balance, in.Amount, in.PromisedOn, day); err != nil {
					return 0, nil, err
				}
				p := PromiseView{
					ID: uc.newID(), ReceivableID: receivableID, FollowUpID: in.FollowUpID, PromisedAmount: in.Amount,
					PromisedOn: in.PromisedOn, Status: collection.PromisePending, CreatedBy: t.UserID(),
				}
				if err := tx.Collection().CreatePromise(ctx, org, p); err != nil {
					return 0, nil, err
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "payment_promise.created", EntityType: "payment_promise", EntityID: p.ID,
					Payload: map[string]any{"after": map[string]any{
						"receivableId": receivableID, "amount": in.Amount.String(), "promisedOn": in.PromisedOn.String(),
						"followUpId": in.FollowUpID, "status": p.Status,
					}},
				}); err != nil {
					return 0, nil, err
				}
				res.Value, err = tx.Collection().GetPromise(ctx, org, p.ID)
				return http.StatusCreated, map[string]string{"promiseId": p.ID.String()}, err
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}

type ListPromises struct{ tx TxManager }

func NewListPromises(tx TxManager) *ListPromises { return &ListPromises{tx: tx} }

func (uc *ListPromises) Execute(ctx context.Context, t tenancy.Context, receivableID uuid.UUID) ([]PromiseView, error) {
	if err := authorize(t, permission.ReceivablesRead); err != nil {
		return nil, err
	}
	var out []PromiseView
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if _, err := tx.Receivables().Get(ctx, t.OrganizationID(), receivableID); err != nil {
			return err
		}
		var err error
		out, err = tx.Collection().ListPromises(ctx, t.OrganizationID(), receivableID)
		return err
	})
	return out, err
}

// ClosePromise cierra una promesa pendiente: kept, broken o cancelled (R8, finales).
type ClosePromise struct{ tx TxManager }

func NewClosePromise(tx TxManager) *ClosePromise { return &ClosePromise{tx: tx} }

func (uc *ClosePromise) Execute(ctx context.Context, t tenancy.Context, key string, id uuid.UUID, to collection.PromiseStatus) (CommandResult[PromiseView], error) {
	if err := authorize(t, permission.PaymentsWrite); err != nil {
		return CommandResult[PromiseView]{}, err
	}
	org, cid := t.OrganizationID(), correlationID(ctx)
	request := map[string]string{"promiseId": id.String(), "status": string(to)}
	var res CommandResult[PromiseView]
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		replayed, err := idempotent(ctx, tx, org, key, request,
			func(map[string]string) error {
				var err error
				res.Value, err = tx.Collection().GetPromise(ctx, org, id)
				return err
			},
			func() (int, map[string]string, error) {
				p, err := tx.Collection().LockPromise(ctx, org, id)
				if err != nil {
					return 0, nil, err
				}
				if err := collection.Transition(p.Status, to); err != nil {
					return 0, nil, err
				}
				if err := tx.Collection().SetPromiseStatus(ctx, org, id, to); err != nil {
					return 0, nil, err
				}
				if err := tx.Audit().Record(ctx, AuditEvent{
					OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), CorrelationID: cid,
					Action: "payment_promise." + string(to), EntityType: "payment_promise", EntityID: id,
					Payload: map[string]any{"before": map[string]any{"status": p.Status}, "after": map[string]any{"status": to}},
				}); err != nil {
					return 0, nil, err
				}
				res.Value, err = tx.Collection().GetPromise(ctx, org, id)
				return http.StatusOK, map[string]string{"promiseId": id.String()}, err
			})
		res.Replayed = replayed
		return err
	})
	return res, err
}
