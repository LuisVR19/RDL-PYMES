# Estado del proyecto · Receivables API

**Última actualización:** 2026-09-24
**Punto de corte:** incremento 1 terminado y probado contra dev. `GRANT` sobre `core` aplicado; sigue el incremento 2.

## Decisiones aprobadas (2026-09-24)

Informe de brechas (ADR 0001), propuesta para `database-platform` (ADR 0002) y el plan completo (`docs/PLAN.md`),
incluidas R1 a R11 con las opciones recomendadas:
R1 la base sigue al contrato (migración de `recalculate_receivable`) · R2/R3 revertir aplicaciones al anular o al
acreditar de más (lo liberado queda disponible en el pago) · R4 la nota de débito conserva el `due_on` de la factura
y sobre una cuenta `cancelled` va a dead letter · R5 sin castigo en V1 · R6 tramos `current, 1_30, 31_60, 61_90,
90_plus` · R7 sin evento de pago anulado · R8 promesas `pending → kept | broken | cancelled` · R9 las revocaciones las
aplica `database-platform` · R10 el endpoint interno admite además `biller` · R11 nuevo problem type
`customer-mismatch`.

## Incrementos

| # | Incremento | Estado |
|---|---|---|
| 1 | Esqueleto, config, health, OTel, logins y baseline | ✅ |
| 2 | JWT, TenantContext, `GET /v1/receivables` | pendiente (desbloqueado) |
| 3–10 | Ver `docs/PLAN.md` §6 | pendiente |

## Incremento 1: qué hay

- `cmd/api`: `/healthz`, `/readyz` (base + JWKS), Problem Details para rutas desconocidas, `X-Correlation-Id`,
  OTel, logs JSON y apagado ordenado.
- `cmd/migrate`: `up`, `up-by-one`, `down`, `status` y `mark-baseline`. Verifica que la sesión sea
  `receivables_migrator`.
- `migrations/00001_baseline.sql`: el schema completo e idempotente (tablas, índices, funciones, triggers, vista, RLS,
  políticas y grants de `receivables_app`). `00002_list_indexes.sql`: los 4 índices de ADR 0001 §4.7.
- Dockerfile distroless no root (contexto: el monorepo), Makefile, `.golangci.yml`, `bitbucket-pipelines.yml`,
  `.env.example`, CLAUDE.md, README y ADR 0003.
- Verificación: `go build`, `go vet`, `go test ./...` (4 paquetes con tests) y `golangci-lint --build-tags=integration`
  con 0 issues. `make test-isolation` todavía no aplica (la suite llega en el incremento 10).

## Verificación contra dev (2026-09-24)

- Logins `receivables_api` y `receivables_migrate` creados (migración `0010_receivables_login_roles` en
  `supabase_migrations`), con contraseña SCRAM asignada por el usuario. `.env` en esta carpeta (ignorado por git).
- `migrate mark-baseline` registró 00001 sin ejecutarla; `migrate up-by-one` aplicó 00002. Historial en
  `receivables.goose_db_version`; tabla e índices a nombre de `receivables_migrator`.
- `go run ./cmd/api`: `/healthz` 200, `/readyz` 200 (`database: ok`, `jwks: ok`), ruta desconocida 404 en Problem Details
  con `correlationId`.

## Lectura de `core` (ADR 0002 §2)

Aplicada en dev el 2026-09-24 por pedido explícito del usuario (migración `0011_receivables_core_read` en
`supabase_migrations`, fuera del flujo normal de `database-platform`, que debe incorporarla a su repo). La consulta de
verificación de privilegios la bloqueó el modo automático; el incremento 2 la valida en la práctica (revalidación de
membresía contra `core`).

## Pendientes y TODOs

- `TODO(P2)`: transporte de eventos.
- `RDL.Contracts`: tag `v0.1.0` sin crear (el `replace` hacia `../RDL.Contracts` se agrega en el incremento 3); PR con
  `customer-mismatch` (R11) y con el OpenAPI de Receivables completo.
- `database-platform`: incorporar a su repo las migraciones 0010 y 0011 aplicadas desde aquí, y las revocaciones de R9 (ADR 0002 §3).
- La baseline se revisó a mano contra el catálogo de dev; no hay una base local para ejecutarla desde cero.
  Se prueba cuando la suite de integración tenga una base (incremento 10).
- Dockerfile: agregar `consumer` y `replay` en el incremento 4. No se construyó la imagen en esta máquina.
