package app

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/domain/membership"
)

func newBranch(t *testing.T, s *store, org uuid.UUID, code string) branch.Branch {
	t.Helper()
	res, err := NewCreateBranch(s).Execute(t.Context(), tenantAs(org, membership.RoleAdmin), "k-"+code, branch.NewInput{Code: code, Name: "Sucursal " + code})
	if err != nil {
		t.Fatal(err)
	}
	return res.Branch
}

func TestCreateBranchIsIdempotentAndAudited(t *testing.T) {
	s := newStore()
	org := uuid.New()
	uc := NewCreateBranch(s)
	admin := tenantAs(org, membership.RoleAdmin)
	in := branch.NewInput{Code: "SJ", Name: "San José"}

	first, err := uc.Execute(t.Context(), admin, "k1", in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := uc.Execute(t.Context(), admin, "k1", in)
	if err != nil || !again.Replayed || again.Branch.ID != first.Branch.ID || len(s.branches) != 1 {
		t.Fatalf("again=%+v err=%v", again, err)
	}
	if len(s.audit) != 1 || s.audit[0].Action != "branch.created" {
		t.Fatalf("audit=%+v", s.audit)
	}
	in.Name = "Otro nombre"
	if _, err := uc.Execute(t.Context(), admin, "k1", in); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateBranchRules(t *testing.T) {
	s := newStore()
	org := uuid.New()
	newBranch(t, s, org, "SJ")
	uc := NewCreateBranch(s)

	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), "k2", branch.NewInput{Code: "SJ", Name: "Duplicada"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("código duplicado: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleBiller), "k3", branch.NewInput{Code: "X", Name: "X"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("facturador: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), "k4", branch.NewInput{Code: "mal código", Name: "X"}); !errors.Is(err, branch.ErrInvalid) {
		t.Fatalf("código inválido: err=%v", err)
	}
	// El mismo código en otra organización es válido: la unicidad es por organización.
	if _, err := uc.Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), "k5", branch.NewInput{Code: "SJ", Name: "Otra org"}); err != nil {
		t.Fatalf("otra organización: err=%v", err)
	}
}

func TestReadBranchesAnyRoleButOnlyOwnOrganization(t *testing.T) {
	s := newStore()
	org, other := uuid.New(), uuid.New()
	b := newBranch(t, s, org, "SJ")
	foreign := newBranch(t, s, other, "HER")

	for _, r := range membership.AllRoles {
		got, err := NewGetBranch(s).Execute(t.Context(), tenantAs(org, r), b.ID)
		if err != nil || got.ID != b.ID {
			t.Fatalf("rol %s: err=%v", r, err)
		}
	}
	if _, err := NewGetBranch(s).Execute(t.Context(), tenantAs(org, membership.RoleOwner), foreign.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sucursal de otra organización: err=%v", err)
	}
	page, err := NewListBranches(s).Execute(t.Context(), tenantAs(org, membership.RoleReadOnly), BranchQuery{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != b.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestUpdateBranchDeactivatesAndAudits(t *testing.T) {
	s := newStore()
	org := uuid.New()
	b := newBranch(t, s, org, "SJ")
	uc := NewUpdateBranch(s)
	off := false

	got, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), b.ID, branch.Patch{IsActive: &off})
	if err != nil || got.IsActive {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	last := s.audit[len(s.audit)-1]
	if last.Action != "branch.deactivated" || last.Before.(map[string]any)["isActive"] != true || len(last.After.(map[string]any)) != 1 {
		t.Fatalf("audit=%+v", last)
	}

	active := true
	page, _ := NewListBranches(s).Execute(t.Context(), tenantAs(org, membership.RoleOwner), BranchQuery{Active: &active})
	if len(page.Items) != 0 {
		t.Fatal("el filtro active=true no debe incluir la desactivada")
	}

	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleCollector), b.ID, branch.Patch{IsActive: &active}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cobrador: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), b.ID, branch.Patch{IsActive: &active}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("otra organización: err=%v", err)
	}
}
