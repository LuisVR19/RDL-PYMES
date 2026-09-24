-- Baseline de billing.
-- Reproduce database-platform 0004_billing y la parte de billing de 0007_grants, tal como estaban en dev el
-- 2026-09-24 (inspección en docs/decisiones/0001-estado-inicial-bd.md §2).
-- Es idempotente: sobre la base dev existente no cambia nada; sobre una base vacía (tests) crea el esquema.
-- Requiere el bootstrap de database-platform (roles, schema billing, shared.*) y corre como billing_migrator.

-- +goose Up
create table if not exists billing.customers (
  id                       uuid primary key default gen_random_uuid(),
  organization_id          uuid not null,
  identification_type_code text not null,
  identification_number    text not null,
  legal_name               text not null,
  trade_name               text,
  email                    text,
  phone                    text,
  province_code            text,
  canton_code              text,
  district_code            text,
  address_details          text,
  is_active                boolean not null default true,
  created_by_user_id       uuid,
  created_at               timestamptz not null default now(),
  updated_at               timestamptz not null default now(),
  constraint customers_org_id_uk unique (organization_id, id),
  constraint customers_identification_uk unique (organization_id, identification_type_code, identification_number)
);
create index if not exists customers_legal_name_idx on billing.customers (organization_id, lower(legal_name));

create table if not exists billing.products (
  id                   uuid primary key default gen_random_uuid(),
  organization_id      uuid not null,
  code                 text not null,
  description          text not null,
  cabys_code           shared.cabys_code not null,
  unit_of_measure_code text not null,
  unit_price           shared.money_amount not null,
  currency_code        shared.currency_code not null default 'CRC',
  is_service           boolean not null default false,
  is_active            boolean not null default true,
  created_at           timestamptz not null default now(),
  updated_at           timestamptz not null default now(),
  constraint products_org_id_uk unique (organization_id, id),
  constraint products_org_code_uk unique (organization_id, code)
);
create index if not exists products_cabys_idx on billing.products (organization_id, cabys_code);

create table if not exists billing.product_taxes (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null,
  product_id      uuid not null,
  tax_type_code   text not null,
  tax_rate_code   text not null,
  constraint product_taxes_org_id_uk unique (organization_id, id),
  constraint product_taxes_product_tax_uk unique (organization_id, product_id, tax_type_code),
  constraint product_taxes_product_fk foreign key (organization_id, product_id)
    references billing.products (organization_id, id) on delete cascade
);

