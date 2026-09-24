# Prompt · P4 Billing API (operación comercial) en Go

> **Cómo usarlo**
> 1. Crea el repo `RDL.Billing.API` vacío (junto a `RDL.Platform.API` y `RDL.Contracts`) y copia en `docs/contexto/` el documento de arquitectura y el planning (los mismos `.md` de `RDL.Platform.API/docs/contexto/`).
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano todo lo que toque RLS, roles de BD, **cálculo de montos e impuestos** y el evento `InvoiceIssued`.

---

## Rol y objetivo

Eres un ingeniero backend senior en Go. Vas a construir la **Billing API** de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. Esta API es dueña de la **operación comercial** de cada organización con sus propios clientes: clientes, productos y servicios, facturas y sus líneas, cálculo de subtotales, descuentos e impuestos, numeración visible al negocio y estados comerciales. Es el **productor** de los eventos que mueven al resto del sistema: E-Invoice crea el documento fiscal y Receivables la cuenta por cobrar a partir de `InvoiceIssued`. Si esta API calcula mal un monto, pierde un céntimo al redondear o emite un evento incompleto, el error llega a Hacienda y al saldo del cliente. Por eso la exactitud, la inmutabilidad de lo emitido y el aislamiento pesan más que la velocidad.

**Entregable:** el repositorio `RDL.Billing.API` listo para staging, con el alcance de las fases **F2 y F3** del planning. Debe compilar, pasar los tests (incluidos los de aislamiento entre tenants y los basados en propiedades del cálculo), tener migraciones versionadas, Dockerfile, health checks y documentación.

## Contexto que debes leer primero

- `docs/contexto/arquitectura-v1.md`: secciones **2.1** (fronteras), **3.1** (Billing), **4** (base de datos, en especial 4.3 FK compuestas y 4.4 snapshots), **5** (multiempresa), **6.1 a 6.4** (flujo de emisión, eventos, confiabilidad), **7.3 y 7.4** (auditoría e inmutabilidad), **11** y **12** (criterios 2 y 5).
- `docs/contexto/planning-v1.md`: sección **P4 Billing API**, "Decisiones que bloquean" y "Cómo trabajamos con Claude Code".
- **Repo de contratos** en `<<ruta a RDL.Contracts>>`: glosario, convenciones (dinero, fechas, errores, idempotencia, paginación), máquina de estados de Invoice, OpenAPI de Billing y el schema `invoice-issued.v1.json`. El paquete Go `pkg/events` se importa, no se copia. **El contrato manda.** Si el contrato y este prompt se contradicen, detente y pregunta. Si `InvoiceIssued` v1 aún no está publicado, **detente**: no lo inventes.
- **Platform API** en `<<ruta a RDL.Platform.API>>`, ya construida y probada. Es la referencia de arquitectura y de lo que ya se resolvió. Reutiliza sus soluciones en lugar de reinventarlas:
  - `pkg/tenancy` (Identity, TenantContext, middlewares, caché de membresía): cópialo tal cual o impórtalo, y justifica la opción en un ADR;
  - `internal/adapters/auth` (verificación JWT con JWKS de Supabase);
  - `internal/adapters/postgres/tx.go` y `txmanager.go` (transacción con `set_config` de `app.current_organization_id` y `app.current_user_id`);
  - `pkg/correlation`, Problem Details, idempotencia con `integration.idempotency_keys` y auditoría con `audit.audit_events`;
  - `internal/wiring` y `tests/isolation` (suite de aislamiento con el router real y un verificador de JWT de prueba);
  - `docs/decisiones/` (sobre todo 0003 sesión RLS, 0004 pooler y pgx, 0007 suite de aislamiento), `docs/ESTADO.md` y `CLAUDE.md`.

## Datos del entorno

