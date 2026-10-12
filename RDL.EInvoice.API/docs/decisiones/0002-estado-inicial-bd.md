# 0002 · Estado inicial de la base de datos (informe de brechas)

- **Fecha:** 2026-10-12
- **Estado:** Propuesto. Espera aprobación. **No se modificó nada** en la base durante la inspección.
- **Fuente:** inspección en solo lectura del proyecto Supabase dev `dzlsnsstuqpxvwegeqcy` vía MCP (`pg_catalog`,
  `pg_policy`, `aclexplode`, `pg_get_functiondef`, `supabase_migrations.schema_migrations`), después de que el
  proyecto se restaurara (estaba pausado).
- **Referencias:** informes 0001 y 0003; `docs/PLAN.md` (decisiones aprobadas); ADR 0001/0002 de Receivables (mismo
  patrón de propuesta a `database-platform`).

## 1. Resumen

El schema `fiscal` existe con sus **26 tablas** (el prompt decía 27: no existe ninguna tabla aparte de las que lista),
**todas vacías**, con RLS activa en todas, FK compuestas por organización y un trigger que protege la inmutabilidad
del documento firmado. La base está **mejor que lo que supone el prompt en integridad**, y le faltan cosas en tres
frentes:

1. **Contra Hacienda:** el `CHECK` de la clave solo admite dígitos (el XSD admite letras); faltan los datos del emisor,
   la referencia y los medios de pago que el XML exige (informe 0003); no hay unicidad por documento de origen.
2. **Contra el worker:** no hay dónde encolar un documento ni guardar su último error sin chocar con el trigger guardián.
3. **Contra la operación:** no hay logins, no hay historial goose, `fiscal_app` no puede revalidar membresías en
   `core` y faltan cinco catálogos del anexo oficial.

Nada de esto requiere tocar datos (no hay). Lo de `fiscal` va como migraciones goose de este repo; lo de `core`,
roles y `shared` como **propuesta a `database-platform`** (§6).

## 2. Inventario

### 2.1 Historial

`supabase_migrations.schema_migrations`: `0003_fiscal` (20260923200118) creó el schema; `0006_audit_integration`,
`0007_grants` y `0008_catalog_policies` lo completan. **No existe `fiscal.goose_db_version`** (sí las de `core`,
`billing` y `receivables`). Dueño de schema, tablas y funciones: `fiscal_migrator`.

### 2.2 Documentos

| Tabla | Claves | CHECK y reglas | Grant de `fiscal_app` |
|---|---|---|---|
| `electronic_documents` | PK; UK `(org,id)`, `(org,source_event_id)`, `(org,consecutive_number)`, `(numeric_key)`; FK `(org,terminal_id)` → terminals, `(org,referenced_electronic_document_id)` → sí misma, `document_type_code` → `document_types` | `status` ∈ 7 estados; `accepted ⇔ accepted_at`; `rejected ⇔ rejected_at`; `source_type` ∈ invoice/credit_note/debit_note; `invoice ⇔ sin referencia`; `numeric_key ~ '^[0-9]{50}$'`; `consecutive_number ~ '^[0-9]{20}$'`; `credit_term_days >= 0` | INSERT, SELECT, **UPDATE** (sin DELETE) |
| `electronic_document_lines` | PK; UK `(org,id)`, `(org,doc,line_number)`; FK compuesta | `line_number > 0` | INSERT, SELECT: **inmutable** |
| `electronic_document_line_taxes` | PK; UK `(org,id)`, `(org,line,tax_type_code)` (un impuesto de cada tipo por línea) | exoneración: los 6 campos o ninguno | INSERT, SELECT |
| `electronic_document_status_history` | PK; UK `(org,id)`; idx `(org,doc,changed_at)` | — | INSERT, SELECT |
| `submission_attempts` | PK; UK `(org,id)`, `(org,doc,attempt_number)`; idx parcial `(next_retry_at) where not null` | `request_type` ∈ `send`, `status_query`; `attempt_number > 0` | INSERT, SELECT: **inmutable** |
| `document_files` | PK; UK `(org,id)`, `(storage_path)`; FK a documento y a intento | `file_kind` ∈ `unsigned_xml`, `signed_xml`, `hacienda_response_xml`, `pdf`; `size_bytes > 0`; `sha256` con dominio `shared.sha256_hex` | INSERT, SELECT |

