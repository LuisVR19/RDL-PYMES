// Package wiring arma los handlers HTTP (y, desde el incremento 4, el consumidor) sobre los adapters. Lo usan cmd/*
// y la suite de aislamiento (tests/isolation), para que la suite pruebe exactamente lo mismo que se despliega.
package wiring

import (
	"log/slog"

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
		Receivables: &httpadapter.ReceivableHandlers{List: app.NewListReceivables(txm)},
	}
}
