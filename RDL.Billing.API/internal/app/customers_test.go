package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/permission"
)

var ctx = context.Background()

func seedCustomers(f *fakeTx, org uuid.UUID, n int) {
	base := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for i := range n {
		f.state.customers = append(f.state.customers, customer.Customer{
			ID: uuid.New(), OrganizationID: org, LegalName: "Cliente", IsActive: true,
			Identification: customer.Identification{TypeCode: "01", Number: uuid.NewString()[:9]},
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
		})
	}
}

func newCustomerInput(number string) customer.NewInput {
	return customer.NewInput{IdentificationTypeCode: "01", IdentificationNumber: number, LegalName: "Ana Pérez"}
}

// --- listar y ver ---

func TestListCustomersEveryRoleCanRead(t *testing.T) {
	org := uuid.New()
	for _, role := range permission.AllRoles {
		f := newFakeTx()
		seedCustomers(f, org, 2)
		page, err := NewListCustomers(f).Execute(ctx, tenant(org, string(role)), CustomerQuery{})
		if err != nil || len(page.Items) != 2 {
			t.Fatalf("rol %s: items=%d err=%v", role, len(page.Items), err)
		}
	}
}

func TestListCustomersWithoutKnownRoleIsForbidden(t *testing.T) {
	f := newFakeTx()
	_, err := NewListCustomers(f).Execute(ctx, tenant(uuid.New(), "auditor-externo"), CustomerQuery{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, se esperaba ErrForbidden", err)
	}
	if f.calls != 0 {
		t.Fatal("un rol sin permiso no debe abrir transacción")
	}
}

func TestListCustomersOnlyActiveOrganization(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	f := newFakeTx()
	seedCustomers(f, a, 1)
	seedCustomers(f, b, 3)
	page, err := NewListCustomers(f).Execute(ctx, tenant(a, "read_only"), CustomerQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if f.org != a || len(page.Items) != 1 || page.Items[0].OrganizationID != a {
		t.Fatalf("se filtraron clientes de otra organización: sesión=%v items=%+v", f.org, page.Items)
	}
}

func TestListCustomersPaginates(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	seedCustomers(f, org, 5)
	uc := NewListCustomers(f)
	tn := tenant(org, "biller")

	var seen []uuid.UUID
	q := CustomerQuery{Limit: 2}
	for range 5 {
		page, err := uc.Execute(ctx, tn, q)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range page.Items {
			seen = append(seen, c.ID)
		}
		if page.Next == nil {
			break
		}
		q.After = page.Next
	}
	if len(seen) != 5 {
		t.Fatalf("se recorrieron %d clientes, se esperaban 5 sin repetir", len(seen))
	}
	for i, c := range f.state.customers {
		if seen[i] != c.ID {
			t.Fatalf("orden inestable en la posición %d", i)
		}
	}
}

func TestListCustomersLimitIsClamped(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	seedCustomers(f, org, MaxPageSize+5)
	page, err := NewListCustomers(f).Execute(ctx, tenant(org, "owner"), CustomerQuery{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != MaxPageSize || page.Next == nil {
		t.Fatalf("items=%d next=%v", len(page.Items), page.Next)
	}
}

func TestGetCustomerOfAnotherOrganizationIsNotFound(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	f := newFakeTx()
	seedCustomers(f, b, 1)
	_, err := NewGetCustomer(f).Execute(ctx, tenant(a, "owner"), f.state.customers[0].ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, se esperaba ErrNotFound", err)
	}
}

// --- crear ---

func TestCreateCustomer(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "biller")
	res, err := NewCreateCustomer(f).Execute(ctx, tn, "k1", newCustomerInput("112340567"))
	if err != nil {
		t.Fatal(err)
	}
	c := res.Customer
	if res.Replayed || c.ID == uuid.Nil || c.OrganizationID != org || c.CreatedByUserID != tn.UserID() || !c.IsActive {
		t.Fatalf("resultado = %+v", res)
	}
	if len(f.state.audit) != 1 || f.state.audit[0].Action != "customer.created" || f.state.audit[0].EntityID != c.ID ||
		f.state.audit[0].OrganizationID != org || f.state.audit[0].ActorUserID != tn.UserID() {
		t.Fatalf("auditoría = %+v", f.state.audit)
	}
}

func TestCreateCustomerRolesThatCannotWrite(t *testing.T) {
	for _, role := range []string{"collector", "accountant", "read_only"} {
		f := newFakeTx()
		_, err := NewCreateCustomer(f).Execute(ctx, tenant(uuid.New(), role), "k", newCustomerInput("1"))
		if !errors.Is(err, ErrForbidden) || len(f.state.customers) != 0 {
			t.Fatalf("rol %s: err=%v", role, err)
		}
	}
}

func TestCreateCustomerSameKeySameBodyReplays(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewCreateCustomer(f)
	tn := tenant(org, "admin")
	first, err := uc.Execute(ctx, tn, "k1", newCustomerInput("112340567"))
	if err != nil {
		t.Fatal(err)
	}
	// Mismo contenido con espacios distintos: el hash es del comando normalizado, no de los bytes.
	in := newCustomerInput(" 112340567 ")
	second, err := uc.Execute(ctx, tn, "k1", in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || second.Customer.ID != first.Customer.ID {
		t.Fatalf("el reintento debe devolver el mismo cliente: %+v", second)
	}
	if len(f.state.customers) != 1 || len(f.state.audit) != 1 {
		t.Fatalf("el reintento no crea nada: clientes=%d audit=%d", len(f.state.customers), len(f.state.audit))
	}
}

func TestCreateCustomerSameKeyOtherBodyIsRejected(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewCreateCustomer(f)
	tn := tenant(org, "admin")
	if _, err := uc.Execute(ctx, tn, "k1", newCustomerInput("1")); err != nil {
		t.Fatal(err)
	}
	_, err := uc.Execute(ctx, tn, "k1", newCustomerInput("2"))
	if !errors.Is(err, ErrIdempotencyKeyReused) || len(f.state.customers) != 1 {
		t.Fatalf("err=%v clientes=%d", err, len(f.state.customers))
	}
}

func TestCreateCustomerKeyIsPerOrganization(t *testing.T) {
	f := newFakeTx()
	uc := NewCreateCustomer(f)
	a, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "k1", newCustomerInput("1"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "k1", newCustomerInput("1"))
	if err != nil || b.Replayed || b.Customer.ID == a.Customer.ID {
		t.Fatalf("la misma clave en otra organización es otra operación: err=%v %+v", err, b)
	}
}

func TestCreateCustomerDuplicateIdentificationRollsBack(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewCreateCustomer(f)
	tn := tenant(org, "biller")
	if _, err := uc.Execute(ctx, tn, "k1", newCustomerInput("112340567")); err != nil {
		t.Fatal(err)
	}
	_, err := uc.Execute(ctx, tn, "k2", newCustomerInput("112340567"))
	if !errors.Is(err, ErrCustomerIdentificationTaken) {
		t.Fatalf("err = %v", err)
	}
	// La clave k2 no quedó reservada: el mismo POST, corregido, puede reintentarse con ella.
	if _, err := uc.Execute(ctx, tn, "k2", newCustomerInput("999")); err != nil {
		t.Fatalf("la reserva de la clave debió revertirse con el error: %v", err)
	}
	if len(f.state.audit) != 2 {
		t.Fatalf("auditoría = %d eventos, se esperaban 2", len(f.state.audit))
	}
}

func TestCreateCustomerSameIdentificationInOtherOrganization(t *testing.T) {
	f := newFakeTx()
	uc := NewCreateCustomer(f)
	if _, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "k1", newCustomerInput("1")); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, tenant(uuid.New(), "owner"), "k2", newCustomerInput("1")); err != nil {
		t.Fatalf("la unicidad es por organización: %v", err)
	}
}

func TestCreateCustomerInvalidDoesNotOpenTransaction(t *testing.T) {
	f := newFakeTx()
	_, err := NewCreateCustomer(f).Execute(ctx, tenant(uuid.New(), "owner"), "k", customer.NewInput{})
	if !errors.Is(err, customer.ErrInvalid) || f.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, f.calls)
	}
}

