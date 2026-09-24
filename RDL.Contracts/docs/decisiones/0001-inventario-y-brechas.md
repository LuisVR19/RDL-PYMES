# 0001 · Inventario de lo ya decidido y brechas

- **Fecha:** 2026-09-24 · **Estado:** Aprobado el 2026-09-24 con todas las propuestas. D1: `bitbucket.org/rdl/contracts` + Bitbucket Pipelines (ruta provisional). Sin especificación de Hacienda
- **Fuentes:** base de dev `dzlsnsstuqpxvwegeqcy` (solo lectura), `RDL.Platform.API` (código, `api/openapi.yaml`, ADR 0001–0007, `docs/ESTADO.md`), `docs/contexto/arquitectura-v1.md` y `planning-v1.md`.
- **No disponible:** la especificación oficial de Hacienda. Sin ella, **ningún código ni regla fiscal se formaliza**: queda como `TODO(fiscal)`.

## 1. Lo que ya está decidido y este repo solo formaliza

### 1.1 Tipos base (dominios de `shared`)

| Dominio | Tipo | Regla | Uso en contratos |
|---|---|---|---|
| `shared.money_amount` | `numeric(18,5)` | `>= 0` | `Money`: string decimal, hasta 13 enteros y 5 decimales, **sin signo** |
| `shared.exchange_rate` | `numeric(18,5)` | `> 0` | `ExchangeRate`: string decimal positivo |
| `shared.quantity` | `numeric(16,3)` | `> 0` | `Quantity`: string decimal, 3 decimales |
| `shared.percentage` | `numeric(7,4)` | `0..100` | `Percentage`: string decimal, 4 decimales |
| `shared.currency_code` | `char(3)` | `^[A-Z]{3}$` | `CurrencyCode` (ISO 4217) |
| `shared.cabys_code` | `char(13)` | `^[0-9]{13}$` | `CabysCode` (solo formato; el catálogo es de E-Invoice) |
| `shared.sha256_hex` | `char(64)` | `^[0-9a-f]{64}$` | `Sha256Hex` |

Fuente: `pg_type` del schema `shared`.

### 1.2 Sobre de eventos ↔ `integration.outbox_messages` / `inbox_messages`

| Campo del sobre (6.3) | Columna del outbox | Columna del inbox |
|---|---|---|
| `eventId` (uuid) | `id` | `event_id` |
| `eventType` | `event_type` | `event_type` |
| `version` (entero > 0) | `event_version` (`> 0`) | — |
| `occurredAt` | `occurred_at` (timestamptz) | — |
| `correlationId` | `correlation_id` | — |
| `organizationId` | `organization_id` | `organization_id` |
| — | `source_service` (`platform`, `billing`, `fiscal`, `receivables`) | `consumer_service` |
| — | `aggregate_type`, `aggregate_id` | — |
| (cuerpo) | `payload` (jsonb) | — |

El sobre de 6.3 cabe en el outbox sin transformaciones. `aggregate_type`/`aggregate_id` salen del cuerpo de cada evento (por ejemplo, `invoice` + `invoiceId`). Ver decisión D3 sobre `sourceService`.

### 1.3 Nombres de servicio

Los `CHECK` de `audit`, `integration.*` e `idempotency_keys` fijan cuatro servicios: `platform`, `billing`, `fiscal`, `receivables`. Se usan tal cual en `source_service`, en `service` de la auditoría y en el `type` de Problem Details. Ver D4 (E-Invoice se llama `fiscal`).

### 1.4 Estados que la base ya impone

| Entidad | Estados (`CHECK`) | Otras reglas |
|---|---|---|
| `billing.invoices` | `draft`, `issued`, `cancelled` | `document_type` ∈ `invoice`, `credit_note`, `debit_note` (notas en la misma tabla); `cancelled` exige `cancelled_at` + `cancellation_reason`; `issued` exige número, `issued_at`, emisor y snapshot del cliente; una nota exige `referenced_invoice_id` + `reference_reason`; flag `requires_correction` aparte del estado |
| `fiscal.electronic_documents` | `processing`, `signed`, `sent`, `accepted`, `rejected`, `contingency`, `error` | `accepted` ⇔ `accepted_at`; `rejected` ⇔ `rejected_at`; `source_type` ∈ `invoice`, `credit_note`, `debit_note`; clave numérica de 50 dígitos; consecutivo de 20 dígitos |
| `receivables.receivables` | `open`, `partially_paid`, `paid`, `cancelled` | `due_on >= issued_on`; `original_amount > 0` |
| `receivables.payments` | `posted`, `voided` | `voided` exige `voided_at` + `void_reason` |
| `receivables.payment_applications` | (sin estado) | reverso con `reversed_at` + `reversal_reason` |
| `receivables.receivable_adjustments` | tipo ∈ `credit_note`, `debit_note`, `cancellation`, `write_off` | salvo `write_off`, exige documento origen |
| `receivables.payment_promises` | `pending`, `kept`, `broken`, `cancelled` | — |
| `core.organizations` | `active`, `suspended`, `closed` | — |
| `core.organization_users` | `active`, `suspended` | — |
| `core.invitations` | `pending`, `accepted`, `revoked`, `expired` | — |
| `core.users` | `active`, `disabled` | — |
| `subscriptions.organization_subscriptions` | `trialing`, `active`, `past_due`, `cancelled` | — |

