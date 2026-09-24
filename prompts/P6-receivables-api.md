# Prompt · P6 Receivables API (cuentas por cobrar y cobranza) en Go

> **Cómo usarlo**
> 1. Crea el repo `RDL.Receivables.API` vacío (junto a `RDL.Platform.API`, `RDL.Contracts` y `RDL.Billing.API`) y copia en `docs/contexto/` el documento de arquitectura y el planning (los mismos `.md` de `RDL.Platform.API/docs/contexto/`).
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano todo lo que toque RLS, roles de BD, **la aplicación de pagos y los saldos**, y el consumo de eventos.

---

## Rol y objetivo

Eres un ingeniero backend senior en Go. Vas a construir la **Receivables API** de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. Esta API es dueña de las **cuentas por cobrar, los pagos, la aplicación de pagos, los ajustes de saldo, los vencimientos, el aging, la morosidad y la cobranza** (seguimientos y promesas de pago). No emite facturas ni habla con Hacienda: **reacciona a los eventos de Billing** (`InvoiceIssued`, notas de crédito y débito, anulaciones) y publica `PaymentReceived` y `ReceivableSettled`.

Un saldo mal calculado significa cobrarle de más a un cliente o dar por pagada una deuda. La planificación lo dice claro: la lógica de aplicación de pagos (un pago a varias facturas, varios pagos a una factura) es la parte que más se beneficia de tests exhaustivos generados desde las invariantes. Por eso la exactitud de los saldos, la idempotencia del consumo de eventos y el aislamiento pesan más que la velocidad.

**Entregable:** el repositorio `RDL.Receivables.API` listo para staging, con el alcance de las fases **F2, F3 y F5** del planning. Debe compilar, pasar los tests (incluidos los de aislamiento entre tenants y los basados en propiedades de las invariantes), tener migraciones versionadas, Dockerfile, health checks y documentación.

## Contexto que debes leer primero

- `docs/contexto/arquitectura-v1.md`: secciones **2.1** (fronteras), **3.3** (Receivables), **4.3** (FK compuestas), **5** (multiempresa), **6.1 a 6.4** (flujo, catálogo de eventos, confiabilidad: outbox, inbox, reintentos, dead letter), **7.3** (auditoría), **11** y **12** (criterios 1, 3, 4 y 9).
- `docs/contexto/planning-v1.md`: sección **P6 Receivables API** (con sus **invariantes que deben tener test**), "Decisiones que bloquean" y "Cómo trabajamos con Claude Code".
- **Repo de contratos** en `<<ruta a RDL.Contracts>>`. **El contrato manda**; si contradice este prompt, detente y pregunta. Lee sobre todo:
  - `docs/convenciones.md` (dinero como string decimal, fechas, errores, idempotencia, paginación) y `docs/glosario.md` (cuenta por cobrar, pago, aplicación, ajuste);
  - `state-machines/receivable.yaml` y `state-machines/payment.yaml` con sus docs en `docs/maquinas-de-estado/`, incluidos sus **TODOs abiertos**;
  - los schemas y docs de los eventos que consume (`InvoiceIssued`, `CreditNoteIssued`, `DebitNoteIssued`, `InvoiceCancelled`) y de los que produce (`PaymentReceived`, `ReceivableSettled`);
  - `openapi/receivables.yaml` (esqueleto a completar) y la ruta interna `/internal/v1/receivables/by-invoice/{invoiceId}` de `openapi/bff-internal.yaml`;
  - `problems/receivables.yaml` (tipos de error propuestos) y `docs/ESTADO.md` (pendientes abiertos).

  El paquete Go `pkg/events` (DTOs, `money`, `Validator`, `ToOutboxRow`) se importa, no se copia.
- **Platform API** en `<<ruta a RDL.Platform.API>>` es la referencia de arquitectura y de lo ya resuelto. Reutiliza en lugar de reinventar: `pkg/tenancy`, `internal/adapters/auth`, la transacción con `set_config` (`tx.go`, `txmanager.go`), `pkg/correlation`, Problem Details, idempotencia HTTP, auditoría, `internal/wiring` y `tests/isolation`. Lee sus ADR 0003, 0004 y 0007, su `docs/ESTADO.md` y su `CLAUDE.md`.
- Si ya existe `RDL.Billing.API`, léelo para ver cómo escribe los eventos en el outbox; si no, basta con los schemas del repo de contratos.

