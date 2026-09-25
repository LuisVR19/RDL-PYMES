package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/pkg/tenancy"
)

type fakeTx struct {
	tz        string
	tzCalls   int
	items     []ReceivableView
	gotOrg    uuid.UUID
	gotQuery  ReceivableQuery
	listCalls int
}

func (f *fakeTx) WithinTenantTx(ctx context.Context, _ tenancy.Context, fn func(context.Context, Tx) error) error {
	return fn(ctx, f)
}
func (f *fakeTx) Organizations() OrganizationReader { return f }
func (f *fakeTx) Receivables() ReceivableReader     { return f }

func (f *fakeTx) Timezone(context.Context, uuid.UUID) (string, error) {
	f.tzCalls++
	return f.tz, nil
}

func (f *fakeTx) List(_ context.Context, org uuid.UUID, q ReceivableQuery) ([]ReceivableView, error) {
	f.listCalls++
	f.gotOrg, f.gotQuery = org, q
	return f.items[:min(len(f.items), q.Limit)], nil
}

func tenant(roles ...string) tenancy.Context {
	return tenancy.NewContext(uuid.New(), "sub", uuid.New(), roles)
}

func views(n int) []ReceivableView {
	out := make([]ReceivableView, n)
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := range out {
		out[i] = ReceivableView{ID: uuid.New(), CreatedAt: base.Add(-time.Duration(i) * time.Hour)}
	}
	return out
}

func TestListReceivablesRequiresReadPermission(t *testing.T) {
	f := &fakeTx{}
	uc := NewListReceivables(f)
	for _, roles := range [][]string{{"biller"}, {}, {"desconocido"}} {
		if _, err := uc.Execute(t.Context(), tenant(roles...), ReceivableQuery{}); !errors.Is(err, ErrForbidden) {
			t.Errorf("roles %v: err=%v, se esperaba ErrForbidden", roles, err)
		}
	}
	if f.listCalls != 0 {
		t.Fatal("sin permiso no debe consultar la base")
	}
}

func TestListReceivablesUsesTenantOrganizationAndPaginates(t *testing.T) {
	f := &fakeTx{items: views(3)}
	tc := tenant("collector")
	page, err := NewListReceivables(f).Execute(t.Context(), tc, ReceivableQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotOrg != tc.OrganizationID() {
		t.Fatalf("org=%s, se esperaba la del TenantContext", f.gotOrg)
	}
	if f.gotQuery.Limit != 3 {
		t.Fatalf("debe pedir limit+1, pidió %d", f.gotQuery.Limit)
	}
	if len(page.Items) != 2 || page.Next == nil || page.Next.ID != f.items[1].ID || !page.Next.At.Equal(f.items[1].CreatedAt) {
		t.Fatalf("page=%+v", page)
	}
}

func TestListReceivablesLastPageHasNoCursor(t *testing.T) {
	f := &fakeTx{items: views(2)}
	page, err := NewListReceivables(f).Execute(t.Context(), tenant("read_only"), ReceivableQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Limit != DefaultPageSize+1 || page.Next != nil || len(page.Items) != 2 {
		t.Fatalf("limit=%d page=%+v", f.gotQuery.Limit, page)
	}
}

func TestListReceivablesOverdueUsesOrganizationToday(t *testing.T) {
	f := &fakeTx{tz: "America/Costa_Rica"}
	uc := NewListReceivables(f)
	uc.now = func() time.Time { return time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC) }
	yes := true
	if _, err := uc.Execute(t.Context(), tenant("owner"), ReceivableQuery{Overdue: &yes}); err != nil {
		t.Fatal(err)
	}
	if got := f.gotQuery.Today.String(); got != "2026-09-24" {
		t.Fatalf("today=%s, se esperaba el día en Costa Rica (2026-09-24)", got)
	}
}

func TestListReceivablesWithoutOverdueSkipsTimezone(t *testing.T) {
	f := &fakeTx{}
	if _, err := NewListReceivables(f).Execute(t.Context(), tenant("owner"), ReceivableQuery{}); err != nil {
		t.Fatal(err)
	}
	if f.tzCalls != 0 || !f.gotQuery.Today.IsZero() {
		t.Fatal("sin filtro overdue no hace falta la zona horaria")
	}
}
