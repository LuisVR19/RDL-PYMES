package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/domain/numbering"
)

type sequences struct{ q *db.Queries }

func (r sequences) Lock(ctx context.Context, org uuid.UUID, documentType string) error {
	if err := r.q.LockSequences(ctx, db.LockSequencesParams{OrganizationID: org.String(), DocumentType: documentType}); err != nil {
		return fmt.Errorf("bloqueando secuencias: %w", err)
	}
	return nil
}

func (r sequences) List(ctx context.Context, org uuid.UUID) ([]numbering.Sequence, error) {
	rows, err := r.q.ListSequences(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("listando secuencias: %w", err)
	}
	out := make([]numbering.Sequence, 0, len(rows))
	for _, row := range rows {
		out = append(out, toSequence(db.GetSequenceForUpdateRow(row)))
	}
	return out, nil
}

// GetForUpdate bloquea la secuencia del alcance. found = false si todavía no existe.
func (r sequences) GetForUpdate(ctx context.Context, org uuid.UUID, scope numbering.Scope) (numbering.Sequence, bool, error) {
	row, err := r.q.GetSequenceForUpdate(ctx, db.GetSequenceForUpdateParams{
		OrganizationID: org, DocumentType: scope.DocumentType, BranchID: optUUID(scope.BranchID),
	})
	if isNoRows(err) {
		return numbering.Sequence{}, false, nil
	}
	if err != nil {
		return numbering.Sequence{}, false, fmt.Errorf("leyendo secuencia: %w", err)
	}
	return toSequence(row), true, nil
}

// Save inserta la secuencia si es nueva (ID nulo) o la actualiza.
func (r sequences) Save(ctx context.Context, s numbering.Sequence) (numbering.Sequence, error) {
	if s.ID == uuid.Nil {
		row, err := r.q.InsertSequence(ctx, db.InsertSequenceParams{
			OrganizationID: s.OrganizationID, DocumentType: s.DocumentType, BranchID: optUUID(s.BranchID),
			Prefix: s.Prefix, NextNumber: s.NextNumber,
		})
		if err != nil {
			return numbering.Sequence{}, fmt.Errorf("creando secuencia: %w", err)
		}
		return toSequence(db.GetSequenceForUpdateRow(row)), nil
	}
	last := pgtype.Int8{}
	if s.LastAssigned != nil {
		last = pgtype.Int8{Int64: *s.LastAssigned, Valid: true}
	}
	row, err := r.q.UpdateSequence(ctx, db.UpdateSequenceParams{
		OrganizationID: s.OrganizationID, ID: s.ID, Prefix: s.Prefix, NextNumber: s.NextNumber, LastAssignedNumber: last,
	})
	if err != nil {
		return numbering.Sequence{}, fmt.Errorf("actualizando secuencia: %w", err)
	}
	return toSequence(db.GetSequenceForUpdateRow(row)), nil
}

func toSequence(r db.GetSequenceForUpdateRow) numbering.Sequence {
	s := numbering.Sequence{
		ID: r.ID, OrganizationID: r.OrganizationID,
		Scope:  numbering.Scope{DocumentType: r.DocumentType, BranchID: ptrUUID(r.BranchID)},
		Prefix: r.Prefix, NextNumber: r.NextNumber, UpdatedAt: r.UpdatedAt,
	}
	if r.LastAssignedNumber.Valid {
		n := r.LastAssignedNumber.Int64
		s.LastAssigned = &n
	}
	return s
}