create table if not exists billing.invoices (
  id                                uuid primary key default gen_random_uuid(),
  organization_id                   uuid not null,
  document_type                     text not null default 'invoice',
  number                            text,
  status                            text not null default 'draft',
  branch_id                         uuid,
  customer_id                       uuid not null,
  customer_identification_type_code text,
  customer_identification_number    text,
  customer_legal_name               text,
  customer_email                    text,
  customer_phone                    text,
  customer_address                  text,
  issued_at                         timestamptz,
  due_date                          date,
  sale_condition_code               text not null,
  credit_term_days                  integer,
  currency_code                     shared.currency_code not null default 'CRC',
  exchange_rate                     shared.exchange_rate not null default 1,
  subtotal_amount                   shared.money_amount not null default 0,
  discount_amount                   shared.money_amount not null default 0,
  tax_amount                        shared.money_amount not null default 0,
  exoneration_amount                shared.money_amount not null default 0,
  total_amount                      shared.money_amount not null default 0,
  notes                             text,
  referenced_invoice_id             uuid,
  reference_reason                  text,
  requires_correction               boolean not null default false,
  fiscal_rejection_reason           text,
  cancellation_reason               text,
  cancelled_at                      timestamptz,
  cancelled_by_user_id              uuid,
  created_by_user_id                uuid not null,
  issued_by_user_id                 uuid,
  created_at                        timestamptz not null default now(),
  updated_at                        timestamptz not null default now(),
  constraint invoices_org_id_uk unique (organization_id, id),
  constraint invoices_number_uk unique (organization_id, document_type, number),
  constraint invoices_document_type_ck check (document_type in ('invoice', 'credit_note', 'debit_note')),
  constraint invoices_status_ck check (status in ('draft', 'issued', 'cancelled')),
  constraint invoices_credit_term_ck check (credit_term_days is null or credit_term_days >= 0),
  constraint invoices_cancelled_ck check ((status = 'cancelled') = (cancelled_at is not null and cancellation_reason is not null)),
  constraint invoices_issued_ck check (
    status = 'draft'
    or (number is not null and issued_at is not null and issued_by_user_id is not null
        and customer_identification_type_code is not null and customer_identification_number is not null
        and customer_legal_name is not null)),
  constraint invoices_reference_ck check (
    ((document_type = 'invoice') = (referenced_invoice_id is null))
    and (referenced_invoice_id is null or reference_reason is not null)),
  constraint invoices_customer_fk foreign key (organization_id, customer_id)
    references billing.customers (organization_id, id),
  constraint invoices_reference_fk foreign key (organization_id, referenced_invoice_id)
    references billing.invoices (organization_id, id)
);
create index if not exists invoices_status_idx     on billing.invoices (organization_id, status, issued_at desc);
create index if not exists invoices_customer_idx   on billing.invoices (organization_id, customer_id);
create index if not exists invoices_reference_idx  on billing.invoices (organization_id, referenced_invoice_id);
create index if not exists invoices_correction_idx on billing.invoices (organization_id) where requires_correction;

create table if not exists billing.invoice_lines (
  id                   uuid primary key default gen_random_uuid(),
  organization_id      uuid not null,
  invoice_id           uuid not null,
  line_number          integer not null,
  product_id           uuid,
  product_code         text,
  cabys_code           shared.cabys_code not null,
  description          text not null,
  unit_of_measure_code text not null,
  is_service           boolean not null default false,
  quantity             shared.quantity not null,
  unit_price           shared.money_amount not null,
  discount_amount      shared.money_amount not null default 0,
  discount_reason      text,
  subtotal_amount      shared.money_amount not null,
  tax_amount           shared.money_amount not null default 0,
  total_amount         shared.money_amount not null,
  constraint invoice_lines_org_id_uk unique (organization_id, id),
  constraint invoice_lines_number_uk unique (organization_id, invoice_id, line_number),
  constraint invoice_lines_number_ck check (line_number > 0),
  constraint invoice_lines_discount_ck check (discount_amount::numeric = 0::numeric or discount_reason is not null),
  constraint invoice_lines_invoice_fk foreign key (organization_id, invoice_id)
    references billing.invoices (organization_id, id) on delete cascade,
  constraint invoice_lines_product_fk foreign key (organization_id, product_id)
    references billing.products (organization_id, id)
);
create index if not exists invoice_lines_product_idx on billing.invoice_lines (organization_id, product_id);

create table if not exists billing.invoice_line_taxes (
  id                             uuid primary key default gen_random_uuid(),
  organization_id                uuid not null,
  invoice_line_id                uuid not null,
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
  constraint invoice_line_taxes_org_id_uk unique (organization_id, id),
  constraint invoice_line_taxes_line_tax_uk unique (organization_id, invoice_line_id, tax_type_code),
  constraint invoice_line_taxes_exoneration_ck check (
    num_nulls(exoneration_document_type_code, exoneration_document_number, exoneration_institution,
              exoneration_issued_at, exoneration_percentage, exoneration_amount) in (0, 6)),
  constraint invoice_line_taxes_line_fk foreign key (organization_id, invoice_line_id)
    references billing.invoice_lines (organization_id, id) on delete cascade
);

