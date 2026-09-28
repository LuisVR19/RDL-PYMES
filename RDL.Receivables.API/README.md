# Receivables API

Cuentas por cobrar, pagos, aplicación de pagos, ajustes, aging y cobranza del SaaS de facturación electrónica para
PYMES de Costa Rica (prompt P6). Go 1.27, `net/http`, `pgx/v5` + sqlc, goose, `shopspring/decimal` y OpenTelemetry,
con la misma arquitectura hexagonal que Platform API.

> Estado: **incrementos 1 a 10 terminados** y probados contra dev. Pendientes para revisar en equipo (transporte de
> eventos P2, PRs a contratos, decisiones abiertas): `docs/ESTADO.md`. Plan y decisiones: `docs/PLAN.md`,
> `docs/decisiones/`.

## Qué hace

- **Consume** de Billing `InvoiceIssued` (crea la cuenta: `open`, saldo = total), `CreditNoteIssued` (resta; si supera
  el saldo revierte aplicaciones, R3), `DebitNoteIssued` (suma y conserva el vencimiento, R4) e `InvoiceCancelled`
  (revierte lo aplicado y anula, R2). Inbox idempotente, reintentos con backoff y dead letter (ADR 0005).
- **Produce** `PaymentReceived` y `ReceivableSettled` en `integration.outbox_messages`, en la misma transacción que el
  cambio y validados contra su JSON Schema.
- **API**: cuentas (listado, detalle, aging por tramos), pagos (registrar con aplicaciones, aplicar, revertir, anular),
  seguimientos y promesas de pago, y el saldo por factura para el BFF. Contrato completo en `api/openapi.yaml`.
- El saldo y el estado **solo** cambian dentro de los agregados (`internal/domain`); la base los recalcula con la
  misma fórmula y el caso de uso compara los dos antes de confirmar (ADR 0006).

## Requisitos

- Go 1.27.1, `golangci-lint` v2 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`) y, para
  regenerar consultas, sqlc v1.31.1 (`CGO_ENABLED=0 go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`).
- `../RDL.Contracts` al lado de este repo (el `replace` de `go.mod`, hasta que exista el tag `v0.1.0`).
- Los logins `receivables_api` y `receivables_migrate` en el proyecto Supabase de dev
  (`scripts/dev/0010_receivables_login_roles.sql`, más las contraseñas con `ALTER ROLE`).
- Un `.env` a partir de `.env.example` (host del pooler, contraseñas y, para las suites, `ISOLATION_ORG_A/B`).

## Uso

```sh
go run ./cmd/migrate status          # conexión y migraciones (login receivables_migrate, modo sesión)
go run ./cmd/migrate mark-baseline   # SOLO una vez en dev: registra 00001 sin ejecutarla
go run ./cmd/migrate up-by-one       # aplica la siguiente migración pendiente (expand → migrate → contract)
go run ./cmd/api                     # http://localhost:8083/healthz y /readyz (base y JWKS)
go run ./cmd/consumer                # consumidor de eventos; health en :8084 (sin transporte hasta P2)
curl -H "Authorization: Bearer $JWT" localhost:8083/v1/receivables   # JWT de Supabase con org_id
```

`make` tiene `run`, `consumer` (`run-consumer`), `replay FILE=…`, `build`, `test`, `test-integration`,
`test-isolation`, `test-e2e`, `lint`, `migrate-up`, `migrate-status`, `migrate-mark-baseline`, `sqlc` y `docker`. La
imagen se construye con el monorepo como contexto (`make docker`), porque el módulo de contratos se toma de
`../RDL.Contracts` hasta que se publique su tag; trae `api` (por defecto), `consumer`, `replay` y `migrate`.

### Reproducir eventos en local

El transporte de eventos todavía no existe (`TODO(P2)`): los eventos entran con `cmd/replay`, que usa exactamente el
mismo procesamiento que el consumidor (validación, inbox, efecto, auditoría, outbox, reintentos y dead letter).

```sh
go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json               # los válidos
go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json               # otra vez: duplicados
go run ./cmd/replay -file ../RDL.Contracts/examples/events/invoice-issued.v1.json -set invalid  # a dead letter
go run ./cmd/replay -file mi-evento.json                                                          # un evento suelto
```

Acepta un evento suelto, un arreglo o un archivo de ejemplos de contratos (los inválidos se arman aplicando su patch).
El resultado de cada evento va al log (`processed`, `duplicate`, `dead_lettered`).

### Tests

```sh
go test ./...            # dominio (propiedades con rapid: las 5 invariantes), casos de uso, HTTP, eventos
make test-integration    # guard de saldos contra dev (rollback siempre)
make test-isolation      # los 6 criterios de aislamiento contra dev (ADR 0008)
make test-e2e            # de punta a punta contra dev; también scripts/dev/e2e.sh
```

Las suites contra dev usan el login `receivables_api` (se niegan a correr con un rol que salte RLS) y dos
organizaciones de prueba (`ISOLATION_ORG_A/B`). Crean cuentas y pagos ficticios en esas organizaciones y no borran
nada.

## Endpoints

| Método y ruta | Roles |
|---|---|
| `GET /v1/receivables` (`status`, `customerId`, `overdue`, cursor) | lectura: owner, admin, collector, accountant, read_only |
| `GET /v1/receivables/{id}` (con aplicaciones y ajustes) | lectura |
| `GET /v1/receivables/aging` (`asOf`, `currency`) | lectura |
| `GET /v1/payments`, `GET /v1/payments/{id}` | lectura |
| `POST /v1/payments` (con `applications` opcionales) | owner, admin, collector |
| `POST /v1/payment-applications` | owner, admin, collector |
| `POST /v1/payment-applications/{id}/reverse` | owner, admin |
| `POST /v1/payments/{id}/void` | owner, admin |
| `GET`/`POST /v1/receivables/{id}/follow-ups` | lectura / owner, admin, collector |
| `GET`/`POST /v1/receivables/{id}/promises`, `POST /v1/payment-promises/{id}/status` | lectura / owner, admin, collector |
| `GET /internal/v1/receivables/by-invoice/{invoiceId}` (BFF) | lectura y biller (R10) |

Todo `POST` exige `Idempotency-Key` (24 h). Errores en Problem Details con los tipos de
`RDL.Contracts/problems/receivables.yaml` (más `customer-mismatch`, R11): otra organización → 404, rol → 403, estado →
409, regla de negocio → 422. Montos siempre como string decimal.

## Estructura

```
cmd/api, cmd/consumer           API HTTP y consumidor de eventos (ADR 0005)
cmd/replay, cmd/migrate         eventos desde archivo (dev) y goose
api/openapi.yaml                contrato de la API (un test lo compara con el router)
internal/domain                 receivable, payment, settlement (reglas entre los dos), aging, collection, amount,
                                civil (fecha de negocio), permission (la matriz, en un solo lugar)