## Datos del entorno

- Proyecto Supabase (dev): `<<project-ref>>`. **Nunca producción.**
- Conexión de solo lectura para inspeccionar: MCP de Supabase en modo read-only.
- Roles de base: ya existen `receivables_app` y `receivables_migrator` (NOLOGIN). Propón logins `receivables_api` y `receivables_migrate` que los hereden, como hizo Platform, con **el SQL para crearlos**; las contraseñas las asigno yo.
- Conexión de la aplicación: login `receivables_api` por Supavisor en **modo transacción** (puerto 6543), host `<<DB_POOLER_HOST>>`.
- Conexión para migrar en dev: login `receivables_migrate` en **modo sesión** (puerto 5432), con `ALTER ROLE ... SET role = 'receivables_migrator'`.
- Proveedor de identidad: **Supabase Auth** (JWT con `org_id` y `org_roles`, hook de Platform). JWKS: `https://<<project-ref>>.supabase.co/auth/v1/.well-known/jwks.json`.
- Módulo del repo de contratos: `<<ej. bitbucket.org/rdl/contracts>>` en la versión `<<vX.Y.Z>>`.
- Transporte de eventos (P2): `<<broker o cola decidido, o "pendiente">>`.
- Versión de Go: `<<1.27+>>`.

## Paso 1 · Inspeccionar la base antes de escribir código (obligatorio)

El schema `receivables` **ya existe** en Supabase. **No supongas el esquema: léelo.** Con la conexión de solo lectura:

1. Tablas, columnas, tipos, PK, FK, unicidades, `CHECK` e índices de `receivables`. Hoy existen, entre otras: `receivables`, `payments`, `payment_applications`, `receivable_adjustments`, `collection_followups` y `payment_promises`. Fíjate en las unicidades que ya dan idempotencia (`receivables (organization_id, source_event_id)`, `(organization_id, source_invoice_id)`, `receivable_adjustments (organization_id, adjustment_type, source_document_id)`) y en los `CHECK` de estados, montos y reversos.
2. Los dominios de `shared` que usa (`money_amount`, `exchange_rate`, `currency_code`) con su precisión.
3. Roles y GRANT de `receivables_app`: qué puede hacer en `receivables`, `audit` e `integration`, y qué puede leer de `core` y `fiscal`. **Hallazgo conocido del repo de contratos:** `receivables_app` **no tiene `USAGE` en `core`**, pero la API lo necesita para revalidar la membresía y para leer la zona horaria de la organización (el aging usa fechas de negocio). Confírmalo y propón el `GRANT` como migración para `database-platform`: **no lo apliques tú**.
4. Políticas RLS de `receivables` y de `integration`. Ten en cuenta que `shared.current_service()` deriva el servicio **del rol de la conexión**: Receivables solo ve sus propias filas del outbox, del inbox y de las dead letters, y **no puede leer el outbox de Billing**. Los eventos le llegan por el transporte de P2, no leyendo la tabla de otro servicio.
5. Triggers, funciones e historial de migraciones.
6. Compara las tablas con las máquinas de estado del repo de contratos y con los eventos que consume: ¿cabe todo lo que necesitas de `InvoiceIssued` (cliente, número, moneda, total, fechas, condición de venta)?

Entrega un **informe de brechas** en `docs/decisiones/0001-estado-inicial-bd.md` con lo que reutilizas tal cual, lo que no cumple (FK sin `organization_id`, tabla sin RLS, `float`, `timestamp` sin zona, índice faltante para el aging), lo que falta y propones crear con el SQL exacto, las propuestas para `database-platform` (el `GRANT` sobre `core`) y las diferencias con este prompt (los nombres reales mandan).

**Detente y espera mi aprobación del informe antes de crear o modificar cualquier cosa en la base.**

## Paso 2 · Cambios de base de datos (solo si hacen falta)

