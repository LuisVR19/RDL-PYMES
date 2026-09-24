package app

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

var ana = tenancy.Identity{Subject: "sub-ana", Email: "ana@example.test", FullName: "Ana Mora"}

func TestGetMeProvisionsOnFirstCallAndAuditsOnce(t *testing.T) {
	s := newStore()
	uc := NewGetMe(s)

	first, err := uc.Execute(t.Context(), ana)
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Execute(t.Context(), ana)
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == uuid.Nil || first.ID != second.ID || first.Email != ana.Email {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if len(s.audit) != 1 || s.audit[0].Action != "user.provisioned" || s.audit[0].EntityID != first.ID {
		t.Fatalf("audit=%+v", s.audit)
	}
}

func TestGetMeDoesNotAuditWhenAnotherRequestCreatedTheUser(t *testing.T) {
	s := newStore()
	s.raceOnCreate = true
	if _, err := NewGetMe(s).Execute(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	if len(s.audit) != 0 {
		t.Fatalf("la otra transacción ya auditó el alta: audit=%+v", s.audit)
	}
}

func TestGetMeWithoutEmailFailsWithoutSideEffects(t *testing.T) {
	s := newStore()
	_, err := NewGetMe(s).Execute(t.Context(), tenancy.Identity{Subject: "sub-tel"})
	if !errors.Is(err, user.ErrEmailRequired) || len(s.users) != 0 || len(s.audit) != 0 {
		t.Fatalf("err=%v users=%d audit=%d", err, len(s.users), len(s.audit))
	}
}

func TestGetMeRejectsDisabledUser(t *testing.T) {
	s := newStore()
	s.users[ana.Subject] = user.User{ID: uuid.New(), Subject: ana.Subject, Status: user.StatusDisabled}
	if _, err := NewGetMe(s).Execute(t.Context(), ana); !errors.Is(err, user.ErrDisabled) {
		t.Fatalf("err=%v", err)
	}
}

func seedMember(s *store, orgs ...uuid.UUID) user.User {
	u := user.User{ID: uuid.New(), Subject: ana.Subject, Email: ana.Email, Status: user.StatusActive}
	s.users[u.Subject] = u
	for _, org := range orgs {
		s.memberships[u.ID] = append(s.memberships[u.ID], membership.Summary{
			OrganizationID: org, Status: membership.StatusActive, Roles: []membership.Role{membership.RoleAccountant},
		})
	}
	return u
}

func TestListMyMembershipsForUnprovisionedUserIsEmpty(t *testing.T) {
	res, err := NewListMyMemberships(newStore()).Execute(t.Context(), ana)
	if err != nil || res.Items == nil || len(res.Items) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestListMyMembershipsReadsUnderTheUsersOwnSession(t *testing.T) {
	s := newStore()
	orgA, orgB := uuid.New(), uuid.New()
	u := seedMember(s, orgA, orgB) // caso contador: dos organizaciones

	res, err := NewListMyMemberships(s).Execute(t.Context(), ana)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 {
		t.Fatalf("items=%d", len(res.Items))
	}
	if last := s.txUsers[len(s.txUsers)-1]; last != u.ID {
		t.Fatalf("el listado debe correr con la sesión del usuario (RLS *_own), sesión=%s", last)
	}
}

func TestSelectActiveOrganizationRequiresActiveMembership(t *testing.T) {
	s := newStore()
	seedMember(s, uuid.New())

	err := NewSelectActiveOrganization(s).Execute(t.Context(), ana, uuid.New())
	if !errors.Is(err, ErrNotFound) || len(s.audit) != 0 {
		t.Fatalf("err=%v audit=%d", err, len(s.audit))
	}
}

func TestSelectActiveOrganizationUnknownUserIsNotFound(t *testing.T) {
	if err := NewSelectActiveOrganization(newStore()).Execute(t.Context(), ana, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestSelectActiveOrganizationStoresAndAudits(t *testing.T) {
	s := newStore()
	orgA := uuid.New()
	u := seedMember(s, orgA)

	if err := NewSelectActiveOrganization(s).Execute(t.Context(), ana, orgA); err != nil {
		t.Fatal(err)
	}
	if s.users[ana.Subject].ActiveOrganizationID != orgA {
		t.Fatal("no se guardó la organización activa")
	}
	if len(s.audit) != 1 {
		t.Fatalf("audit=%+v", s.audit)
	}
	e := s.audit[0]
	if e.Action != "user.active_organization_selected" || e.OrganizationID != orgA || e.ActorUserID != u.ID || e.Before == nil || e.After == nil {
		t.Fatalf("evento=%+v", e)
	}

	// Repetir la misma selección es idempotente y no genera ruido en la auditoría.
	if err := NewSelectActiveOrganization(s).Execute(t.Context(), ana, orgA); err != nil {
		t.Fatal(err)
	}
	if len(s.audit) != 1 {
		t.Fatalf("una selección repetida no debe auditarse, audit=%d", len(s.audit))
	}
}
