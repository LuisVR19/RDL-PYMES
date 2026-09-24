package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

func validInput() organization.NewInput {
	return organization.NewInput{
		LegalName: "Soluciones Alfa S.A.", IdentificationTypeCode: "02", IdentificationNumber: "3101123456",
		Email: "facturas@alfa.test",
	}
}

func TestCreateOrganizationMakesCreatorOwnerAndActive(t *testing.T) {
	s := newStore()
	res, err := NewCreateOrganization(s).Execute(t.Context(), ana, "key-1", validInput())
	if err != nil {
		t.Fatal(err)
	}
	u := s.users[ana.Subject]
	o := res.Organization

	if res.Replayed || o.ID != OrganizationIDFor(u.ID, "key-1") || o.Timezone != organization.DefaultTimezone {
		t.Fatalf("res=%+v", res)
	}
	if u.ActiveOrganizationID != o.ID {
		t.Fatal("la primera organización debe quedar activa")
	}
	m := s.memberships[u.ID]
	if len(m) != 1 || m[0].OrganizationID != o.ID || len(m[0].Roles) != 1 || m[0].Roles[0] != membership.RoleOwner {
		t.Fatalf("memberships=%+v", m)
	}

	var actions []string
	for _, e := range s.audit {
		actions = append(actions, e.Action)
	}
	want := []string{"user.provisioned", "organization.created", "membership.created"}
	if len(actions) != len(want) {
		t.Fatalf("audit=%v", actions)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Fatalf("audit=%v", actions)
		}
	}

	rec := s.idem[o.ID.String()+"|key-1"]
	if rec.Status != http.StatusCreated || rec.Result["organizationId"] != o.ID.String() {
		t.Fatalf("idempotencia=%+v", rec)
	}
	// La transacción de alta corre en la organización nueva, como owner.
	last := s.tenantTxs[len(s.tenantTxs)-1]
	if last.OrganizationID() != o.ID || last.UserID() != u.ID {
		t.Fatalf("tenant de alta=%+v", last)
	}
}

func TestCreateOrganizationSameKeySameBodyReplays(t *testing.T) {
	s := newStore()
	uc := NewCreateOrganization(s)
	first, _ := uc.Execute(t.Context(), ana, "key-1", validInput())
	auditAfterFirst := len(s.audit)

	second, err := uc.Execute(t.Context(), ana, "key-1", validInput())
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Organization.ID != first.Organization.ID || len(s.orgs) != 1 || len(s.audit) != auditAfterFirst {
		t.Fatalf("second=%+v orgs=%d audit=%d", second, len(s.orgs), len(s.audit))
	}
}

func TestCreateOrganizationSameKeyDifferentBodyIsRejected(t *testing.T) {
	s := newStore()
	uc := NewCreateOrganization(s)
	_, _ = uc.Execute(t.Context(), ana, "key-1", validInput())

	other := validInput()
	other.LegalName = "Otra S.A."
	if _, err := uc.Execute(t.Context(), ana, "key-1", other); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("err=%v", err)
	}
	if len(s.orgs) != 1 {
		t.Fatalf("orgs=%d", len(s.orgs))
	}
}

