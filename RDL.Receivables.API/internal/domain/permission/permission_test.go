package permission

import "testing"

// La tabla repite docs/PLAN.md §4: si cambia la matriz, este test obliga a cambiar el plan (o al revés).
func TestMatrix(t *testing.T) {
	all := []Role{RoleOwner, RoleAdmin, RoleCollector, RoleAccountant, RoleReadOnly, RoleBiller}
	want := map[Permission][]Role{
		ReceivablesRead:    {RoleOwner, RoleAdmin, RoleCollector, RoleAccountant, RoleReadOnly},
		InvoiceBalanceRead: {RoleOwner, RoleAdmin, RoleCollector, RoleAccountant, RoleReadOnly, RoleBiller},
		PaymentsWrite:      {RoleOwner, RoleAdmin, RoleCollector},
		PaymentsReverse:    {RoleOwner, RoleAdmin},
	}
	for p, allowed := range want {
		for _, r := range all {
			exp := false
			for _, a := range allowed {
				exp = exp || a == r
			}
			if got := Can([]string{string(r)}, p); got != exp {
				t.Errorf("%s / %s: Can=%v, se esperaba %v", p, r, got, exp)
			}
		}
	}
}

func TestUnknownIsDenied(t *testing.T) {
	if Can([]string{"superuser"}, ReceivablesRead) {
		t.Error("un rol desconocido no debe conceder nada")
	}
	if Can([]string{string(RoleOwner)}, Permission("otra.cosa")) {
		t.Error("un permiso desconocido debe denegarse")
	}
	if Can(nil, ReceivablesRead) {
		t.Error("sin roles no hay permisos")
	}
}

func TestAnyRoleGrants(t *testing.T) {
	if !Can([]string{string(RoleBiller), string(RoleCollector)}, PaymentsWrite) {
		t.Error("basta con que uno de los roles conceda el permiso")
	}
}
