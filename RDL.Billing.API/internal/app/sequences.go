package app

import (
	"context"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/pkg/tenancy"
)

// ListSequences devuelve las secuencias configuradas de la organización (owner, admin).
type ListSequences struct{ tx TxManager }

func NewListSequences(tx TxManager) *ListSequences { return &ListSequences{tx: tx} }

func (uc *ListSequences) Execute(ctx context.Context, t tenancy.Context) ([]numbering.Sequence, error) {
	if err := authorize(t, permission.SequencesRead); err != nil {
		return nil, err
	}
	var out []numbering.Sequence
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = tx.Sequences().List(ctx, t.OrganizationID())
		return err
	})
	return out, err
}

// ConfigureSequence fija prefijo y próximo número de la secuencia de un tipo de documento, de la organización o de una
// sucursal (owner, admin). Solo si nunca se usó (409 sequence-in-use) y sin repetir el prefijo de otra secuencia del
// mismo tipo (409 conflict). PUT: idempotente por diseño.
type ConfigureSequence struct{ tx TxManager }

func NewConfigureSequence(tx TxManager) *ConfigureSequence { return &ConfigureSequence{tx: tx} }

type SequenceInput struct {
	DocumentType invoice.DocumentType
	BranchID     *uuid.UUID
	Prefix       string
	NextNumber   int64
}

func (uc *ConfigureSequence) Execute(ctx context.Context, t tenancy.Context, in SequenceInput) (numbering.Sequence, error) {
	if err := authorize(t, permission.SequencesManage); err != nil {
		return numbering.Sequence{}, err
	}
	scope := numbering.Scope{DocumentType: string(in.DocumentType), BranchID: in.BranchID}
	var out numbering.Sequence
	err := uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if in.BranchID != nil {
			if err := (builder{tx: tx, org: t.OrganizationID()}).checkBranch(ctx, in.BranchID); err != nil {
				return err
			}
		}
		// Con el candado, dos configuraciones simultáneas no pueden elegir el mismo prefijo.
		if err := tx.Sequences().Lock(ctx, t.OrganizationID(), scope.DocumentType); err != nil {
			return err
		}
		current, found, err := tx.Sequences().GetForUpdate(ctx, t.OrganizationID(), scope)
		if err != nil {
			return err
		}
		if !found {
			current = numbering.New(t.OrganizationID(), scope)
		}
		siblings, err := tx.Sequences().List(ctx, t.OrganizationID())
		if err != nil {
			return err
		}
		next, err := current.Configure(in.Prefix, in.NextNumber, siblings)
		if err != nil {
			return err
		}
		if found && next.Prefix == current.Prefix && next.NextNumber == current.NextNumber {
			out = current
			return nil
		}
		if out, err = tx.Sequences().Save(ctx, next); err != nil {
			return err
		}
		before := map[string]any{}
		if found {
			before = sequenceAudit(current)
		}
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "sequence.configured", EntityType: "document_sequence", EntityID: out.ID,
			Before: before, After: sequenceAudit(out),
		})
	})
	return out, err
}

func sequenceAudit(s numbering.Sequence) map[string]any {
	branch := ""
	if s.BranchID != nil {
		branch = s.BranchID.String()
	}
	return map[string]any{"documentType": s.DocumentType, "branchId": branch, "prefix": s.Prefix, "nextNumber": s.NextNumber}
}
