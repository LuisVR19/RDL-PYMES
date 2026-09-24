// Package wiring arma los handlers HTTP sobre los adapters de Postgres. Lo usan cmd/api y la suite de aislamiento
// (tests/isolation), para que la suite pruebe exactamente los mismos endpoints que se despliegan.
package wiring

import (
	"log/slog"

	httpadapter "rdl/billing-api/internal/adapters/http"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/platform/health"
	"rdl/billing-api/pkg/tenancy"
)

// Deps completa httpadapter.Deps con todos los casos de uso. verifier, health y txm vienen de afuera porque la suite
// de aislamiento usa un verificador de prueba (no puede firmar tokens de Supabase) y un TxManager que no deja datos.
func Deps(log *slog.Logger, checks *health.Handler, verifier tenancy.TokenVerifier,
	memberships tenancy.MembershipResolver, txm app.TxManager) httpadapter.Deps {
	return httpadapter.Deps{
		Log:         log,
		Health:      checks,
		Verifier:    verifier,
		Memberships: memberships,
		Customers: &httpadapter.CustomerHandlers{
			Create: app.NewCreateCustomer(txm),
			List:   app.NewListCustomers(txm),
			Get:    app.NewGetCustomer(txm),
			Update: app.NewUpdateCustomer(txm),
		},
		Products: &httpadapter.ProductHandlers{
			Create: app.NewCreateProduct(txm),
			List:   app.NewListProducts(txm),
			Get:    app.NewGetProduct(txm),
			Update: app.NewUpdateProduct(txm),
		},
		Invoices: &httpadapter.InvoiceHandlers{
			Create:       app.NewCreateInvoiceDraft(txm),
			List:         app.NewListInvoices(txm),
			Get:          app.NewGetInvoice(txm),
			Update:       app.NewUpdateInvoiceDraft(txm),
			ReplaceLines: app.NewReplaceInvoiceLines(txm),
			Discard:      app.NewDiscardInvoiceDraft(txm),
			History:      app.NewGetInvoiceHistory(txm),
			Issue:        app.NewIssueInvoice(txm),
		},
		Sequences: &httpadapter.SequenceHandlers{
			List:      app.NewListSequences(txm),
			Configure: app.NewConfigureSequence(txm),
		},
	}
}
