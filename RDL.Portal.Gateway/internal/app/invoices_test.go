package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"rdl/portal-gateway/internal/domain/view"
)

type fakeLister struct {
	rows view.InvoiceRows
	err  error
	got  map[string][]string
}

func (f *fakeLister) ListInvoices(_ context.Context, q map[string][]string) (view.InvoiceRows, error) {
	f.got = q
	return f.rows, f.err
}

type fakeFiscalBatch struct {
	found map[string]view.FiscalStatus
	err   error
	calls [][]string
}

func (f *fakeFiscalBatch) FiscalStatusesBySource(_ context.Context, ids []string) (map[string]view.FiscalStatus, error) {
	f.calls = append(f.calls, ids)
	return f.found, f.err
}

type fakeBalanceBatch struct {
	found map[string]view.Balance
	err   error
	calls [][]string
}

func (f *fakeBalanceBatch) BalancesByInvoice(_ context.Context, ids []string) (map[string]view.Balance, error) {
	f.calls = append(f.calls, ids)
	return f.found, f.err
}

func listService(l *fakeLister, f *fakeFiscalBatch, b *fakeBalanceBatch) *InvoiceListService {
	return &InvoiceListService{Invoices: l, Fiscal: f, Balances: b, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestListInvoicesAsksEachBatchOnceWithThePageIDs(t *testing.T) {
	l := &fakeLister{rows: view.InvoiceRows{Rows: []view.InvoiceRow{{ID: "a"}, {ID: "b"}, {ID: "c"}}}}
	f := &fakeFiscalBatch{found: map[string]view.FiscalStatus{"a": {Status: "accepted"}}}
	b := &fakeBalanceBatch{found: map[string]view.Balance{"c": {Status: "open"}}}
	q := map[string][]string{"status": {"issued"}}

	page, err := listService(l, f, b).ListInvoices(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if l.got["status"][0] != "issued" {
		t.Errorf("filtros a Billing = %v", l.got)
	}
	want := []string{"a", "b", "c"}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) || len(b.calls) != 1 || !slices.Equal(b.calls[0], want) {
		t.Fatalf("lotes: fiscal=%v saldos=%v", f.calls, b.calls)
	}
	if page.Items[0].Fiscal.Availability != view.Available || page.Items[1].Fiscal.Availability != view.Absent ||
		page.Items[2].Receivable.Availability != view.Available || page.Items[0].Receivable.Availability != view.Absent {
		t.Fatalf("página = %+v", page)
	}
}

func TestListInvoicesFailsOnlyWhenBillingFails(t *testing.T) {
	boom := errors.New("billing caída")
	f, b := &fakeFiscalBatch{}, &fakeBalanceBatch{}
	if _, err := listService(&fakeLister{err: boom}, f, b).ListInvoices(context.Background(), nil); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if len(f.calls)+len(b.calls) != 0 {
		t.Fatal("sin página no se consulta ningún lote")
	}
}

// Un lote que falla (incluido un 404: la ruta no está) deja sus partes unavailable, nunca absent.
func TestListInvoicesDegradesAFailedBatch(t *testing.T) {
	l := &fakeLister{rows: view.InvoiceRows{Rows: []view.InvoiceRow{{ID: "a"}, {ID: "b"}}}}
	f := &fakeFiscalBatch{err: ErrNotFound}
	b := &fakeBalanceBatch{err: ErrNotConfigured}

	page, err := listService(l, f, b).ListInvoices(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range page.Items {
		if it.Fiscal.Availability != view.Unavailable || it.Receivable.Availability != view.Unavailable {
			t.Fatalf("fila %s = %+v", it.Invoice.ID, it)
		}
	}
}

func TestListInvoicesEmptyPageAsksNothingElse(t *testing.T) {
	f, b := &fakeFiscalBatch{}, &fakeBalanceBatch{}
	page, err := listService(&fakeLister{}, f, b).ListInvoices(context.Background(), nil)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if len(f.calls)+len(b.calls) != 0 {
		t.Fatal("una página vacía no llama a los lotes")
	}
}