internal/app                    casos de uso y puertos (un struct por caso, incluido HandleEvent)
internal/adapters/http          router, handlers, Problem Details, paginación por cursor, idempotencia
internal/adapters/events        validación con los schemas de contratos, traducción a comandos, runner del consumidor
internal/adapters/postgres      pgx sobre Supavisor: TxManager (set_config), ledger de agregados, lecturas, inbox,
                                outbox, dead letters, auditoría, idempotencia, membresía en core
internal/adapters/postgres/db   generado por sqlc desde queries/ (no editar)
internal/adapters/auth          verificación de JWT contra el JWKS de Supabase (copiado de Platform)
internal/platform               config (.env), logger, OTel, health, examples (ejemplos de contratos)
internal/wiring                 arma handlers y procesador de eventos; lo usan cmd/* y las suites
migrations                      goose, historial en receivables.goose_db_version
pkg/{tenancy,correlation,requestinfo}  copiados de Platform (ADR 0003)
tests/{integration,isolation,e2e,testkit}  suites contra dev (tag integration)
docs/                           contexto, decisiones (ADR 0001 a 0008), plan y estado
```

## Configuración

| Variable | Por defecto | Descripción |
|---|---|---|
| `SUPABASE_URL` | — (obligatoria) | Deriva el issuer y el JWKS |
| `DB_POOLER_HOST` | — | Host de Supavisor; con él se arma la URL de `receivables_api` (6543) y `receivables_migrate` (5432) |
| `DB_PASSWORD`, `MIGRATE_DB_PASSWORD` | — | Contraseñas de los logins |
| `DATABASE_URL`, `MIGRATE_DATABASE_URL` | — | Alternativa a `DB_POOLER_HOST` |
| `HTTP_ADDR` | `:8083` | |
| `CONSUMER_HTTP_ADDR` | `:8084` | `/healthz` y `/readyz` del consumidor |
| `CONSUMER_MESSAGE_TIMEOUT` | `2m` | Tiempo máximo de un evento con sus reintentos (mínimo 30 s) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | vacío | Sin valor, trazas y métricas quedan en no-op |
| `AUTH_MEMBERSHIP_CACHE_TTL` | `30s` | Máximo 60 s |
| `ISOLATION_ORG_A`, `ISOLATION_ORG_B` | — | Solo suites: organizaciones de prueba de dev |

Métricas del consumidor: `receivables.consumer.messages` (por `event_type` y `outcome`),
`receivables.consumer.attempts` y `receivables.consumer.lag` (segundos desde `occurredAt`).