**Trigger `electronic_documents_guard`** (BEFORE UPDATE):
- el consecutivo y la clave, una vez asignados, **no cambian**;
- fuera de `processing` **solo** pueden cambiar `status`, `hacienda_status_message`, `accepted_at`, `rejected_at` y
  `updated_at`;
- `accepted` y `rejected` son finales.

Consecuencias para el diseño (coinciden con el informe 0003 §6.3): el consecutivo, la clave y todo el snapshot se
fijan **mientras el documento está en `processing`**; firmar es la última escritura de contenido. La base **no**
valida qué transición de estado es legal (salvo las finales): eso lo hace el dominio.

### 2.3 Configuración

| Tabla | Claves | Reglas |
|---|---|---|
| `taxpayer_profiles` | UK `(organization_id)`: uno por organización; FK a `identification_types` y `districts` | `environment` ∈ `sandbox`, `production`, **por defecto `sandbox`**; credenciales de Hacienda **por referencia** (`hacienda_username_secret_id`, `hacienda_password_secret_id`); ubicación con códigos de provincia, cantón y distrito + `address_details` |
| `certificates` | UK `(org,id)` | `valid_to > valid_from`; `storage_path` y `pin_secret_id` obligatorios; sujeto, emisor y serie; `is_active` |
| `organization_economic_activities` | UK `(org,activity_code)`; FK a `economic_activities` | `is_primary` |
| `establishments` | UK `(org,code)`, `(org,branch_id)`, `(org,id)` | `code ~ '^[0-9]{3}$'`; **`branch_id NOT NULL` y sin FK a `core.branches`** |
| `terminals` | UK `(org,establishment,code)`; FK compuesta a establecimiento | `code ~ '^[0-9]{5}$'` |
| `document_sequences` | UK `(org,terminal_id,document_type_code)` | `next_value` entre 1 y 9 999 999 999 |

La secuencia por (organización, terminal, tipo de comprobante) **ya coincide con la decisión 4**: el establecimiento
sale de la terminal.

### 2.4 Catálogos (todos con 0 filas)

`identification_types`, `document_types`, `tax_types`, `tax_rates` (con `rate`), `sale_conditions` (con
`requires_credit_term`), `payment_methods`, `units_of_measure`, `exoneration_document_types`, `economic_activities`,
`provinces` → `cantons` → `districts` (FK compuestas), `cabys_versions` (`version_label` único, `source_sha256`,
`is_current`) → `cabys_items` (PK `(version_id, code)`, índice trigram sobre la descripción para la búsqueda).

Políticas de catálogo: lectura para todos (`*_read`, `true`); **INSERT, UPDATE y DELETE para `fiscal_app`**.

### 2.5 RLS, grants y roles

- Tablas de organización: una política `*_tenant` para todos los comandos,
  `organization_id = (select shared.current_organization_id())`, en USING y WITH CHECK.
- `fiscal_app`: lo de la tabla de §2.2–2.4 en `fiscal`; **`core.branches` SELECT** y nada más de `core`;
  `audit.audit_events` INSERT, SELECT; `integration.inbox_messages`, `outbox_messages`, `dead_letters` INSERT, SELECT,
  UPDATE e `idempotency_keys` con DELETE. USAGE en `shared`, `extensions`, `audit`, `integration`, `core` y `fiscal`.
  **Sin** acceso a `billing`, `receivables` ni `subscriptions` (criterio 4 ✅).
