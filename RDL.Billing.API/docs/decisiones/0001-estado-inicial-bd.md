# 0001 · Estado inicial de la base de datos (informe de brechas)

- **Fecha:** 2026-09-24
- **Estado:** Aprobado (2026-09-24; decisiones §4 aprobadas el 2026-09-25).
- **Fuente:** inspección en solo lectura del proyecto Supabase `dzlsnsstuqpxvwegeqcy` vía MCP (`pg_catalog`,
  `pg_policies`, `has_*_privilege`, `supabase_migrations`).
- **Referencias:** prompt P4, `docs/contexto/arquitectura-v1.md` (4.3, 4.4, 5, 6, 7.4), repo de contratos v0.1.0
  (`state-machines/invoice.yaml`, `schemas/events/invoice-issued.v1.json`, `docs/convenciones.md`,
  `openapi/billing.yaml`, `problems/billing.yaml`), Platform ADR 0001–0004.

## 1. Resumen

El schema `billing` está **muy completo** y ya protege en la base casi todo lo que el prompt exige: FK compuestas
internas, RLS por organización en las 9 tablas, dominios `shared.*` en todos los montos y **triggers que impiden
modificar o borrar un documento emitido y su detalle**. El trabajo de base de datos propio de Billing se reduce a:

1. una **baseline** goose de `billing` (idempotente, sin cambios reales) con el historial en `billing.goose_db_version`;
2. **una migración expand**: FK compuestas de `invoices.branch_id` y `document_sequences.branch_id` hacia `core.branches`
   (depende del punto 3).

Y hay **dos brechas que no se resuelven en este repo** y bloquean incrementos concretos:

3. **`billing_app` no puede leer la membresía** (`core.find_user_by_subject`, `core.users`,
   `core.organization_users`, `core.organization_user_roles`, `core.organizations`). Sin eso no hay revalidación de membresía, rol efectivo ni zona horaria de la organización
   → bloquea el incremento 2 (tenancy). Propuesta a `database-platform`/Platform en `0002-propuesta-database-platform.md`.
4. **Las tarifas de impuesto no están en `billing`**: `product_taxes` solo guarda códigos y el catálogo
   `fiscal.tax_rates` (legible por `billing_app`) **está vacío**, igual que todos los catálogos fiscales. Junto con el
   redondeo pendiente (D2) bloquea el cálculo y la emisión (incrementos 5 y 8). Ver §4.2.

## 2. Inventario

### 2.1 Historial de migraciones

`supabase_migrations.schema_migrations` (aplicadas por `database-platform`): `0001_bootstrap` … `0009_platform_login_roles`.
`billing` se creó en `0004_billing` y sus grants en `0007_grants`. **No existe `billing.goose_db_version`** (solo
`core.goose_db_version`, de Platform). No hay datos: 0 filas en todas las tablas de `billing` y en los catálogos de `fiscal`.

### 2.2 Roles

| Rol | Login | Miembro de | Notas |
|---|---|---|---|
| `billing_app` | no | — | Sin BYPASSRLS. Miembros: solo `postgres` |
| `billing_migrator` | no | — | Dueño del schema `billing` y de sus 9 tablas y 2 funciones |
| `billing_api` / `billing_migrate` | **no existen** | | Se proponen en `scripts/dev/0010_billing_login_roles.sql` |

`shared.current_service()` devuelve `'billing'` para cualquier miembro de `billing_app`, así que las políticas de
`audit.audit_events`, `integration.outbox_messages` e `integration.idempotency_keys` funcionarán con `billing_api`.

### 2.3 Privilegios efectivos de `billing_app`

| Schema | Privilegios | ¿Cumple? |
|---|---|---|
| `billing` | USAGE; S/I/U/D en todo, salvo `invoice_status_history` (**solo S/I**, append-only) | ✅ |
| `core` | USAGE; **solo SELECT en `branches`**. Nada en `organizations`, `organization_users`, `organization_user_roles`, `roles`, `users`; sin EXECUTE en `core.find_user_by_subject` | ❌ falta lectura de membresía (§3.1) |
| `audit` | S/I en `audit_events` (trigger append-only bloquea U/D/TRUNCATE) | ✅ |
| `integration` | S/I/U en `outbox_messages`, `inbox_messages`, `dead_letters`; S/I/U/D en `idempotency_keys` | ✅ |
| `fiscal` | USAGE; SELECT solo en catálogos (`tax_rates`, `tax_types`, `units_of_measure`, `identification_types`, `sale_conditions`, `payment_methods`, `exoneration_document_types`, `cabys_*`, `provinces`/`cantons`/`districts`, `document_types`) | ✅ (lectura de catálogos) |
| `receivables`, `subscriptions` | sin USAGE | ✅ |
| cualquiera | sin CREATE | ✅ |

