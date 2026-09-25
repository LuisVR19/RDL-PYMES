# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# RDL PYMES — SaaS de facturación electrónica (Costa Rica)

Un solo repo git que contiene varios **repositorios lógicos independientes** (cada uno se publica por separado). Todo
—código, comentarios, documentación, interfaz— se escribe en **español de Costa Rica**, trato de «usted».

| Carpeta | Qué es | Estado |
|---|---|---|
| `RDL.Contracts` | Fuente única de verdad: glosario, convenciones, ownership de schemas, máquinas de estado, eventos (AsyncAPI 3 + JSON Schema 2020-12), OpenAPI 3.1, registro de errores. CLI `contractsctl` + `pkg/events`. Go | v0.2.0 |
| `RDL.Platform.API` | Identidad y tenancy. Dueña de `core` y `subscriptions`. Go, `:8080` | P3 completo |
| `RDL.Billing.API` | Operación comercial. Dueña de `billing`. Productora de `InvoiceIssued`. Go, `:8081` | F2 y F3 |
| `RDL.Portal.Gateway` | BFF del portal: verifica el token, reenvía a la API dueña y compone vistas. **Sin base de datos.** Go, `:8090` | Incrementos 1–4 de P7 |
| `RDL.Web.Portal` | Portal React de la PYME. Consume solo el Portal Gateway (`/portal/v1`) | Diseño, datos simulados |
| `prompts/` | Prompts P0–P8c: la especificación de cada repo, incluidos los que aún no existen (E-Invoice, Receivables, Landing) | — |
| `designs/`, `docs/`, `RDL.Web.Portal/design/` | Prototipos de Claude Design, arquitectura y planning (.docx), PDFs de Hacienda | — |

**Antes de tocar un módulo, lea su `CLAUDE.md`** (`RDL.Contracts/CLAUDE.md`, etc.): ahí están las reglas duras de ese
repo. Este archivo solo cubre lo transversal.

## Jerarquía de autoridad

1. **El contrato manda.** Si un `CLAUDE.md`, un prompt o el documento de arquitectura contradicen a `RDL.Contracts`,
   deténgase y pregunte.
2. Lo que la base ya impone (dominios `shared.*`, `CHECK` de estados, `core.roles`) manda sobre el documento de
   arquitectura (ADR 0001 de contratos).
3. No se inventan campos fiscales, códigos de Hacienda, catálogos, eventos ni estados: `TODO(fiscal)` y a
   `docs/ESTADO.md`.

## Comandos

Requisitos Go: **Go 1.27.1**, `golangci-lint`, `sqlc`; goose se corre vía `./cmd/migrate`. Portal: **Node ≥ 22**.

```sh
# Contracts
cd RDL.Contracts && make check          # build + test + lint + validate + contracts-lint (lo mismo que el CI)
go run ./cmd/contractsctl validate      # coherencia entre artefactos
go run ./cmd/contractsctl lint          # convenciones (dinero como string, tenancy, rutas /v1/…)
go run ./cmd/contractsctl breaking -base ../contracts-v0.1.0   # incompatibilidades contra un tag extraído
go run ./cmd/contractsctl diagram state-machines/invoice.yaml  # regenerar el Mermaid de una máquina de estado

# Platform API / Billing API (mismo Makefile)
make run              # levanta la API
make test             # go vet + go test -race -count=1 ./...
make test-isolation   # suite de aislamiento entre tenants (necesita la base dev)
make lint             # golangci-lint (con --build-tags=integration)
make migrate-status / make migrate-up
make sqlc             # regenera internal/adapters/postgres/db desde queries/ + migrations/
cd RDL.Billing.API && make test-integration   # SQL real de los adapters (tag integration)

# Portal Gateway (sin base de datos: no usa sqlc ni migraciones)
make run              # :8090
make routes           # imprime las 55 rutas públicas que expone
make test / make lint

# Una sola prueba en Go
go test ./internal/domain/invoice -run TestPropertyCalculation -v
go test -tags=integration ./tests/isolation/... -run TestNombre -v

# Portal
npm run dev | build | typecheck | lint | test | test:e2e
npm test -- src/shared/money/money.test.ts -t "redondeo"   # una sola prueba
npx playwright test e2e/shell.spec.ts
npm run texts:import   # regenera src/shared/i18n/messages.design.ts desde design/textos.csv
```

Las suites que tocan la base leen `TEST_DATABASE_URL` o el `.env` del módulo; **sin base se saltan con aviso**, no
fallan. `make` no interpreta las comillas del `.env`: cargue con `set -a; . ./.env; set +a` antes.

`go.mod` de las APIs usa `replace bitbucket.org/rdl/contracts => ../RDL.Contracts`, así que los repos tienen que estar
uno al lado del otro y **un cambio en contratos se ve de inmediato** en Billing (y rompe su build si es incompatible).
El `Dockerfile` de Billing se construye con la carpeta padre como contexto por esa razón (`make docker`).

## Arquitectura

### Servicios Go (Platform, Billing, y los que vengan)

Todos comparten la misma forma hexagonal; al agregar código, respete la capa. El Portal Gateway sigue la misma
estructura **menos** todo lo de base de datos (`postgres/`, `queries/`, `migrations/`, `sqlc/`): no tiene base,
no escribe en ningún schema y no revalida membresías — eso lo hace cada API dueña.

```
cmd/api, cmd/migrate       binarios (servidor HTTP y goose)
internal/domain/<agregado> reglas puras: sin SQL, sin HTTP, sin librerías de infraestructura
internal/app               casos de uso + puertos (ports.go); orquesta transacciones
internal/adapters/         http (handlers, problem details), postgres (sqlc en db/), auth (JWKS), events (outbox)
internal/wiring            arma el router y las dependencias; una regla nueva es un Check aquí
internal/platform/         config, health, logger, telemetry (OTel)
internal/archtest          reglas que ningún linter cubre — p. ej. prohibir float en todo el código de producción
pkg/                       correlation, requestinfo, tenancy
queries/ + sqlc/ + migrations/   fuente de internal/adapters/postgres/db (no editar lo generado)
tests/isolation            aislamiento entre organizaciones sobre el router real (tag integration)
```

