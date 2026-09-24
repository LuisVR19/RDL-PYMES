package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/platform/health"
	"rdl/platform-api/pkg/tenancy"
)

type fakeBranchUseCases struct {
	createErr error
	listGot   app.BranchQuery
}

func (f *fakeBranchUseCases) create(_ context.Context, _ tenancy.Context, _ string, in branch.NewInput) (app.CreateBranchResult, error) {
	if f.createErr != nil {
		return app.CreateBranchResult{}, f.createErr
	}
	b, err := branch.New(uuid.New(), in)
	return app.CreateBranchResult{Branch: b}, err
}

type createBranchFunc func(context.Context, tenancy.Context, string, branch.NewInput) (app.CreateBranchResult, error)

func (f createBranchFunc) Execute(ctx context.Context, t tenancy.Context, k string, in branch.NewInput) (app.CreateBranchResult, error) {
	return f(ctx, t, k, in)
}

type listBranchesFunc func(context.Context, tenancy.Context, app.BranchQuery) (app.BranchPage, error)

func (f listBranchesFunc) Execute(ctx context.Context, t tenancy.Context, q app.BranchQuery) (app.BranchPage, error) {
	return f(ctx, t, q)
}

func branchesRouter(f *fakeBranchUseCases) http.Handler {
	log := slog.New(slog.DiscardHandler)
	return NewRouter(Deps{
		Log:         log,
		Health:      health.New(log, time.Second),
		Verifier:    stubVerifier{id: tenancy.Identity{Subject: "s", OrganizationID: uuid.New()}},
		Memberships: stubMemberships{},
		Branches: &BranchHandlers{
			Create: createBranchFunc(f.create),
			List: listBranchesFunc(func(_ context.Context, _ tenancy.Context, q app.BranchQuery) (app.BranchPage, error) {
				f.listGot = q
				return app.BranchPage{}, nil
			}),
		},
	})
}

func TestCreateBranch(t *testing.T) {
	f := &fakeBranchUseCases{}
	h := branchesRouter(f)

	rec := call(h, http.MethodPost, "/v1/organizations/current/branches", `{"code":"SJ","name":"San José"}`, "Idempotency-Key: k")
	if rec.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, "/v1/organizations/current/branches", `{"code":"SJ","name":"San José"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin Idempotency-Key: code=%d", rec.Code)
	}
	// El dominio rechaza el código con espacios: 422 con el campo señalado.
	rec = call(h, http.MethodPost, "/v1/organizations/current/branches", `{"code":"con espacio","name":"X"}`, "Idempotency-Key: k2")
	if p := problemOf(t, rec); rec.Code != http.StatusUnprocessableEntity || len(p.Errors) != 1 || p.Errors[0].Field != "code" {
		t.Fatalf("code=%d problem=%+v", rec.Code, p)
	}
}

func TestCreateBranchConflictShowsDetail(t *testing.T) {
	f := &fakeBranchUseCases{createErr: fmt.Errorf("%w: ya existe una sucursal con el código \"SJ\"", app.ErrConflict)}
	rec := call(branchesRouter(f), http.MethodPost, "/v1/organizations/current/branches", `{"code":"SJ","name":"X"}`, "Idempotency-Key: k")
	if p := problemOf(t, rec); rec.Code != http.StatusConflict || p.Type != problem.TypeBase+"conflict" || p.Detail != `ya existe una sucursal con el código "SJ"` {
		t.Fatalf("code=%d problem=%+v", rec.Code, p)
	}
}

func TestListBranchesActiveFilter(t *testing.T) {
	f := &fakeBranchUseCases{}
	h := branchesRouter(f)
	call(h, http.MethodGet, "/v1/organizations/current/branches?active=false", "")
	if f.listGot.Active == nil || *f.listGot.Active {
		t.Fatalf("active=%v", f.listGot.Active)
	}
	rec := call(h, http.MethodGet, "/v1/organizations/current/branches?active=quizas", "")
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d", rec.Code)
	}
}
