package app

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/domain/view"
)

// OverviewService arma la vista transversal de una factura (arquitectura 2.2): el total lo da Billing,
// el estado de Hacienda E-Invoice y el saldo Receivables. Las tres APIs se consultan en paralelo y ninguna
// se entera de la otra: el Portal Gateway junta, no crea dependencias entre dominios (criterio 9).
type OverviewService struct {
	Invoices InvoiceSummaryReader
	Fiscal   FiscalStatusReader
	Balances BalanceReader
	Log      *slog.Logger
}

// maxParallelUpstreams acota la concurrencia por petición: son tres llamadas y no queremos que una página
// de facturas multiplique conexiones contra las APIs.
const maxParallelUpstreams = 3

// InvoiceOverview devuelve la vista o un error SOLO si falla la fuente principal (Billing): sin documento
// no hay nada que mostrar. Si fallan las secundarias, la vista sale con esas partes marcadas.
func (s *OverviewService) InvoiceOverview(ctx context.Context, invoiceID string) (view.InvoiceOverview, error) {
	var (
		summary  view.InvoiceSummary
		fiscal   view.FiscalStatus
		balance  view.Balance
		fiscalEr error
		balanceE error
	)

	// Sin errgroup.WithContext a propósito: que E-Invoice falle no puede cancelar la llamada a Billing.
	// La cancelación real viene del contexto del request, cuando el usuario se va.
	var g errgroup.Group
	g.SetLimit(maxParallelUpstreams)

	g.Go(func() error {
		var err error
		summary, err = s.Invoices.InvoiceSummary(ctx, invoiceID)
		return err // el único error que aborta la vista
	})
	g.Go(func() error {
		fiscal, fiscalEr = s.Fiscal.FiscalStatusBySource(ctx, invoiceID)
		return nil
	})
	g.Go(func() error {
		balance, balanceE = s.Balances.BalanceByInvoice(ctx, invoiceID)
		return nil
	})

	if err := g.Wait(); err != nil {
		return view.InvoiceOverview{}, err
	}

	overview := view.InvoiceOverview{
		Invoice:    summary,
		Fiscal:     view.NewFiscalPart(availabilityOf(fiscalEr), &fiscal),
		Receivable: view.NewBalancePart(availabilityOf(balanceE), &balance),
	}
	s.logDegradations(ctx, invoiceID, overview, fiscalEr, balanceE)
	return overview, nil
}

// logDegradations deja en el log por qué una parte no salió. El detalle nunca viaja en la respuesta:
// el portal solo necesita saber que no se pudo consultar.
func (s *OverviewService) logDegradations(ctx context.Context, invoiceID string, o view.InvoiceOverview, fiscalEr, balanceE error) {
	if s.Log == nil || !o.Degraded() {
		return
	}
	for _, d := range []degradedService{{routes.Fiscal, fiscalEr}, {routes.Receivables, balanceE}} {
		if d.Err == nil || availabilityOf(d.Err) != view.Unavailable {
			continue
		}
		s.Log.WarnContext(ctx, "composición degradada",
			slog.String("view", "invoice-overview"),
			slog.String("invoiceId", invoiceID),
			slog.String("upstream", string(d.Service)),
			slog.Any("error", d.Err),
		)
	}
}
