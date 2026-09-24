package app

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/product"
)

func newProductInput(code string) product.NewInput {
	return product.NewInput{
		Code: code, Description: "Consultoría", CabysCode: "8314100000100", UnitOfMeasureCode: "Sp",
		UnitPrice: money.MustAmountForTest("25000.5"), Currency: money.MustCurrencyForTest("CRC"), IsService: true,
		Taxes: []product.Tax{{TypeCode: "01", RateCode: "08"}},
	}
}

func TestCreateProduct(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "biller")
	res, err := NewCreateProduct(f).Execute(ctx, tn, "k1", newProductInput("SERV-01"))
	if err != nil {
		t.Fatal(err)
	}
	p := res.Product
	if res.Replayed || p.ID == uuid.Nil || p.OrganizationID != org || len(p.Taxes) != 1 || !p.IsActive {
		t.Fatalf("resultado = %+v", res)
	}
	a := f.state.audit
	if len(a) != 1 || a[0].Action != "product.created" || a[0].EntityID != p.ID ||
		a[0].After.(map[string]any)["taxes"] != "01:08" || a[0].After.(map[string]any)["unitPrice"] != "25000.5" {
		t.Fatalf("auditoría = %+v", a)
	}
}

func TestCreateProductIdempotency(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewCreateProduct(f)
	tn := tenant(org, "owner")
	first, err := uc.Execute(ctx, tn, "k1", newProductInput("SERV-01"))
	if err != nil {
		t.Fatal(err)
	}
	// El mismo precio con otros ceros es la misma petición.
	in := newProductInput("SERV-01")
	in.UnitPrice = money.MustAmountForTest("25000.50000")
	again, err := uc.Execute(ctx, tn, "k1", in)
	if err != nil || !again.Replayed || again.Product.ID != first.Product.ID || len(f.state.products) != 1 {
		t.Fatalf("reintento: %+v err=%v", again, err)
	}
	in.UnitPrice = money.MustAmountForTest("1")
	if _, err := uc.Execute(ctx, tn, "k1", in); !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Fatalf("otro contenido con la misma clave: err = %v", err)
	}
}

func TestCreateProductCodeTaken(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	uc := NewCreateProduct(f)
	tn := tenant(org, "admin")
	if _, err := uc.Execute(ctx, tn, "k1", newProductInput("SERV-01")); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Execute(ctx, tn, "k2", newProductInput("SERV-01")); !errors.Is(err, ErrProductCodeTaken) {
		t.Fatalf("err = %v", err)
	}
	if len(f.state.products) != 1 || len(f.state.audit) != 1 {
		t.Fatal("el 409 no deja nada escrito")
	}
	if _, err := uc.Execute(ctx, tenant(uuid.New(), "admin"), "k2", newProductInput("SERV-01")); err != nil {
		t.Fatalf("el código es único por organización: %v", err)
	}
}

func TestProductRoles(t *testing.T) {
	for _, role := range []string{"collector", "accountant", "read_only"} {
		f := newFakeTx()
		if _, err := NewCreateProduct(f).Execute(ctx, tenant(uuid.New(), role), "k", newProductInput("A")); !errors.Is(err, ErrForbidden) {
			t.Fatalf("rol %s crea: err = %v", role, err)
		}
	}
	org := uuid.New()
	f := newFakeTx()
	created, _ := NewCreateProduct(f).Execute(ctx, tenant(org, "owner"), "k", newProductInput("A"))
	for _, role := range []string{"collector", "accountant", "read_only"} {
		if _, err := NewGetProduct(f).Execute(ctx, tenant(org, role), created.Product.ID); err != nil {
			t.Fatalf("rol %s lee: %v", role, err)
		}
	}
}