- Proyecto Supabase (dev): `<<project-ref>>`. **Nunca producción.**
- Conexión de solo lectura para inspeccionar: MCP de Supabase en modo read-only.
- Roles de base: ya existen `billing_app` y `billing_migrator` (NOLOGIN). Hacen falta logins que los hereden, como hizo Platform (`platform_api`, `platform_migrate`): propón `billing_api` y `billing_migrate` y **el SQL para crearlos**; las contraseñas las asigno yo.
- Conexión de la aplicación: login `billing_api` por el pooler de Supavisor en **modo transacción** (puerto 6543), host `<<DB_POOLER_HOST>>`.
- Conexión para migrar en dev: login `billing_migrate` en **modo sesión** (puerto 5432), con `ALTER ROLE ... SET role = 'billing_migrator'`.
- Proveedor de identidad: **Supabase Auth**, igual que Platform. El JWT trae `org_id` y `org_roles` (hook de Platform). JWKS: `https://<<project-ref>>.supabase.co/auth/v1/.well-known/jwks.json`.
- Módulo del repo de contratos: `<<ej. bitbucket.org/rdl/contracts>>` en la versión `<<vX.Y.Z>>`.
- Versión de Go: `<<1.27+>>` (la misma que Platform).

## Paso 1 · Inspeccionar la base antes de escribir código (obligatorio)

El schema `billing` **ya existe** en Supabase, con tablas bastante completas. **No supongas el esquema: léelo.** Con la conexión de solo lectura:

1. Lista tablas, columnas, tipos, PK, FK, unicidades, `CHECK` e índices de `billing`. Hoy existen, entre otras: `customers`, `products`, `product_taxes`, `invoices` (con `document_type` para factura, nota de crédito y nota de débito), `invoice_lines`, `invoice_line_taxes`, `invoice_payment_methods`, `invoice_status_history` y `document_sequences`. Confirma todo y busca lo que falte.
2. Lista los dominios de `shared` que usa (`money_amount`, `quantity`, `percentage`, `exchange_rate`, `currency_code`, `cabys_code`...) con su precisión y escala: el código de cálculo debe respetarlas.
3. Lista roles y GRANT: qué puede hacer `billing_app` en `billing`, qué puede **leer** en `core` (organizaciones, membresías, sucursales: Billing necesita revalidar la membresía y validar `branch_id`) y qué puede escribir en `audit` e `integration`. Confirma que **no** puede escribir en `core`, `fiscal`, `receivables` ni `subscriptions`.
4. Lista las políticas RLS de `billing` y de las tablas de `core` que Billing lee, y qué funciones de sesión usan (`shared.current_organization_id()`, `shared.current_user_id()`).
5. Revisa triggers, funciones y si hay historial de migraciones para `billing`.
6. Compara las tablas con la máquina de estados de Invoice y el schema `InvoiceIssued` v1 del repo de contratos: ¿cabe todo lo que el evento necesita? ¿Coinciden los estados (`draft`, `issued`, `cancelled`) y el campo `requires_correction`?

Entrega un **informe de brechas** en `docs/decisiones/0001-estado-inicial-bd.md` con:
- lo que ya existe y vas a **reutilizar tal cual**;
- lo que existe pero **no cumple** las reglas de este prompt o del contrato (FK sin `organization_id`, tabla sin RLS, `float`, `timestamp` sin zona, columna que el evento necesita y no existe, etc.);
- lo que **falta** y propones crear, con el SQL exacto;
- las diferencias con lo que dice este prompt (los nombres reales mandan sobre los de este prompt).

**Detente y espera mi aprobación del informe antes de crear o modificar cualquier cosa en la base.**

## Paso 2 · Cambios de base de datos (solo si hacen falta)

- Herramienta: **goose**, igual que Platform, con las migraciones SQL en `migrations/` y el historial dentro del schema `billing` (`billing.goose_db_version`).
- La **primera migración es una baseline** que refleja lo que ya existe, escrita de forma idempotente (`IF NOT EXISTS`) y marcada como aplicada en dev. Así el repo pasa a ser la fuente de verdad de `billing`.
- Esta API **solo migra `billing`**. Nunca generes DDL ni DML sobre `core`, `fiscal`, `receivables` o `subscriptions`. Lo que haga falta en `shared`, `audit` o `integration` (propiedad compartida) va como propuesta separada en `docs/decisiones/` para `database-platform`, no como migración de este repo.
- Nada destructivo: patrón **expand → migrate → contract**, cada paso en su propia migración. Añade el comando `migrate up-by-one` como en Platform.
- Cada tabla de negocio lleva `organization_id uuid NOT NULL`, PK `uuid`, `UNIQUE (organization_id, id)`, FK compuestas `(organization_id, x_id)` (también hacia `core.branches` y hacia la factura referenciada de una nota), `timestamptz` y RLS habilitado con política por organización activa.
- Montos, cantidades y porcentajes **solo** con los dominios de `shared` (`numeric`). Prohibido `float`, `real` o `double precision`.
- Aplica las migraciones **solo en dev y en local**, y muéstrame el SQL antes de aplicarlo.

