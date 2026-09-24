package app

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/numbering"
)

func seqInput(prefix string, next int64, branch *uuid.UUID) SequenceInput {
	return SequenceInput{DocumentType: invoice.TypeInvoice, BranchID: branch, Prefix: prefix, NextNumber: next}
}

func TestConfigureSequenceCreatesAndUpdates(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewConfigureSequence(f)
	tn := tenant(org, "admin")
	s, err := uc.Execute(ctx, tn, seqInput("FAC-", 1, nil))
	if err != nil || s.ID == uuid.Nil || s.Prefix != "FAC-" || s.NextNumber != 1 || s.Used() {
		t.Fatalf("creada: %+v %v", s, err)
	}
	s2, err := uc.Execute(ctx, tn, seqInput("F-", 1000, nil))
	if err != nil || s2.ID != s.ID || s2.Prefix != "F-" || s2.NextNumber != 1000 || len(f.state.sequences) != 1 {
		t.Fatalf("reconfigurada: %+v %v", s2, err)
	}
	if len(f.state.audit) != 2 || f.state.audit[1].Action != "sequence.configured" ||
		f.state.audit[1].Before.(map[string]any)["prefix"] != "FAC-" {
		t.Fatalf("auditoría = %+v", f.state.audit)
	}
	if len(f.state.locks) != 2 || f.state.locks[0] != org.String()+":invoice" {
		t.Fatalf("candado = %v", f.state.locks)
	}
	// El mismo PUT otra vez no cambia nada ni se audita (idempotente por diseño).
	if _, err := uc.Execute(ctx, tn, seqInput("F-", 1000, nil)); err != nil || len(f.state.audit) != 2 {
		t.Fatalf("repetido: %v, auditoría %d", err, len(f.state.audit))
	}
}

func TestConfigureUsedSequenceIsRejected(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "owner")
	s, _ := NewConfigureSequence(f).Execute(ctx, tn, seqInput("FAC-", 1, nil))
	_, used := s.Assign() // lo que hará la emisión
	f.state.sequences[0] = used
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("X-", 1, nil)); !errors.Is(err, numbering.ErrInUse) {
		t.Fatalf("err = %v", err)
	}
	if f.state.sequences[0].Prefix != "FAC-" {
		t.Fatal("la secuencia usada quedó intacta")
	}
}

func TestConfigureSequencePrefixAcrossBranches(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "admin")
	b1, b2 := f.addBranch(org, true), f.addBranch(org, true)
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("S1-", 1, &b1)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("S1-", 1, &b2)); !errors.Is(err, numbering.ErrPrefixTaken) {
		t.Fatalf("prefijo repetido entre sucursales: err = %v", err)
	}
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("S2-", 1, &b2)); err != nil {
		t.Fatalf("prefijo distinto: %v", err)
	}
}

func TestConfigureSequenceBranchIsValidated(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "admin")
	foreign := f.addBranch(uuid.New(), true)
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("A-", 1, &foreign)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sucursal de otra organización: err = %v", err)
	}
	closed := f.addBranch(org, false)
	if _, err := NewConfigureSequence(f).Execute(ctx, tn, seqInput("A-", 1, &closed)); !errors.Is(err, invoice.ErrInvalid) {
		t.Fatalf("sucursal inactiva: err = %v", err)
	}
	if len(f.state.sequences) != 0 {
		t.Fatal("nada quedó escrito")
	}
}

func TestSequenceRoles(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	for _, role := range []string{"biller", "collector", "accountant", "read_only"} {
		if _, err := NewConfigureSequence(f).Execute(ctx, tenant(org, role), seqInput("A-", 1, nil)); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s configura: err = %v", role, err)
		}
		if _, err := NewListSequences(f).Execute(ctx, tenant(org, role)); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s lista: err = %v", role, err)
		}
	}
}

func TestListSequencesOnlyActiveOrganization(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	f := newFakeTx()
	if _, err := NewConfigureSequence(f).Execute(ctx, tenant(a, "owner"), seqInput("A-", 1, nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConfigureSequence(f).Execute(ctx, tenant(b, "owner"), seqInput("A-", 1, nil)); err != nil {
		t.Fatalf("el mismo prefijo en otra organización no choca: %v", err)
	}
	seqs, err := NewListSequences(f).Execute(ctx, tenant(a, "admin"))
	if err != nil || len(seqs) != 1 || seqs[0].OrganizationID != a {
		t.Fatalf("secuencias = %+v err=%v", seqs, err)
	}
}