func TestUpdateProduct(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "biller")
	created, _ := NewCreateProduct(f).Execute(ctx, tn, "k", newProductInput("SERV-01"))
	price := money.MustAmountForTest("30000")
	taxes := []product.Tax{{TypeCode: "01", RateCode: "04"}}
	p, err := NewUpdateProduct(f).Execute(ctx, tn, created.Product.ID, product.Patch{UnitPrice: &price, Taxes: &taxes})
	if err != nil || p.UnitPrice.String() != "30000" || p.Taxes[0].RateCode != "04" {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	last := f.state.audit[len(f.state.audit)-1]
	before := last.Before.(map[string]any)
	if last.Action != "product.updated" || len(before) != 2 || before["unitPrice"] != "25000.5" || before["taxes"] != "01:08" {
		t.Fatalf("auditoría = %+v", last)
	}
}

func TestUpdateProductDeactivateAndNoop(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "admin")
	created, _ := NewCreateProduct(f).Execute(ctx, tn, "k", newProductInput("SERV-01"))
	same := money.MustAmountForTest("25000.500")
	if _, err := NewUpdateProduct(f).Execute(ctx, tn, created.Product.ID, product.Patch{UnitPrice: &same}); err != nil {
		t.Fatal(err)
	}
	if len(f.state.audit) != 1 {
		t.Fatal("el mismo precio escrito de otra forma no es un cambio")
	}
	off := false
	if _, err := NewUpdateProduct(f).Execute(ctx, tn, created.Product.ID, product.Patch{IsActive: &off}); err != nil {
		t.Fatal(err)
	}
	if f.state.audit[1].Action != "product.deactivated" || f.state.products[0].IsActive {
		t.Fatalf("auditoría = %+v", f.state.audit[1])
	}
}

func TestUpdateProductCodeTakenRollsBack(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "owner")
	a, _ := NewCreateProduct(f).Execute(ctx, tn, "k1", newProductInput("A"))
	if _, err := NewCreateProduct(f).Execute(ctx, tn, "k2", newProductInput("B")); err != nil {
		t.Fatal(err)
	}
	code := "B"
	if _, err := NewUpdateProduct(f).Execute(ctx, tn, a.Product.ID, product.Patch{Code: &code}); !errors.Is(err, ErrProductCodeTaken) {
		t.Fatalf("err = %v", err)
	}
	if f.state.products[0].Code != "A" || len(f.state.audit) != 2 {
		t.Fatal("un 409 no cambia ni audita nada")
	}
}

func TestProductOfAnotherOrganizationIsNotFound(t *testing.T) {
	f := newFakeTx()
	created, _ := NewCreateProduct(f).Execute(ctx, tenant(uuid.New(), "owner"), "k", newProductInput("A"))
	other := tenant(uuid.New(), "owner")
	if _, err := NewGetProduct(f).Execute(ctx, other, created.Product.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: err = %v", err)
	}
	off := false
	if _, err := NewUpdateProduct(f).Execute(ctx, other, created.Product.ID, product.Patch{IsActive: &off}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update: err = %v", err)
	}
	if !f.state.products[0].IsActive {
		t.Fatal("el producto de la otra organización quedó intacto")
	}
}

func TestListProductsPaginates(t *testing.T) {
	org := uuid.New()
	f := newFakeTx()
	tn := tenant(org, "read_only")
	for _, code := range []string{"A", "B", "C"} {
		if _, err := NewCreateProduct(f).Execute(ctx, tenant(org, "owner"), code, newProductInput(code)); err != nil {
			t.Fatal(err)
		}
	}
	page, err := NewListProducts(f).Execute(ctx, tn, ProductQuery{Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Next == nil {
		t.Fatalf("página 1: %d items, next=%v, err=%v", len(page.Items), page.Next, err)
	}
	page, err = NewListProducts(f).Execute(ctx, tn, ProductQuery{Limit: 2, After: page.Next})
	if err != nil || len(page.Items) != 1 || page.Items[0].Code != "C" || page.Next != nil {
		t.Fatalf("página 2: %+v err=%v", page, err)
	}
}