// --- editar ---

func TestUpdateCustomer(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "biller")
	created, _ := NewCreateCustomer(f).Execute(ctx, tn, "k", newCustomerInput("1"))
	name := "Ana Pérez Solís"
	c, err := NewUpdateCustomer(f).Execute(ctx, tn, created.Customer.ID, customer.Patch{LegalName: &name})
	if err != nil || c.LegalName != name {
		t.Fatalf("c=%+v err=%v", c, err)
	}
	last := f.state.audit[len(f.state.audit)-1]
	before, after := last.Before.(map[string]any), last.After.(map[string]any)
	if last.Action != "customer.updated" || len(before) != 1 || before["legalName"] != "Ana Pérez" || after["legalName"] != name {
		t.Fatalf("auditoría = %+v", last)
	}
}

func TestUpdateCustomerDeactivateAndReactivate(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "admin")
	created, _ := NewCreateCustomer(f).Execute(ctx, tn, "k", newCustomerInput("1"))
	uc := NewUpdateCustomer(f)
	off, on := false, true
	if _, err := uc.Execute(ctx, tn, created.Customer.ID, customer.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, tn, created.Customer.ID, customer.Patch{IsActive: &on}); err != nil {
		t.Fatal(err)
	}
	actions := []string{f.state.audit[1].Action, f.state.audit[2].Action}
	if actions[0] != "customer.deactivated" || actions[1] != "customer.reactivated" {
		t.Fatalf("acciones = %v", actions)
	}
}

