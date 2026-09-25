# Receivables API

Cuentas por cobrar, pagos, aplicación de pagos, ajustes, aging y cobranza del SaaS de facturación electrónica para
PYMES de Costa Rica (prompt P6). Go 1.27, `net/http`, `pgx/v5`, goose y OpenTelemetry, con la misma arquitectura
hexagonal que Platform API.

> Estado: **incremento 2 de 10** (tenancy y `GET /v1/receivables`). Ver `docs/ESTADO.md` y el plan en `docs/PLAN.md`.

## Requisitos

- Go 1.27.1, `golangci-lint` v2 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`) y, para
  regenerar consultas, sqlc v1.31.1 (`CGO_ENABLED=0 go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`).
- Los logins `receivables_api` y `receivables_migrate` en el proyecto Supabase de dev
  (`scripts/dev/0010_receivables_login_roles.sql`, más las contraseñas con `ALTER ROLE`).
- Un `.env` a partir de `.env.example` (host del pooler y contraseñas).

## Uso

```sh
go run ./cmd/migrate status          # conexión y migraciones (login receivables_migrate, modo sesión)
go run ./cmd/migrate mark-baseline   # SOLO una vez en dev: registra 00001 sin ejecutarla
go run ./cmd/migrate up-by-one       # aplica la siguiente migración pendiente (expand → migrate → contract)
go run ./cmd/api                     # http://localhost:8083/healthz y /readyz (base y JWKS)
curl -H "Authorization: Bearer $JWT" localhost:8083/v1/receivables   # JWT de Supabase con org_id
```

`make` tiene `run`, `build`, `test`, `test-isolation`, `lint`, `migrate-up`, `migrate-status`,
`migrate-mark-baseline`, `sqlc` y `docker`. La imagen se construye con el monorepo como contexto (`make docker`), porque el
módulo de contratos se toma de `../RDL.Contracts` hasta que se publique su tag.

## Estructura

```
cmd/api, cmd/migrate            binarios (consumer y replay llegan en el incremento 4)
internal/adapters/auth          verificación de JWT contra el JWKS de Supabase (copiado de Platform)
internal/adapters/http          router, handlers, Problem Details, paginación por cursor
internal/adapters/postgres      pgx sobre Supavisor: TxManager (set_config), membresía en core, repositorios
internal/adapters/postgres/db   generado por sqlc desde queries/ (no editar)
internal/app                    casos de uso y puertos
internal/domain                 permission (matriz), civil (fecha de negocio), receivable
internal/platform               config (.env), logger, OTel, health
internal/wiring                 arma los handlers; lo usan cmd/* y tests/isolation
migrations                      goose, historial en receivables.goose_db_version
pkg/{tenancy,correlation,requestinfo}  copiados de Platform (ADR 0003)
docs/                           contexto, decisiones (ADR), plan y estado
```

## Configuración

| Variable | Por defecto | Descripción |
|---|---|---|
| `SUPABASE_URL` | — (obligatoria) | Deriva el issuer y el JWKS |
| `DB_POOLER_HOST` | — | Host de Supavisor; con él se arma la URL de `receivables_api` (6543) y `receivables_migrate` (5432) |
| `DB_PASSWORD`, `MIGRATE_DB_PASSWORD` | — | Contraseñas de los logins |
| `DATABASE_URL`, `MIGRATE_DATABASE_URL` | — | Alternativa a `DB_POOLER_HOST` |
| `HTTP_ADDR` | `:8083` | |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | vacío | Sin valor, trazas y métricas quedan en no-op |
| `AUTH_MEMBERSHIP_CACHE_TTL` | `30s` | Máximo 60 s |
