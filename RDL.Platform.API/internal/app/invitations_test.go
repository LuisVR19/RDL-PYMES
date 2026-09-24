package app

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

var carla = tenancy.Identity{Subject: "sub-carla", Email: "carla@contadores.test", FullName: "Carla Contadora"}

func invite(t *testing.T, s *store, org uuid.UUID, email string, role membership.Role) CreateInvitationResult {
	t.Helper()
	res, err := NewCreateInvitation(s).Execute(t.Context(), tenantAs(org, membership.RoleOwner), "k-"+email+string(role), email, role)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestCreateInvitationReturnsTokenOnceAndStoresOnlyTheHash(t *testing.T) {
	s := newStore()
	org := uuid.New()
	uc := NewCreateInvitation(s)
	admin := tenantAs(org, membership.RoleAdmin)

	res, err := uc.Execute(t.Context(), admin, "k1", "Carla@Contadores.test", membership.RoleAccountant)
	if err != nil {
		t.Fatal(err)
	}
	if !invitation.LooksLikeToken(res.Token) || res.Invitation.Email != "carla@contadores.test" || res.Replayed {
		t.Fatalf("res=%+v", res)
	}
	if _, ok := s.tokenHashes[invitation.HashToken(res.Token)]; !ok {
		t.Fatal("se debe guardar el hash del token")
	}
	for _, e := range s.audit {
		for _, v := range []any{e.Before, e.After} {
			if v != nil && strings.Contains(strings.ToLower(fmtAny(v)), res.Token[:12]) {
				t.Fatal("el token no puede quedar en la auditoría")
			}
		}
	}

	again, err := uc.Execute(t.Context(), admin, "k1", "carla@contadores.test", membership.RoleAccountant)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Replayed || again.Token != "" || again.Invitation.ID != res.Invitation.ID || len(s.invitations) != 1 {
		t.Fatalf("again=%+v", again)
	}
	if _, err := uc.Execute(t.Context(), admin, "k1", "otra@x.test", membership.RoleAccountant); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("err=%v", err)
	}
}

func TestCreateInvitationRules(t *testing.T) {
	s := newStore()
	org := uuid.New()
	seedMembers(s, membership.Member{Email: "ya@miembro.test", Status: membership.StatusActive, Roles: []membership.Role{membership.RoleBiller}})
	uc := NewCreateInvitation(s)

	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleBiller), "a", "x@y.test", membership.RoleBiller); !errors.Is(err, ErrForbidden) {
		t.Fatalf("facturador: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleAdmin), "b", "x@y.test", membership.RoleOwner); !errors.Is(err, membership.ErrOwnerRequired) {
		t.Fatalf("admin invitando owner: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), "c", "YA@miembro.test", membership.RoleAdmin); !errors.Is(err, ErrConflict) {
		t.Fatalf("miembro existente: err=%v", err)
	}
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), "d", "no-es-email", membership.RoleAdmin); !errors.Is(err, invitation.ErrInvalidEmail) {
		t.Fatalf("email inválido: err=%v", err)
	}
	invite(t, s, org, "nueva@x.test", membership.RoleBiller)
	if _, err := uc.Execute(t.Context(), tenantAs(org, membership.RoleOwner), "e", "nueva@x.test", membership.RoleCollector); !errors.Is(err, ErrConflict) {
		t.Fatalf("pendiente duplicada: err=%v", err)
	}
}

func TestRevokeInvitation(t *testing.T) {
	s := newStore()
	org := uuid.New()
	res := invite(t, s, org, "x@y.test", membership.RoleBiller)
	uc := NewRevokeInvitation(s)
	owner := tenantAs(org, membership.RoleOwner)
	auditBefore := len(s.audit)

	if err := uc.Execute(t.Context(), owner, res.Invitation.ID); err != nil {
		t.Fatal(err)
	}
	if err := uc.Execute(t.Context(), owner, res.Invitation.ID); err != nil {
		t.Fatalf("revocar dos veces debe ser inocuo: %v", err)
	}
	if s.invitations[res.Invitation.ID].Status != invitation.StatusRevoked || len(s.audit) != auditBefore+1 {
		t.Fatalf("status=%s audit=%d", s.invitations[res.Invitation.ID].Status, len(s.audit)-auditBefore)
	}
	if err := uc.Execute(t.Context(), owner, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("inexistente: err=%v", err)
	}
	if err := uc.Execute(t.Context(), tenantAs(org, membership.RoleReadOnly), res.Invitation.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("solo lectura: err=%v", err)
	}
}