## Paso 3 · Tenancy (igual que Platform)

**Regla de oro:** la organización activa sale **del `org_id` del token verificado**, nunca del body, la query, un header ni la ruta.

- Reutiliza el diseño de Platform (ADR 0003): verificar JWT con JWKS (firma, `exp`, `aud`, `iss`), tomar `sub` y `org_id`, **revalidar la membresía activa en `core`** con caché corta (30 s) y construir un `TenantContext` inmutable.
- El rol efectivo sale de la membresía revalidada en la base, no del claim `org_roles` (el claim puede estar viejo).
- Cada transacción ejecuta `set_config('app.current_organization_id', ..., true)` y `set_config('app.current_user_id', ..., true)` antes de cualquier consulta. Nunca `SET` de sesión.
- Con Supavisor en modo transacción: pgx con `QueryExecModeExec` y sin caché de statements (ver ADR 0004 de Platform). Los parámetros `jsonb` se envían como texto (`::text::jsonb`), porque un `[]byte` viaja como `bytea`.
- Un `branch_id` recibido en el body **se valida** contra `core.branches` de la organización activa (existe, está activa y es de esta organización). Nunca se confía en él solo porque venga.
- Un `customer_id` o `product_id` de otra organización se comporta como inexistente: **404**.

## Paso 4 · Alcance funcional

Base de rutas: `/v1`. El `id` de la organización nunca viaja en la URL: todas las rutas operan sobre la organización activa del token. Si el repo de contratos define otra convención de rutas, **manda el contrato**.

**F2: esqueleto**
- Proyecto, config, logger, OpenTelemetry, `GET /healthz`, `GET /readyz`, Dockerfile, pipeline, logins de BD y un primer endpoint protegido por tenant (`GET /v1/customers`).

**F3: clientes, productos, facturas y emisión**

*Clientes*
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `GET /v1/customers` | Lista paginada, con búsqueda por nombre o identificación y filtro `active`. | todos |
| `POST /v1/customers` | Crea un cliente. Identificación única por organización (409 si se repite). Idempotente. | owner, admin, biller |
| `GET /v1/customers/{id}` | Detalle. | todos |
| `PATCH /v1/customers/{id}` | Edita o desactiva (baja lógica). **No altera facturas emitidas.** | owner, admin, biller |

*Productos y servicios*
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `GET /v1/products` | Lista paginada con búsqueda por código o descripción y filtro `active`. | todos |
| `POST /v1/products` | Crea un producto o servicio con código CABYS, unidad de medida, precio, moneda e impuestos. Código único por organización. Idempotente. | owner, admin, biller |
| `GET /v1/products/{id}` | Detalle con sus impuestos. | todos |
| `PATCH /v1/products/{id}` | Edita o desactiva. **No altera facturas emitidas.** | owner, admin, biller |

*Facturas (borrador)*
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `POST /v1/invoices` | Crea una factura en `draft` con cliente, sucursal, moneda, condición de venta y, opcionalmente, líneas. Idempotente. | owner, admin, biller |
| `GET /v1/invoices` | Lista paginada con filtros por estado, cliente, fechas y `requiresCorrection`. | todos |
| `GET /v1/invoices/{id}` | Detalle con líneas, impuestos, medios de pago y totales. | todos |
| `PATCH /v1/invoices/{id}` | Edita el encabezado. **Solo en `draft`** (409 si no). | owner, admin, biller |
| `PUT /v1/invoices/{id}/lines` | Reemplaza las líneas del borrador y recalcula. Propón en el plan si conviene esto o `POST/PATCH/DELETE` por línea. | owner, admin, biller |
| `DELETE /v1/invoices/{id}` | Descarta un borrador. Nunca una factura emitida. | owner, admin, biller |
| `GET /v1/invoices/{id}/history` | Historial de estados (`invoice_status_history`). | todos |