create table if not exists billing.invoice_payment_methods (
  id                  uuid primary key default gen_random_uuid(),
  organization_id     uuid not null,
  invoice_id          uuid not null,
  payment_method_code text not null,
  amount              shared.money_amount,
  constraint invoice_payment_methods_org_id_uk unique (organization_id, id),
  constraint invoice_payment_methods_method_uk unique (organization_id, invoice_id, payment_method_code),
  constraint invoice_payment_methods_invoice_fk foreign key (organization_id, invoice_id)
    references billing.invoices (organization_id, id) on delete cascade
);

create table if not exists billing.invoice_status_history (
  id                 uuid primary key default gen_random_uuid(),
  organization_id    uuid not null,
  invoice_id         uuid not null,
  from_status        text,
  to_status          text not null,
  reason             text,
  changed_by_user_id uuid,
  changed_at         timestamptz not null default now(),
  constraint invoice_status_history_org_id_uk unique (organization_id, id),
  constraint invoice_status_history_invoice_fk foreign key (organization_id, invoice_id)
    references billing.invoices (organization_id, id)
);
create index if not exists invoice_status_history_invoice_idx
  on billing.invoice_status_history (organization_id, invoice_id, changed_at);

create table if not exists billing.document_sequences (
  id              uuid primary key default gen_random_uuid(),
  organization_id uuid not null,
  document_type   text not null,
  branch_id       uuid,
  prefix          text not null default '',
  next_number     bigint not null default 1,
  updated_at      timestamptz not null default now(),
  constraint document_sequences_org_id_uk unique (organization_id, id),
  constraint document_sequences_scope_uk unique nulls not distinct (organization_id, document_type, branch_id),
  constraint document_sequences_type_ck check (document_type in ('invoice', 'credit_note', 'debit_note')),
  constraint document_sequences_next_number_ck check (next_number > 0)
);

-- Inmutabilidad de lo emitido (arquitectura 7.4). La base lo garantiza aunque la app tenga un error.
-- +goose StatementBegin
create or replace function billing.invoices_guard()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_mutable constant text[] := array['status', 'requires_correction', 'fiscal_rejection_reason',
                                     'cancellation_reason', 'cancelled_at', 'cancelled_by_user_id', 'updated_at'];
begin
  if tg_op = 'DELETE' then
    if old.status <> 'draft' then
      raise exception 'El documento % ya fue emitido y no puede eliminarse', old.id using errcode = 'restrict_violation';
    end if;
    return old;
  end if;

  if old.status = 'draft' then
    if new.status not in ('draft', 'issued') then
      raise exception 'Transición inválida: % -> %', old.status, new.status using errcode = 'check_violation';
    end if;
    return new;
  end if;

  if (old.status, new.status) not in (('issued', 'issued'), ('issued', 'cancelled'), ('cancelled', 'cancelled')) then
    raise exception 'Transición inválida: % -> %', old.status, new.status using errcode = 'check_violation';
  end if;

  if (to_jsonb(new) - v_mutable) is distinct from (to_jsonb(old) - v_mutable) then
    raise exception 'El documento % está emitido y es inmutable; use notas de crédito/débito o anulación', old.id
      using errcode = 'restrict_violation';
  end if;

  if old.status = 'cancelled'
     and (new.cancellation_reason, new.cancelled_at, new.cancelled_by_user_id)
         is distinct from (old.cancellation_reason, old.cancelled_at, old.cancelled_by_user_id) then
    raise exception 'La anulación del documento % no puede modificarse', old.id using errcode = 'restrict_violation';
  end if;

  return new;
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function billing.guard_draft_children()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_rows    jsonb[] := '{}';
  v_row     jsonb;
  v_org     uuid;
  v_invoice uuid;
  v_status  text;
