// Package wiring arma los handlers y los clientes a partir de la configuración. Lo usan cmd/gateway y las
// pruebas, para que ambas ejerciten exactamente el mismo router.
package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"rdl/portal-gateway/internal/adapters/downstream"
	httpadapter "rdl/portal-gateway/internal/adapters/http"
	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/platform/config"
	"rdl/portal-gateway/internal/platform/health"
)

// Clients arma un cliente por servicio. Los no configurados quedan con BaseURL vacía: sus rutas responden
// 503 `upstream-not-configured` y sus partes de una composición degradan, en vez de desaparecer del gateway.
func Clients(cfg config.Config, log *slog.Logger) map[routes.Service]*downstream.Client {
	clients := make(map[routes.Service]*downstream.Client, len(routes.Services()))
	for _, s := range routes.Services() {
		up := cfg.Upstreams.Get(s)
		clients[s] = downstream.New(s, up.URL, up.Timeout, log)
	}
	return clients
}

// Deps arma todo lo que necesita el router.
func Deps(cfg config.Config, log *slog.Logger, verifier httpadapter.Verifier,
	clients map[routes.Service]*downstream.Client, checks *health.Handler) httpadapter.Deps {

	overview := &app.OverviewService{
		Invoices: &downstream.BillingReader{
			Client: clients[routes.Billing],
			Source: downstream.SummarySource(cfg.Upstreams.BillingSummarySource),
		},
		Fiscal:   &downstream.FiscalReader{Client: clients[routes.Fiscal]},
		Balances: &downstream.ReceivablesReader{Client: clients[routes.Receivables]},
		Log:      log,
	}

	return httpadapter.Deps{
		Log:      log,
		Health:   checks,
		Verifier: verifier,
		Proxy: &httpadapter.Proxy{
			Clients: clients,
			MaxBody: cfg.HTTP.MaxRequestBody,
			Log:     log,
		},
		Overview: &httpadapter.OverviewHandler{Service: overview, Log: log},
		Budget:   cfg.Upstreams.Budget,
		CORS:     cfg.CORS.AllowedOrigins,
		Service:  cfg.ServiceName,
	}
}

// HealthChecks arma el readiness. El JWKS es crítico: sin él no se autentica a nadie. Cada API destino es
// degradable, y una que no está configurada en este ambiente ni se chequea (ADR 0002).
func HealthChecks(cfg config.Config, log *slog.Logger) *health.Handler {
	probe := &http.Client{Timeout: 3 * time.Second}

	var checkers []health.Checker
	checkers = append(checkers, health.Check{
		CheckName: "jwks", IsCritical: true,
		Fn: func(ctx context.Context) error { return reachable(ctx, probe, cfg.Auth.JWKSURL) },
	})
	for _, s := range routes.Services() {
		up := cfg.Upstreams.Get(s)
		if !up.Configured() {
			continue
		}
		url := up.URL + "/healthz"
		checkers = append(checkers, health.Check{
			CheckName: string(s), IsCritical: false,
			Fn: func(ctx context.Context) error { return reachable(ctx, probe, url) },
		})
	}
	return health.New(log, 5*time.Second, checkers...)
}

func reachable(ctx context.Context, c *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s respondió %d", url, resp.StatusCode)
	}
	return nil
}