*Emisión*
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `POST /v1/invoices/{id}/issue` | `draft` → `issued`. Exige `Idempotency-Key`. Ver reglas abajo. | owner, admin, biller |

*Numeración*
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `GET /v1/document-sequences` | Secuencias por tipo de documento y sucursal. | owner, admin |
| `PUT /v1/document-sequences/{documentType}` | Configura prefijo y número inicial, solo si aún no se usó. | owner, admin |

**Reglas de la emisión** (el corazón de esta API):
1. Solo desde `draft`. Emitir una factura ya emitida devuelve 409; reintentar con la misma `Idempotency-Key` devuelve la misma respuesta 201.
2. Valida que tenga cliente activo, al menos una línea, sucursal válida y totales consistentes.
3. Copia los **snapshots** a la factura: datos del cliente (identificación, nombre, email, teléfono, dirección) y, en cada línea, los del producto (código, CABYS, descripción, unidad, precio e impuestos). Desde ese momento, cambiar el cliente o el producto **no altera nada** de lo emitido (criterio 5).
4. Asigna el **número visible** desde `document_sequences` bajo bloqueo (`SELECT ... FOR UPDATE`), sin huecos por concurrencia dentro de la transacción. Documenta en un ADR qué pasa con los números si la transacción se revierte.
5. En **una sola transacción**: actualiza la factura, escribe `invoice_status_history`, el audit event y el mensaje en `integration.outbox_messages` con `InvoiceIssued` v1 **validado contra su JSON Schema** (usa el DTO de `pkg/events` del repo de contratos).
6. Responde **201 sin llamar a ningún otro servicio**. Billing emite aunque E-Invoice esté caída (criterio 2). Publicar el outbox es trabajo del worker de infraestructura (P2), no de esta API.

**Cálculo de montos** (decisión por defecto del planning: *Billing calcula y E-Invoice valida*; si ya se decidió otra cosa, detente y pregunta):
- Todo en decimal exacto: `github.com/shopspring/decimal` o `github.com/cockroachdb/apd` (justifica la elección en un ADR). **Prohibido `float32`/`float64`** en montos, cantidades, tasas o tipo de cambio.
- El cálculo vive en el **dominio**, en una sola función pura por nivel (línea → impuestos de la línea → totales de la factura), sin tocar la base.
- La regla de redondeo (escala, modo y en qué paso se redondea) sale del repo de contratos. Si el contrato no la define, **no la inventes**: detente y pregunta.
- Invariantes que deben cumplirse siempre: la suma de las líneas cuadra con los totales, ningún monto es negativo, el descuento no supera el precio y ningún redondeo pierde céntimos.

**Fuera de alcance** (fases siguientes; el diseño **no** debe impedirlas): F4 (condiciones de venta y medios de pago completos, exoneraciones reales, campos que pida el XML, consumo de `ElectronicDocumentRejected` para marcar `requires_correction`) y F5 (notas de crédito y débito, anulación con motivo y los eventos `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled`). La tabla `invoices` ya soporta notas y anulación: modela el dominio para que se agreguen sin reescribirlo.

**No inventes códigos fiscales.** CABYS, tipos y tarifas de impuesto, unidades de medida, condición de venta, medios de pago y tipos de identificación se guardan como códigos. Valida su **formato**, pero no inventes catálogos ni reglas de Hacienda: deja un `TODO(fiscal)` y lístalo al final. El catálogo CABYS es de E-Invoice.

**Permisos:** la matriz vive en **un solo lugar** (`internal/domain/...`), como en Platform. La propuesta de arriba (owner, admin y biller escriben; collector, accountant y read_only leen) es para revisarla en equipo.