### 1.5 Roles (`core.roles`)

`owner` (Propietario), `admin` (Administrador), `biller` (Facturador), `collector` (Cobrador), `accountant` (Contador), `read_only` (Solo lectura). V1: un rol por membresía. El JWT trae `org_id` y `org_roles`.

### 1.6 Convenciones que Platform ya implementa

| Tema | Implementación actual | Fuente |
|---|---|---|
| Errores | Problem Details, `application/problem+json`, `type = urn:rdl:platform:problem:<código>` | `internal/adapters/http/problem` |
| Tipos en uso | `email-required`, `idempotency-key-required`, `idempotency-key-reused`, `invitation-expired`, `invitation-not-pending`, `last-owner`, `malformed-request`, `membership-inactive`, `method-not-allowed`, `no-active-organization`, `not-found`, `owner-required`, `user-disabled` | ídem |
| Idempotencia | header `Idempotency-Key` en POST; misma clave + mismo cuerpo → misma respuesta; otro cuerpo → 422; vigencia 24 h; tabla `integration.idempotency_keys` (`service`, `organization_id`, `request_hash`) | `internal/app/organizations.go` |
| Correlación | `X-Correlation-Id`; solo se acepta UUID, otro valor se reemplaza | `pkg/correlation` |
| Tenancy | organización solo del `org_id` del JWT + membresía revalidada; rutas `/v1/organizations/current/...`; otra organización → 404, rol insuficiente → 403 | ADR 0003 de Platform |
| Paginación | `limit` 1–100 (defecto 20), cursor opaco, respuesta `items` + `nextCursor` | `internal/app/members.go` |
| IDs | UUID v4 (`gen_random_uuid()`, `uuid.New()`); v5 para el id de organización idempotente | Platform + baseline |
| Fechas | `timestamptz`, JSON RFC 3339 en UTC | Platform |
| OpenAPI | 3.1.0, versión 0.7.0 | `api/openapi.yaml` |

### 1.7 Catálogo de eventos (sección 6.2)

`InvoiceIssued`, `InvoiceCancelled`, `CreditNoteIssued`, `DebitNoteIssued` (Billing → fiscal y receivables); `ElectronicDocumentAccepted`, `ElectronicDocumentRejected` (fiscal → BFF, Billing, notificaciones); `PaymentReceived`, `ReceivableSettled` (receivables → BFF, reportes). Todos v1.

## 2. Contradicciones entre fuentes

| # | Contradicción | Fuentes | Propuesta |
|---|---|---|---|
| C1 | El contrato mínimo de `InvoiceIssued` (6.3) usa montos como `number` (`"subtotal": 10000`) | arquitectura 6.3 vs planning P0 y 6.3 ("nunca float") | Montos como **string decimal**. El ejemplo de 6.3 queda como ilustrativo |
| C2 | Rol "facturador": `billing_clerk` en el prompt P3, `biller` en la base | prompt P3 vs `core.roles` | Manda la base: `biller` |
| C3 | Estados fiscales: la arquitectura menciona Processing, Accepted, Rejected y contingencia; la base agrega `signed`, `sent` y `error` | arquitectura 3.2/6.1 vs `CHECK` de `fiscal.electronic_documents` | Formalizar los 7 de la base; el dueño de fiscal confirma las transiciones |
| C4 | `customerSnapshot` de 6.3 trae `name`, `identification`, `email`; Billing guarda además tipo de identificación, teléfono y dirección | arquitectura 6.3 vs `billing.invoices` | Snapshot con todos los campos que Billing ya guarda; identificación como `{typeCode, number}` |
| C5 | El documento habla de "E-Invoice API"; la base y la auditoría usan el servicio `fiscal` | arquitectura vs `CHECK` de `integration`/`audit` | Ver D4 |
| C6 | Dos numeraciones: `billing.document_sequences` (número visible) y `fiscal.document_sequences` (consecutivo de 20 dígitos y clave de 50) | base | No es error: el glosario las separa (número comercial vs consecutivo fiscal) |
| C7 | La arquitectura 4.1 lista tablas (`credit_notes`, `debit_notes`, `consecutives`, `cabys`...) que no coinciden con la base | arquitectura 4.1 vs base | Manda la base. Las notas viven en `billing.invoices` |

