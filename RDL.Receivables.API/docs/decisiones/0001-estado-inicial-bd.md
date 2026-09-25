# 0001 · Estado inicial de la base de datos (informe de brechas)

- **Fecha:** 2026-09-24
- **Estado:** Aprobado el 2026-09-24. No se modificó nada en la base durante la inspección.
- **Fuente:** inspección en solo lectura del proyecto Supabase dev `dzlsnsstuqpxvwegeqcy` vía MCP (`information_schema`,
  `pg_catalog`, `pg_policy`, `pg_get_functiondef`, `supabase_migrations.schema_migrations`).
- **Referencias:** prompt P6; `RDL.Contracts` v0.1.0 (`state-machines/receivable.yaml`, `payment.yaml`, eventos,
  `openapi/receivables.yaml`, `problems/receivables.yaml`, `docs/ESTADO.md`); ADR 0003, 0004 y 0007 de Platform.

## 1. Resumen

El schema `receivables` está **más completo de lo que supone el prompt**: además de tablas, FK compuestas y RLS, ya
tiene **triggers que calculan el saldo y el estado** (`recalculate_receivable`) y que protegen la inmutabilidad de
cuentas, pagos y aplicaciones. Esto es bueno (la base es la última defensa de las invariantes), pero obliga a:

1. que el dominio en Go y la función de la base calculen **exactamente lo mismo** (sección 4.1: hoy la base y la
   máquina de estados del contrato **no coinciden** en cuándo una cuenta está `open` o `partially_paid`);
2. una **baseline** goose idempotente y unas pocas migraciones pequeñas en `receivables` (sección 5);
3. una **propuesta a `database-platform`** con los logins, el `GRANT` sobre `core` (bloquea el incremento 2) y dos
   ajustes de privilegios (`0002-propuesta-database-platform.md`).

No hay datos: 0 cuentas, 0 pagos y 0 ajustes. Los catálogos `fiscal.payment_methods` y `fiscal.sale_conditions`
están vacíos.

## 2. Inventario

### 2.1 Historial

`supabase_migrations.schema_migrations`: `0005_receivables` (20260923200302) creó el schema; `0006_audit_integration`,
`0007_grants` y `0008_catalog_policies` completan. No existe `receivables.goose_db_version`.

### 2.2 Tablas (dueño `receivables_migrator`, RLS activa en todas)

| Tabla | Claves e índices | CHECK | Notas |
|---|---|---|---|
| `receivables` | PK `id`; UK `(org,id)`, `(org,source_event_id)`, `(org,source_invoice_id)`; idx `(org,customer_id)`, `(org,due_on) where status in (open,partially_paid)` | `status` ∈ 4 estados; `original_amount > 0`; `due_on >= issued_on` | Snapshot: `customer_identification_number`, `customer_legal_name`, `document_number`, `sale_condition_code`. Sin `exchange_rate` ni `branch_id` |
| `payments` | PK; UK `(org,id)`; idx `(org,customer_id,received_on desc)` | `amount > 0`; `status` ∈ `posted, voided`; `voided ⇔ voided_at y void_reason` | `exchange_rate` default 1 |
| `payment_applications` | PK; UK `(org,id)`; **UK parcial `(org,payment_id,receivable_id) where reversed_at is null`**; idx `(org,receivable_id)`; FK compuestas a pago y cuenta | `amount > 0`; `reversed_at is null ⇔ reversal_reason is null` | Se revierte, no se borra (sin `DELETE` en el grant) |
| `receivable_adjustments` | PK; UK `(org,id)`, `(org,source_event_id)`, `(org,adjustment_type,source_document_id)`; FK compuesta | `amount > 0`; tipo ∈ `credit_note, debit_note, cancellation, write_off`; `source_document_id` obligatorio salvo `write_off` | Solo `INSERT, SELECT` para la app: inmutable |
| `collection_followups` | PK; UK `(org,id)`; idx `(org,receivable_id,performed_at desc)`, `(org,next_action_on)` | tipo ∈ `call, email, visit, message, note` | |
| `payment_promises` | PK; UK `(org,id)`; FK a cuenta y a seguimiento (opcional); idx por cuenta, seguimiento y `pending` | `promised_amount > 0`; `status` ∈ `pending, kept, broken, cancelled` | Sin reglas de transición en la base |
| vista `receivable_aging` | `security_invoker = true` | — | Usa `CURRENT_DATE` (UTC) y tramos `current, 1_30, 31_60, 61_90, 90_plus` |

Todas las políticas son `organization_id = (select shared.current_organization_id())` para `ALL`, con `USING` y
`WITH CHECK`. Todas las fechas de instante son `timestamptz`; las de negocio, `date`. Montos con `shared.money_amount`
(`numeric(18,5) >= 0`). **No hay `float`, `real` ni `double precision`.**

### 2.3 Triggers y funciones (`search_path = ''`, sin `security definer`)

