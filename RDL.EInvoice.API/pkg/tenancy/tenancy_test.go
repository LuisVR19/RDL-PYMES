package tenancy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeVerifier struct {
	id  Identity
	err error
}

func (f fakeVerifier) Verify(context.Context, string) (Identity, error) { return f.id, f.err }

type fakeResolver struct {
	calls int
	m     Membership
	err   error
}

func (f *fakeResolver) ActiveMembership(context.Context, string, uuid.UUID) (Membership, error) {
	f.calls++
	return f.m, f.err
}

func recordErr(got *error) ErrorWriter {
	return func(w http.ResponseWriter, _ *http.Request, err error) {
		*got = err
		w.WriteHeader(http.StatusTeapot)
	}
}

func serve(h http.Handler, header string, mutate ...func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/organizations/current", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	for _, m := range mutate {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAuthenticateRequiresBearer(t *testing.T) {
	for _, header := range []string{"", "Basic abc", "Bearer ", "Bearer"} {
		var got error
		h := Authenticate(fakeVerifier{}, recordErr(&got))(http.NotFoundHandler())
		serve(h, header)
		if !errors.Is(got, ErrUnauthenticated) {
			t.Errorf("header %q: err=%v", header, got)
		}
	}
}

func TestAuthenticateRejectsInvalidToken(t *testing.T) {
	var got error
	h := Authenticate(fakeVerifier{err: errors.New("firma inválida")}, recordErr(&got))(http.NotFoundHandler())
	serve(h, "Bearer x.y.z")
	if !errors.Is(got, ErrUnauthenticated) {
		t.Fatalf("err=%v", got)
	}
}

func chain(v TokenVerifier, r MembershipResolver, got *error, final http.Handler) http.Handler {
	return Authenticate(v, recordErr(got))(RequireOrganization(r, recordErr(got))(final))
}

func TestRequireOrganizationBuildsTenantFromTokenAndMembership(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	res := &fakeResolver{m: Membership{UserID: user, Roles: []string{"admin"}}}
	var got error
	var tenant Context
	final := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		tenant, _ = From(r.Context())
	})

	serve(chain(fakeVerifier{id: Identity{Subject: "sub-1", OrganizationID: org}}, res, &got, final), "Bearer t")

	if got != nil {
		t.Fatalf("err=%v", got)
	}
	if tenant.OrganizationID() != org || tenant.UserID() != user || tenant.Subject() != "sub-1" || tenant.Roles()[0] != "admin" {
		t.Fatalf("tenant=%+v", tenant)
	}
}

func TestRequireOrganizationWithoutOrgClaimIsRejected(t *testing.T) {
	res := &fakeResolver{}
	var got error
	serve(chain(fakeVerifier{id: Identity{Subject: "sub-1"}}, res, &got, http.NotFoundHandler()), "Bearer t")
	if !errors.Is(got, ErrNoActiveOrganization) || res.calls != 0 {
		t.Fatalf("err=%v calls=%d", got, res.calls)
	}
}

func TestRequireOrganizationWithRevokedMembershipIsRejected(t *testing.T) {
	res := &fakeResolver{err: ErrNoMembership}
	var got error
	serve(chain(fakeVerifier{id: Identity{Subject: "sub-1", OrganizationID: uuid.New()}}, res, &got, http.NotFoundHandler()), "Bearer t")
	if !errors.Is(got, ErrNoMembership) {
		t.Fatalf("err=%v", got)
	}
}

// Criterio de aislamiento 6: un organization_id en body, query o header no cambia de tenant.
func TestClientSuppliedOrganizationIsIgnored(t *testing.T) {
	tokenOrg, otherOrg := uuid.New(), uuid.New()
	res := &fakeResolver{m: Membership{UserID: uuid.New(), Roles: []string{"owner"}}}
	var got error
	var tenant Context
	final := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { tenant, _ = From(r.Context()) })

	serve(chain(fakeVerifier{id: Identity{Subject: "s", OrganizationID: tokenOrg}}, res, &got, final), "Bearer t",
		func(r *http.Request) {
			r.Header.Set("X-Organization-Id", otherOrg.String())
			q := r.URL.Query()
			q.Set("organization_id", otherOrg.String())
			r.URL.RawQuery = q.Encode()
		})

	if tenant.OrganizationID() != tokenOrg {
		t.Fatalf("tenant=%s, se esperaba la del token %s", tenant.OrganizationID(), tokenOrg)
	}
}

func TestTenantContextIsImmutable(t *testing.T) {
	roles := []string{"admin"}
	c := NewContext(uuid.New(), "s", uuid.New(), roles)
	roles[0] = "owner"
	c.Roles()[0] = "owner"
	if c.Roles()[0] != "admin" {
		t.Fatal("el TenantContext cambió desde fuera")
	}
}

func TestCachedResolverCachesOnlySuccessWithinTTL(t *testing.T) {
	now := time.Unix(0, 0)
	inner := &fakeResolver{m: Membership{UserID: uuid.New(), Roles: []string{"admin"}}}
	c := NewCachedResolver(inner, 30*time.Second)
	c.now = func() time.Time { return now }
	org := uuid.New()

	for range 3 {
		if _, err := c.ActiveMembership(t.Context(), "s", org); err != nil {
			t.Fatal(err)
		}
	}
	if inner.calls != 1 {
		t.Fatalf("calls=%d, se esperaba 1 dentro del TTL", inner.calls)
	}

	now = now.Add(31 * time.Second)
	inner.err = ErrNoMembership
	if _, err := c.ActiveMembership(t.Context(), "s", org); !errors.Is(err, ErrNoMembership) {
		t.Fatalf("tras el TTL una membresía revocada debe fallar, err=%v", err)
	}
	inner.err = nil
	_, _ = c.ActiveMembership(t.Context(), "s", org)
	if inner.calls != 3 {
		t.Fatalf("los errores no deben cachearse, calls=%d", inner.calls)
	}
}

func TestCachedResolverInvalidate(t *testing.T) {
	inner := &fakeResolver{m: Membership{UserID: uuid.New()}}
	c := NewCachedResolver(inner, time.Minute)
	org := uuid.New()
	_, _ = c.ActiveMembership(t.Context(), "s", org)
	c.Invalidate("s", org)
	_, _ = c.ActiveMembership(t.Context(), "s", org)
	if inner.calls != 2 {
		t.Fatalf("calls=%d", inner.calls)
	}
}
