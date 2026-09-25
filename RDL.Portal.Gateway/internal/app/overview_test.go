package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"rdl/portal-gateway/internal/domain/view"
)

type fakeInvoices struct {
	summary view.InvoiceSummary
	err     error
	delay   time.Duration
}

func (f fakeInvoices) InvoiceSummary(ctx context.Context, _ string) (view.InvoiceSummary, error) {
	if err := wait(ctx, f.delay); err != nil {
		return view.InvoiceSummary{}, err
	}
	return f.summary, f.err
}

type fakeFiscal struct {
	status view.FiscalStatus
	err    error
	delay  time.Duration
}

func (f fakeFiscal) FiscalStatusBySource(ctx context.Context, _ string) (view.FiscalStatus, error) {
	if err := wait(ctx, f.delay); err != nil {
		return view.FiscalStatus{}, err
	}
	return f.status, f.err
}

type fakeBalances struct {
	balance view.Balance
	err     error
	delay   time.Duration
}

func (f fakeBalances) BalanceByInvoice(ctx context.Context, _ string) (view.Balance, error) {
	if err := wait(ctx, f.delay); err != nil {
		return view.Balance{}, err
	}
	return f.balance, f.err
}

// wait simula latencia respetando la cancelación: así se prueban timeout y cliente que se va.
func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
	}
}

func service(inv fakeInvoices, fis fakeFiscal, bal fakeBalances) *OverviewService {
	return &OverviewService{
		Invoices: inv, Fiscal: fis, Balances: bal,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

var (
	summaryOK = view.InvoiceSummary{ID: "inv-1", DocumentType: "invoice", Number: "FE00100034",
		Status: "issued", CustomerLegalName: "Comercial Los Almendros S.A.", Currency: "CRC", Total: "113000.00"}
	fiscalOK  = view.FiscalStatus{ElectronicDocumentID: "doc-1", Status: "accepted"}
	balanceOK = view.Balance{ReceivableID: "rec-1", Status: "partially_paid", Currency: "CRC",
		BalanceAmount: "63000.00", DueOn: "2026-10-24"}
)

// El ejemplo de la arquitectura 2.2: total ₡113 000 · Hacienda Aceptada · saldo ₡63 000.
func TestOverviewJoinsTheThreeAPIs(t *testing.T) {
	s := service(fakeInvoices{summary: summaryOK}, fakeFiscal{status: fiscalOK}, fakeBalances{balance: balanceOK})

	got, err := s.InvoiceOverview(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Invoice.Total != "113000.00" || got.Invoice.Number != "FE00100034" {
		t.Errorf("factura: %+v", got.Invoice)
	}
	if got.Fiscal.Availability != view.Available || got.Fiscal.Status.Status != "accepted" {
		t.Errorf("fiscal: %+v", got.Fiscal)
	}
	if got.Receivable.Availability != view.Available || got.Receivable.Balance.BalanceAmount != "63000.00" {
		t.Errorf("saldo: %+v", got.Receivable)
	}
	if got.Degraded() {
		t.Error("no debería estar degradada")
	}
}

// Criterio 2: el portal sigue mostrando la factura aunque E-Invoice esté caída.
func TestOverviewDegradesWhenFiscalIsDown(t *testing.T) {
	s := service(fakeInvoices{summary: summaryOK},
		fakeFiscal{err: fmt.Errorf("%w: connection refused", ErrUnavailable)},
		fakeBalances{balance: balanceOK})

	got, err := s.InvoiceOverview(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("la vista no debe fallar entera: %v", err)
	}
	if got.Invoice.Total != "113000.00" {
		t.Errorf("la parte principal tiene que salir igual: %+v", got.Invoice)
	}
	if got.Fiscal.Availability != view.Unavailable || got.Fiscal.Status != nil {
		t.Errorf("fiscal: %+v", got.Fiscal)
	}
	if got.Receivable.Availability != view.Available {
		t.Errorf("el saldo sí estaba disponible: %+v", got.Receivable)
	}
	if !got.Degraded() {
		t.Error("Degraded() debería ser true")
	}
}

// E-Invoice y Receivables todavía no existen: sin URL configurada la vista sale igual, marcada.
func TestOverviewDegradesWhenSecondariesAreNotConfigured(t *testing.T) {
	s := service(fakeInvoices{summary: summaryOK},
		fakeFiscal{err: ErrNotConfigured}, fakeBalances{err: ErrNotConfigured})

	got, err := s.InvoiceOverview(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Fiscal.Availability != view.Unavailable || got.Receivable.Availability != view.Unavailable {
		t.Errorf("fiscal=%s saldo=%s", got.Fiscal.Availability, got.Receivable.Availability)
	}
}

// Un borrador no tiene documento electrónico ni cuenta por cobrar: eso es "absent", no una falla.
func TestOverviewMarksMissingPartsAsAbsent(t *testing.T) {
	s := service(fakeInvoices{summary: summaryOK},
		fakeFiscal{err: ErrNotFound}, fakeBalances{err: ErrNotFound})

	got, err := s.InvoiceOverview(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Fiscal.Availability != view.Absent || got.Receivable.Availability != view.Absent {
		t.Errorf("fiscal=%s saldo=%s", got.Fiscal.Availability, got.Receivable.Availability)
	}
	if got.Degraded() {
		t.Error("una parte ausente no es una degradación")
	}
}

// Si falla la API principal, la vista responde ese error tal cual (una factura de otra organización es 404).
func TestOverviewFailsWhenBillingFails(t *testing.T) {
	for _, want := range []error{ErrNotFound, ErrUnavailable} {
		s := service(fakeInvoices{err: want}, fakeFiscal{status: fiscalOK}, fakeBalances{balance: balanceOK})
		if _, err := s.InvoiceOverview(context.Background(), "inv-1"); !errors.Is(err, want) {
			t.Errorf("err=%v, want %v", err, want)
		}
	}
}

// Receivables lento: se respeta el deadline y la vista sale sin saldo.
func TestOverviewRespectsDeadlineOfSlowSecondary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	s := service(fakeInvoices{summary: summaryOK}, fakeFiscal{status: fiscalOK},
		fakeBalances{balance: balanceOK, delay: 2 * time.Second})

	start := time.Now()
	got, err := s.InvoiceOverview(ctx, "inv-1")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("esperó %v: no respetó el deadline", elapsed)
	}
	if got.Receivable.Availability != view.Unavailable {
		t.Errorf("saldo: %+v", got.Receivable)
	}
}

// El cliente se va: las llamadas internas se cancelan y nadie se queda esperando.
func TestOverviewCancelsUpstreamsWhenClientLeaves(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := service(fakeInvoices{summary: summaryOK, delay: 2 * time.Second},
		fakeFiscal{status: fiscalOK, delay: 2 * time.Second},
		fakeBalances{balance: balanceOK, delay: 2 * time.Second})

	start := time.Now()
	if _, err := s.InvoiceOverview(ctx, "inv-1"); err == nil {
		t.Fatal("debería fallar: la fuente principal se canceló")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("esperó %v tras la cancelación", elapsed)
	}
}