`billing_migrator` tiene USAGE+CREATE en `billing` y **ningún privilegio en `core`** (ni REFERENCES en `core.branches`).

### 2.4 Tablas de `billing`

Todas: PK `uuid` (`gen_random_uuid()`), `organization_id uuid NOT NULL`, `UNIQUE (organization_id, id)`,
RLS habilitado con política `<tabla>_tenant` (`organization_id = (select shared.current_organization_id())`, USING y
WITH CHECK), `timestamptz` en todos los instantes y dominios `shared.*` en todos los montos. **Ningún `float`.**

| Tabla | Unicidades / CHECK relevantes | FK compuestas | Notas |
|---|---|---|---|
| `customers` | UK `(org, identification_type_code, identification_number)` | — | `legal_name`, `trade_name`, `email`, `phone`, `province/canton/district_code`, `address_details`, `is_active`, `created_by_user_id` |
| `products` | UK `(org, code)` | — | `cabys_code` (`shared.cabys_code`), `unit_of_measure_code`, `unit_price`, `currency_code`, `is_service`, `is_active` |
| `product_taxes` | UK `(org, product_id, tax_type_code)` | → products CASCADE | **Solo `tax_type_code` y `tax_rate_code`, sin tasa** (§4.2) |
| `invoices` | UK `(org, document_type, number)`; CHECKs de estado, tipo, anulación, referencia de notas, crédito ≥ 0 y **`invoices_issued_ck`** (emitido ⇒ número, `issued_at`, `issued_by_user_id` y snapshot mínimo del cliente) | → customers; → invoices (referenciada) | `document_type` ∈ invoice/credit_note/debit_note; `status` ∈ draft/issued/cancelled; `requires_correction`; snapshots `customer_*`; totales `subtotal/discount/tax/exoneration/total_amount`; **`branch_id` sin FK** |
| `invoice_lines` | UK `(org, invoice_id, line_number)`; `line_number > 0`; descuento ≠ 0 ⇒ `discount_reason` | → invoices CASCADE; → products | Snapshot del producto: `product_code`, `cabys_code`, `description`, `unit_of_measure_code`, `is_service`, `unit_price` |
| `invoice_line_taxes` | UK `(org, invoice_line_id, tax_type_code)`; exoneración todo-o-nada | → invoice_lines CASCADE | `rate`, `taxable_base`, `tax_amount`, `exoneration_*` |
| `invoice_payment_methods` | UK `(org, invoice_id, payment_method_code)` | → invoices CASCADE | `amount` opcional |
| `invoice_status_history` | — | → invoices (**sin CASCADE**) | `from_status`, `to_status`, `reason`, `changed_by_user_id`, `changed_at` |
| `document_sequences` | UK `NULLS NOT DISTINCT (org, document_type, branch_id)`; `next_number > 0` | — | `prefix`, `next_number bigint`; **`branch_id` sin FK** |

Índices: los de las UK más `customers (org, lower(legal_name))`, `products (org, cabys_code)`,
`invoices (org, status, issued_at desc)`, `(org, customer_id)`, `(org, referenced_invoice_id)`, parcial
`WHERE requires_correction`, `invoice_lines (org, product_id)`, `invoice_status_history (org, invoice_id, changed_at)`.

### 2.5 Dominios de `shared`

| Dominio | Tipo | CHECK | Go |
|---|---|---|---|
| `money_amount` | `numeric(18,5)` | `>= 0` | 13 enteros, 5 decimales |
| `quantity` | `numeric(16,3)` | `> 0` | 13 enteros, 3 decimales |
| `percentage` | `numeric(7,4)` | `0..100` | |
| `exchange_rate` | `numeric(18,5)` | `> 0` | |
| `currency_code` | `char(3)` | `^[A-Z]{3}$` | |
| `cabys_code` | `char(13)` | `^[0-9]{13}$` | |

Coinciden exactamente con `schemas/common/*.json` del contrato.

### 2.6 Triggers y funciones