func TestSameKeyFromAnotherUserIsIndependent(t *testing.T) {
	s := newStore()
	uc := NewCreateOrganization(s)
	a, _ := uc.Execute(t.Context(), ana, "key-1", validInput())

	beto := tenancy.Identity{Subject: "sub-beto", Email: "beto@example.test"}
	in := validInput()
	in.IdentificationNumber = "3101999999"
	b, err := uc.Execute(t.Context(), beto, "key-1", in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Organization.ID == b.Organization.ID || b.Replayed {
		t.Fatal("la misma clave de otro usuario no debe colisionar")
	}
}

func TestCreateOrganizationFailureLeavesNoTrace(t *testing.T) {
	s := newStore()
	uc := NewCreateOrganization(s)
	_, _ = uc.Execute(t.Context(), ana, "key-1", validInput())
	orgs, idem, audit := len(s.orgs), len(s.idem), len(s.audit)

	// Misma identificación con otra clave: conflicto, y la reserva de la clave nueva se descarta.
	if _, err := uc.Execute(t.Context(), ana, "key-2", validInput()); !errors.Is(err, ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	if len(s.orgs) != orgs || len(s.idem) != idem || len(s.audit) != audit {
		t.Fatalf("quedaron restos: orgs=%d idem=%d audit=%d", len(s.orgs), len(s.idem), len(s.audit))
	}
}

func TestCreateOrganizationValidatesBeforeWriting(t *testing.T) {
	s := newStore()
	in := validInput()
	in.Timezone = "Marte/Olympus"
	_, err := NewCreateOrganization(s).Execute(t.Context(), ana, "key-1", in)
	if !errors.Is(err, organization.ErrInvalid) || len(s.orgs) != 0 || len(s.idem) != 0 {
		t.Fatalf("err=%v orgs=%d idem=%d", err, len(s.orgs), len(s.idem))
	}
}

func TestCreateOrganizationKeepsExistingActiveOrganization(t *testing.T) {
	s := newStore()
	previous := uuid.New()
	s.users[ana.Subject] = user.User{ID: uuid.New(), Subject: ana.Subject, Email: ana.Email, Status: user.StatusActive, ActiveOrganizationID: previous}
	if _, err := NewCreateOrganization(s).Execute(t.Context(), ana, "key-1", validInput()); err != nil {
		t.Fatal(err)
	}
	if s.users[ana.Subject].ActiveOrganizationID != previous {
		t.Fatal("no debe cambiar la organización activa que el usuario ya eligió")
	}
}

func tenantAs(org uuid.UUID, role membership.Role) tenancy.Context {
	return tenancy.NewContext(uuid.New(), "sub", org, []string{string(role)})
}

func seedOrg(s *store) organization.Organization {
	o, _ := organization.New(uuid.New(), validInput())
	s.orgs[o.ID] = o
	return o
}

func TestGetCurrentOrganizationAnyRole(t *testing.T) {
	s := newStore()
	o := seedOrg(s)
	for _, r := range membership.AllRoles {
		got, err := NewGetCurrentOrganization(s).Execute(t.Context(), tenantAs(o.ID, r))
		if err != nil || got.ID != o.ID {
			t.Fatalf("rol %s: err=%v", r, err)
		}
	}
}

func TestUpdateCurrentOrganizationRequiresOwnerOrAdmin(t *testing.T) {
	s := newStore()
	o := seedOrg(s)
	name := "Alfa"
	for _, r := range []membership.Role{membership.RoleBiller, membership.RoleCollector, membership.RoleAccountant, membership.RoleReadOnly} {
		_, err := NewUpdateCurrentOrganization(s).Execute(t.Context(), tenantAs(o.ID, r), organization.Patch{TradeName: &name})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s: err=%v", r, err)
		}
	}
	if s.orgs[o.ID].TradeName != "" || len(s.audit) != 0 {
		t.Fatal("un rol sin permiso no debe cambiar nada")
	}
}

func TestUpdateCurrentOrganizationAuditsOnlyChangedFields(t *testing.T) {
	s := newStore()
	o := seedOrg(s)
	name, tz := "Alfa", organization.DefaultTimezone
	got, err := NewUpdateCurrentOrganization(s).Execute(t.Context(), tenantAs(o.ID, membership.RoleAdmin),
		organization.Patch{TradeName: &name, Timezone: &tz})
	if err != nil {
		t.Fatal(err)
	}
	if got.TradeName != "Alfa" || len(s.audit) != 1 {
		t.Fatalf("got=%+v audit=%d", got, len(s.audit))
	}
	before, after := s.audit[0].Before.(map[string]any), s.audit[0].After.(map[string]any)
	if len(before) != 1 || before["tradeName"] != "" || after["tradeName"] != "Alfa" {
		t.Fatalf("before=%v after=%v", before, after)
	}

	// Repetir el mismo cambio no genera un evento vacío.
	if _, err := NewUpdateCurrentOrganization(s).Execute(t.Context(), tenantAs(o.ID, membership.RoleOwner), organization.Patch{TradeName: &name}); err != nil {
		t.Fatal(err)
	}
	if len(s.audit) != 1 {
		t.Fatalf("audit=%d", len(s.audit))
	}
}

func TestUpdateCurrentOrganizationValidates(t *testing.T) {
	s := newStore()
	o := seedOrg(s)
	bad := "no-es-email"
	_, err := NewUpdateCurrentOrganization(s).Execute(t.Context(), tenantAs(o.ID, membership.RoleOwner), organization.Patch{Email: &bad})
	if !errors.Is(err, organization.ErrInvalid) || s.orgs[o.ID].Email != o.Email {
		t.Fatalf("err=%v", err)
	}
}
