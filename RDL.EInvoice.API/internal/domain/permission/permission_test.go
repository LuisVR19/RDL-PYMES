package permission

import "testing"

func TestMatrix(t *testing.T) {
	cases := []struct {
		perm  Permission
		allow []Role
	}{
		{FiscalProfileRead, []Role{RoleOwner, RoleAdmin, RoleAccountant}},
		{FiscalProfileWrite, []Role{RoleOwner, RoleAdmin}},
		{CertificateUpload, []Role{RoleOwner}},
		{HaciendaCredentialsWrite, []Role{RoleOwner}},
		{EstablishmentsRead, []Role{RoleOwner, RoleAdmin, RoleAccountant}},
		{EstablishmentsWrite, []Role{RoleOwner, RoleAdmin}},
		{DocumentsRead, everyone},
		{DocumentsRetry, []Role{RoleOwner, RoleAdmin}},
		{CatalogsRead, everyone},
	}
	for _, c := range cases {
		allowed := map[Role]bool{}
		for _, r := range c.allow {
			allowed[r] = true
		}
		for _, r := range everyone {
			if got := Can([]string{string(r)}, c.perm); got != allowed[r] {
				t.Errorf("Can(%s, %s) = %v, se esperaba %v", r, c.perm, got, allowed[r])
			}
		}
	}
}

func TestUnknownIsDenied(t *testing.T) {
	if Can([]string{"owner"}, "fiscal.unknown") {
		t.Error("un permiso desconocido debe denegarse")
	}
	if Can([]string{"superuser", ""}, FiscalProfileRead) || Can(nil, CatalogsRead) {
		t.Error("un rol desconocido o la ausencia de roles debe denegarse")
	}
}