| Trigger | Tabla | Efecto |
|---|---|---|
| `invoices_guard` (BEFORE UPDATE/DELETE) | `invoices` | Solo se borra un `draft`. Desde `draft` solo a `draft`/`issued`. Emitido: solo `issued→issued`, `issued→cancelled`, `cancelled→cancelled`, y **solo cambian** `status`, `requires_correction`, `fiscal_rejection_reason`, `cancellation_*`, `cancelled_*`, `updated_at`. La anulación no se reescribe |
| `*_guard` → `guard_draft_children()` | `invoice_lines`, `invoice_line_taxes`, `invoice_payment_methods` | Rechaza INSERT/UPDATE/DELETE si la factura no está en `draft` |
| `*_set_updated_at` | `customers`, `products`, `invoices`, `document_sequences` | `updated_at = now()` |
| `audit_events_append_only` / `_no_truncate` | `audit.audit_events` | Append-only |

Funciones `billing.*` sin SECURITY DEFINER y con `search_path = ''`. ✅

### 2.7 RLS de lo que Billing lee fuera de `billing`

- `core.branches`: `branches_tenant` por organización activa. ✅ Suficiente para validar `branch_id`.
- `core.organization_users`: `_tenant` y `_own` (`user_id = current_user_id()`); `core.organization_user_roles`:
  `_tenant` y `_own`; `core.organizations`: `organizations_read` (activa o membresía activa). Las políticas ya sirven;
  **falta el GRANT** (§3.1).
- `integration.outbox_messages`: por `source_service = current_service()` (no por organización; correcto para el worker).
- `integration.idempotency_keys`: por servicio **y** organización. `audit.audit_events`: INSERT por servicio, SELECT por organización.
- Ninguna tabla tiene `FORCE ROW LEVEL SECURITY`. No afecta: `billing_app` no es dueño (igual que Platform, ADR 0003).

### 2.8 Contra el contrato

| Contrato | Base | ¿Cabe? |
|---|---|---|
| Estados `draft`, `issued`, `cancelled` y bandera `requires_correction` | CHECK `invoices_status_ck` + columna | ✅ idénticos |
| Transición `draft→issued` (invoice) y `issued→cancelled` | `invoices_guard` | ✅ (y además la base impide otras) |
| `InvoiceIssued` v1: `invoiceId`, `branchId?`, `invoiceNumber`, `issuedAt`, `issuedByUserId`, `saleConditionCode`, `currency`, `exchangeRate`, `creditTermDays?`, `notes?`, totales | columnas de `invoices` | ✅ |
| `issueDate` (fecha de negocio en la zona de la organización) | `core.organizations.timezone` | ❌ **no legible** por `billing_app` (§3.1) |
| `dueDate` obligatorio | `invoices.due_date` (nullable) | ⚠️ se fija al emitir (§4.4) |
| `customerSnapshot` (`customerId`, identificación, `legalName`, `email?`, `phone?`, `address?`) | `invoices.customer_*` | ✅ |
| `lines[]` con snapshot del producto, `taxes[]` con `rate`, `taxableBase`, `amount`, `exoneration?` | `invoice_lines` + `invoice_line_taxes` | ✅ (la exoneración de línea se deriva de sus impuestos) |
| `paymentMethods[]` | `invoice_payment_methods` | ✅ |
| Longitudes máximas (`description` 200, `productCode` 50, `legalName` 200, `phone` 30, `email` 254, `address` 500, `notes` 2000, `discountReason` 80, `invoiceNumber` 50, `prefix` 10) | `text` sin límite | ⚠️ se validan en la app (§3.4) |

## 3. Lo que existe pero no cumple

### 3.1 ❌ `billing_app` sin lectura de membresía ni de la organización (bloqueante, fuera de este repo)

Billing debe revalidar la membresía activa y tomar el rol de la base (prompt Paso 3, convenciones §5 y §12), y
convertir `issued_at` a la fecha de negocio con `core.organizations.timezone`. Hoy no puede. `core` es de Platform
(`platform_migrator`), así que **este repo no genera el GRANT**: va en la propuesta 0002. Se pide solo SELECT:

Platform resuelve la membresía así (`queries/membership.sql`): `sub` → `core.find_user_by_subject()` (SECURITY
DEFINER, hoy solo ejecutable por `platform_app`) → `core.users` activo → `organization_users` activa → `organizations`
activa → `organization_user_roles`. Billing reutiliza la misma consulta, por lo que necesita:

```sql
grant execute on function core.find_user_by_subject(text) to billing_app;
grant select (id, status) on core.users to billing_app;
grant select on core.organization_users, core.organization_user_roles to billing_app;
grant select (id, status, timezone, default_currency_code) on core.organizations to billing_app;
```

Las políticas existentes (`users_self`, `*_own`, `organizations_read`) ya limitan lo visible al propio usuario, su
membresía y la organización activa. No se pide escritura.

### 3.2 ❌ `branch_id` sin FK compuesta a `core.branches`

`invoices.branch_id` y `document_sequences.branch_id` no tienen FK: hoy la base aceptaría una sucursal de otra
organización. La app lo valida siempre (prompt Paso 3), pero la regla 4.3 pide FK compuesta. Requiere que el dueño de
`core` otorgue `REFERENCES` a `billing_migrator` (propuesta 0002). Después, migración expand de este repo:

```sql
-- 00003_branch_fk.sql (tras el GRANT USAGE + REFERENCES)
alter table billing.invoices
  add constraint invoices_branch_fk foreign key (organization_id, branch_id)
  references core.branches (organization_id, id) not valid;
alter table billing.invoices validate constraint invoices_branch_fk;

alter table billing.document_sequences
  add constraint document_sequences_branch_fk foreign key (organization_id, branch_id)
  references core.branches (organization_id, id) not valid;
alter table billing.document_sequences validate constraint document_sequences_branch_fk;
```

Con `branch_id NULL` la FK compuesta no se evalúa (MATCH SIMPLE), que es lo que se quiere.

### 3.3 ⚠️ `invoice_status_history` sin CASCADE y descartar borradores ✅ aprobada la propuesta

`DELETE /v1/invoices/{id}` borra la fila (`state-machines/invoice.yaml`: "se borra la fila"). Si al crear el borrador
se escribe historial (`null → draft`), la FK sin CASCADE impide el borrado. **Propuesta:** el historial registra
**transiciones** (primera fila: `draft → issued`); la creación y el descarte quedan en `audit.audit_events`. Sin
migración. Alternativa: `ON DELETE CASCADE` en la FK (expand/contract); la descarto porque borra historial.

### 3.4 ⚠️ Columnas `text` sin longitud máxima

El contrato fija máximos (tabla 2.8). Se validan en la capa HTTP y en el dominio. No propongo CHECKs en esta fase
(serían cambios de `billing` sin urgencia); quedan como TODO para cuando haya datos reales.

### 3.5 ℹ️ Ids de usuario sin FK a `core.users`

`created_by_user_id`, `issued_by_user_id`, `changed_by_user_id`, `cancelled_by_user_id` no tienen FK. Salen siempre del
`TenantContext` (token verificado + membresía), nunca del cliente. Lo dejo así (FK entre servicios hacia una tabla
global); si se quiere, se pide `REFERENCES` junto con 3.2.

## 4. Decisiones

### 4.1 D2 — redondeo ✅ resuelto

Decidido el 2026-09-25 en el ADR 0007 del repo de contratos (Anexo de Hacienda 4.4): 5 decimales, mitad hacia
arriba, cada campo calculado de la línea. Implementado en Billing según el ADR 0005.

**Consecuencia para la base:** `invoice_line_taxes.exoneration_percentage` guarda la **tarifa exonerada** en puntos
(6.5 de un IVA de 13), no un porcentaje del impuesto. `shared.percentage` ya admite el formato 4,2 de Hacienda: no
hace falta migrar. Renombrar la columna es opcional (expand → contract).

### 4.2 ¿De dónde sale la tasa de un impuesto? ✅ aprobada la opción A

`product_taxes` guarda `tax_type_code` y `tax_rate_code`, no el porcentaje. `invoice_line_taxes.rate` sí es obligatorio.
La tasa solo existe en `fiscal.tax_rates (code, name, rate, is_active)`, que Billing puede leer **pero está vacío**.
Opciones:

- **A (recomendada):** al escribir las líneas del borrador, resolver `rate` por `tax_rate_code` en `fiscal.tax_rates`
  y copiarlo a `invoice_line_taxes.rate` (snapshot). Sin cambios de esquema. Requiere que fiscal cargue el catálogo
  (`TODO(fiscal)`); mientras tanto, en dev se cargan filas de prueba **por fiscal**, no por Billing.
- **B:** la tasa viaja en el request de la línea. Rechazada: confía en un valor fiscal del cliente.
- **C:** columna `rate` en `product_taxes`. Rechazada: duplica el catálogo de Hacienda en Billing.

