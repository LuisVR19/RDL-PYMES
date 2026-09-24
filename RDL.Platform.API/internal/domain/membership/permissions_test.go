package membership

import "testing"

func TestMatrix(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleOwner, PermOrganizationUpdate, true},
		{RoleAdmin, PermOrganizationUpdate, true},
		{RoleBiller, PermOrganizationUpdate, false},
		{RoleAccountant, PermOrganizationUpdate, false},
		{RoleReadOnly, PermOrganizationUpdate, false},
		{RoleReadOnly, PermOrganizationRead, true},
		{RoleCollector, PermBranchesRead, true},
		{RoleCollector, PermBranchesManage, false},
		{RoleAccountant, PermMembersRead, false},
		{RoleAdmin, PermMembersManage, true},
		{RoleBiller, PermInvitationsManage, false},
	}
	for _, c := range cases {
		if got := Can([]Role{c.role}, c.perm); got != c.want {
			t.Errorf("%s %s = %v, want %v", c.role, c.perm, got, c.want)
		}
	}
}

func TestEveryRoleCanReadItsOrganization(t *testing.T) {
	for _, r := range AllRoles {
		if !Can([]Role{r}, PermOrganizationRead) {
			t.Errorf("%s no puede leer su organización", r)
		}
	}
}

func TestUnknownPermissionOrRoleIsDenied(t *testing.T) {
	if Can([]Role{RoleOwner}, Permission("billing.issue")) {
		t.Error("un permiso desconocido debe denegarse")
	}
	if Can([]Role{"superadmin"}, PermOrganizationRead) || Role("superadmin").Valid() {
		t.Error("un rol desconocido no concede nada")
	}
	if Can(nil, PermOrganizationRead) {
		t.Error("sin roles no hay permisos")
	}
}