- `fiscal_migrator`: dueño de `fiscal` (USAGE + CREATE). **Sin USAGE en `core`.**
- `fiscal_app` y `fiscal_migrator` son NOLOGIN y sin `BYPASSRLS`. **No existen `fiscal_api` ni `fiscal_migrate`.**
- `integration.*`: los `CHECK` de `consumer_service`, `source_service` y `service` ya admiten `fiscal`; el outbox se
  filtra por `source_service = shared.current_service()`, que devuelve `fiscal` para los miembros de `fiscal_app`.
- `shared`: dominios `money_amount` (18,5 ≥ 0), `exchange_rate` (> 0), `quantity` (16,3 > 0), `percentage` (7,4,
  0–100), `currency_code`, `cabys_code` (13 dígitos), `sha256_hex`; funciones `current_organization_id()`,
  `current_user_id()`, `current_service()`, `set_updated_at()`, `prevent_modification()`.

## 3. Brechas contra Hacienda (anexo oficial, XSD e informe 0003)

| # | Brecha | Propuesta | Dónde |
|---|---|---|---|
| B1 | `numeric_key ~ '^[0-9]{50}$'`, pero `ClaveType` del XSD es `[a-zA-Z0-9]{50}` | Reemplazar por `'^[0-9A-Za-z]{50}$'`. Relajar un CHECK no rompe filas (no hay) | Migración fiscal |
| B2 | Sin unicidad por documento de origen: reprocesar **otro** evento del mismo documento crearía un segundo comprobante (criterio 3, solo cubierto por `source_event_id`) | UK `(organization_id, source_type, source_document_id)` | Migración fiscal |
| B3 | Falta el número visible del documento comercial, que exigen `ElectronicDocumentAccepted/Rejected` (`sourceDocumentNumber`) | `source_document_number text not null` (tabla vacía: se puede crear `not null`) | Migración fiscal |
| B4 | Falta `FechaEmision` aparte de `issued_at` (0003 §1: la fecha del XML es la de generación, no la de Billing) | `issued_at` se queda como el `issuedAt` de Billing; nueva `generated_at timestamptz` (FechaEmision, la misma que la clave) | Migración fiscal |
| B5 | Snapshot del emisor incompleto: solo tipo, número, nombre y actividad | `issuer_trade_name`, `issuer_province_code`, `issuer_canton_code`, `issuer_district_code`, `issuer_neighborhood`, `issuer_address_details`, `issuer_phone`, `issuer_email` | Migración fiscal |
| B6 | Snapshot del receptor sin teléfono ni ubicación (para cuando contratos agregue C5) | `receiver_phone`, `receiver_province_code`, `receiver_canton_code`, `receiver_district_code`, `receiver_address`, `receiver_economic_activity_code` (todas opcionales) | Migración fiscal |
| B7 | Referencia sin código de la Nota 9 (decisiones 2 y 3; C6 y C11) | `reference_code text` + `CHECK (source_type = 'invoice' or reference_code is not null)` **en NOT VALID→VALIDATE** (expand). En la factura, solo con sustitución de rechazado (TipoDocIR 10, código 16) | Migración fiscal |
| B8 | No hay medios de pago (`MedioPago {1,4}`) | Tabla `electronic_document_payment_methods` (org, documento, línea 1–4, `payment_method_code` → catálogo, `amount` opcional, `other_description`) con RLS de organización e INSERT/SELECT | Migración fiscal + grant |
| B9 | Línea sin `MontoTotal` (`gross_amount`) ni código de descuento (Nota 20) | `gross_amount shared.money_amount` (NOT VALID→VALIDATE como `not null` una vez que el consumidor la llene; hoy la tabla está vacía y puede ser `not null` directo), `discount_code text` | Migración fiscal |
| B10 | Exoneración del oficial: institución codificada (Nota 23), artículo, inciso, «otro»; y `exoneration_percentage numeric(7,4)` debe guardar **puntos de tarifa** (hallazgo 17 de contratos) | Columnas opcionales `exoneration_institution_code`, `exoneration_institution_other`, `exoneration_document_type_other`, `exoneration_article`, `exoneration_subsection`. **No** se tocan las 6 del `CHECK` todo-o-nada. `exoneration_percentage` se **documenta** como puntos de tarifa (`COMMENT ON COLUMN`): renombrarla sería un *contract* sin beneficio con la tabla vacía de usos | Migración fiscal |
| B11 | Faltan catálogos del anexo oficial | Tablas `reference_codes` (Nota 9), `reference_document_types` (Nota 10), `discount_codes` (Nota 20), `exoneration_institutions` (Nota 23), `currencies` (Nota 13.1), con las mismas políticas que los demás catálogos | Migración fiscal + políticas |
| B12 | Los catálogos no guardan versión ni fuente (solo CABYS) | Tabla `catalog_imports` (`catalog`, `version_label`, `source_file`, `source_sha256`, `row_count`, `imported_at`) que escribe `cmd/import-catalogs` | Migración fiscal |
| B13 | Nada impide dos certificados activos ni dos actividades principales | Índices únicos parciales `certificates (organization_id) where is_active` y `organization_economic_activities (organization_id) where is_primary` | Migración fiscal |

