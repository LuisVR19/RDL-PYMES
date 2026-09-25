package app

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/domain/view"
)

// InvoiceListService arma el listado de documentos (pantalla 12): la página de Billing, más el estado fiscal y
// el saldo de sus filas, con UNA llamada por API por página (sin N+1). Primero Billing, porque los lotes
// necesitan sus ids; después E-Invoice y Receivables en paralelo.
type InvoiceListService struct {
	Invoices InvoiceLister
	Fiscal   FiscalStatusBatchReader
	Balances BalanceBatchReader
	Log      *slog.Logger
}

// ListInvoices devuelve un error SOLO si falla Billing. Si falla un lote, sus partes salen Unavailable en
// todas las filas y el listado se muestra igual (criterio 2).
func (s *InvoiceListService) ListInvoices(ctx context.Context, query map[string][]string) (view.InvoicePage, error) {
	rows, err := s.Invoices.ListInvoices(ctx, query)
	if err != nil {
		return view.InvoicePage{}, err
	}

	fiscal := view.Batch[view.FiscalStatus]{Available: true}
	balances := view.Batch[view.Balance]{Available: true}
	var fiscalEr, balanceE error

	// Una página vacía no pregunta nada: no hay filas que enriquecer.
	if ids := rows.InvoiceIDs(); len(ids) > 0 {
		// Sin WithContext a propósito, igual que en la vista transversal: que un lote falle no cancela el otro.
		var g errgroup.Group
		g.Go(func() error {
			fiscal.Found, fiscalEr = s.Fiscal.FiscalStatusesBySource(ctx, ids)
			return nil
		})
		g.Go(func() error {
			balances.Found, balanceE = s.Balances.BalancesByInvoice(ctx, ids)
			return nil
		})
		_ = g.Wait()
		// Cualquier error de un lote es "no se pudo consultar": a diferencia de la ruta de a uno, un 404 del
		// lote no significa que las facturas no tengan dato, sino que la ruta no está.
		fiscal.Available, balances.Available = fiscalEr == nil, balanceE == nil
	}

	page := view.JoinInvoicePage(rows, fiscal, balances)
	s.logDegradations(ctx, len(rows.Rows), fiscalEr, balanceE)
	return page, nil
}

func (s *InvoiceListService) logDegradations(ctx context.Context, rows int, fiscalEr, balanceE error) {
	if s.Log == nil {
		return
	}
	for _, d := range []degradedService{{routes.Fiscal, fiscalEr}, {routes.Receivables, balanceE}} {
		if d.Err == nil {
			continue
		}
		s.Log.WarnContext(ctx, "composición degradada",
			slog.String("view", "invoice-list"),
			slog.Int("rows", rows),
			slog.String("upstream", string(d.Service)),
			slog.Any("error", d.Err),
		)
	}
}
