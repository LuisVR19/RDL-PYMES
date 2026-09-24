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
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/tenancy"
)

type fakeListMembers struct {
	got  app.MemberQuery
	page app.MemberPage
}

func (f *fakeListMembers) Execute(_ context.Context, _ tenancy.Context, q app.MemberQuery) (app.MemberPage, error) {
	f.got = q
	return f.page, nil
}

type fakeUpdateMember struct {
	err     error
	gotUser uuid.UUID
	got     membership.Change
}

func (f *fakeUpdateMember) Execute(_ context.Context, _ tenancy.Context, userID uuid.UUID, c membership.Change) (membership.Member, error) {
	f.gotUser, f.got = userID, c
	return membership.Member{UserID: userID, Status: membership.StatusActive, Roles: []membership.Role{membership.RoleCollector}}, f.err
}

func membersRouter(m *MemberHandlers) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    stubVerifier{id: tenancy.Identity{Subject: "s", OrganizationID: uuid.New()}},
		Memberships: stubMemberships{},
		Members:     m,
	})
}

func TestListMembersParsesQueryAndReturnsOpaqueCursor(t *testing.T) {
	next := app.PageCursor{At: time.Date(2026, 9, 1, 12, 0, 0, 123, time.UTC), ID: uuid.New()}
	list := &fakeListMembers{page: app.MemberPage{
		Items: []membership.Member{{UserID: uuid.New(), Email: "a@x.test", Status: membership.StatusActive, Roles: []membership.Role{membership.RoleAdmin}}},
		Next:  &next,
	}}
	h := membersRouter(&MemberHandlers{List: list})

	rec := call(h, http.MethodGet, "/v1/organizations/current/users?limit=10&status=suspended", "")
	var body memberPageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("code=%d err=%v", rec.Code, err)
	}
	if list.got.Limit != 10 || list.got.Status != membership.StatusSuspended || body.NextCursor == nil || len(body.Items) != 1 {
		t.Fatalf("query=%+v body=%+v", list.got, body)
	}

	// El cursor devuelto se acepta de vuelta y se decodifica a la misma posición.
	call(h, http.MethodGet, "/v1/organizations/current/users?cursor="+*body.NextCursor, "")
	if list.got.After == nil || !list.got.After.At.Equal(next.At) || list.got.After.ID != next.ID {
		t.Fatalf("after=%+v, want %+v", list.got.After, next)
	}
}

func TestListMembersValidatesQuery(t *testing.T) {
	h := membersRouter(&MemberHandlers{List: &fakeListMembers{}})
	for _, q := range []string{"limit=0", "limit=101", "limit=diez", "status=deleted", "cursor=bm90LWpzb24", "cursor=eyJ4IjoxfQ"} {
		rec := call(h, http.MethodGet, "/v1/organizations/current/users?"+q, "")
		if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || p.Type != problem.TypeBase+"validation" {
			t.Errorf("%s: code=%d problem=%+v", q, rec.Code, p)
		}
	}
}

func TestUpdateMemberRequest(t *testing.T) {
	up := &fakeUpdateMember{}
	h := membersRouter(&MemberHandlers{Update: up})
	target := uuid.New()

	rec := call(h, http.MethodPatch, "/v1/organizations/current/users/"+target.String(), `{"role":"collector","status":"active"}`)
	if rec.Code != http.StatusOK || up.gotUser != target || *up.got.Role != membership.RoleCollector || *up.got.Status != membership.StatusActive {
		t.Fatalf("code=%d got=%+v", rec.Code, up.got)
	}

	rec = call(h, http.MethodPatch, "/v1/organizations/current/users/no-es-uuid", `{"role":"admin"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("id inválido: code=%d", rec.Code)
	}

	rec = call(h, http.MethodPatch, "/v1/organizations/current/users/"+target.String(), `{"role":"superadmin"}`)
	if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field != "role" {
		t.Fatalf("rol inválido: code=%d problem=%+v", rec.Code, p)
	}
}

func TestUpdateMemberMapsBusinessRules(t *testing.T) {
	cases := []struct {
		err    error
		status int
		slug   string
	}{
		{membership.ErrLastOwner, http.StatusConflict, "last-owner"},
		{membership.ErrOwnerRequired, http.StatusForbidden, "owner-required"},
		{app.ErrForbidden, http.StatusForbidden, "forbidden"},
		{app.ErrNotFound, http.StatusNotFound, "not-found"},
		{membership.ErrEmptyChange, http.StatusUnprocessableEntity, "validation"},
	}
	for _, c := range cases {
		h := membersRouter(&MemberHandlers{Update: &fakeUpdateMember{err: c.err}})
		rec := call(h, http.MethodPatch, "/v1/organizations/current/users/"+uuid.NewString(), `{"status":"suspended"}`)
		if p := problemOf(t, rec); rec.Code != c.status || p.Type != problem.TypeBase+c.slug {
			t.Errorf("err=%v: code=%d problem=%+v", c.err, rec.Code, p)
		}
	}
}