## 4. Brechas contra el worker

| # | Brecha | Propuesta |
|---|---|---|
| W1 | No hay cola: `submission_attempts` es solo INSERT, así que su `next_retry_at` no se puede «consumir»; un documento recién creado no tiene intento | `electronic_documents.next_attempt_at timestamptz` + `last_error text` + índice parcial `(next_attempt_at) where status in ('processing','signed','sent','contingency')`. El worker toma con `FOR UPDATE SKIP LOCKED` |
| W2 | El guardián rechaza cambiar `next_attempt_at` y `last_error` fuera de `processing` | Agregar las dos columnas a `v_mutable` en `fiscal.electronic_documents_guard` (`create or replace`, el dueño es `fiscal_migrator`). No relaja nada de lo fiscal: siguen fijos clave, consecutivo y snapshot |
| W3 | `request_type` ∈ `send`, `status_query`: no hay dónde registrar un fallo **antes** de enviar (XML inválido contra el XSD, firma, certificado vencido), que la máquina de estados pide «en `submission_attempts`» | Ampliar el CHECK a `build`, `sign`, `send`, `status_query` (relaja: compatible) |

## 5. Lo que este repo migra (goose, solo `fiscal`)

Historial en `fiscal.goose_db_version`. Patrón expand → migrate → contract; nada destructivo.

1. **`00001_baseline.sql`**: reproduce `0003_fiscal` + la parte fiscal de `0007_grants` y `0008_catalog_policies`
   tal como están (§2), con `create … if not exists` / bloques `do $$ … if not exists`. En dev se registra con
   `migrate mark-baseline` **sin ejecutarse**; en una base vacía (local, tests) crea el schema. Requiere el bootstrap
   de `database-platform` (roles, `shared.*`).
2. **`00002_hacienda_4_4.sql`** (expand, B1–B11 y B13):

