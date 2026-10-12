-- Expand: lo que el schema necesita para emitir comprobantes 4.4 (docs/decisiones/0002-estado-inicial-bd.md §3 y §4,
-- aprobado el 2026-10-12). Todo es aditivo o relaja una regla; las tablas están vacías en dev, por eso las columnas
-- obligatorias nuevas se crean `not null` sin valor por defecto.

-- +goose Up

-- B1 · Anexo 1 pág. 18 y XSD ClaveType [a-zA-Z0-9]{50}: la clave admite letras para personas jurídicas.
alter table fiscal.electronic_documents drop constraint if exists electronic_documents_numeric_key_ck;
alter table fiscal.electronic_documents
  add constraint electronic_documents_numeric_key_ck check (numeric_key ~ '^[0-9A-Za-z]{50}$');

-- B2 · Criterio 3: un solo comprobante por documento comercial, aunque llegue otro evento del mismo documento.
alter table fiscal.electronic_documents
  add constraint electronic_documents_source_document_uk unique (organization_id, source_type, source_document_id);

-- B3–B7, W1 · Snapshot de emisor y receptor, FechaEmision, referencia y cola del worker.
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
  add column if not exists next_attempt_at                 timestamptz,
  add column if not exists last_error                      text;
comment on column fiscal.electronic_documents.issued_at is 'issuedAt del evento de Billing (emisión comercial).';
comment on column fiscal.electronic_documents.generated_at is
  'FechaEmision del XML: instante de generación, el mismo de la clave (Anexo 1 pág. 19).';
alter table fiscal.electronic_documents
  add constraint electronic_documents_issuer_district_fk
    foreign key (issuer_province_code, issuer_canton_code, issuer_district_code)
    references fiscal.districts (province_code, canton_code, code),
  add constraint electronic_documents_note_reference_code_ck
    check (source_type = 'invoice' or reference_code is not null);
create index if not exists electronic_documents_queue_idx on fiscal.electronic_documents (next_attempt_at)
  where status in ('processing', 'signed', 'sent', 'contingency');

-- B8 · MedioPago {1,4} (Anexo 1 págs. 55–56, Nota 6).
create table if not exists fiscal.electronic_document_payment_methods (
  id                     uuid primary key default gen_random_uuid(),
  organization_id        uuid not null,
  electronic_document_id uuid not null,
  position               smallint not null,
  payment_method_code    text not null references fiscal.payment_methods (code),
  amount                 shared.money_amount,
  other_description      text,
  constraint electronic_document_payment_methods_org_id_uk unique (organization_id, id),
  constraint electronic_document_payment_methods_position_uk unique (organization_id, electronic_document_id, position),
  constraint electronic_document_payment_methods_position_ck check (position between 1 and 4),
  constraint electronic_document_payment_methods_document_fk foreign key (organization_id, electronic_document_id)
    references fiscal.electronic_documents (organization_id, id)
);
alter table fiscal.electronic_document_payment_methods enable row level security;
-- +goose StatementBegin
do $$
begin
  if not exists (select 1 from pg_catalog.pg_policies where schemaname = 'fiscal'
                 and tablename = 'electronic_document_payment_methods'
                 and policyname = 'electronic_document_payment_methods_tenant') then
    create policy electronic_document_payment_methods_tenant on fiscal.electronic_document_payment_methods for all
      using (organization_id = (select shared.current_organization_id()))
      with check (organization_id = (select shared.current_organization_id()));
  end if;
end $$;
-- +goose StatementEnd
grant select, insert on fiscal.electronic_document_payment_methods to fiscal_app;

-- B9 · MontoTotal y CodigoDescuento por línea (Anexo 1 págs. 35–36, Nota 20).
alter table fiscal.electronic_document_lines
  add column if not exists gross_amount  shared.money_amount not null,
  add column if not exists discount_code text;

-- B10 · Exoneración 4.4 (Anexo 1 págs. 44–46, Nota 23). Las 6 columnas del CHECK todo-o-nada no cambian.
alter table fiscal.electronic_document_line_taxes
  add column if not exists exoneration_institution_code    text,
  add column if not exists exoneration_institution_other   text,
  add column if not exists exoneration_document_type_other text,
  add column if not exists exoneration_article             integer,
  add column if not exists exoneration_subsection          integer;
comment on column fiscal.electronic_document_line_taxes.exoneration_percentage is
  'Puntos de tarifa exonerados (TarifaExonerada, 4,2), no un porcentaje del impuesto. Anexo 1 págs. 45–46; contratos v0.2.0.';

