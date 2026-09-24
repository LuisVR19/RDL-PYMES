package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// fakeTx simula la base: RLS (cada repositorio solo ve la organización de la transacción), unicidades y atomicidad
// (si fn falla, todo lo escrito en la transacción se revierte, como con Postgres).
type fakeTx struct {
	state fakeState
	org   uuid.UUID // app.current_organization_id de la transacción en curso
	calls int
	now   time.Time
	cat   fakeCatalog // core y fiscal: no los escribe Billing, no entran en el rollback
	// outboxErr simula que el evento no se pudo escribir (por ejemplo, no valida contra su schema).
	outboxErr error
}

type outboxEntry struct {
	Invoice   invoice.Invoice
	IssueDate string
}

type fakeState struct {
	customers []customer.Customer
	products  []product.Product
	invoices  []invoice.Invoice
	sequences []numbering.Sequence
	locks     []string
	outbox    []outboxEntry
	history   map[uuid.UUID][]StatusChange
	idem      map[string]IdempotencyRecord // organización/clave
	audit     []AuditEvent
}

func newFakeTx() *fakeTx {
	return &fakeTx{
		state: fakeState{idem: map[string]IdempotencyRecord{}, history: map[uuid.UUID][]StatusChange{}},
		now:   time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC),
		cat: fakeCatalog{
			local: money.MustCurrencyForTest("CRC"), branches: map[uuid.UUID]Branch{},
			rates: map[string]money.Percentage{"08": money.MustPercentageForTest("13"), "04": money.MustPercentageForTest("4")},
		},
	}
}

func (s fakeState) clone() fakeState {
	return fakeState{customers: slices.Clone(s.customers), products: slices.Clone(s.products),
		invoices: slices.Clone(s.invoices), history: maps.Clone(s.history), sequences: slices.Clone(s.sequences),
		locks: slices.Clone(s.locks), outbox: slices.Clone(s.outbox), idem: maps.Clone(s.idem), audit: slices.Clone(s.audit)}
}

func (f *fakeTx) WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(context.Context, Tx) error) error {
	f.calls++
	f.org = t.OrganizationID()
	snapshot := f.state.clone()
	if err := fn(ctx, f); err != nil {
		f.state = snapshot
		return err
	}
	return nil
}

func (f *fakeTx) Customers() CustomerRepository { return fakeCustomers{f} }
func (f *fakeTx) Products() ProductRepository   { return fakeProducts{f} }
func (f *fakeTx) Invoices() InvoiceRepository   { return fakeInvoices{f} }
func (f *fakeTx) Catalog() Catalog              { return fakeCatalogOf{f} }
func (f *fakeTx) Sequences() SequenceRepository { return fakeSequences{f} }
func (f *fakeTx) Outbox() EventOutbox           { return fakeOutbox{f} }
func (f *fakeTx) Idempotency() IdempotencyStore { return fakeIdempotency{f} }
func (f *fakeTx) Audit() AuditRecorder          { return fakeAudit{f} }

type fakeCustomers struct{ f *fakeTx }

func (r fakeCustomers) visible(org uuid.UUID) bool { return org == r.f.org }

func (r fakeCustomers) Create(_ context.Context, c customer.Customer) (customer.Customer, error) {
	for _, other := range r.f.state.customers {
		if other.OrganizationID == c.OrganizationID && other.Identification == c.Identification {
			return customer.Customer{}, ErrCustomerIdentificationTaken
		}
	}
	r.f.now = r.f.now.Add(time.Second)
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), r.f.now, r.f.now
	r.f.state.customers = append(r.f.state.customers, c)
	return c, nil
}