```sql
-- +goose Up
-- B1 · Anexo 1 pág. 18 y XSD ClaveType: la clave admite letras para personas jurídicas.
alter table fiscal.electronic_documents drop constraint if exists electronic_documents_numeric_key_ck;
alter table fiscal.electronic_documents
  add constraint electronic_documents_numeric_key_ck check (numeric_key ~ '^[0-9A-Za-z]{50}$');

-- B2 · Criterio 3: un comprobante por documento comercial.
alter table fiscal.electronic_documents
  add constraint electronic_documents_source_document_uk unique (organization_id, source_type, source_document_id);

-- B3, B4, B5, B6, B7 · Snapshot y referencia (tabla vacía: not null directo donde aplica).
alter table fiscal.electronic_documents
  add column if not exists source_document_number          text not null,
  add column if not exists generated_at                    timestamptz,
  add column if not exists issuer_trade_name               text,
  add column if not exists issuer_province_code            text,
  add column if not exists issuer_canton_code              text,
  add column if not exists issuer_district_code            text,
  add column if not exists issuer_neighborhood             text,
  add column if not exists issuer_address_details          text,
  add column if not exists issuer_phone                    text,
  add column if not exists issuer_email                    text,
  add column if not exists receiver_phone                  text,
  add column if not exists receiver_province_code          text,
  add column if not exists receiver_canton_code            text,
  add column if not exists receiver_district_code          text,
  add column if not exists receiver_address                text,
  add column if not exists receiver_economic_activity_code text,
  add column if not exists reference_code                  text,
  add column if not exists next_attempt_at                 timestamptz,   -- W1
  add column if not exists last_error                      text;          -- W1
alter table fiscal.electronic_documents
  add constraint electronic_documents_issuer_district_fk
    foreign key (issuer_province_code, issuer_canton_code, issuer_district_code)
    references fiscal.districts (province_code, canton_code, code),
  add constraint electronic_documents_note_reference_code_ck
    check (source_type = 'invoice' or reference_code is not null);
create index if not exists electronic_documents_queue_idx on fiscal.electronic_documents (next_attempt_at)
  where status in ('processing', 'signed', 'sent', 'contingency');

-- B8 · MedioPago {1,4} (Anexo 1 págs. 55–56).
create table if not exists fiscal.electronic_document_payment_methods (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  position               smallint not null check (position between 1 and 4),
  payment_method_code    text not null references fiscal.payment_methods (code),
  amount                 shared.money_amount,
  other_description      text,
  constraint electronic_document_payment_methods_org_id_uk unique (organization_id, id),
  constraint electronic_document_payment_methods_position_uk unique (organization_id, electronic_document_id, position),
  constraint electronic_document_payment_methods_document_fk
    foreign key (organization_id, electronic_document_id) references fiscal.electronic_documents (organization_id, id)
);
alter table fiscal.electronic_document_payment_methods enable row level security;
create policy electronic_document_payment_methods_tenant on fiscal.electronic_document_payment_methods
  using (organization_id = (select shared.current_organization_id()))
  with check (organization_id = (select shared.current_organization_id()));
grant select, insert on fiscal.electronic_document_payment_methods to fiscal_app;

-- B9 · MontoTotal y CodigoDescuento (Anexo 1 págs. 35–36, Nota 20).
alter table fiscal.electronic_document_lines
  add column if not exists gross_amount  shared.money_amount not null,
  add column if not exists discount_code text;

-- B10 · Exoneración 4.4 (Anexo 1 págs. 44–46, Nota 23).
alter table fiscal.electronic_document_line_taxes
  add column if not exists exoneration_institution_code    text,
  add column if not exists exoneration_institution_other   text,
  add column if not exists exoneration_document_type_other text,
  add column if not exists exoneration_article             integer,
  add column if not exists exoneration_subsection          integer;
comment on column fiscal.electronic_document_line_taxes.exoneration_percentage is
  'Puntos de tarifa exonerados (TarifaExonerada, 4,2), no un porcentaje del impuesto. Anexo 1 págs. 45–46; contratos v0.2.0.';

-- W3 · Fallos antes del envío.
alter table fiscal.submission_attempts drop constraint if exists submission_attempts_request_type_ck;
alter table fiscal.submission_attempts add constraint submission_attempts_request_type_ck
  check (request_type in ('build', 'sign', 'send', 'status_query'));

-- B13
create unique index if not exists certificates_one_active_uk on fiscal.certificates (organization_id) where is_active;
create unique index if not exists organization_economic_activities_one_primary_uk
  on fiscal.organization_economic_activities (organization_id) where is_primary;
```

   Con B11 y B12 (cinco catálogos nuevos y `catalog_imports`, todos con el patrón de §2.4) y W2
   (`create or replace function fiscal.electronic_documents_guard()` con `next_attempt_at` y `last_error` en
   `v_mutable`). El `-- +goose Down` revierte en orden inverso; es posible porque todo es aditivo y la tabla está
   vacía.