### 4.3 Snapshot de las líneas ✅ aprobado

La base guarda los datos del producto en `invoice_lines` desde que se escribe la línea (son necesarios para mostrar y
calcular el borrador). **Propuesta:** la línea copia el producto al escribirse (`PUT /lines`); al emitir, se **revalida**
que el producto sigue activo y se congela tal cual está en el borrador (lo que el usuario vio es lo que se emite). El
snapshot del **cliente** se copia al emitir (`invoices_issued_ck` lo exige), como dice el prompt. Con esto se cumple
el criterio 5: después de emitir, la base (`invoices_guard`, `guard_draft_children`) impide cualquier cambio.

### 4.4 Vencimiento al emitir ✅ aprobado

`dueDate` es obligatorio en el evento y nunca anterior a `issueDate`. **Propuesta:** `due_date = issueDate +
credit_term_days` si hay plazo; si no, `due_date = issueDate` (contado). Si el borrador trae `due_date` explícito, debe
ser ≥ `issueDate` (409/422 si no). Qué condición de venta exige plazo lo dice `fiscal.sale_conditions.requires_credit_term`
(vacío hoy): `TODO(fiscal)`.

### 4.5 Numeración ✅ aprobada

- Formato del número visible: `prefix || lpad(next_number, 8, '0')` (ej. `FAC-00000001`), ≤ 50 caracteres. No lo fija el
  contrato: se documenta en un ADR.
- Alcance: se usa la secuencia `(document_type, branch_id de la factura)` y, si no existe, la de la organización
  `(document_type, NULL)`; si tampoco existe, se crea en la misma transacción (`prefix = ''`, `next_number = 1`,
  `INSERT … ON CONFLICT DO NOTHING` y luego `SELECT … FOR UPDATE`).
- ⚠️ `invoices_number_uk` es `(org, document_type, number)`, **sin sucursal**: dos sucursales con el mismo prefijo
  chocarían. La app rechaza (409) configurar un prefijo ya usado por otra secuencia del mismo tipo.
- Huecos: el número se asigna bajo `FOR UPDATE` dentro de la transacción de emisión; si la transacción se revierte,
  el incremento también, así que **no hay huecos por rollback** (a costa de serializar emisiones por secuencia).

## 5. Lo que falta y se crea en este repo

| # | Migración | Contenido |
|---|---|---|
| 00001 | `baseline.sql` | Refleja las 9 tablas, índices, políticas, triggers y funciones de `billing` con `IF NOT EXISTS` / `CREATE OR REPLACE`. Se marca como aplicada en dev (`goose` crea `billing.goose_db_version`) |
| 00002 | `branch_fk.sql` | §3.2, solo después del GRANT REFERENCES |

Fuera de este repo (propuesta 0002 a `database-platform`/Platform): logins `billing_api`/`billing_migrate`, SELECT de
membresía y organización para `billing_app`, REFERENCES en `core.branches` para `billing_migrator`.

## 6. Diferencias con el prompt (manda la base / el contrato)

| Prompt | Realidad |
|---|---|
| "productos … con impuestos" (tasa) | `product_taxes` guarda códigos, no tasa (§4.2) |
| `PUT /lines` o CRUD por línea: "propón" | El contrato ya define `PUT /v1/invoices/{id}/lines` |
| Historial de estados en cada operación | Solo transiciones; creación y descarte van al audit (§3.3) |
| "un rol por membresía" (convenciones §12) | La base admite varios (`organization_user_roles`); como Platform, el `TenantContext` lleva el conjunto y basta con que un rol tenga el permiso |
| `billing_app` lee `core` (membresía, sucursales) | Solo `core.branches` (§3.1) |
| Contrato define `POST /v1/invoices/{id}/cancel` | Es F5: no se implementa, el dominio lo deja previsto |
| Catálogos fiscales para validar códigos | Existen pero están **vacíos**: solo se valida formato (`TODO(fiscal)`) |

## 7. TODOs

- `TODO(fiscal)` D2: modo y paso de redondeo.
- `TODO(fiscal)`: carga de `fiscal.tax_rates`, `tax_types`, `units_of_measure`, `identification_types`, `sale_conditions`, `payment_methods`.
- `TODO(fiscal)`: dirección del snapshot con códigos de provincia/cantón/distrito (hoy `customer_address` texto = `address_details`).
- `TODO`: CHECKs de longitud en columnas `text` (§3.4).