## 3. Lo que falta decidir

| # | Decisión | Propuesta | Aprueba |
|---|---|---|---|
| D1 | Ruta del módulo Go y hosting del CI | `<<bitbucket.org/rdl/contracts>>` con Bitbucket Pipelines (hay MCP de Bitbucket configurado) | Equipo |
| D2 | **Regla de redondeo** y dónde vive el cálculo de impuestos (**resuelta en ADR 0007**: 5 decimales, mitad hacia arriba, según el borrador de los Anexos v4.4) | Billing calcula y fiscal valida (defecto del planning). Escala 5 como la base. Modo y paso de redondeo: **pendiente** (depende de la especificación de Hacienda) | Billing + fiscal |
| D3 | ¿El sobre lleva el servicio productor? | Agregar `sourceService` obligatorio (mapea a `outbox.source_service`) para que el consumidor valide el origen. Es un séptimo campo sobre los 6 del planning | Equipo |
| D4 | Nombre del servicio E-Invoice en contratos | `fiscal` en campos máquina (`sourceService`, URN de errores); "E-Invoice API" solo en la documentación | Equipo |
| D5 | Convención del `type` de Problem Details | `urn:rdl:<servicio>:problem:<código-kebab>`, como Platform, más un registro por servicio en `problems/<servicio>.yaml` | Equipo |
| D6 | UUID v4 o v7 | **v4**, porque ya es lo que usan la base y Platform. v7 no aporta lo suficiente para cambiar | Equipo |
| D7 | Rutas de Billing, fiscal y Receivables | `/v1/<recurso>` con la organización del token (`/v1/invoices`), sin `current` en la ruta; Platform mantiene `/v1/organizations/current/...` porque su recurso es la organización | Equipo |
| D8 | Eventos de Platform (`OrganizationCreated`, `MemberAdded`, ...) | **Fuera de v1**, porque no están en 6.2. Se revisan cuando algún consumidor los necesite | Equipo |
| D9 | Catálogo de `identificationTypeCode` (TODO de Platform) | `TODO(fiscal)`: formato string; valores y validación por tipo según la especificación de Hacienda | fiscal |
| D10 | Fechas sin hora (`due_date`, `issued_on`, `due_on`, `received_on`) | `YYYY-MM-DD` interpretado en la zona horaria de la organización | Equipo |
| D11 | Alcance de los consecutivos y relación sucursal ↔ establecimiento/terminal | **Sin valor por defecto** (planning): el glosario lo deja abierto | fiscal + contabilidad |
| D12 | Montos negativos | Nunca: `money_amount >= 0`. Descuentos y ajustes se expresan con su tipo, no con el signo | Equipo |
| D13 | ¿`InvoiceCancelled` y las notas usan la misma forma que `InvoiceIssued`? | Notas: misma estructura de líneas más `referencedInvoiceId` + `referenceReason` (como en la base). Anulación: sobre + ids + motivo, sin líneas | Billing |

## 4. Hallazgos para otros repos (no se corrigen aquí)

- **Platform:** `identificationTypeCode` sin catálogo (D9); OpenAPI local con `TODO(contracts)`; los 4 TODOs de contrato de `docs/ESTADO.md` se responden con D5, D8 y D9.
- **Base de datos:** `billing.invoices.branch_id` y `billing.document_sequences.branch_id` no tienen FK compuesta hacia `core.branches`. Es para `database-platform` o Billing.

## 5. Plan de incrementos (del prompt P0)

1. Esqueleto del repo, `go.mod`, CLI vacío, `Makefile`, lint y CI.
2. Convenciones, glosario y ownership (docs + YAML) y `contractsctl validate` para el ownership.
3. Tipos comunes (§1.1), sobre de eventos (§1.2) y `pkg/events/money`.
4. `InvoiceIssued` v1 completo.
5. Los otros 7 eventos y `asyncapi.yaml`.
6. Máquinas de estado (§1.4).
7. OpenAPI: componentes comunes, Platform importado y esqueletos.
8. `contractsctl lint` y `breaking`.
9. Endurecimiento, `v0.1.0` y lista de TODOs.