func (r fakeCustomers) List(_ context.Context, org uuid.UUID, q CustomerQuery) ([]customer.Customer, error) {
	var out []customer.Customer
	for _, c := range r.f.state.customers {
		if !r.visible(c.OrganizationID) || c.OrganizationID != org {
			continue
		}
		if q.Active != nil && c.IsActive != *q.Active {
			continue
		}
		if q.Search != "" && !strings.Contains(strings.ToLower(c.LegalName), strings.ToLower(q.Search)) {
			continue
		}
		if q.After != nil && (c.CreatedAt.Before(q.After.At) || c.CreatedAt.Equal(q.After.At) && c.ID.String() <= q.After.ID.String()) {
			continue
		}
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b customer.Customer) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r fakeCustomers) Get(_ context.Context, org, id uuid.UUID) (customer.Customer, error) {
	for _, c := range r.f.state.customers {
		if c.ID == id && c.OrganizationID == org && r.visible(c.OrganizationID) {
			return c, nil
		}
	}
	return customer.Customer{}, ErrNotFound
}

func (r fakeCustomers) GetForUpdate(ctx context.Context, org, id uuid.UUID) (customer.Customer, error) {
	return r.Get(ctx, org, id)
}

func (r fakeCustomers) Update(_ context.Context, c customer.Customer) (customer.Customer, error) {
	for i, cur := range r.f.state.customers {
		if cur.ID == c.ID && cur.OrganizationID == c.OrganizationID && r.visible(c.OrganizationID) {
			r.f.now = r.f.now.Add(time.Second)
			c.UpdatedAt = r.f.now
			r.f.state.customers[i] = c
			return c, nil
		}
	}
	return customer.Customer{}, ErrNotFound
}

type fakeProducts struct{ f *fakeTx }

func (r fakeProducts) codeTaken(p product.Product) bool {
	for _, other := range r.f.state.products {
		if other.OrganizationID == p.OrganizationID && other.Code == p.Code && other.ID != p.ID {
			return true
		}
	}
	return false
}

func (r fakeProducts) Create(_ context.Context, p product.Product) (product.Product, error) {
	if r.codeTaken(p) {
		return product.Product{}, ErrProductCodeTaken
	}
	r.f.now = r.f.now.Add(time.Second)
	p.ID, p.CreatedAt, p.UpdatedAt = uuid.New(), r.f.now, r.f.now
	r.f.state.products = append(r.f.state.products, p)
	return p, nil
}

func (r fakeProducts) List(_ context.Context, org uuid.UUID, q ProductQuery) ([]product.Product, error) {
	var out []product.Product
	for _, p := range r.f.state.products {
		if p.OrganizationID != r.f.org || p.OrganizationID != org {
			continue
		}
		if q.Active != nil && p.IsActive != *q.Active {
			continue
		}
		if q.After != nil && !p.CreatedAt.After(q.After.At) {
			continue
		}
		out = append(out, p)
	}
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r fakeProducts) Get(_ context.Context, org, id uuid.UUID) (product.Product, error) {
	for _, p := range r.f.state.products {
		if p.ID == id && p.OrganizationID == org && org == r.f.org {
			return p, nil
		}
	}
	return product.Product{}, ErrNotFound
}

func (r fakeProducts) GetForUpdate(ctx context.Context, org, id uuid.UUID) (product.Product, error) {
	return r.Get(ctx, org, id)
}

func (r fakeProducts) Update(_ context.Context, p product.Product) (product.Product, error) {
	if r.codeTaken(p) {
		return product.Product{}, ErrProductCodeTaken
	}
	for i, cur := range r.f.state.products {
		if cur.ID == p.ID && cur.OrganizationID == p.OrganizationID && p.OrganizationID == r.f.org {
			r.f.now = r.f.now.Add(time.Second)
			p.UpdatedAt = r.f.now
			r.f.state.products[i] = p
			return p, nil
		}
	}
	return product.Product{}, ErrNotFound
}

type fakeInvoices struct{ f *fakeTx }

func (r fakeInvoices) index(org, id uuid.UUID) int {
	for i, inv := range r.f.state.invoices {
		if inv.ID == id && inv.OrganizationID == org && org == r.f.org {
			return i
		}
	}
	return -1
}

