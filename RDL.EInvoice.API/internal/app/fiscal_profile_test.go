package app

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/einvoice-api/pkg/tenancy"
)

type fakeTxm struct {
	profiles map[uuid.UUID]FiscalProfile
	calls    int
}

func (f *fakeTxm) WithinTenantTx(ctx context.Context, _ tenancy.Context, fn func(context.Context, Tx) error) error {
	f.calls++
	return fn(ctx, fakeTx{f})
}

type fakeTx struct{ f *fakeTxm }

func (t fakeTx) FiscalProfiles() FiscalProfileReader { return t }
func (t fakeTx) FiscalStatuses() FiscalStatusReader  { return nil }
func (t fakeTx) FiscalConfig() FiscalConfigStore     { return nil }
func (t fakeTx) Catalogs() CatalogReader             { return nil }
func (t fakeTx) Audit() AuditRecorder                { return nil }

func (t fakeTx) Get(_ context.Context, org uuid.UUID) (FiscalProfile, error) {
	p, ok := t.f.profiles[org]
	if !ok {
		return FiscalProfile{}, ErrNotFound
	}
	return p, nil
}

func TestGetFiscalProfile(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	txm := &fakeTxm{profiles: map[uuid.UUID]FiscalProfile{orgA: {LegalName: "A", Environment: "sandbox"}}}
	uc := NewGetFiscalProfile(txm)

	p, err := uc.Execute(context.Background(), tenancy.NewContext(uuid.New(), "s", orgA, []string{"accountant"}))
	if err != nil || p.LegalName != "A" {
		t.Fatalf("perfil de A = %+v, %v", p, err)
	}
	if _, err := uc.Execute(context.Background(), tenancy.NewContext(uuid.New(), "s", orgB, []string{"owner"})); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sin perfil: err = %v, se esperaba ErrNotFound", err)
	}
	calls := txm.calls
	if _, err := uc.Execute(context.Background(), tenancy.NewContext(uuid.New(), "s", orgA, []string{"biller"})); !errors.Is(err, ErrForbidden) {
		t.Fatalf("biller: err = %v, se esperaba ErrForbidden", err)
	}
	if txm.calls != calls {
		t.Error("un rol sin permiso no debe abrir transacción")
	}
}