- **goose**, como Platform, con las migraciones en `migrations/` y el historial en `receivables.goose_db_version`. La primera migración es una **baseline** idempotente marcada como aplicada en dev. Agrega el comando `migrate up-by-one`.
- Esta API **solo migra `receivables`**. Nada sobre `core`, `billing`, `fiscal` o `subscriptions`. Lo de `shared`, `audit` o `integration` va como propuesta en `docs/decisiones/` para `database-platform`.
- Nada destructivo: **expand → migrate → contract**, cada paso en su propia migración.
- Cada tabla de negocio: `organization_id uuid NOT NULL`, PK `uuid`, `UNIQUE (organization_id, id)`, FK compuestas, `timestamptz` y RLS por organización activa.
- Montos solo con los dominios de `shared` (`numeric`). Prohibido `float`, `real` o `double precision`.
- `customer_id` y `source_invoice_id` son identificadores de Billing: **no hay FK hacia `billing`** (otra API, otro schema). Los datos del cliente son el snapshot que llega en el evento.
- Aplica las migraciones **solo en dev y en local**, y muéstrame el SQL antes.

## Paso 3 · Tenancy (igual que Platform) y consumo de eventos

**Peticiones HTTP.** La organización sale **del `org_id` del token verificado**, nunca del body, la query, un header ni la ruta. Reutiliza el diseño de Platform: JWKS con caché, **revalidación de la membresía en `core`** (con caché corta), rol efectivo desde la base y `TenantContext` inmutable. Cada transacción fija `app.current_organization_id` y `app.current_user_id` con `set_config(..., true)`. Con Supavisor: `QueryExecModeExec`, sin caché de statements, y `jsonb` como texto (ADR 0004 de Platform). Hasta que `database-platform` aplique el `GRANT` sobre `core`, **no inventes un atajo**: detente en ese incremento y avísame.

**Eventos.** El consumidor es un proceso aparte del servidor HTTP (`cmd/consumer`) o un componente con su propio ciclo de vida, y sigue este flujo por cada mensaje:
1. Valida el payload con `events.Validator` del repo de contratos. Si no pasa, va a `integration.dead_letters` con el motivo: **no lo "arregles"**.
2. Abre una transacción con `app.current_organization_id` = `organizationId` del sobre (sin usuario). El tenant del consumidor sale **del evento**, nunca de otro lado.
3. Registra el `eventId` en `integration.inbox_messages` (`consumer_service = 'receivables'`). Si ya estaba, el mensaje es un duplicado: confirma y no hagas nada más (criterio de aceptación 3).
4. Aplica el efecto de negocio, el audit event (`actor_type = 'service'`, con el `correlationId` del evento) y, si corresponde, el evento que produce en el outbox. **Todo en la misma transacción.**
5. Si falla, reintentos con backoff; al agotar los reintentos, dead letter. Documenta la política en un ADR.

**Transporte.** El publicador del outbox y el broker son de infraestructura (P2), y la decisión de mensajería está pendiente. **No inventes un broker.** Diseña el consumidor detrás de un puerto (`EventSource` o equivalente) y entrega:
- el caso de uso `HandleEvent(ctx, payload []byte)`, independiente del transporte;
- un adapter de desarrollo que reproduzca eventos desde archivos JSON (`cmd/replay` o `make replay FILE=...`), usando los ejemplos válidos del repo de contratos;
- un `TODO(P2)` con el punto exacto donde se conectará el transporte real (`<<transporte>>` si ya está decidido).

## Paso 4 · Alcance funcional

Base de rutas: `/v1`, sin `organizationId` en la URL (D7 del repo de contratos). Completa `openapi/receivables.yaml` del repo de contratos y alinéalo con este repo.

**F2: esqueleto.** Proyecto, config, logger, OpenTelemetry, `GET /healthz`, `GET /readyz`, Dockerfile, pipeline, logins de BD y un primer endpoint protegido por tenant (`GET /v1/receivables`).

**F3: cuentas por cobrar desde facturas**

| Origen | Qué hace |
|---|---|
| Evento `InvoiceIssued` | Crea la cuenta por cobrar en `open` con saldo igual al total, `issued_on` = `issueDate`, `due_on` = `dueDate`, condición de venta, moneda y el snapshot del cliente (número, identificación y nombre). Idempotente por `eventId` y por `source_invoice_id`. |
| `GET /internal/v1/receivables/by-invoice/{invoiceId}` | Saldo de la cuenta de una factura, para el BFF (`openapi/bff-internal.yaml`). |

**F5: pagos, aplicaciones, ajustes, aging y cobranza**