**Transversales en todos los endpoints** (los mismos de Platform):
- Problem Details (RFC 9457) con `type` estable según la convención del repo de contratos (`urn:rdl:billing:problem:<código>`).
- `Idempotency-Key` en todos los `POST`, guardada en `integration.idempotency_keys` con `service = 'billing'`.
- `X-Correlation-Id` propagado a logs, trazas, audit events y al `correlationId` del evento.
- **Cada operación sensible escribe un audit event en la misma transacción** (crear o editar cliente, producto o factura, emitir, descartar). En la emisión, con el estado anterior y el nuevo.
- Paginación por cursor (`limit`, `cursor`, `nextCursor`).
- Recurso de otra organización: **404**. Rol insuficiente: **403**. Estado que no permite la operación: **409**.

## Paso 5 · Arquitectura y calidad de código

La misma arquitectura hexagonal (ports & adapters) con Clean Architecture que Platform. Las dependencias apuntan siempre hacia el dominio.

```
RDL.Billing.API/
├── cmd/
│   ├── api/main.go              # composition root: config, wiring, arranque; nada de lógica
│   └── migrate/main.go          # goose: up, up-by-one, down, status (se niega a correr si no es billing_migrator)
├── internal/
│   ├── domain/                  # sin imports de infraestructura
│   │   ├── money/               # Money, Quantity, Percentage sobre decimal; redondeo en un solo lugar
│   │   ├── customer/
│   │   ├── product/
│   │   ├── invoice/             # agregado Invoice + líneas, máquina de estados, cálculo, snapshots
│   │   ├── numbering/           # secuencias y formato del número visible
│   │   └── permission/          # matriz de permisos de Billing
│   ├── app/                     # casos de uso (un struct por caso) + puertos (interfaces)
│   ├── adapters/
│   │   ├── http/                # handlers, DTOs, validación, Problem Details, router
│   │   ├── postgres/            # repositorios (pgx + sqlc), TxManager con sesión RLS, outbox, audit, idempotencia
│   │   ├── auth/                # verificación JWT (igual que Platform)
│   │   └── events/              # mapeo Invoice → InvoiceIssuedV1 de pkg/events + validación con el JSON Schema
│   ├── wiring/                  # arma los handlers; lo usan cmd/api y tests/isolation
│   └── platform/                # config (.env), logger, OTel, health
├── pkg/{tenancy,correlation,requestinfo}/   # reutilizados de Platform (ver ADR)
├── migrations/  queries/  sqlc/external.sql
├── tests/{integration,isolation}/
├── scripts/dev/                 # SQL de logins y e2e.sh
├── docs/{contexto,decisiones}/  docs/ESTADO.md
├── api/openapi.yaml
├── CLAUDE.md, README.md, Dockerfile, Makefile, .golangci.yml, sqlc.yaml
```

**Stack:** el mismo que Platform: `net/http` con el router de Go 1.22+, `pgx/v5` + `pgxpool`, `sqlc`, `goose`, `log/slog` en JSON, OpenTelemetry, la misma librería de JWKS, `go-playground/validator`, `golangci-lint`. Más: una librería decimal (ADR), `santhosh-tekuri/jsonschema/v6` para validar el evento antes de escribirlo y `pgregory.net/rapid` para tests basados en propiedades. El módulo de contratos para los DTOs de eventos. Ninguna otra dependencia sin justificarla.

