package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/tenancy"
)

type fakeCreateInvitation struct{ res app.CreateInvitationResult }

func (f fakeCreateInvitation) Execute(context.Context, tenancy.Context, string, string, membership.Role) (app.CreateInvitationResult, error) {
	return f.res, nil
}

type fakeAccept struct {
	err      error
	gotToken string
}

func (f *fakeAccept) Execute(_ context.Context, _ tenancy.Identity, _, token string) (app.AcceptInvitationResult, error) {
	f.gotToken = token
	return app.AcceptInvitationResult{OrganizationID: uuid.New(), Role: membership.RoleBiller}, f.err
}

func invitationsRouter(h *InvitationHandlers) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    stubVerifier{id: tenancy.Identity{Subject: "s", OrganizationID: uuid.New()}},
		Memberships: stubMemberships{},
		Invitations: h,
	})
}

func TestCreateInvitationReturnsTokenOnlyOnFirstCall(t *testing.T) {
	inv := invitation.Invitation{ID: uuid.New(), Email: "a@x.test", Role: membership.RoleBiller, Status: invitation.StatusPending, ExpiresAt: time.Now().Add(time.Hour)}
	body := `{"email":"a@x.test","role":"biller"}`

	first := call(invitationsRouter(&InvitationHandlers{Create: fakeCreateInvitation{res: app.CreateInvitationResult{Invitation: inv, Token: "inv_secreto"}}}),
		http.MethodPost, "/v1/organizations/current/invitations", body, "Idempotency-Key: k")
	var got invitationResponse
	_ = json.Unmarshal(first.Body.Bytes(), &got)
	if first.Code != http.StatusCreated || got.Token != "inv_secreto" || got.AcceptPath != "/v1/invitations/inv_secreto/accept" {
		t.Fatalf("code=%d body=%s", first.Code, first.Body.String())
	}

	replay := call(invitationsRouter(&InvitationHandlers{Create: fakeCreateInvitation{res: app.CreateInvitationResult{Invitation: inv, Replayed: true}}}),
		http.MethodPost, "/v1/organizations/current/invitations", body, "Idempotency-Key: k")
	got = invitationResponse{}
	_ = json.Unmarshal(replay.Body.Bytes(), &got)
	if replay.Code != http.StatusCreated || got.Token != "" || replay.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay code=%d body=%s", replay.Code, replay.Body.String())
	}
}

func TestCreateInvitationRequiresIdempotencyKeyAndValidRole(t *testing.T) {
	h := invitationsRouter(&InvitationHandlers{Create: fakeCreateInvitation{}})
	rec := call(h, http.MethodPost, "/v1/organizations/current/invitations", `{"email":"a@x.test","role":"biller"}`)
	if p := problemOf(t, rec); rec.Code != http.StatusBadRequest || p.Type != problem.TypeBase+"idempotency-key-required" {
		t.Fatalf("sin clave: code=%d problem=%+v", rec.Code, p)
	}
	rec = call(h, http.MethodPost, "/v1/organizations/current/invitations", `{"email":"a@x.test","role":"dios"}`, "Idempotency-Key: k")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("rol inválido: code=%d", rec.Code)
	}
}

func TestAcceptInvitationMapsErrors(t *testing.T) {
	cases := []struct {
		err    error
		status int
		slug   string
	}{
		{invitation.ErrExpired, http.StatusGone, "invitation-expired"},
		{invitation.ErrNotPending, http.StatusConflict, "invitation-not-pending"},
		{app.ErrNotFound, http.StatusNotFound, "not-found"},
		{app.ErrIdempotencyKeyReused, http.StatusUnprocessableEntity, "idempotency-key-reused"},
	}
	for _, c := range cases {
		acc := &fakeAccept{err: c.err}
		rec := call(invitationsRouter(&InvitationHandlers{Accept: acc}), http.MethodPost, "/v1/invitations/inv_abc/accept", "", "Idempotency-Key: k")
		if p := problemOf(t, rec); rec.Code != c.status || p.Type != problem.TypeBase+c.slug {
			t.Errorf("err=%v: code=%d problem=%+v", c.err, rec.Code, p)
		}
		if acc.gotToken != "inv_abc" {
			t.Errorf("token recibido=%q", acc.gotToken)
		}
	}
}
