// Package wiring arma los handlers HTTP y el procesamiento de eventos sobre los adapters. Lo usan cmd/* y la suite de aislamiento
// (tests/isolation), para que la suite pruebe exactamente lo mismo que se despliega.
package wiring

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	eventsadapter "rdl/einvoice-api/internal/adapters/events"
	httpadapter "rdl/einvoice-api/internal/adapters/http"
	"rdl/einvoice-api/internal/adapters/pkcs12"
	"rdl/einvoice-api/internal/adapters/postgres"
	"rdl/einvoice-api/internal/adapters/secrets"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/pkg/tenancy"
)

// Deps completa httpadapter.Deps con todos los casos de uso. verifier y health vienen de afuera porque la suite
// de aislamiento firma sus propios tokens y cada binario decide qué dependencias revisa.
func Deps(log *slog.Logger, checks *health.Handler, verifier tenancy.TokenVerifier,
	memberships tenancy.MembershipResolver, txm *postgres.TxManager, secrets app.SecretStore) httpadapter.Deps {
	return httpadapter.Deps{
		Log:           log,
		Health:        checks,
		Verifier:      verifier,
		Memberships:   memberships,
		FiscalProfile: &httpadapter.FiscalProfileHandlers{Get: app.NewGetFiscalProfile(txm)},
		FiscalConfig: &httpadapter.FiscalConfigHandlers{
			Update:      app.NewUpdateFiscalProfile(txm),
			Credentials: app.NewSetHaciendaCredentials(txm, secrets),
			Certificate: app.NewUploadCertificate(txm, secrets, pkcs12.Reader{}),
		},
		Catalogs: &httpadapter.CatalogHandlers{List: app.NewListCatalog(txm), Cabys: app.NewSearchCabys(txm)},
		FiscalStatus: &httpadapter.FiscalStatusHandlers{
			BySource:  app.NewGetFiscalStatusBySource(txm),
			BySources: app.NewGetFiscalStatusesBySource(txm),
		},
	}
}

// EventProcessor arma el procesamiento de eventos (lo usan cmd/consumer, cmd/replay y las suites de integración).
// Un tipo de evento nuevo se registra aquí (handler) y en events.NewDecoder (traducción).
func EventProcessor(pool *pgxpool.Pool, txm *postgres.TxManager, policy app.RetryPolicy) (*app.ProcessEvent, error) {
	validator, err := Validator()
	if err != nil {
		return nil, err
	}
	decoder := eventsadapter.NewDecoder(validator)
	handle := app.NewHandleEvent(decoder, txm, map[string]app.EventHandler{
		contracts.InvoiceIssuedType: app.NewCreateDocumentFromInvoice(),
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

// SecretStore arma el SecretStore de desarrollo. Sin llave maestra devuelve nil: la API arranca y las rutas que lo
// necesitan responden 503 (app.ErrSecretsUnavailable).
func SecretStore(dir, masterKey string) (app.SecretStore, error) {
	if masterKey == "" {
		return nil, nil
	}
	s, err := secrets.NewFileStore(dir, masterKey)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// TxManager arma el TxManager con el validador del outbox.
func TxManager(pool *pgxpool.Pool) (*postgres.TxManager, error) {
	v, err := Validator()
	if err != nil {
		return nil, err
	}
	return postgres.NewTxManager(pool, v), nil
}