**Principios que se tienen que notar en el código:**
- **SRP:** un caso de uso por struct; los handlers solo traducen HTTP ↔ caso de uso.
- **Dominio rico:** `Invoice` es un agregado que protege sus invariantes. Las transiciones (`Issue`, más adelante `Cancel`) son métodos que devuelven error de dominio si no se permiten; ningún handler cambia `status` a mano. El cálculo es puro y determinista.
- **OCP/LSP:** tipos de documento nuevos (notas de crédito y débito) y permisos nuevos se agregan sin tocar los handlers existentes.
- **ISP:** interfaces pequeñas, **definidas del lado del consumidor** (en `app`).
- **DIP:** casos de uso sobre puertos; adapters que los implementan; wiring en `internal/wiring` con inyección por constructor. Sin estado global ni `init()` con efectos.
- `TxManager.WithinTenantTx(ctx, fn)` que fija el tenant y escribe entidad, historial, audit y outbox de forma atómica.
- `context.Context` como primer parámetro; timeouts en servidor y BD; graceful shutdown.
- Errores de dominio tipados con `errors.Is`/`As`, mapeados a HTTP en un único lugar. No filtres errores internos al cliente.
- Configuración por variables de entorno validadas al arrancar, leyendo `.env` como Platform. Cero secretos en el código.
- Fechas en UTC (`timestamptz`); `due_date` es `date` y se interpreta en la zona horaria de la organización (documenta cómo).
- Nombres claros, funciones cortas, sin comentarios obvios; comenta el **porqué**, no el qué.

## Paso 6 · Tests (escríbelos desde los requisitos, no desde la implementación)

- **Unitarios del dominio:** máquina de estados (solo `draft` se edita y se emite), snapshots, numeración, matriz de permisos y validaciones de cliente y producto.
- **Basados en propiedades del cálculo** (definición de terminado del planning) con `rapid`: para cualquier conjunto de líneas, cantidades, precios, descuentos y tasas válidos:
  1. la suma de las líneas es igual a los totales de la factura;
  2. subtotal − descuento + impuesto = total, en cada línea y en la factura;
  3. ningún redondeo pierde ni inventa céntimos (compara contra un cálculo de referencia con precisión mayor);
  4. el resultado no depende del orden de las líneas;
  5. ningún monto es negativo.
- **Casos de uso** con fakes de los puertos: emisión feliz, doble emisión con la misma `Idempotency-Key`, factura ya emitida (409), factura de otra organización (404), editar una factura emitida (409), cliente inactivo.
- **Contrato del evento:** el `InvoiceIssued` que se escribe en el outbox valida contra `invoice-issued.v1.json` para facturas con varias líneas, descuentos, varios impuestos y exoneración. Un test falla si aparece un `float` en cualquier struct de dominio o DTO (por reflexión).
- **Snapshots (criterio 5):** emitir, cambiar el cliente y el producto, y comprobar que la factura y su evento no cambiaron.
- **Integración** contra la base de dev con el login `billing_api` (no superusuario), como la suite de Platform: las migraciones reales, la emisión completa y el outbox escrito en la misma transacción (si falla el outbox, no queda factura emitida).
- **Aislamiento** (`tests/isolation`, `make test-isolation`), los mismos 6 criterios de Platform adaptados:
  1. con el token de A, todos los endpoints sobre recursos de B (clientes, productos, facturas, secuencias) devuelven 404/403, en un test tabla, y B queda intacto;
  2. con la sesión en A, un `SELECT` directo con `billing_app` no devuelve filas de B en ninguna tabla con `organization_id`;
  3. una línea o una factura que referencia un cliente, producto, sucursal o factura de otra organización se rechaza (FK compuesta);
  4. `billing_app` no escribe en `core`, `fiscal`, `receivables` ni `subscriptions`, ni hace `UPDATE`/`DELETE` en `audit`;
  5. un token sin `org_id`, o con la membresía suspendida, no accede;
  6. un `organization_id` o `branch_id` de B en el body, query o header no cambia el tenant ni se acepta.
- **Handlers HTTP** con `httptest`: camino feliz, validación y cross-tenant por endpoint, y reintentos con la misma `Idempotency-Key`.
- **E2E** (`scripts/dev/e2e.sh`, como en Platform): login en Supabase Auth, crear cliente y producto, crear borrador, emitir, verificar el outbox y comprobar el aislamiento con un segundo usuario.
- Si un test de aislamiento falla por falta de una política en la base, **no debilites el test**: repórtalo y propone la migración.

## Paso 7 · Operación y entrega