| Método y ruta / evento | Qué hace | Roles |
|---|---|---|
| `GET /v1/receivables` | Lista con saldo; filtros por estado, cliente y vencidas. | owner, admin, collector, accountant, read_only |
| `GET /v1/receivables/{id}` | Detalle con aplicaciones, ajustes y seguimientos. | ídem |
| `GET /v1/receivables/aging` | Aging por tramos a una fecha (`asOf`) y moneda. | ídem |
| `POST /v1/payments` | Registra un pago (`posted`) y, opcionalmente, sus aplicaciones. Emite `PaymentReceived` y, por cada cuenta que llega a cero, `ReceivableSettled`. Idempotente. | owner, admin, collector |
| `GET /v1/payments`, `GET /v1/payments/{id}` | Lista y detalle con aplicaciones. | lectura |
| `POST /v1/payments/{id}/void` | `posted` → `voided` con motivo; revierte las aplicaciones vigentes y recalcula los saldos. | owner, admin |
| `POST /v1/payment-applications` | Aplica (parte de) un pago ya registrado a una cuenta. | owner, admin, collector |
| `POST /v1/payment-applications/{id}/reverse` | Revierte una aplicación con motivo. | owner, admin |
| `GET/POST /v1/receivables/{id}/follow-ups` | Gestiones de cobro (llamada, email, visita, mensaje, nota). | lectura / owner, admin, collector |
| `POST /v1/receivables/{id}/payment-promises` y `PATCH .../payment-promises/{promiseId}` | Promesas de pago y su estado (`pending`, `kept`, `broken`, `cancelled`). | owner, admin, collector |
| Evento `CreditNoteIssued` | Ajuste `credit_note` que disminuye el saldo de la cuenta de la factura referenciada. | — |
| Evento `DebitNoteIssued` | Ajuste `debit_note` que aumenta el saldo (según `state-machines/receivable.yaml`, reabre una cuenta `paid`). | — |
| Evento `InvoiceCancelled` | Ajuste `cancellation` por el saldo y la cuenta pasa a `cancelled`. | — |

**Reglas de negocio** (el corazón de esta API):
- El estado de la cuenta **se deriva del saldo**, según `state-machines/receivable.yaml`: `open` si el saldo es el total adeudado, `partially_paid` si está entre cero y el total, `paid` si es cero (`settled_at`), `cancelled` si se anuló. Ningún handler cambia `status` a mano.
- Un pago se aplica a **varias** cuentas y una cuenta recibe **varios** pagos. Las aplicaciones se crean y se revierten; **nunca se borran ni se editan**.
- Anular un pago revierte sus aplicaciones vigentes. Revertir una aplicación exige motivo.
- **Moneda:** en V1 un pago solo se aplica a cuentas en su misma moneda (`currency-mismatch`). Si el equipo quiere aplicaciones entre monedas, el tipo de cambio y el redondeo son una decisión nueva: detente y pregunta.
- **Fechas:** `received_on`, `issued_on`, `due_on` y `asOf` son fechas de negocio en la zona horaria de la organización (`core.organizations.timezone`). El aging calcula los días de atraso con esas fechas, no con instantes UTC.
- Los montos se suman y se restan en decimal exacto (`shopspring/decimal` o `cockroachdb/apd`, con un ADR). **Prohibido `float`.** Receivables no calcula impuestos: si una regla parece necesitar redondeo, detente y pregunta.
- **Pendientes del repo de contratos que no debes resolver por tu cuenta:** qué pasa con el castigo (`write_off`), la anulación de una factura ya pagada o parcialmente pagada (¿saldo a favor?), la ausencia de un evento de pago anulado y los tramos del aging (propuesta 0-30, 31-60, 61-90, +90). Propón una opción en el plan y **espera mi decisión**.

**Invariantes que deben cumplirse siempre** (planning P6):
1. La suma de las aplicaciones vigentes de un pago nunca supera el monto del pago.
2. El saldo de una cuenta nunca es negativo y siempre es igual a `original + débitos − créditos − cancelación − aplicaciones vigentes`.
3. Revertir una aplicación deja los saldos **exactamente** como estaban antes de aplicarla.
4. Reprocesar cualquier evento no cambia nada (misma cuenta, mismos ajustes, mismos saldos).
5. El estado siempre corresponde al saldo.