-- B11 · Catálogos del anexo oficial que no existían. Mismas políticas que los demás catálogos.
create table if not exists fiscal.reference_codes (           -- Nota 9
  code text primary key, name text not null, is_active boolean not null default true
);
create table if not exists fiscal.reference_document_types (  -- Nota 10
  code text primary key, name text not null, is_active boolean not null default true
);
create table if not exists fiscal.discount_codes (            -- Nota 20
  code text primary key, name text not null, is_active boolean not null default true
);
create table if not exists fiscal.exoneration_institutions (  -- Nota 23
  code text primary key, name text not null, is_active boolean not null default true
);
create table if not exists fiscal.currencies (                -- Nota 13.1
  code shared.currency_code primary key, name text not null, is_active boolean not null default true
);

-- B12 · Versión y fuente de cada catálogo importado (cmd/import-catalogs).
create table if not exists fiscal.catalog_imports (
  id             uuid primary key default gen_random_uuid(),
  catalog        text not null,
  version_label  text not null,
  source_file    text not null,
  source_sha256  shared.sha256_hex not null,
  row_count      integer not null,
  imported_at    timestamptz not null default now(),
  constraint catalog_imports_row_count_ck check (row_count >= 0),
  constraint catalog_imports_version_uk unique (catalog, version_label, source_sha256)
);

-- +goose StatementBegin
do $$
declare
  t text;
begin
  foreach t in array array['reference_codes', 'reference_document_types', 'discount_codes',
                           'exoneration_institutions', 'currencies', 'catalog_imports']
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
grant select, insert, update, delete on fiscal.reference_codes, fiscal.reference_document_types,
  fiscal.discount_codes, fiscal.exoneration_institutions, fiscal.currencies to fiscal_app;
grant select, insert on fiscal.catalog_imports to fiscal_app;

-- B13 · Un certificado activo y una actividad principal por organización.
create unique index if not exists certificates_one_active_uk on fiscal.certificates (organization_id) where is_active;
create unique index if not exists organization_economic_activities_one_primary_uk
  on fiscal.organization_economic_activities (organization_id) where is_primary;

-- W3 · Fallos antes del envío (XML inválido contra el XSD, firma, certificado).
alter table fiscal.submission_attempts drop constraint if exists submission_attempts_request_type_ck;
alter table fiscal.submission_attempts add constraint submission_attempts_request_type_ck
  check (request_type in ('build', 'sign', 'send', 'status_query'));

-- W2 · El worker puede reprogramar y anotar el último error fuera de processing; lo fiscal sigue fijo.
-- +goose StatementBegin
create or replace function fiscal.electronic_documents_guard()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_mutable constant text[] := array['status', 'hacienda_status_message', 'accepted_at', 'rejected_at', 'updated_at',
                                     'next_attempt_at', 'last_error'];
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

-- +goose Down
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
alter table fiscal.submission_attempts drop constraint if exists submission_attempts_request_type_ck;
alter table fiscal.submission_attempts add constraint submission_attempts_request_type_ck
  check (request_type in ('send', 'status_query'));
drop index if exists fiscal.organization_economic_activities_one_primary_uk;
drop index if exists fiscal.certificates_one_active_uk;
drop table if exists fiscal.catalog_imports;
drop table if exists fiscal.currencies;
drop table if exists fiscal.exoneration_institutions;
drop table if exists fiscal.discount_codes;
drop table if exists fiscal.reference_document_types;
drop table if exists fiscal.reference_codes;
alter table fiscal.electronic_document_line_taxes
  drop column if exists exoneration_subsection,
  drop column if exists exoneration_article,
  drop column if exists exoneration_document_type_other,
  drop column if exists exoneration_institution_other,
  drop column if exists exoneration_institution_code;
alter table fiscal.electronic_document_lines drop column if exists discount_code, drop column if exists gross_amount;
drop table if exists fiscal.electronic_document_payment_methods;
drop index if exists fiscal.electronic_documents_queue_idx;
alter table fiscal.electronic_documents
  drop constraint if exists electronic_documents_note_reference_code_ck,
  drop constraint if exists electronic_documents_issuer_district_fk,
  drop column if exists last_error,
  drop column if exists next_attempt_at,
  drop column if exists reference_code,
  drop column if exists receiver_economic_activity_code,
  drop column if exists receiver_address,
  drop column if exists receiver_district_code,
  drop column if exists receiver_canton_code,
  drop column if exists receiver_province_code,
  drop column if exists receiver_phone,
  drop column if exists issuer_email,
  drop column if exists issuer_phone,
  drop column if exists issuer_address_details,
  drop column if exists issuer_neighborhood,
  drop column if exists issuer_district_code,
  drop column if exists issuer_canton_code,
  drop column if exists issuer_province_code,
  drop column if exists issuer_trade_name,
  drop column if exists generated_at,
  drop column if exists source_document_number,
  drop constraint if exists electronic_documents_source_document_uk;
alter table fiscal.electronic_documents drop constraint if exists electronic_documents_numeric_key_ck;
alter table fiscal.electronic_documents
  add constraint electronic_documents_numeric_key_ck check (numeric_key ~ '^[0-9]{50}$');
