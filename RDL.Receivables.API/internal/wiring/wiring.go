// Package wiring arma los handlers HTTP (y, desde el incremento 4, el consumidor) sobre los adapters. Lo usan cmd/*
// y la suite de aislamiento (tests/isolation), para que la suite pruebe exactamente lo mismo que se despliega.
package wiring

import (
	"log/slog"

	httpadapter "rdl/receivables-api/internal/adapters/http"
	"rdl/receivables-api/internal/platform/health"
)

// Deps completa httpadapter.Deps. health viene de afuera porque cada binario decide qué dependencias revisa.
func Deps(log *slog.Logger, checks *health.Handler) httpadapter.Deps {
	return httpadapter.Deps{Log: log, Health: checks}
}
