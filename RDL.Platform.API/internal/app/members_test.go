package app

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
)

func seedMembers(s *store, specs ...membership.Member) []membership.Member {
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	out := make([]membership.Member, 0, len(specs))
	for i, m := range specs {
		m.UserID, m.MembershipID = uuid.New(), uuid.New()
		m.Subject = "sub-" + m.UserID.String()
		m.JoinedAt = base.Add(time.Duration(i) * time.Minute)
		s.members[m.UserID] = m
		out = append(out, m)
	}
	return out
}

func active(roles ...membership.Role) membership.Member {
	return membership.Member{Status: membership.StatusActive, Roles: roles}
}

func TestListMembersRequiresOwnerOrAdmin(t *testing.T) {
	s := newStore()
	for _, r := range []membership.Role{membership.RoleBiller, membership.RoleCollector, membership.RoleAccountant, membership.RoleReadOnly} {
		if _, err := NewListMembers(s).Execute(t.Context(), tenantAs(uuid.New(), r), MemberQuery{}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s: err=%v", r, err)
		}
	}
}

func TestListMembersPaginatesWithCursor(t *testing.T) {
	s := newStore()
	seedMembers(s, active(membership.RoleOwner), active(membership.RoleAdmin), active(membership.RoleBiller),
		active(membership.RoleCollector), active(membership.RoleReadOnly))
	uc := NewListMembers(s)
	admin := tenantAs(uuid.New(), membership.RoleAdmin)

	seen := map[uuid.UUID]int{}
	q := MemberQuery{Limit: 2}
	for pages := 1; ; pages++ {
		if pages > 5 {
			t.Fatal("la paginación no termina")
		}
		page, err := uc.Execute(t.Context(), admin, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Items {
			seen[m.UserID]++
		}
		if page.Next == nil {
			if pages != 3 {
				t.Fatalf("páginas=%d, se esperaban 3 (2+2+1)", pages)
			}
			break
		}
		q.After = page.Next
	}
	if len(seen) != 5 {
		t.Fatalf("vistos=%d, se esperaban 5", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("%s apareció %d veces", id, n)
		}
	}
}

func TestListMembersClampsLimit(t *testing.T) {
	s := newStore()
	specs := make([]membership.Member, MaxPageSize+5)
	for i := range specs {
		specs[i] = active(membership.RoleBiller)
	}
	seedMembers(s, specs...)
	page, err := NewListMembers(s).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), MemberQuery{Limit: 1000})
	if err != nil || len(page.Items) != MaxPageSize || page.Next == nil {
		t.Fatalf("items=%d next=%v err=%v", len(page.Items), page.Next, err)
	}
}

func TestUpdateMemberChangesRoleAuditsAndInvalidatesCache(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleBiller))
	cache := &fakeCache{}
	role := membership.RoleCollector

	got, err := NewUpdateMember(s, cache).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleAdmin), m[1].UserID, membership.Change{Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if got.Roles[0] != membership.RoleCollector || s.members[m[1].UserID].Roles[0] != membership.RoleCollector {
		t.Fatalf("got=%+v", got)
	}
	if len(s.audit) != 1 || s.audit[0].Action != "membership.role_changed" || s.audit[0].EntityID != m[1].MembershipID {
		t.Fatalf("audit=%+v", s.audit)
	}
	if len(cache.invalidated) != 1 || cache.invalidated[0] != m[1].Subject {
		t.Fatalf("cache=%v", cache.invalidated)
	}
}

func TestUpdateMemberSuspendAndReactivate(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleBiller))
	uc := NewUpdateMember(s, &fakeCache{})
	owner := tenantAs(uuid.New(), membership.RoleOwner)

	suspended, reactivated := membership.StatusSuspended, membership.StatusActive
	if _, err := uc.Execute(t.Context(), owner, m[1].UserID, membership.Change{Status: &suspended}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(t.Context(), owner, m[1].UserID, membership.Change{Status: &reactivated}); err != nil {
		t.Fatal(err)
	}
	if len(s.audit) != 2 || s.audit[0].Action != "membership.suspended" || s.audit[1].Action != "membership.reactivated" {
		t.Fatalf("audit=%+v", s.audit)
	}
}

func TestUpdateMemberCannotRemoveLastOwner(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleAdmin))
	cache := &fakeCache{}
	role := membership.RoleAdmin
	_, err := NewUpdateMember(s, cache).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), m[0].UserID, membership.Change{Role: &role})
	if !errors.Is(err, membership.ErrLastOwner) {
		t.Fatalf("err=%v", err)
	}
	if s.members[m[0].UserID].Roles[0] != membership.RoleOwner || len(s.audit) != 0 || len(cache.invalidated) != 0 {
		t.Fatal("un cambio rechazado no debe dejar efectos")
	}
}

func TestUpdateMemberAdminCannotTouchOwners(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleOwner), active(membership.RoleBiller))
	uc := NewUpdateMember(s, &fakeCache{})
	admin := tenantAs(uuid.New(), membership.RoleAdmin)
	owner, suspended := membership.RoleOwner, membership.StatusSuspended

	if _, err := uc.Execute(t.Context(), admin, m[2].UserID, membership.Change{Role: &owner}); !errors.Is(err, membership.ErrOwnerRequired) {
		t.Fatalf("promover a owner: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), admin, m[0].UserID, membership.Change{Status: &suspended}); !errors.Is(err, membership.ErrOwnerRequired) {
		t.Fatalf("suspender owner: err=%v", err)
	}
}

func TestUpdateMemberNotInOrganizationIsNotFound(t *testing.T) {
	role := membership.RoleAdmin
	_, err := NewUpdateMember(newStore(), &fakeCache{}).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), uuid.New(), membership.Change{Role: &role})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateMemberRequiresPermission(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleBiller))
	role := membership.RoleReadOnly
	_, err := NewUpdateMember(s, &fakeCache{}).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleAccountant), m[1].UserID, membership.Change{Role: &role})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
}

func TestUpdateMemberNoopDoesNotAuditOrInvalidate(t *testing.T) {
	s := newStore()
	m := seedMembers(s, active(membership.RoleOwner), active(membership.RoleBiller))
	cache := &fakeCache{}
	role := membership.RoleBiller
	if _, err := NewUpdateMember(s, cache).Execute(t.Context(), tenantAs(uuid.New(), membership.RoleOwner), m[1].UserID, membership.Change{Role: &role}); err != nil {
		t.Fatal(err)
	}
	if len(s.audit) != 0 || len(cache.invalidated) != 0 {
		t.Fatalf("audit=%d cache=%d", len(s.audit), len(cache.invalidated))
	}
}