**Concurrencia:** dos pagos aplicados a la misma cuenta al mismo tiempo no pueden dejar el saldo negativo. Bloquea las filas afectadas (`SELECT ... FOR UPDATE` sobre la cuenta y el pago) en un orden estable para no provocar deadlocks, y documenta la estrategia en un ADR.

**Permisos:** la matriz vive en **un solo lugar** del dominio. La propuesta de la tabla (owner, admin y collector registran; accountant y read_only leen; biller sin acceso) es para revisarla en equipo.

**Transversales** (los mismos de Platform): Problem Details con los tipos de `problems/receivables.yaml`; `Idempotency-Key` en todos los `POST` (`integration.idempotency_keys` con `service = 'receivables'`); `X-Correlation-Id` propagado a logs, trazas, auditoría y eventos; **audit event en la misma transacción** en cada operación sensible (pagos, anulaciones, aplicaciones, reversos, ajustes por evento); paginación por cursor; 404 para recursos de otra organización, 403 por rol, 409 por estado.

**Fuera de alcance:** conciliación bancaria, cobro automático, recordatorios por email (dependen del servicio de notificaciones) y cualquier cálculo fiscal.

## Paso 5 · Arquitectura y calidad de código

La misma arquitectura hexagonal (ports & adapters) con Clean Architecture que Platform. Las dependencias apuntan hacia el dominio.

```
RDL.Receivables.API/
├── cmd/
│   ├── api/main.go              # servidor HTTP: config, wiring, arranque; nada de lógica
│   ├── consumer/main.go         # consumidor de eventos (o componente con ciclo de vida propio, justificado en ADR)
│   ├── replay/main.go           # adapter de desarrollo: reproduce eventos desde archivos JSON
│   └── migrate/main.go          # goose: up, up-by-one, down, status
├── internal/
│   ├── domain/                  # sin imports de infraestructura
│   │   ├── receivable/          # agregado CuentaPorCobrar: saldo, estado derivado, ajustes, invariantes
│   │   ├── payment/             # agregado Pago: aplicaciones, reversos, anulación, invariante de suma
│   │   ├── aging/               # tramos y días de atraso sobre fechas de negocio (funciones puras)
│   │   ├── collection/          # seguimientos y promesas de pago
│   │   └── permission/          # matriz de permisos de Receivables
│   ├── app/                     # casos de uso (un struct por caso, incluido HandleEvent) + puertos
│   ├── adapters/
│   │   ├── http/                # handlers, DTOs, validación, Problem Details, router
│   │   ├── postgres/            # repositorios (pgx + sqlc), TxManager, inbox, outbox, dead letters, audit, idempotencia
│   │   ├── auth/                # verificación JWT (igual que Platform)
│   │   └── events/              # decodificación y validación con pkg/events; mapeo a comandos del dominio
│   ├── wiring/                  # arma los handlers y el consumidor; lo usan cmd/* y tests/isolation
│   └── platform/                # config (.env), logger, OTel, health
├── pkg/{tenancy,correlation,requestinfo}/   # reutilizados de Platform (ver ADR)
├── migrations/  queries/  sqlc/external.sql
├── tests/{integration,isolation}/
├── scripts/dev/                 # SQL de logins, e2e.sh
├── docs/{contexto,decisiones}/  docs/ESTADO.md
├── api/openapi.yaml
├── CLAUDE.md, README.md, Dockerfile, Makefile, .golangci.yml, sqlc.yaml
```

**Stack:** el mismo que Platform (`net/http`, `pgx/v5`, `sqlc`, `goose`, `slog`, OpenTelemetry, JWKS, `go-playground/validator`, `golangci-lint`), más una librería decimal (ADR), `pgregory.net/rapid` para los tests basados en propiedades y el módulo de contratos (`pkg/events`, `money`, `Validator`). Ninguna otra dependencia sin justificarla.

