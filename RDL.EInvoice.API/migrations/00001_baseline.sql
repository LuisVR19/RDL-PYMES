-- Baseline del schema fiscal.
-- Reproduce database-platform 0003_fiscal y la parte de fiscal de 0007_grants y 0008_catalog_policies, tal como
-- estaban en dev el 2026-10-12 (inspección de docs/decisiones/0002-estado-inicial-bd.md §2).
-- Es idempotente: sobre la base dev no cambia nada (y allí se registra con `migrate mark-baseline`, sin ejecutarla);
-- sobre una base vacía (local, tests) crea el schema.
-- Requiere que el bootstrap de database-platform (roles, schema fiscal, shared.*, extensión pg_trgm) ya exista.

-- +goose Up

-- Catálogos (Anexo 1 de Hacienda; se cargan con cmd/import-catalogs, nunca a mano).
create table if not exists fiscal.identification_types (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.document_types (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.tax_types (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.tax_rates (
  code      text primary key,
  name      text not null,
  rate      shared.percentage not null,
  is_active boolean not null default true
);
create table if not exists fiscal.sale_conditions (
  code                 text primary key,
  name                 text not null,
  requires_credit_term boolean not null default false,
  is_active            boolean not null default true
);
create table if not exists fiscal.payment_methods (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.units_of_measure (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.exoneration_document_types (
  code      text primary key,
  name      text not null,
  is_active boolean not null default true
);
create table if not exists fiscal.economic_activities (
  code        text primary key,
  description text not null,
  is_active   boolean not null default true
);
create table if not exists fiscal.provinces (
  code text primary key,
  name text not null
);
create table if not exists fiscal.cantons (
  province_code text not null references fiscal.provinces (code),
  code          text not null,
  name          text not null,
  constraint cantons_pk primary key (province_code, code)
);
create table if not exists fiscal.districts (
  province_code text not null,
  canton_code   text not null,
  code          text not null,
  name          text not null,
  constraint districts_pk primary key (province_code, canton_code, code),
  constraint districts_canton_fk foreign key (province_code, canton_code) references fiscal.cantons (province_code, code)
);
create table if not exists fiscal.cabys_versions (
  id            uuid primary key default gen_random_uuid(),
  version_label text not null,
  published_on  date,
  source_sha256 shared.sha256_hex,
  is_current    boolean not null default false,
  imported_at   timestamptz not null default now(),
  constraint cabys_versions_label_uk unique (version_label)
);
create table if not exists fiscal.cabys_items (
  version_id  uuid not null references fiscal.cabys_versions (id),
  code        shared.cabys_code not null,
  description text not null,
  tax_rate    shared.percentage,
  constraint cabys_items_pk primary key (version_id, code)
);
create index if not exists cabys_items_code_idx on fiscal.cabys_items (code);
create index if not exists cabys_items_description_trgm_idx on fiscal.cabys_items
  using gin (description extensions.gin_trgm_ops);

-- Configuración por organización.
create table if not exists fiscal.taxpayer_profiles (
  id                          uuid primary key default gen_random_uuid(),
  organization_id             uuid not null,
  identification_type_code    text not null references fiscal.identification_types (code),
  identification_number       text not null,
  legal_name                  text not null,
  trade_name                  text,
  email                       text not null,
  phone                       text,
  province_code               text,
  canton_code                 text,
  district_code               text,
  address_details             text,
  environment                 text not null default 'sandbox',
  hacienda_username_secret_id uuid,
  hacienda_password_secret_id uuid,
  created_at                  timestamptz not null default now(),
  updated_at                  timestamptz not null default now(),
  constraint taxpayer_profiles_org_uk unique (organization_id),
  constraint taxpayer_profiles_org_id_uk unique (organization_id, id),
  constraint taxpayer_profiles_environment_ck check (environment in ('sandbox', 'production')),
  constraint taxpayer_profiles_district_fk foreign key (province_code, canton_code, district_code)
    references fiscal.districts (province_code, canton_code, code)
);
create index if not exists taxpayer_profiles_district_idx on fiscal.taxpayer_profiles (province_code, canton_code, district_code);
create index if not exists taxpayer_profiles_identification_type_idx on fiscal.taxpayer_profiles (identification_type_code);

create table if not exists fiscal.certificates (
  id                  uuid primary key default gen_random_uuid(),
  organization_id     uuid not null,
  storage_path        text not null,
  pin_secret_id       uuid not null,
  subject             text,
  issuer              text,
  serial_number       text,
  valid_from          timestamptz not null,
  valid_to            timestamptz not null,
  is_active           boolean not null default true,
  uploaded_by_user_id uuid,
  created_at          timestamptz not null default now(),
  constraint certificates_org_id_uk unique (organization_id, id),
  constraint certificates_validity_ck check (valid_to > valid_from)
);

create table if not exists fiscal.organization_economic_activities (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null,
  activity_code   text not null references fiscal.economic_activities (code),
  is_primary      boolean not null default false,
  created_at      timestamptz not null default now(),
  constraint organization_economic_activities_org_id_uk unique (organization_id, id),
  constraint organization_economic_activities_org_activity_uk unique (organization_id, activity_code)
);
create index if not exists organization_economic_activities_activity_idx on fiscal.organization_economic_activities (activity_code);

create table if not exists fiscal.establishments (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null,
  branch_id       uuid not null,
  code            char(3) not null,
  name            text not null,
  is_active       boolean not null default true,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now(),
  constraint establishments_org_id_uk unique (organization_id, id),
  constraint establishments_org_code_uk unique (organization_id, code),
  constraint establishments_org_branch_uk unique (organization_id, branch_id),
  constraint establishments_code_ck check (code ~ '^[0-9]{3}$')
);

create table if not exists fiscal.terminals (
  id               uuid primary key default gen_random_uuid(),
  organization_id  uuid not null,
  establishment_id uuid not null,
  code             char(5) not null,
  name             text not null,
  is_active        boolean not null default true,
  created_at       timestamptz not null default now(),
  updated_at       timestamptz not null default now(),
  constraint terminals_org_id_uk unique (organization_id, id),
  constraint terminals_establishment_code_uk unique (organization_id, establishment_id, code),
  constraint terminals_code_ck check (code ~ '^[0-9]{5}$'),
  constraint terminals_establishment_fk foreign key (organization_id, establishment_id)
    references fiscal.establishments (organization_id, id)
);

create table if not exists fiscal.document_sequences (
  id                 uuid primary key default gen_random_uuid(),
  organization_id    uuid not null,
  terminal_id        uuid not null,
  document_type_code text not null references fiscal.document_types (code),
  next_value         bigint not null default 1,
  updated_at         timestamptz not null default now(),
  constraint document_sequences_org_id_uk unique (organization_id, id),
  constraint document_sequences_scope_uk unique (organization_id, terminal_id, document_type_code),
  constraint document_sequences_next_value_ck check (next_value >= 1 and next_value <= 9999999999),
  constraint document_sequences_terminal_fk foreign key (organization_id, terminal_id)
    references fiscal.terminals (organization_id, id)
);
create index if not exists document_sequences_document_type_idx on fiscal.document_sequences (document_type_code);

-- Documentos electrónicos.
create table if not exists fiscal.electronic_documents (
  id                                uuid primary key default gen_random_uuid(),
  organization_id                   uuid not null,
  source_type                       text not null,
  source_document_id                uuid not null,
  source_event_id                   uuid not null,
  document_type_code                text not null references fiscal.document_types (code),
  terminal_id                       uuid,
  consecutive_number                varchar(20),
  numeric_key                       char(50),
  status                            text not null default 'processing',
  issued_at                         timestamptz not null,
  issuer_identification_type_code   text not null,
  issuer_identification_number      text not null,
  issuer_legal_name                 text not null,
  issuer_economic_activity_code     text,
  receiver_identification_type_code text,
  receiver_identification_number    text,
  receiver_legal_name               text,
  receiver_email                    text,
  sale_condition_code               text not null,
  credit_term_days                  integer,
  currency_code                     shared.currency_code not null,
  exchange_rate                     shared.exchange_rate not null default 1,
  subtotal_amount                   shared.money_amount not null,
  discount_amount                   shared.money_amount not null default 0,
  tax_amount                        shared.money_amount not null default 0,
  exoneration_amount                shared.money_amount not null default 0,
  total_amount                      shared.money_amount not null,
  referenced_electronic_document_id uuid,
  reference_reason                  text,
  hacienda_status_message           text,
  accepted_at                       timestamptz,
  rejected_at                       timestamptz,
  created_at                        timestamptz not null default now(),
  updated_at                        timestamptz not null default now(),
  constraint electronic_documents_org_id_uk unique (organization_id, id),
  constraint electronic_documents_source_event_uk unique (organization_id, source_event_id),
  constraint electronic_documents_consecutive_uk unique (organization_id, consecutive_number),
  constraint electronic_documents_numeric_key_uk unique (numeric_key),
  constraint electronic_documents_source_type_ck check (source_type in ('invoice', 'credit_note', 'debit_note')),
  constraint electronic_documents_status_ck
    check (status in ('processing', 'signed', 'sent', 'accepted', 'rejected', 'contingency', 'error')),
  constraint electronic_documents_accepted_ck check ((status = 'accepted') = (accepted_at is not null)),
  constraint electronic_documents_rejected_ck check ((status = 'rejected') = (rejected_at is not null)),
  constraint electronic_documents_reference_ck check ((source_type = 'invoice') = (referenced_electronic_document_id is null)),
  constraint electronic_documents_numeric_key_ck check (numeric_key ~ '^[0-9]{50}$'),
  constraint electronic_documents_consecutive_ck check (consecutive_number ~ '^[0-9]{20}$'),
  constraint electronic_documents_credit_term_ck check (credit_term_days is null or credit_term_days >= 0),
  constraint electronic_documents_terminal_fk foreign key (organization_id, terminal_id)
    references fiscal.terminals (organization_id, id),
  constraint electronic_documents_reference_fk foreign key (organization_id, referenced_electronic_document_id)
    references fiscal.electronic_documents (organization_id, id)
);
create index if not exists electronic_documents_document_type_idx on fiscal.electronic_documents (document_type_code);
create index if not exists electronic_documents_reference_idx on fiscal.electronic_documents (organization_id, referenced_electronic_document_id);
create index if not exists electronic_documents_status_idx on fiscal.electronic_documents (organization_id, status, created_at);
create index if not exists electronic_documents_terminal_idx on fiscal.electronic_documents (organization_id, terminal_id);

create table if not exists fiscal.electronic_document_lines (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  line_number            integer not null,
  cabys_code             shared.cabys_code not null,
  product_code           text,
  description            text not null,
  unit_of_measure_code   text not null,
  quantity               shared.quantity not null,
  unit_price             shared.money_amount not null,
  discount_amount        shared.money_amount not null default 0,
  discount_reason        text,
  subtotal_amount        shared.money_amount not null,
  tax_amount             shared.money_amount not null default 0,
  total_amount           shared.money_amount not null,
  constraint electronic_document_lines_org_id_uk unique (organization_id, id),
  constraint electronic_document_lines_number_uk unique (organization_id, electronic_document_id, line_number),
  constraint electronic_document_lines_number_ck check (line_number > 0),
  constraint electronic_document_lines_document_fk foreign key (organization_id, electronic_document_id)
    references fiscal.electronic_documents (organization_id, id)
);

create table if not exists fiscal.electronic_document_line_taxes (
  id                             uuid primary key default gen_random_uuid(),
  organization_id                uuid not null,
  electronic_document_line_id    uuid not null,
  tax_type_code                  text not null,
  tax_rate_code                  text,
  rate                           shared.percentage not null,
  taxable_base                   shared.money_amount not null,
  tax_amount                     shared.money_amount not null,
  exoneration_document_type_code text,
  exoneration_document_number    text,
  exoneration_institution        text,
  exoneration_issued_at          timestamptz,
  exoneration_percentage         shared.percentage,
  exoneration_amount             shared.money_amount,
  constraint electronic_document_line_taxes_org_id_uk unique (organization_id, id),
  constraint electronic_document_line_taxes_line_tax_uk unique (organization_id, electronic_document_line_id, tax_type_code),
  constraint electronic_document_line_taxes_exoneration_ck check (num_nulls(exoneration_document_type_code,
    exoneration_document_number, exoneration_institution, exoneration_issued_at, exoneration_percentage,
    exoneration_amount) in (0, 6)),
  constraint electronic_document_line_taxes_line_fk foreign key (organization_id, electronic_document_line_id)
    references fiscal.electronic_document_lines (organization_id, id)
);

create table if not exists fiscal.electronic_document_status_history (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  from_status            text,
  to_status              text not null,
  reason                 text,
  changed_at             timestamptz not null default now(),
  constraint electronic_document_status_history_org_id_uk unique (organization_id, id),
  constraint electronic_document_status_history_document_fk foreign key (organization_id, electronic_document_id)
    references fiscal.electronic_documents (organization_id, id)
);
create index if not exists electronic_document_status_history_document_idx
  on fiscal.electronic_document_status_history (organization_id, electronic_document_id, changed_at);

create table if not exists fiscal.submission_attempts (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  attempt_number         integer not null,
  request_type           text not null,
  attempted_at           timestamptz not null default now(),
  http_status            integer,
  hacienda_status        text,
  response_message       text,
  next_retry_at          timestamptz,
  constraint submission_attempts_org_id_uk unique (organization_id, id),
  constraint submission_attempts_number_uk unique (organization_id, electronic_document_id, attempt_number),
  constraint submission_attempts_number_ck check (attempt_number > 0),
  constraint submission_attempts_request_type_ck check (request_type in ('send', 'status_query')),
  constraint submission_attempts_document_fk foreign key (organization_id, electronic_document_id)
    references fiscal.electronic_documents (organization_id, id)
);
create index if not exists submission_attempts_retry_idx on fiscal.submission_attempts (next_retry_at)
  where next_retry_at is not null;

create table if not exists fiscal.document_files (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  submission_attempt_id  uuid,
  file_kind              text not null,
  storage_path           text not null,
  sha256                 shared.sha256_hex not null,
  size_bytes             bigint not null,
  created_at             timestamptz not null default now(),
  constraint document_files_org_id_uk unique (organization_id, id),
  constraint document_files_storage_path_uk unique (storage_path),
  constraint document_files_kind_ck check (file_kind in ('unsigned_xml', 'signed_xml', 'hacienda_response_xml', 'pdf')),
  constraint document_files_size_ck check (size_bytes > 0),
  constraint document_files_document_fk foreign key (organization_id, electronic_document_id)
    references fiscal.electronic_documents (organization_id, id),
  constraint document_files_attempt_fk foreign key (organization_id, submission_attempt_id)
    references fiscal.submission_attempts (organization_id, id)
);
create index if not exists document_files_document_idx on fiscal.document_files (organization_id, electronic_document_id);
create index if not exists document_files_attempt_idx on fiscal.document_files (organization_id, submission_attempt_id);

-- Inmutabilidad del documento firmado (arquitectura 7.4): clave y consecutivo fijos; fuera de processing solo cambia
-- el estado; accepted y rejected son finales.
-- +goose StatementBegin
create or replace function fiscal.electronic_documents_guard()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_mutable constant text[] := array['status', 'hacienda_status_message', 'accepted_at', 'rejected_at', 'updated_at'];
begin
  if old.consecutive_number is not null and new.consecutive_number is distinct from old.consecutive_number then
    raise exception 'El consecutivo del documento % no puede cambiar', old.id using errcode = 'restrict_violation';
  end if;
  if old.numeric_key is not null and new.numeric_key is distinct from old.numeric_key then
    raise exception 'La clave numérica del documento % no puede cambiar', old.id using errcode = 'restrict_violation';
  end if;
  if old.status <> 'processing'
     and (to_jsonb(new) - v_mutable) is distinct from (to_jsonb(old) - v_mutable) then
    raise exception 'El documento electrónico % ya fue firmado: solo puede cambiar su estado', old.id
      using errcode = 'restrict_violation';
  end if;
  if old.status in ('accepted', 'rejected') and new.status <> old.status then
    raise exception 'El documento electrónico % está en estado final (%)', old.id, old.status
      using errcode = 'restrict_violation';
  end if;
  return new;
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
do $$
declare
  t text;
begin
  if not exists (select 1 from pg_catalog.pg_trigger where tgname = 'electronic_documents_guard'
                 and tgrelid = 'fiscal.electronic_documents'::regclass) then
    create trigger electronic_documents_guard before update on fiscal.electronic_documents
      for each row execute function fiscal.electronic_documents_guard();
  end if;
  foreach t in array array['document_sequences', 'electronic_documents', 'establishments', 'taxpayer_profiles', 'terminals']
  loop
    if not exists (select 1 from pg_catalog.pg_trigger where tgname = t || '_set_updated_at'
                   and tgrelid = ('fiscal.' || t)::regclass) then
      execute format('create trigger %I before update on fiscal.%I for each row execute function shared.set_updated_at()',
        t || '_set_updated_at', t);
    end if;
  end loop;
end $$;
-- +goose StatementEnd

-- RLS: todas las tablas. Las de organización con una política *_tenant; los catálogos con lectura para todos y
-- escritura solo para fiscal_app (0008_catalog_policies).
-- +goose StatementBegin
do $$
declare
  t text;
begin
  foreach t in array array['taxpayer_profiles', 'certificates', 'organization_economic_activities', 'establishments',
                           'terminals', 'document_sequences', 'electronic_documents', 'electronic_document_lines',
                           'electronic_document_line_taxes', 'electronic_document_status_history',
                           'submission_attempts', 'document_files']
  loop
    execute format('alter table fiscal.%I enable row level security', t);
    if not exists (select 1 from pg_catalog.pg_policies
                   where schemaname = 'fiscal' and tablename = t and policyname = t || '_tenant') then
      execute format(
        'create policy %I on fiscal.%I for all
           using (organization_id = (select shared.current_organization_id()))
           with check (organization_id = (select shared.current_organization_id()))',
        t || '_tenant', t);
    end if;
  end loop;

  foreach t in array array['identification_types', 'document_types', 'tax_types', 'tax_rates', 'sale_conditions',
                           'payment_methods', 'units_of_measure', 'exoneration_document_types', 'economic_activities',
                           'provinces', 'cantons', 'districts', 'cabys_versions', 'cabys_items']
  loop
    execute format('alter table fiscal.%I enable row level security', t);
    if not exists (select 1 from pg_catalog.pg_policies where schemaname = 'fiscal' and tablename = t
                   and policyname = t || '_read') then
      execute format('create policy %I on fiscal.%I for select using (true)', t || '_read', t);
      execute format('create policy %I on fiscal.%I for insert to fiscal_app with check (true)', t || '_insert', t);
      execute format('create policy %I on fiscal.%I for update to fiscal_app using (true) with check (true)', t || '_update', t);
      execute format('create policy %I on fiscal.%I for delete to fiscal_app using (true)', t || '_delete', t);
    end if;
  end loop;
end $$;
-- +goose StatementEnd

-- Privilegios de fiscal_app tal como los dejó 0007_grants. Los ajustes propuestos (docs/decisiones/0002 §6) los
-- aplica database-platform o una migración posterior; esta baseline no los adelanta.
grant usage on schema fiscal to fiscal_app;
grant select, insert, update, delete on fiscal.identification_types, fiscal.document_types, fiscal.tax_types,
  fiscal.tax_rates, fiscal.sale_conditions, fiscal.payment_methods, fiscal.units_of_measure,
  fiscal.exoneration_document_types, fiscal.economic_activities, fiscal.provinces, fiscal.cantons, fiscal.districts,
  fiscal.cabys_versions, fiscal.cabys_items, fiscal.taxpayer_profiles, fiscal.certificates,
  fiscal.organization_economic_activities, fiscal.establishments, fiscal.terminals, fiscal.document_sequences
  to fiscal_app;
grant select, insert, update on fiscal.electronic_documents to fiscal_app;
grant select, insert on fiscal.electronic_document_lines, fiscal.electronic_document_line_taxes,
  fiscal.electronic_document_status_history, fiscal.submission_attempts, fiscal.document_files to fiscal_app;

-- +goose Down
-- La baseline nunca se revierte: el schema preexiste al repo (lo creó database-platform).
select 1;