| Trigger | Qué hace |
|---|---|
| `receivables_guard` (BEFORE INSERT/UPDATE) | En el alta fuerza `balance = original`, `status = open`. Después impide cambiar saldo, estado o `settled_at` salvo con el GUC `receivables.recalculating = on`, e impide cambiar los datos de origen |
| `payment_applications_validate` (AFTER INSERT/UPDATE) | Bloquea el pago (`FOR UPDATE`); rechaza pago anulado, cuenta anulada, **cliente distinto**, moneda distinta y suma aplicada > pago; solo deja actualizar la reversión; llama a `recalculate_receivable` |
| `receivable_adjustments_apply` (AFTER INSERT) | Llama a `recalculate_receivable` |
| `payments_guard` (BEFORE UPDATE) | Con aplicaciones, monto, cliente y moneda son inmutables; `voided` es final; **no deja anular un pago con aplicaciones vigentes** (hay que revertirlas antes, en la misma transacción) |
| `recalculate_receivable(org, id)` | Bloquea la cuenta (`FOR UPDATE`), calcula `saldo = original + débitos − (créditos + cancelación + castigos) − aplicaciones vigentes`, falla si queda negativo y deriva el estado: `cancelled` si hay ajuste de anulación; `paid` si saldo = 0; `partially_paid` si hay aplicaciones vigentes; si no, `open` |

El orden de bloqueo de la base es **pago → cuenta**. La estrategia de concurrencia de la app debe respetarlo (ADR de
concurrencia).

### 2.4 Roles y privilegios de `receivables_app`

| Schema | USAGE | Tablas |
|---|---|---|
| `receivables` | sí | `receivables`, `payments`, `payment_applications`: I/S/U · `receivable_adjustments`: I/S · `collection_followups`, `payment_promises`: **I/S/U/D** · vista `receivable_aging`: I/S/U/D |
| `audit` | sí | `audit_events`: I/S (política de insert: `service = current_service()`) |
| `integration` | sí | `outbox_messages`, `inbox_messages`, `dead_letters`: I/S/U · `idempotency_keys`: I/S/U/D. Políticas por `current_service()` (y por organización en `idempotency_keys`) |
| `fiscal` | sí | `payment_methods`, `sale_conditions`: S (vacías) |
| `shared` | sí | dominios y funciones |
| **`core`** | **no** | — |
| `billing`, `subscriptions` | no | — |

Confirmado: `receivables_app` no puede escribir en `core`, `billing`, `fiscal` ni `subscriptions`, no puede
actualizar ni borrar en `audit`, y por la política de `outbox_messages` **no ve el outbox de otros servicios**.
`receivables_migrator` es dueño de las tablas, tiene `CREATE` en `receivables` y ningún privilegio fuera de él.
No existen logins que hereden estos roles.

## 3. Lo que se reutiliza tal cual

- Las 6 tablas, sus FK compuestas, unicidades, `CHECK` y políticas RLS.
- Las unicidades que dan idempotencia al consumo: `receivables (org, source_event_id)` y `(org, source_invoice_id)`;
  `receivable_adjustments (org, source_event_id)` y `(org, adjustment_type, source_document_id)`.
- Los triggers de inmutabilidad y de validación de aplicaciones como **última defensa**: el dominio valida primero para
  devolver el problem type correcto (422/409) y la base rechaza lo que se escape.
- `integration.*`, `audit.audit_events` y `shared.*` como están.

## 4. Lo que existe pero no cumple (o choca con el contrato)

### 4.1 El estado derivado de la base contradice `state-machines/receivable.yaml` · **decisión R1**

| Situación | Contrato | Base hoy |
|---|---|---|
| Nota de crédito parcial sin pagos | `open → partially_paid` (transición explícita) | `open` (no hay aplicaciones) |
| Cuenta pagada solo con notas de crédito y llega una nota de débito | `paid → partially_paid` | `open` |
| Se revierte la última aplicación de una cuenta que tenía nota de crédito | `open` solo si el saldo vuelve al total | `open` aunque el saldo sea menor que el total |

El contrato deriva el estado del **saldo frente al total adeudado** (`original + débitos`); la base, de si hay
**aplicaciones vigentes**. Opciones:

- **A (recomendada):** migración en `receivables` que reemplaza `recalculate_receivable` para seguir el contrato:
  `open` si saldo = original + débitos; `partially_paid` si 0 < saldo < eso; `paid` si 0; `cancelled` si hay anulación.
  Es `create or replace function`, sin cambio de datos (no hay filas).
- **B:** cambiar el contrato (`receivable.yaml` v1 → quitar las transiciones por nota de crédito). Necesita PR con 2
  aprobaciones en `RDL.Contracts`.

### 4.2 El guard de saldos se puede saltar desde la app · propuesta de migración

`receivables_guard` confía en el GUC `receivables.recalculating`, que **cualquier sesión** puede fijar con
`set_config`. Con `receivables_app` bastaría `select set_config('receivables.recalculating','on',true)` y un
`UPDATE` para escribir un saldo arbitrario. Propuesta: que el guard exija además `pg_trigger_depth() > 1` (el
recálculo siempre corre desde un trigger de aplicaciones o ajustes; un `UPDATE` directo de la app tiene profundidad 1).
Migración `create or replace function` en `receivables`, con test de aislamiento que lo pruebe.