**Principios que se tienen que notar en el código:**
- **Dominio rico:** `Receivable` y `Payment` son agregados que protegen sus invariantes. Aplicar, revertir, anular y ajustar son métodos que devuelven error de dominio si no se permiten; el estado se deriva del saldo dentro del agregado.
- **SRP:** un caso de uso por struct; los handlers HTTP y el consumidor solo traducen a caso de uso. El mismo caso de uso sirve a un comando HTTP y a un evento cuando la regla es la misma.
- **OCP/ISP/DIP:** interfaces pequeñas del lado del consumidor, wiring en `internal/wiring` por constructor, sin estado global ni `init()` con efectos. Un tipo de ajuste o de evento nuevo se agrega sin tocar los existentes.
- `TxManager.WithinTenantTx` para HTTP y un equivalente para el consumidor (tenant del evento, sin usuario), ambos escribiendo entidad, audit, inbox y outbox de forma atómica.
- `context.Context` primero; timeouts; graceful shutdown del servidor **y del consumidor** (terminar el mensaje en curso antes de salir).
- Errores de dominio tipados, mapeados a HTTP en un único lugar; sin errores internos al cliente.
- Configuración por variables de entorno validadas al arrancar, leyendo `.env` como Platform.
- Nombres claros, funciones cortas; comenta el **porqué**, no el qué.

## Paso 6 · Tests (escríbelos desde las invariantes, no desde la implementación)

- **Basados en propiedades** con `rapid` (el núcleo de esta API): genera secuencias aleatorias de operaciones (facturas emitidas, pagos, aplicaciones, reversos, anulaciones de pago, notas de crédito y débito, anulaciones de factura, **reprocesos de eventos**) y comprueba después de cada paso las **5 invariantes** del Paso 4. Incluye pagos a varias cuentas, varias cuentas por pago y montos con decimales.
- **Unitarios del dominio:** estado derivado del saldo, transiciones de `state-machines/receivable.yaml` y `payment.yaml` (genera casos a partir de esos YAML si puedes), aging por tramos con fechas en la zona de la organización (una factura que vence hoy no está atrasada; bordes de cada tramo) y matriz de permisos.
- **Consumo de eventos:** cada ejemplo válido de `examples/events/` del repo de contratos produce el efecto esperado; **el mismo evento dos veces no duplica nada**; un payload inválido va a dead letter sin efectos; un evento de otra organización no toca datos de esta.
- **Contrato de lo que publica:** `PaymentReceived` y `ReceivableSettled` escritos en el outbox validan contra sus schemas y tienen el `correlationId` del origen.
- **Concurrencia:** dos aplicaciones simultáneas sobre la misma cuenta (test de integración con goroutines) nunca dejan saldo negativo.
- **Integración** contra la base de dev con el login `receivables_api` (no superusuario): la creación desde evento, un pago con varias aplicaciones y su anulación, todo con su audit y su outbox en la misma transacción.
- **Aislamiento** (`tests/isolation`, `make test-isolation`), los 6 criterios de Platform adaptados:
  1. con el token de A, todos los endpoints sobre recursos de B (cuentas, pagos, aplicaciones, seguimientos, promesas) devuelven 404/403, y B queda intacto;
  2. con la sesión en A, un `SELECT` directo con `receivables_app` no devuelve filas de B;
  3. una aplicación de un pago de A a una cuenta de B se rechaza (FK compuesta);
  4. `receivables_app` no escribe en `core`, `billing`, `fiscal` ni `subscriptions`, ni hace `UPDATE`/`DELETE` en `audit`, ni lee el outbox de otro servicio;
  5. un token sin `org_id` o con la membresía suspendida no accede;
  6. un `organization_id` en el body, query o header no cambia el tenant; **un evento con `organizationId` de B solo afecta a B**.
- **Handlers HTTP** con `httptest`: camino feliz, validación y cross-tenant por endpoint, y reintentos con la misma `Idempotency-Key`.
- **E2E** (`scripts/dev/e2e.sh`): reproducir `InvoiceIssued` con `cmd/replay`, registrar un pago que la cancele, comprobar `ReceivableSettled` en el outbox, anular el pago y comprobar que el saldo volvió.
- Si un test de aislamiento falla por falta de una política en la base, **no debilites el test**: repórtalo y propone la migración.

## Paso 7 · Operación y entrega