func (r fakeInvoices) Create(_ context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	r.f.now = r.f.now.Add(time.Second)
	inv.ID, inv.CreatedAt, inv.UpdatedAt = uuid.New(), r.f.now, r.f.now
	r.f.state.invoices = append(r.f.state.invoices, inv)
	return inv, nil
}

func (r fakeInvoices) Get(_ context.Context, org, id uuid.UUID) (invoice.Invoice, error) {
	if i := r.index(org, id); i >= 0 {
		return r.f.state.invoices[i], nil
	}
	return invoice.Invoice{}, ErrNotFound
}

func (r fakeInvoices) GetForUpdate(ctx context.Context, org, id uuid.UUID) (invoice.Invoice, error) {
	return r.Get(ctx, org, id)
}

// SaveDraft y DeleteDraft imitan a invoices_guard: sobre algo que no es borrador, ErrNotDraft.
func (r fakeInvoices) SaveDraft(_ context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	i := r.index(inv.OrganizationID, inv.ID)
	if i < 0 || r.f.state.invoices[i].Status != invoice.StatusDraft {
		return invoice.Invoice{}, invoice.ErrNotDraft
	}
	r.f.now = r.f.now.Add(time.Second)
	inv.UpdatedAt = r.f.now
	r.f.state.invoices[i] = inv
	return inv, nil
}

func (r fakeInvoices) MarkIssued(_ context.Context, inv invoice.Invoice) (invoice.Invoice, error) {
	i := r.index(inv.OrganizationID, inv.ID)
	if i < 0 || r.f.state.invoices[i].Status != invoice.StatusDraft {
		return invoice.Invoice{}, invoice.ErrNotDraft
	}
	for _, other := range r.f.state.invoices {
		if other.OrganizationID == inv.OrganizationID && other.DocumentType == inv.DocumentType && other.Number == inv.Number {
			return invoice.Invoice{}, errors.New("invoices_number_uk")
		}
	}
	r.f.now = r.f.now.Add(time.Second)
	inv.UpdatedAt = r.f.now
	r.f.state.invoices[i] = inv
	return inv, nil
}

func (r fakeInvoices) AddStatusChange(_ context.Context, _, id uuid.UUID, c StatusChange) error {
	r.f.state.history[id] = append(r.f.state.history[id], c)
	return nil
}

func (r fakeInvoices) DeleteDraft(_ context.Context, org, id uuid.UUID) error {
	i := r.index(org, id)
	if i < 0 || r.f.state.invoices[i].Status != invoice.StatusDraft {
		return invoice.ErrNotDraft
	}
	r.f.state.invoices = slices.Delete(r.f.state.invoices, i, i+1)
	return nil
}

func (r fakeInvoices) List(_ context.Context, org uuid.UUID, q InvoiceQuery) ([]invoice.Invoice, error) {
	var out []invoice.Invoice
	for _, inv := range r.f.state.invoices {
		if inv.OrganizationID != org || org != r.f.org || q.Status != "" && inv.Status != q.Status {
			continue
		}
		if q.CustomerID != nil && inv.CustomerID != *q.CustomerID {
			continue
		}
		if q.After != nil && !inv.CreatedAt.After(q.After.At) {
			continue
		}
		out = append(out, inv)
	}
	if len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r fakeInvoices) History(_ context.Context, _, id uuid.UUID) ([]StatusChange, error) {
	return r.f.state.history[id], nil
}

type fakeCatalog struct {
	local    money.Currency
	branches map[uuid.UUID]Branch // por id; la organización se guarda aparte
	branchOf map[uuid.UUID]uuid.UUID
	rates    map[string]money.Percentage
}

type fakeCatalogOf struct{ f *fakeTx }

func (c fakeCatalogOf) Organization(context.Context, uuid.UUID) (OrganizationSettings, error) {
	return OrganizationSettings{LocalCurrency: c.f.cat.local, Timezone: "America/Costa_Rica"}, nil
}