- `GET /healthz` (vivo) y `GET /readyz` (BD y JWKS accesibles).
- Dockerfile multi-stage con imagen distroless y usuario no root, con los binarios `api` y `migrate`.
- `Makefile` con: `run`, `build`, `test`, `test-isolation`, `lint`, `migrate-up`, `migrate-status`, `sqlc`.
- `api/openapi.yaml` sincronizado con los handlers y alineado con el OpenAPI de Billing del repo de contratos (usando sus componentes comunes).
- `CLAUDE.md` con el bloque común de reglas de plataforma adaptado a Billing, más: "el cálculo vive solo en `internal/domain/invoice`", "una factura emitida no se edita", "prohibido `float`", y el comando de cierre `go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation`.
- `README.md` con el mismo nivel que el de Platform: qué hace la API, endpoints, roles, cómo levantarla en local, variables de entorno, comandos, pruebas y problemas comunes.
- `docs/ESTADO.md` como handoff al cerrar cada sesión.
- ADRs cortos en `docs/decisiones/`: reutilización de `pkg/tenancy`, librería decimal y redondeo, numeración y huecos, forma de editar líneas, mapeo al evento y validación contra el schema.

## Forma de trabajo

1. **Plan primero:** informe de brechas de la BD, SQL de los logins, plan de migraciones, diseño del agregado `Invoice` y del cálculo, y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por incrementos verticales**, cada uno compilando y con sus tests en verde:
   1. esqueleto, config, health, observabilidad, logins y baseline;
   2. JWT, TenantContext y primer endpoint protegido;
   3. clientes;
   4. productos e impuestos del producto;
   5. dominio de dinero y cálculo, con los tests basados en propiedades;
   6. facturas en borrador y sus líneas;
   7. numeración;
   8. emisión con snapshots, historial, audit y `InvoiceIssued` en el outbox;
   9. endurecimiento: suite de aislamiento, E2E, autorrevisión, README y TODOs.
3. Al terminar cada incremento: `go build ./... && go vet ./... && golangci-lint run && go test ./...`, actualiza `docs/ESTADO.md`, resume lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente (`branch_id`, `customer_id`, `product_id`), `float` en montos, `timestamp` sin zona, facturas emitidas que se puedan modificar, eventos que no validen contra el schema, secretos y errores internos expuestos.

## Prohibido

- Conectarte a producción, usar la `service_role` key o usar certificados o datos reales.
- Tomar el tenant de algo que no sea el token verificado, o aceptar un `branch_id`, `customer_id` o `product_id` sin validarlo contra la organización activa.
- Usar `float32`/`float64` o `double precision` para montos, cantidades, tasas o tipo de cambio.
- Modificar una factura emitida fuera de un flujo explícito (notas o anulación, que son de F5).
- Llamar de forma síncrona a E-Invoice, Receivables u otro servicio durante la emisión.
- Desactivar RLS o debilitar un test para que pase.
- Migrar schemas que no son de esta API o hacer cambios destructivos sin expand/contract.
- Inventar campos fiscales, códigos, catálogos, reglas de redondeo, eventos o estados que no estén en el contrato: deja un TODO y lístalo al final.

## Definición de terminado

- [ ] Informe de brechas aprobado, logins creados y migraciones aplicadas en dev, con la baseline incluida.
- [ ] Todos los endpoints de F2 y F3 implementados, documentados en OpenAPI y con tests.
- [ ] Emite facturas con E-Invoice apagada: la emisión solo escribe en la base y en el outbox (criterio 2).
- [ ] Cambiar un cliente o un producto no altera ninguna factura emitida ni su evento (criterio 5).
- [ ] Tests basados en propiedades del cálculo en verde: las líneas cuadran con los totales y ningún redondeo pierde céntimos.
- [ ] `InvoiceIssued` v1 escrito en el outbox en la misma transacción y validado contra el schema del repo de contratos.
- [ ] Suite de aislamiento en verde y ejecutable con `make test-isolation`; E2E en verde.
- [ ] Audit event en cada operación sensible; idempotencia en cada comando; correlationId de punta a punta.
- [ ] `golangci-lint` limpio, cobertura alta en dominio y casos de uso, imagen Docker construida y health checks respondiendo.
- [ ] Lista final de TODOs (fiscales, de contrato y de decisiones) para revisar en equipo.