3. **Catálogos:** no van en migraciones sino en `cmd/import-catalogs`, desde `docs/hacienda/` con la versión y el
   sha256 de la fuente en `catalog_imports` (prompt, Paso 2). **Excepción necesaria para el incremento 3:**
   `electronic_documents.document_type_code` tiene FK a `document_types`, así que el consumidor no puede crear un
   documento sin los tipos 01, 02 y 03 de la Nota 3 (pág. 66). Se importan primero, también con el comando.

## 6. Propuesta a `database-platform` (no la aplica este repo)

Archivo listo para el SQL Editor en el incremento 1: `scripts/dev/fiscal_login_roles.sql`. Igual que
`0010_receivables_login_roles` y `0011_receivables_core_read`.

```sql
-- 1 · Logins (las contraseñas se asignan después, fuera de banda, y quedan solo en el .env local).
create role fiscal_api login inherit nobypassrls connection limit 20 in role fiscal_app;
create role fiscal_migrate login inherit nobypassrls connection limit 2 in role fiscal_migrator;
alter role fiscal_migrate set role = 'fiscal_migrator';
-- alter role fiscal_api     password '<generada>';
-- alter role fiscal_migrate password '<generada>';

-- 2 · Revalidar la membresía en cada petición y leer la zona horaria (bloquea el incremento 2).
grant select on core.organizations, core.organization_users, core.organization_user_roles, core.users to fiscal_app;
grant execute on function core.find_user_by_subject(text) to fiscal_app;

-- 3 · Decisión 4: establecimiento ↔ sucursal con FK real (hoy branch_id no tiene FK).
grant usage on schema core to fiscal_migrator;
grant references on core.branches to fiscal_migrator;
```

Con el punto 3, este repo agrega en una migración posterior (expand con `not valid` + `validate`):

```sql
alter table fiscal.establishments
  add constraint establishments_branch_fk foreign key (organization_id, branch_id)
  references core.branches (organization_id, id) not valid;
alter table fiscal.establishments validate constraint establishments_branch_fk;
```

### Para decidir (recomendaciones, no bloquean)

- **`branch_id NOT NULL` en `establishments` contra la decisión 4** («factura sin sucursal → 001»): o cada
  organización tiene al menos una sucursal (casa matriz) en `core.branches`, o `branch_id` pasa a admitir nulo con un
  único establecimiento sin sucursal por organización. **Recomiendo lo primero** (la casa matriz es una sucursal real
  con su dirección); depende de Platform, que hoy no crea una sucursal al crear la organización.
- **Escritura de catálogos por `fiscal_app`:** las políticas dejan a la API de runtime insertar y borrar catálogos.
  Recomiendo que `cmd/import-catalogs` corra como `fiscal_migrate` y que se revoque a `fiscal_app` la escritura sobre
  los catálogos (`revoke insert, update, delete on … from fiscal_app`). Lo puede hacer este repo en una migración (los
  objetos son de `fiscal_migrator`).
- `audit.audit_events`: igual que anotó Receivables, la lectura no está acotada al servicio.

## 7. Notas

- `document_sequences.next_value ≤ 9 999 999 999` coincide con el número de 10 posiciones de la Nota 3; al
  llegar al tope el dominio reinicia en 1 (Nota 3). La UK `(org, consecutive_number)` chocaría solo después de
  10¹⁰ comprobantes del mismo tipo en la misma terminal.
- **Al intentar conectar con `platform_api` y `platform_migrate` por el pooler** (2026-10-12, después de restaurar el
  proyecto), Supavisor respondió `password authentication failed`. Los roles existen. Revisar las contraseñas o la
  configuración del pooler antes de levantar Platform otra vez.
