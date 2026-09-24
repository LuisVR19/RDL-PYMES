package membership

import (
	"errors"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func member(status Status, roles ...Role) Member {
	return Member{Status: status, Roles: roles}
}

func TestPlanChangeReplacesSingleRole(t *testing.T) {
	next, err := PlanChange([]Role{RoleAdmin}, member(StatusActive, RoleBiller), Change{Role: ptr(RoleCollector)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Roles) != 1 || next.Roles[0] != RoleCollector {
		t.Fatalf("roles=%v", next.Roles)
	}
}

func TestPlanChangeRules(t *testing.T) {
	cases := []struct {
		name   string
		actor  Role
		target Member
		change Change
		owners int
		want   error
	}{
		{"admin suspende a un facturador", RoleAdmin, member(StatusActive, RoleBiller), Change{Status: ptr(StatusSuspended)}, 1, nil},
		{"admin no puede asignar owner", RoleAdmin, member(StatusActive, RoleBiller), Change{Role: ptr(RoleOwner)}, 1, ErrOwnerRequired},
		{"admin no puede degradar a un owner", RoleAdmin, member(StatusActive, RoleOwner), Change{Role: ptr(RoleAdmin)}, 2, ErrOwnerRequired},
		{"admin no puede suspender a un owner", RoleAdmin, member(StatusActive, RoleOwner), Change{Status: ptr(StatusSuspended)}, 2, ErrOwnerRequired},
		{"owner promueve a owner", RoleOwner, member(StatusActive, RoleAdmin), Change{Role: ptr(RoleOwner)}, 1, nil},
		{"owner degrada a otro owner si quedan más", RoleOwner, member(StatusActive, RoleOwner), Change{Role: ptr(RoleAdmin)}, 2, nil},
		{"no se puede degradar al último owner", RoleOwner, member(StatusActive, RoleOwner), Change{Role: ptr(RoleAdmin)}, 1, ErrLastOwner},
		{"no se puede suspender al último owner", RoleOwner, member(StatusActive, RoleOwner), Change{Status: ptr(StatusSuspended)}, 1, ErrLastOwner},
		{"reactivar a un owner suspendido no toca el mínimo", RoleOwner, member(StatusSuspended, RoleOwner), Change{Status: ptr(StatusActive)}, 1, nil},
		{"rol inexistente", RoleOwner, member(StatusActive, RoleBiller), Change{Role: ptr(Role("superadmin"))}, 1, ErrInvalidRole},
		{"estado inexistente", RoleOwner, member(StatusActive, RoleBiller), Change{Status: ptr(Status("deleted"))}, 1, ErrInvalidStatus},
		{"sin cambios", RoleOwner, member(StatusActive, RoleBiller), Change{}, 1, ErrEmptyChange},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := PlanChange([]Role{c.actor}, c.target, c.change, c.owners)
			if !errors.Is(err, c.want) {
				t.Fatalf("err=%v, want %v", err, c.want)
			}
		})
	}
}

func TestPlanChangeDoesNotMutateTarget(t *testing.T) {
	target := member(StatusActive, RoleBiller)
	_, _ = PlanChange([]Role{RoleOwner}, target, Change{Role: ptr(RoleAdmin)}, 1)
	if target.Roles[0] != RoleBiller {
		t.Fatal("PlanChange modificó el miembro original")
	}
}
