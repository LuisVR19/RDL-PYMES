# Billing API

Operación comercial del SaaS de facturación electrónica para PYMES de Costa Rica: clientes, productos y servicios,
facturas y sus líneas, cálculo de montos e impuestos, numeración visible y emisión. Es el **productor** de
`InvoiceIssued`: E-Invoice crea el documento fiscal y Receivables la cuenta por cobrar a partir de ese evento.

Go 1.27, arquitectura hexagonal, sobre el schema `billing` de Supabase Postgres. Alcance: fases F2 y F3 del planning.
Estado y pendientes: [`docs/ESTADO.md`](docs/ESTADO.md). Decisiones: [`docs/decisiones/`](docs/decisiones).

## Qué garantiza

- **Aislamiento:** la organización sale solo del `org_id` del JWT verificado, con la membresía revalidada en la base
  en cada request (caché de 30 s). RLS por organización en todas las tablas; FK compuestas `(organization_id, id)`.
  Un recurso de otra organización responde 404, un rol insuficiente 403.
- **Montos exactos:** decimales como string en JSON y como `numeric` en la base; nunca `float`. Redondeo D2 del
  contrato (5 decimales, mitad hacia arriba, cada campo de línea); los totales son sumas exactas de las líneas.
- **Lo emitido no cambia:** después de emitir, ni la API ni la base (triggers) dejan editar la factura ni sus líneas;
  cambiar el cliente o el producto no la altera (snapshots).
- **Emisión atómica y desacoplada:** número visible, snapshot, historial, audit e `InvoiceIssued` v1 (validado contra
  el JSON Schema del contrato) en una sola transacción, sin llamar a otros servicios.
- **Idempotencia** en todos los `POST` (`Idempotency-Key`, 24 h) y **audit** de cada operación sensible con
  `correlationId`, en la misma transacción que el cambio.

## Endpoints

Todo `/v1/...` exige `Authorization: Bearer <access token de Supabase>` con `org_id` y una membresía activa en esa
organización. La organización nunca viaja en la URL, la query, el body ni un header. Contrato completo en
[`api/openapi.yaml`](api/openapi.yaml) (alineado con `openapi/billing.yaml` del repo de contratos).

| Ruta | Qué hace | Roles |
|---|---|---|
| `GET /healthz` · `GET /readyz` | El proceso vive · la base y el JWKS responden | público |
| `GET /v1/customers` | Clientes: `limit`, `cursor`, `q` (nombre o identificación), `active` | todos |
| `POST /v1/customers` | Crea un cliente. 409 si la identificación ya existe en la organización | escritores |
| `GET /v1/customers/{id}` · `PATCH` | Detalle · edita o desactiva (la identificación no se edita) | todos · escritores |
| `GET /v1/products` | Productos con impuestos: `limit`, `cursor`, `q` (código o descripción), `active` | todos |
| `POST /v1/products` | Crea un producto o servicio (CABYS, unidad, precio, moneda, impuestos). 409 si el código existe | escritores |
| `GET /v1/products/{id}` · `PATCH` | Detalle · edita o desactiva; `taxes` reemplaza la lista | todos · escritores |
| `GET /v1/invoices` | Facturas con líneas: `status`, `documentType`, `customerId`, `requiresCorrection`, `issuedFrom`/`issuedTo` | todos |
| `POST /v1/invoices` | Crea un borrador (líneas opcionales; snapshot y tasas desde la base) | escritores |
| `GET /v1/invoices/{id}` · `PATCH` · `DELETE` | Detalle · edita el encabezado · descarta. Solo en borrador (409) | todos · escritores |
| `PUT /v1/invoices/{id}/lines` | Reemplaza las líneas del borrador y recalcula | escritores |
| `POST /v1/invoices/{id}/issue` | **Emite** (draft → issued) | escritores |
| `GET /v1/invoices/{id}/history` | Cambios de estado | todos |
| `GET /v1/document-sequences` · `PUT /v1/document-sequences/{tipo}` | Numeración visible: prefijo + 8 dígitos, solo si nunca se usó | owner, admin |

Escritores = `owner`, `admin`, `biller`. `collector`, `accountant` y `read_only` solo leen. La matriz vive en
`internal/domain/permission` y es una propuesta para revisar en equipo. Errores: Problem Details (RFC 9457) con
`type` = `urn:rdl:billing:problem:<código>` de `problems/billing.yaml` del contrato.

## Levantar en local

Requisitos: Go 1.27.1, `golangci-lint` y `sqlc` (`brew install go golangci-lint sqlc`). El repo de contratos debe
estar al lado (`../RDL.Contracts`): `go.mod` lo usa con `replace` hasta que tenga un tag publicado.