func TestUpdateCustomerNoChangesWritesNothing(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "owner")
	created, _ := NewCreateCustomer(f).Execute(ctx, tn, "k", newCustomerInput("1"))
	same := created.Customer.LegalName
	if _, err := NewUpdateCustomer(f).Execute(ctx, tn, created.Customer.ID, customer.Patch{LegalName: &same}); err != nil {
		t.Fatal(err)
	}
	if len(f.state.audit) != 1 {
		t.Fatalf("un PATCH sin cambios no se audita: %d eventos", len(f.state.audit))
	}
}

func TestUpdateCustomerOfAnotherOrganizationIsNotFound(t *testing.T) {
	f := newFakeTx()
	created, _ := NewCreateCustomer(f).Execute(ctx, tenant(uuid.New(), "owner"), "k", newCustomerInput("1"))
	name := "Intruso"
	_, err := NewUpdateCustomer(f).Execute(ctx, tenant(uuid.New(), "owner"), created.Customer.ID, customer.Patch{LegalName: &name})
	if !errors.Is(err, ErrNotFound) || f.state.customers[0].LegalName != "Ana Pérez" {
		t.Fatalf("err=%v cliente=%+v", err, f.state.customers[0])
	}
}

func TestUpdateCustomerForbiddenAndInvalid(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	created, _ := NewCreateCustomer(f).Execute(ctx, tenant(org, "owner"), "k", newCustomerInput("1"))
	name := "X"
	if _, err := NewUpdateCustomer(f).Execute(ctx, tenant(org, "read_only"), created.Customer.ID, customer.Patch{LegalName: &name}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read_only: err = %v", err)
	}
	blank := ""
	if _, err := NewUpdateCustomer(f).Execute(ctx, tenant(org, "owner"), created.Customer.ID, customer.Patch{LegalName: &blank}); !errors.Is(err, customer.ErrInvalid) {
		t.Fatalf("nombre vacío: err = %v", err)
	}
	if f.state.customers[0].LegalName != "Ana Pérez" {
		t.Fatal("un PATCH rechazado no cambia nada")
	}
}