**Tenancy.** La organización sale **solo** del `org_id` del JWT verificado (Supabase Auth, ES256 vía JWKS) más la
membresía revalidada en la base. Nunca de un body, query, header ni URL. Toda operación corre dentro de
`TxManager.WithinTenantTx`, que fija `app.current_organization_id` y `app.current_user_id` con `set_config(..., true)`
—nunca `SET` de sesión, porque Supavisor va en modo transacción— y RLS hace el resto. Recurso ajeno → 404, rol
insuficiente → 403, estado que no permite la operación → 409.

**Ownership de schemas** (`RDL.Contracts/ownership/ownership.yaml`, verificado por `contractsctl validate`): cada API
escribe solo en su schema (`core`/`subscriptions`, `billing`, `fiscal`, `receivables`); `audit` es append-only e
`integration` se comparte. Lo que haga falta en un schema ajeno va como **propuesta a `database-platform`**, no como
migración local.

**Eventos.** Entidad + fila de outbox en la **misma transacción**, con el evento validado contra su JSON Schema de
`pkg/events`. La emisión nunca llama a otro servicio: Billing responde 201 aunque E-Invoice esté caída. Igual el
audit event con `correlationId`.

**pgx en `QueryExecModeExec`** (Supavisor): los `jsonb`, los arreglos y **los montos** viajan como texto y se
convierten en SQL (`::text::jsonb`, `::text[]::uuid[]`, `::text::numeric` al escribir, `::text` al leer). Nunca un
tipo numérico del driver.

**Migraciones**: goose en `migrations/`, historial en el schema propio, patrón expand → migrate → contract
(`go run ./cmd/migrate up-by-one` aplica solo la fase expand). Solo dev y local.

### Portal

Vite 8 · React 19 · TS 6 estricto · React Router 8 · TanStack Query · Radix UI · CSS Modules sobre tokens · big.js.
La capa de datos son **puertos** (`src/shared/api/ports.ts`) con un adaptador simulado (`mock/` + `src/mocks`); hoy no
hay llamadas reales. `src/app/screens.ts` es el registro de las 36 pantallas (ruta, permiso, referencia de diseño) y
`src/app/router.tsx` las conecta. La fuente de verdad visual es `RDL.Web.Portal/design/` (y el prototipo
`design/referencias/RDL Portal · Prototipo completo.dc.html`): lea los estilos exactos, no adivine.

### Invariantes que valen en los tres repos

- **Montos decimales exactos.** `numeric` en la base, `shopspring/decimal` en Go, `string` + `big.js` en TS. Prohibido
  `float`/`double`/`number`/`parseFloat` (lo bloquean `internal/archtest` y oxlint). Redondeo solo en `money.Round`
  (D2: 5 decimales, mitad hacia arriba, por campo de línea); dividir entre 100 es `Shift(-2)`, nunca `decimal.Div`.
- **Fechas** en UTC (`timestamptz`, RFC 3339 con `Z`); las de negocio (`issueDate`, `due_date`) en la zona de la
  organización, con `formatBusinessDate`, nunca `new Date('YYYY-MM-DD')`.
- **Permisos** en un solo lugar (`internal/domain/permission` / `membership`, `shared/permissions` + `can()`). Nada de
  `if role == ...` en handlers ni pantallas. Roles: `owner, admin, biller, collector, accountant, read_only`.
- **Errores**: Problem Details (RFC 9457) con `type = urn:rdl:<servicio>:problem:<código>` registrado en
  `RDL.Contracts/problems/<servicio>.yaml`.
- Un schema publicado (`vN` con tag) es inmutable salvo cambios compatibles; lo incompatible va en `vN+1` y en Go en un
  struct `...V2`, sin tocar `V1`.

## Documentación viva

- `docs/CHECKPOINTS.md` — **tablero de control de todo el proyecto**: qué está hecho, qué falta, qué bloquea a
  qué y las acciones manuales pendientes en dev. Se lee primero.
- `<módulo>/docs/ESTADO.md` — **handoff entre sesiones**: qué se terminó, qué falta, TODOs y decisiones abiertas. Se
  lee al empezar y se actualiza al cerrar cada incremento. Manda sobre el resumen de `CHECKPOINTS.md`.
- `<módulo>/docs/decisiones/` — ADRs. Leer el 0001 de cada repo antes de tocar su base o sus contratos.
- `<módulo>/docs/contexto/` — arquitectura y planning (copias del `.docx` de `docs/`).
- `RDL.Contracts/CHANGELOG.md` — todo cambio de contrato necesita entrada aquí y 2 aprobaciones.

## Reglas de trabajo

- **No ejecute comandos de git**: los repositorios los publica el usuario (incluidos los tags de contratos).
- Nunca use la `service_role` key, producción, datos reales ni certificados reales. El MCP de Supabase está en
  **solo lectura** contra el proyecto de **dev** (`dzlsnsstuqpxvwegeqcy`).
- No debilite un test de aislamiento ni desactive RLS para que algo pase: repórtelo y proponga la migración.
- Al terminar una tarea, corra la cadena completa del `CLAUDE.md` del módulo y revise el diff buscando: consultas sin
  tenant, escrituras a schemas ajenos, confianza en ids del cliente, `float` en montos, timestamps sin zona, facturas
  emitidas modificables, eventos que no validen contra su schema, secretos y errores internos expuestos.