1. Copie `.env.example` a `.env` y complete `DB_POOLER_HOST`, `DB_PASSWORD` (login `billing_api`) y
   `MIGRATE_DB_PASSWORD` (login `billing_migrate`). Los logins se crean con `scripts/dev/0010_billing_login_roles.sql`.
2. `make migrate-status` / `make migrate-up` (modo sesión, puerto 5432, como `billing_migrator`).
3. `make run` → `:8081` (Platform usa `:8080`). `curl localhost:8081/readyz`.

### Variables de entorno

| Variable | Obligatoria | Por defecto | Para qué |
|---|---|---|---|
| `SUPABASE_URL` | sí | — | Proyecto (issuer y JWKS de Auth, project ref del pooler) |
| `DB_POOLER_HOST` | sí* | — | Host de Supavisor (se acepta la cadena completa del Dashboard) |
| `DB_PASSWORD` · `MIGRATE_DB_PASSWORD` | sí | — | Contraseñas de `billing_api` · `billing_migrate` |
| `DATABASE_URL` · `MIGRATE_DATABASE_URL` | no | — | *En lugar de `DB_POOLER_HOST` |
| `HTTP_ADDR` | no | `:8081` | |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | vacío | Vacío = sin exportar trazas ni métricas |
| `AUTH_MEMBERSHIP_CACHE_TTL` | no | `30s` | Máximo 60 s: una suspensión rige en ese plazo |
| `DB_MAX_CONNS` · `DB_STATEMENT_TIMEOUT` | no | `10` · `5s` | |

## Comandos

| Comando | Qué hace |
|---|---|
| `make run` · `make build` | Arranca la API · compila `bin/api` y `bin/migrate` |
| `make test` | `go vet` + tests con `-race` (dominio, casos de uso, handlers, contrato del evento) |
| `make test-integration` | SQL de los adapters y la emisión completa contra dev con `billing_api`; revierte todo |
| `make test-isolation` | Suite de aislamiento (6 criterios); necesita los fixtures de `scripts/dev/0011_…sql` |
| `make lint` | golangci-lint (incluye los archivos `integration`) |
| `make migrate-up` · `make migrate-status` · `go run ./cmd/migrate up-by-one` | Migraciones goose de `billing` (expand → migrate → contract) |
| `make sqlc` | Regenera `internal/adapters/postgres/db` desde `queries/` |
| `make docker` | Imagen distroless no root con `api` y `migrate` (contexto: la carpeta padre) |
| `bash scripts/dev/e2e.sh` | E2E con login real en Supabase Auth (ver el script: **confirma datos en dev**) |

Cierre de cada tarea: `go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation`.

## Pruebas

- **Dominio:** máquina de estados, snapshots, numeración, permisos, validaciones y cálculo, con **tests basados en
  propiedades** (`rapid`) de las 5 invariantes del planning contra una referencia racional exacta, y los ejemplos de
  `InvoiceIssued` del repo de contratos recalculados idénticos.
- **Contrato del evento:** todo `InvoiceIssued` que Billing arma valida contra `invoice-issued.v1.json`; un test falla
  si aparece un `float` en el código, el SQL o cualquier struct del dominio o DTO.
- **Integración y aislamiento:** contra la base dev con el login de la app (no superusuario). Corren el código real
  dentro de una transacción que se revierte: no dejan datos, audit ni eventos. Las variables `TEST_MEMBER_SUBJECT` y
  `TEST_MEMBER_ORG` (una membresía de prueba de Platform) habilitan las pruebas que necesitan una organización real.

## Problemas comunes

| Síntoma | Causa y arreglo |
|---|---|
| `configuración inválida: DB_POOLER_HOST es obligatoria` | Falta el host en `.env` (Dashboard → Connect → Session pooler) |
| `migrate: la sesión corre como "billing_migrate"` | Falta `alter role billing_migrate set role = 'billing_migrator'` (script 0010) |
| `permission denied for schema core` al migrar `00003` | Falta `grant usage on schema core to billing_migrator` (script 0010) |
| 403 `membership-inactive` con un token válido | La membresía está suspendida o el `org_id` del token no es de una organización del usuario |
| 403 `no-active-organization` | El token no trae `org_id`: elegir organización en Platform y refrescar el token |
| 422 "la tarifa … no está vigente en el catálogo fiscal" | `fiscal.tax_rates` está vacío: lo carga fiscal; Billing nunca inventa una tasa |
| `cannot find encode plan` con un arreglo | Con Supavisor (`QueryExecModeExec`) los arreglos viajan como texto: `::text[]::uuid[]` |
| Las suites de Go dicen "saltada" | Sin base (`.env`) o sin fixtures: el aviso dice cuál |