func (c fakeCatalogOf) Branch(_ context.Context, org, id uuid.UUID) (Branch, error) {
	b, ok := c.f.cat.branches[id]
	if !ok || c.f.cat.branchOf[id] != org || org != c.f.org {
		return Branch{}, ErrNotFound
	}
	return b, nil
}

func (c fakeCatalogOf) TaxRates(_ context.Context, codes []string) (map[string]money.Percentage, error) {
	out := map[string]money.Percentage{}
	for _, code := range codes {
		if r, ok := c.f.cat.rates[code]; ok {
			out[code] = r
		}
	}
	return out, nil
}

// addBranch registra una sucursal de org en el catálogo de prueba.
func (f *fakeTx) addBranch(org uuid.UUID, active bool) uuid.UUID {
	id := uuid.New()
	if f.cat.branchOf == nil {
		f.cat.branchOf = map[uuid.UUID]uuid.UUID{}
	}
	f.cat.branches[id] = Branch{ID: id, IsActive: active}
	f.cat.branchOf[id] = org
	return id
}

type fakeSequences struct{ f *fakeTx }

func (r fakeSequences) Lock(_ context.Context, org uuid.UUID, documentType string) error {
	r.f.state.locks = append(r.f.state.locks, org.String()+":"+documentType)
	return nil
}

func (r fakeSequences) List(_ context.Context, org uuid.UUID) ([]numbering.Sequence, error) {
	var out []numbering.Sequence
	for _, s := range r.f.state.sequences {
		if s.OrganizationID == org && org == r.f.org {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r fakeSequences) GetForUpdate(_ context.Context, org uuid.UUID, scope numbering.Scope) (numbering.Sequence, bool, error) {
	for _, s := range r.f.state.sequences {
		if s.OrganizationID == org && org == r.f.org && s.Equal(scope) {
			return s, true, nil
		}
	}
	return numbering.Sequence{}, false, nil
}

func (r fakeSequences) Save(_ context.Context, s numbering.Sequence) (numbering.Sequence, error) {
	r.f.now = r.f.now.Add(time.Second)
	s.UpdatedAt = r.f.now
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
		r.f.state.sequences = append(r.f.state.sequences, s)
		return s, nil
	}
	for i, cur := range r.f.state.sequences {
		if cur.ID == s.ID {
			r.f.state.sequences[i] = s
			return s, nil
		}
	}
	return numbering.Sequence{}, ErrNotFound
}

type fakeOutbox struct{ f *fakeTx }

func (o fakeOutbox) InvoiceIssued(_ context.Context, inv invoice.Invoice, issueDate string) error {
	if o.f.outboxErr != nil {
		return o.f.outboxErr
	}
	o.f.state.outbox = append(o.f.state.outbox, outboxEntry{Invoice: inv, IssueDate: issueDate})
	return nil
}

type fakeIdempotency struct{ f *fakeTx }

func (s fakeIdempotency) Claim(_ context.Context, org uuid.UUID, key, hash string, _ time.Duration) (*IdempotencyRecord, error) {
	k := org.String() + "/" + key
	if prev, ok := s.f.state.idem[k]; ok {
		return &prev, nil
	}
	s.f.state.idem[k] = IdempotencyRecord{RequestHash: hash}
	return nil, nil
}

func (s fakeIdempotency) Complete(_ context.Context, org uuid.UUID, key string, r IdempotencyRecord) error {
	s.f.state.idem[org.String()+"/"+key] = r
	return nil
}

type fakeAudit struct{ f *fakeTx }

func (a fakeAudit) Record(_ context.Context, e AuditEvent) error {
	a.f.state.audit = append(a.f.state.audit, e)
	return nil
}

func tenant(org uuid.UUID, roles ...string) tenancy.Context {
	return tenancy.NewContext(uuid.New(), "sub", org, roles)
}