- `GET /healthz` y `GET /readyz` (BD y JWKS) en la API; el consumidor expone su propio health y métricas (mensajes procesados, duplicados, reintentos, dead letters, antigüedad del último mensaje).
- Dockerfile multi-stage distroless, usuario no root, con los binarios `api`, `consumer`, `replay` y `migrate`.
- `Makefile` con: `run`, `run-consumer`, `replay`, `build`, `test`, `test-isolation`, `lint`, `migrate-up`, `migrate-status`, `sqlc`.
- `api/openapi.yaml` sincronizado con los handlers y alineado con `openapi/receivables.yaml` del repo de contratos (componentes comunes).
- `CLAUDE.md` con el bloque común de reglas de plataforma adaptado a Receivables, más: "el saldo y el estado solo cambian dentro de los agregados", "las aplicaciones se revierten, nunca se borran", "el tenant de un evento sale de su `organizationId`", "prohibido `float`", y el comando de cierre `go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation`.
- `README.md` con el mismo nivel que el de Platform, incluido cómo reproducir eventos en local.
- `docs/ESTADO.md` como handoff al cerrar cada sesión.
- ADRs cortos: reutilización de `pkg/tenancy`, librería decimal, consumo de eventos (inbox, reintentos, dead letter, transporte pendiente), bloqueo y concurrencia en las aplicaciones, tramos del aging.

## Forma de trabajo

1. **Plan primero:** informe de brechas de la BD, SQL de los logins, propuesta del `GRANT` sobre `core` para `database-platform`, plan de migraciones, diseño de los agregados y del consumidor, opciones para los pendientes del repo de contratos y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por incrementos verticales**, cada uno compilando y con sus tests en verde:
   1. esqueleto, config, health, observabilidad, logins y baseline;
   2. JWT, TenantContext y primer endpoint protegido (requiere el `GRANT` sobre `core`);
   3. dominio de saldos y cuentas por cobrar, con los tests basados en propiedades;
   4. consumidor de eventos (inbox, dead letter, `cmd/replay`) y `InvoiceIssued`;
   5. endpoint interno de saldo para el BFF;
   6. pagos y aplicaciones (con concurrencia), `PaymentReceived` y `ReceivableSettled`;
   7. reversos y anulación de pagos;
   8. notas de crédito, débito y anulaciones de factura;
   9. aging, seguimientos y promesas de pago;
   10. endurecimiento: suite de aislamiento, E2E, autorrevisión, README y TODOs.
3. Al terminar cada incremento: `go build ./... && go vet ./... && golangci-lint run && go test ./...`, actualiza `docs/ESTADO.md`, resume lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente, tenant de un evento tomado de otro lado que no sea su `organizationId`, `float` en montos, `timestamp` sin zona, fechas de negocio calculadas en UTC, saldos que puedan quedar negativos, efectos que se dupliquen al reprocesar un evento, secretos y errores internos expuestos.

## Prohibido

- Conectarte a producción, usar la `service_role` key o usar datos reales.
- Tomar el tenant de algo que no sea el token verificado (HTTP) o el `organizationId` del evento (consumidor).
- Leer el outbox de otro servicio, desactivar RLS o usar un rol con más permisos para saltarte una política.
- Usar `float32`/`float64` o `double precision` para montos.
- Borrar o editar aplicaciones, pagos o ajustes: se revierten o se anulan con motivo.
- Aplicar migraciones sobre `core`, `billing`, `fiscal` o `subscriptions`, o cambios destructivos sin expand/contract.
- Inventar un broker, eventos que no estén en el catálogo (por ejemplo un "pago anulado") o reglas abiertas del repo de contratos: deja un TODO y lístalo al final.

## Definición de terminado

- [ ] Informe de brechas aprobado, logins creados, `GRANT` sobre `core` propuesto (y aplicado por `database-platform`) y migraciones aplicadas en dev, con la baseline.
- [ ] Todos los endpoints de F2, F3 y F5 implementados, documentados en OpenAPI y con tests.
- [ ] Consumidor de `InvoiceIssued`, `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled` con inbox idempotente y dead letter; reprocesar un evento no duplica nada (criterio 3).
- [ ] Las 5 invariantes verificadas con tests basados en propiedades.
- [ ] `PaymentReceived` y `ReceivableSettled` en el outbox en la misma transacción, validados contra sus schemas.
- [ ] Endpoint interno de saldo por factura para el BFF (criterio 9).
- [ ] Suite de aislamiento en verde con `make test-isolation`; E2E en verde.
- [ ] Audit event en cada operación sensible; idempotencia en cada comando; correlationId de punta a punta.
- [ ] `golangci-lint` limpio, cobertura alta en dominio y casos de uso, imagen Docker construida y health checks respondiendo.
- [ ] Lista final de TODOs (transporte P2, pendientes del repo de contratos, decisiones) para revisar en equipo.
