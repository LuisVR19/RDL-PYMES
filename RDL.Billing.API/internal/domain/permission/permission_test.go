package permission

import "testing"

func TestMatrix(t *testing.T) {
	all := []Permission{CustomersRead, CustomersManage, ProductsRead, ProductsManage, InvoicesRead,
		InvoicesManage, InvoicesIssue, SequencesRead, SequencesManage}
	// Tabla escrita desde el prompt P4, no desde la implementación.
	want := map[Role][]Permission{
		RoleOwner:      all,
		RoleAdmin:      all,
		RoleBiller:     {CustomersRead, CustomersManage, ProductsRead, ProductsManage, InvoicesRead, InvoicesManage, InvoicesIssue},
		RoleCollector:  {CustomersRead, ProductsRead, InvoicesRead},
		RoleAccountant: {CustomersRead, ProductsRead, InvoicesRead},
		RoleReadOnly:   {CustomersRead, ProductsRead, InvoicesRead},
	}
	for _, role := range AllRoles {
		granted := map[Permission]bool{}
		for _, p := range want[role] {
			granted[p] = true
		}
		for _, p := range all {
			if got := Can([]Role{role}, p); got != granted[p] {
				t.Errorf("Can(%s, %s) = %v, se esperaba %v", role, p, got, granted[p])
			}
		}
	}
}

func TestAnyRoleGrants(t *testing.T) {
	if !Can([]Role{RoleReadOnly, RoleBiller}, InvoicesIssue) {
		t.Fatal("basta con que uno de los roles conceda el permiso")
	}
}

func TestUnknownIsDenied(t *testing.T) {
	if Can([]Role{"superuser"}, CustomersRead) {
		t.Fatal("un rol desconocido no concede nada")
	}
	if Can([]Role{RoleOwner}, Permission("invoices.delete-everything")) {
		t.Fatal("un permiso desconocido se deniega")
	}
	if Can(nil, CustomersRead) {
		t.Fatal("sin roles no hay permisos")
	}
}