func TestAcceptInvitationJoinsOrganization(t *testing.T) {
	s := newStore()
	org := uuid.New()
	res := invite(t, s, org, carla.Email, membership.RoleAccountant)

	out, err := NewAcceptInvitation(s).Execute(t.Context(), carla, "acc-1", res.Token)
	if err != nil {
		t.Fatal(err)
	}
	u := s.users[carla.Subject]
	if out.OrganizationID != org || out.Role != membership.RoleAccountant || !out.ActiveOrganizationSelected {
		t.Fatalf("out=%+v", out)
	}
	if u.ActiveOrganizationID != org || s.invitations[res.Invitation.ID].Status != invitation.StatusAccepted {
		t.Fatal("la invitación debe quedar aceptada y la organización activa")
	}
	m := s.memberships[u.ID]
	if len(m) != 1 || m[0].Roles[0] != membership.RoleAccountant {
		t.Fatalf("memberships=%+v", m)
	}
	last := s.tenantTxs[len(s.tenantTxs)-1]
	if last.OrganizationID() != org || last.UserID() != u.ID {
		t.Fatal("la aceptación debe correr en la organización de la invitación, con el usuario autenticado")
	}

	// Reintento con la misma clave: misma respuesta, sin efectos nuevos.
	auditAfter := len(s.audit)
	again, err := NewAcceptInvitation(s).Execute(t.Context(), carla, "acc-1", res.Token)
	if err != nil || !again.Replayed || again.OrganizationID != org || len(s.audit) != auditAfter {
		t.Fatalf("again=%+v err=%v", again, err)
	}
	// Con otra clave, la invitación ya está usada: un solo uso.
	if _, err := NewAcceptInvitation(s).Execute(t.Context(), carla, "acc-2", res.Token); !errors.Is(err, invitation.ErrNotPending) {
		t.Fatalf("segundo uso: err=%v", err)
	}
}

func TestAcceptInvitationKeepsActiveOrganizationOfAccountant(t *testing.T) {
	s := newStore()
	previous := uuid.New()
	s.users[carla.Subject] = user.User{ID: uuid.New(), Subject: carla.Subject, Email: carla.Email, Status: user.StatusActive, ActiveOrganizationID: previous}
	res := invite(t, s, uuid.New(), carla.Email, membership.RoleAccountant)

	out, err := NewAcceptInvitation(s).Execute(t.Context(), carla, "acc", res.Token)
	if err != nil {
		t.Fatal(err)
	}
	if out.ActiveOrganizationSelected || s.users[carla.Subject].ActiveOrganizationID != previous {
		t.Fatal("una segunda organización no debe cambiar la activa")
	}
}

func TestAcceptInvitationForSomeoneElseIsNotFound(t *testing.T) {
	s := newStore()
	res := invite(t, s, uuid.New(), carla.Email, membership.RoleAccountant)
	intruder := tenancy.Identity{Subject: "sub-x", Email: "intruso@x.test"}

	if _, err := NewAcceptInvitation(s).Execute(t.Context(), intruder, "k", res.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if s.invitations[res.Invitation.ID].Status != invitation.StatusPending {
		t.Fatal("un intento ajeno no debe consumir la invitación")
	}
}

func TestAcceptInvitationExpired(t *testing.T) {
	s := newStore()
	res := invite(t, s, uuid.New(), carla.Email, membership.RoleAccountant)
	uc := NewAcceptInvitation(s)
	uc.now = func() time.Time { return time.Now().Add(invitation.TTL + time.Minute) }

	if _, err := uc.Execute(t.Context(), carla, "k", res.Token); !errors.Is(err, invitation.ErrExpired) {
		t.Fatalf("err=%v", err)
	}
	if len(s.idem) != 1 { // solo la reserva de la invitación; la de la aceptación fallida se descarta
		t.Fatalf("idem=%d", len(s.idem))
	}
}

func TestAcceptInvitationInvalidTokenIsNotFound(t *testing.T) {
	for _, token := range []string{"", "abc", "inv_" + strings.Repeat("x", 43)} {
		if _, err := NewAcceptInvitation(newStore()).Execute(t.Context(), carla, "k", token); !errors.Is(err, ErrNotFound) {
			t.Errorf("token %q: err=%v", token, err)
		}
	}
}

func TestAcceptInvitationReactivatesSuspendedMember(t *testing.T) {
	s := newStore()
	org := uuid.New()
	u := user.User{ID: uuid.New(), Subject: carla.Subject, Email: carla.Email, Status: user.StatusActive, ActiveOrganizationID: org}
	s.users[carla.Subject] = u
	s.members[u.ID] = membership.Member{MembershipID: uuid.New(), UserID: u.ID, Email: u.Email, Status: membership.StatusSuspended, Roles: []membership.Role{membership.RoleBiller}}
	res := invite(t, s, org, carla.Email, membership.RoleCollector)

	if _, err := NewAcceptInvitation(s).Execute(t.Context(), carla, "k", res.Token); err != nil {
		t.Fatal(err)
	}
	m := s.members[u.ID]
	if m.Status != membership.StatusActive || m.Roles[0] != membership.RoleCollector {
		t.Fatalf("member=%+v", m)
	}
}

func fmtAny(v any) string {
	var b strings.Builder
	if m, ok := v.(map[string]any); ok {
		for k, val := range m {
			b.WriteString(k)
			if s, ok := val.(string); ok {
				b.WriteString(s)
			}
		}
	}
	return b.String()
}