begin
  if tg_op in ('UPDATE', 'DELETE') then v_rows := v_rows || to_jsonb(old); end if;
  if tg_op in ('INSERT', 'UPDATE') then v_rows := v_rows || to_jsonb(new); end if;

  foreach v_row in array v_rows loop
    v_org := (v_row ->> 'organization_id')::uuid;
    if tg_table_name = 'invoice_line_taxes' then
      select l.invoice_id into v_invoice
        from billing.invoice_lines l
       where l.organization_id = v_org and l.id = (v_row ->> 'invoice_line_id')::uuid;
    else
      v_invoice := (v_row ->> 'invoice_id')::uuid;
    end if;

    select i.status into v_status
      from billing.invoices i
     where i.organization_id = v_org and i.id = v_invoice;

    if v_status is not null and v_status <> 'draft' then
      raise exception 'El documento % no está en borrador; su detalle es inmutable', v_invoice
        using errcode = 'restrict_violation';
    end if;
  end loop;

  if tg_op = 'DELETE' then return old; end if;
  return new;
end;
$function$;
-- +goose StatementEnd

create or replace trigger customers_set_updated_at          before update on billing.customers          for each row execute function shared.set_updated_at();
create or replace trigger products_set_updated_at           before update on billing.products           for each row execute function shared.set_updated_at();
create or replace trigger invoices_set_updated_at           before update on billing.invoices           for each row execute function shared.set_updated_at();
create or replace trigger document_sequences_set_updated_at before update on billing.document_sequences for each row execute function shared.set_updated_at();

create or replace trigger invoices_guard                before update or delete          on billing.invoices                for each row execute function billing.invoices_guard();
create or replace trigger invoice_lines_guard           before insert or update or delete on billing.invoice_lines           for each row execute function billing.guard_draft_children();
create or replace trigger invoice_line_taxes_guard      before insert or update or delete on billing.invoice_line_taxes      for each row execute function billing.guard_draft_children();
create or replace trigger invoice_payment_methods_guard before insert or update or delete on billing.invoice_payment_methods for each row execute function billing.guard_draft_children();

alter table billing.customers               enable row level security;
alter table billing.products                enable row level security;
alter table billing.product_taxes           enable row level security;
alter table billing.invoices                enable row level security;
alter table billing.invoice_lines           enable row level security;
alter table billing.invoice_line_taxes      enable row level security;
alter table billing.invoice_payment_methods enable row level security;
alter table billing.invoice_status_history  enable row level security;
alter table billing.document_sequences      enable row level security;

-- Todas las tablas de billing tienen la misma política: solo la organización activa de la transacción.
-- +goose StatementBegin
do $$
declare
  t text;
begin
  foreach t in array array['customers', 'products', 'product_taxes', 'invoices', 'invoice_lines',
                           'invoice_line_taxes', 'invoice_payment_methods', 'invoice_status_history',
                           'document_sequences'] loop
    if not exists (
      select 1 from pg_catalog.pg_policies
      where schemaname = 'billing' and tablename = t and policyname = t || '_tenant'
    ) then
      execute format(
        'create policy %I on billing.%I for all
           using (organization_id = (select shared.current_organization_id()))
           with check (organization_id = (select shared.current_organization_id()))',
        t || '_tenant', t);
    end if;
  end loop;
end $$;
-- +goose StatementEnd

-- Privilegios de 0007_grants. El historial de estados es append-only para la app.
grant usage on schema billing to billing_app;
grant select, insert, update, delete on
  billing.customers, billing.products, billing.product_taxes, billing.invoices, billing.invoice_lines,
  billing.invoice_line_taxes, billing.invoice_payment_methods, billing.document_sequences
  to billing_app;
grant select, insert on billing.invoice_status_history to billing_app;
revoke update, delete on billing.invoice_status_history from billing_app;
grant execute on function billing.invoices_guard(), billing.guard_draft_children() to billing_app;
alter default privileges in schema billing grant select, insert, update, delete on tables to billing_app;
-- goose crea su historial antes de esta migración y los default privileges se lo abren a la app: se cierra.
revoke all on billing.goose_db_version from billing_app;

-- +goose Down
-- La baseline nunca se revierte: estas tablas preexisten al repo y otras APIs dependen de ellas.
select 1;