### 4.3 La vista `receivable_aging` no respeta la zona de la organización

Usa `CURRENT_DATE`, que es la fecha **UTC** de la sesión: entre las 18:00 y las 24:00 de Costa Rica ya es "mañana".
La API **no usará la vista**: el aging se calcula con `asOf` (fecha de negocio en la zona de la organización,
calculada en la app) como parámetro de la consulta. Propuesta: dejar la vista (no rompe nada) y retirarla en una
migración *contract* cuando se confirme que nadie más la usa. Sus tramos (`current, 1_30, 31_60, 61_90, 90_plus`) se
proponen como códigos de la respuesta (decisión R6).

### 4.4 Privilegios de más (para `database-platform`, no se tocan aquí)

- `DELETE` sobre `collection_followups` y `payment_promises`: las gestiones de cobro son historial auditable.
  Propuesta: revocar (una promesa se cancela con su estado).
- `INSERT/UPDATE/DELETE` sobre la vista `receivable_aging`: es auto-actualizable. Con `security_invoker` no permite
  nada que la app no pueda hacer ya sobre la tabla, pero sobra. Propuesta: dejar solo `SELECT`.
- `audit_events_read` deja a `receivables_app` leer los audit events de **otros servicios** de la misma organización.
  Bajo riesgo; se anota.

### 4.5 Lo que la base valida y el contrato no tiene como error

`payment_applications_validate` exige que el pago y la cuenta sean **del mismo cliente**. `problems/receivables.yaml`
no tiene un tipo para eso. Propuesta de PR a contratos: `customer-mismatch` (422).

### 4.6 Casos que la base no puede registrar (dependen de decisiones abiertas)

- **Anular una factura ya pagada:** el ajuste `cancellation` sería por el saldo (0) y `CHECK amount > 0` lo rechaza.
- **Nota de crédito mayor que el saldo** (p. ej., después del pago total): el saldo quedaría negativo y
  `recalculate_receivable` lo rechaza.
- **Nota de débito:** trae su propio `dueDate`, pero se registra como ajuste sobre la cuenta de la factura, que tiene
  un solo `due_on`. El vencimiento de la nota se pierde.
- **Ajuste sobre una cuenta `cancelled`:** ningún trigger lo impide.

Ver decisiones R2 a R5 en `docs/PLAN.md`.

### 4.7 Índices que faltan

| Índice propuesto | Para |
|---|---|
| `payment_applications (organization_id, payment_id)` | Detalle del pago con aplicaciones revertidas (el UK parcial solo cubre las vigentes) y anulación |
| `receivables (organization_id, created_at desc, id desc)` | Paginación por cursor de `GET /v1/receivables` |
| `receivables (organization_id, status, created_at desc, id desc)` | Filtro por estado |
| `payments (organization_id, created_at desc, id desc)` | Paginación de `GET /v1/payments` |

El índice de aging (`receivables_open_due_idx`) ya existe.

## 5. Plan de migraciones (goose, solo `receivables`, historial en `receivables.goose_db_version`)

Todas se muestran antes de aplicarlas y se aplican solo en dev y local, con `receivables_migrate` (rol
`receivables_migrator`).

| # | Tipo | Contenido |
|---|---|---|
| 00001 | baseline | Reproduce el schema actual con `create ... if not exists` / `create or replace`; en dev se **marca como aplicada** sin ejecutar. Sirve para levantar una base local |
| 00002 | expand | Índices de 4.7 (`create index if not exists`; sin `concurrently` porque goose corre en transacción y las tablas están vacías) |
| 00003 | migrate | `receivables_guard` con `pg_trigger_depth() > 1` (4.2) |
| 00004 | migrate | `recalculate_receivable` según la decisión R1 (solo si se elige la opción A) |
| (futura) | contract | Retirar la vista `receivable_aging` (4.3), cuando se confirme |

Lo de `core`, `audit`, `integration` y los privilegios va en `0002-propuesta-database-platform.md`.

## 6. Diferencias con el prompt (mandan los nombres reales)

| Prompt | Base real |
|---|---|
| "estado derivado: `open` si el saldo es el total adeudado" | La base usa "hay aplicaciones vigentes" (4.1) |
| Snapshot del cliente "número, identificación y nombre" | Solo `customer_identification_number` y `customer_legal_name`: no se guarda el tipo de identificación ni el email |
| Anular un pago "revierte sus aplicaciones vigentes" | La base **exige** revertirlas antes del `UPDATE` a `voided`, en la misma transacción. Es el mismo resultado |
| Promesas `pending, kept, broken, cancelled` | Coinciden; la base no restringe transiciones (se hace en el dominio) |
| `receivables_app` necesita leer la zona horaria de la organización | Confirmado: no tiene `USAGE` en `core` |
| El aging "por tramos" | Existe una vista con tramos pero en UTC (4.3) |
