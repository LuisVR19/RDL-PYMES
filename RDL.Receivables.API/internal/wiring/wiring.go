// Package wiring arma los handlers HTTP y el procesamiento de eventos sobre los adapters. Lo usan cmd/*
// y la suite de aislamiento (tests/isolation), para que la suite pruebe exactamente lo mismo que se despliega.
package wiring

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	eventsadapter "rdl/receivables-api/internal/adapters/events"
	httpadapter "rdl/receivables-api/internal/adapters/http"
	"rdl/receivables-api/internal/adapters/postgres"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/platform/health"
	"rdl/receivables-api/pkg/tenancy"
)

// Deps completa httpadapter.Deps con todos los casos de uso. verifier y health vienen de afuera porque la suite
// de aislamiento firma sus propios tokens y cada binario decide qué dependencias revisa.
func Deps(log *slog.Logger, checks *health.Handler, verifier tenancy.TokenVerifier,
	memberships tenancy.MembershipResolver, txm *postgres.TxManager) httpadapter.Deps {
	return httpadapter.Deps{
		Log:         log,
		Health:      checks,
		Verifier:    verifier,
		Memberships: memberships,
		Receivables: &httpadapter.ReceivableHandlers{
			List:       app.NewListReceivables(txm),
			Get:        app.NewGetReceivable(txm),
			ByInvoice:  app.NewGetBalanceByInvoice(txm),
			ByInvoices: app.NewGetBalancesByInvoice(txm),
			Aging:      app.NewGetAging(txm),
		},
		Payments: &httpadapter.PaymentHandlers{
			Create:  app.NewCreatePayment(txm),
			List:    app.NewListPayments(txm),
			Get:     app.NewGetPayment(txm),
			Void:    app.NewVoidPayment(txm),
			Apply:   app.NewApplyPayment(txm),
			Reverse: app.NewReversePaymentApplication(txm),
		},
		Collection: &httpadapter.CollectionHandlers{
			CreateFollowUp: app.NewCreateFollowUp(txm),
			ListFollowUps:  app.NewListFollowUps(txm),
			CreatePromise:  app.NewCreatePromise(txm),
			ListPromises:   app.NewListPromises(txm),
			ClosePromise:   app.NewClosePromise(txm),
		},
	}
}

// EventProcessor arma el procesamiento de eventos (lo usan cmd/consumer, cmd/replay y los tests de integración).
// Un tipo de evento nuevo se registra aquí (handler) y en events.NewDecoder (traducción).
func EventProcessor(pool *pgxpool.Pool, txm *postgres.TxManager, policy app.RetryPolicy) (*app.ProcessEvent, error) {
	validator, err := Validator()
	if err != nil {
		return nil, err
	}
	decoder := eventsadapter.NewDecoder(validator)
	handle := app.NewHandleEvent(decoder, txm, map[string]app.EventHandler{
		contracts.InvoiceIssuedType:         app.NewCreateReceivableFromInvoice(),
		contracts.CreditNoteIssuedSpec.Type: app.NewApplyCreditNote(),
		contracts.DebitNoteIssuedSpec.Type:  app.NewApplyDebitNote(),
		contracts.InvoiceCancelledSpec.Type: app.NewCancelReceivable(),
	})
	return app.NewProcessEvent(handle, decoder, postgres.NewDeadLetters(pool), policy), nil
}

// Validator compila una vez los schemas de contratos: los usan el decodificador de eventos y el outbox.
func Validator() (*contracts.Validator, error) {
	v, err := contracts.DefaultValidator()
	if err != nil {
		return nil, fmt.Errorf("compilando los schemas de contratos: %w", err)
	}
	return v, nil
}

// TxManager arma el TxManager con el validador del outbox.
func TxManager(pool *pgxpool.Pool) (*postgres.TxManager, error) {
	v, err := Validator()
	if err != nil {
		return nil, err
	}
	return postgres.NewTxManager(pool, v), nil
}
