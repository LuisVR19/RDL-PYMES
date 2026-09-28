-- Baseline del schema receivables.
-- Reproduce database-platform 0005_receivables y la parte de receivables de 0007_grants, tal como estaban en dev el
-- 2026-09-24 (inspección de docs/decisiones/0001-estado-inicial-bd.md §2).
-- Es idempotente: sobre la base dev no cambia nada (y allí se registra con `migrate mark-baseline`, sin ejecutarla);
-- sobre una base vacía (local, tests) crea el schema.
-- Requiere que el bootstrap de database-platform (roles, schema receivables, shared.*) ya exista.

-- +goose Up
create table if not exists receivables.receivables (
  id                             uuid primary key default gen_random_uuid(),
  organization_id                uuid not null,
  source_invoice_id              uuid not null,
  source_event_id                uuid not null,
  customer_id                    uuid not null,
  customer_identification_number text not null,
  customer_legal_name            text not null,
  document_number                text not null,
  currency_code                  shared.currency_code not null,
  original_amount                shared.money_amount not null,
  balance_amount                 shared.money_amount not null,
  issued_on                      date not null,
  due_on                         date not null,
  sale_condition_code            text not null,
  status                         text not null default 'open',
  settled_at                     timestamptz,
  created_at                     timestamptz not null default now(),
  updated_at                     timestamptz not null default now(),
  constraint receivables_org_id_uk unique (organization_id, id),
  constraint receivables_source_event_uk unique (organization_id, source_event_id),
  constraint receivables_source_invoice_uk unique (organization_id, source_invoice_id),
  constraint receivables_status_ck check (status in ('open', 'partially_paid', 'paid', 'cancelled')),
  constraint receivables_original_ck check (original_amount > 0),
  constraint receivables_due_ck check (due_on >= issued_on)
);

create table if not exists receivables.receivable_adjustments (
  id                 uuid primary key default gen_random_uuid(),
  organization_id    uuid not null,
  receivable_id      uuid not null,
  adjustment_type    text not null,
  source_document_id uuid,
  source_event_id    uuid,
  amount             shared.money_amount not null,
  reason             text not null,
  created_by_user_id uuid,
  created_at         timestamptz not null default now(),
  constraint receivable_adjustments_org_id_uk unique (organization_id, id),
  constraint receivable_adjustments_event_uk unique (organization_id, source_event_id),
  constraint receivable_adjustments_source_uk unique (organization_id, adjustment_type, source_document_id),
  constraint receivable_adjustments_receivable_fk foreign key (organization_id, receivable_id)
    references receivables.receivables (organization_id, id),
  constraint receivable_adjustments_type_ck
    check (adjustment_type in ('credit_note', 'debit_note', 'cancellation', 'write_off')),
  constraint receivable_adjustments_source_ck check (adjustment_type = 'write_off' or source_document_id is not null),
  constraint receivable_adjustments_amount_ck check (amount > 0)
);

create table if not exists receivables.payments (
  id                  uuid primary key default gen_random_uuid(),
  organization_id     uuid not null,
  customer_id         uuid not null,
  received_on         date not null,
  amount              shared.money_amount not null,
  currency_code       shared.currency_code not null,
  exchange_rate       shared.exchange_rate not null default 1,
  payment_method_code text not null,
  reference           text,
  notes               text,
  status              text not null default 'posted',
  void_reason         text,
  voided_at           timestamptz,
  received_by_user_id uuid,
  created_at          timestamptz not null default now(),
  updated_at          timestamptz not null default now(),
  constraint payments_org_id_uk unique (organization_id, id),
  constraint payments_amount_ck check (amount > 0),
  constraint payments_status_ck check (status in ('posted', 'voided')),
  constraint payments_voided_ck check ((status = 'voided') = (voided_at is not null and void_reason is not null))
);

create table if not exists receivables.payment_applications (
  id                  uuid primary key default gen_random_uuid(),
  organization_id     uuid not null,
  payment_id          uuid not null,
  receivable_id       uuid not null,
  amount              shared.money_amount not null,
  applied_at          timestamptz not null default now(),
  applied_by_user_id  uuid,
  reversed_at         timestamptz,
  reversed_by_user_id uuid,
  reversal_reason     text,
  constraint payment_applications_org_id_uk unique (organization_id, id),
  constraint payment_applications_payment_fk foreign key (organization_id, payment_id)
    references receivables.payments (organization_id, id),
  constraint payment_applications_receivable_fk foreign key (organization_id, receivable_id)
    references receivables.receivables (organization_id, id),
  constraint payment_applications_amount_ck check (amount > 0),
  constraint payment_applications_reversal_ck check ((reversed_at is null) = (reversal_reason is null))
);

create table if not exists receivables.collection_followups (
  id                   uuid primary key default gen_random_uuid(),
  organization_id      uuid not null,
  receivable_id        uuid not null,
  followup_type        text not null,
  notes                text not null,
  performed_at         timestamptz not null default now(),
  performed_by_user_id uuid,
  next_action_on       date,
  created_at           timestamptz not null default now(),
  constraint collection_followups_org_id_uk unique (organization_id, id),
  constraint collection_followups_receivable_fk foreign key (organization_id, receivable_id)
    references receivables.receivables (organization_id, id),
  constraint collection_followups_type_ck check (followup_type in ('call', 'email', 'visit', 'message', 'note'))
);

create table if not exists receivables.payment_promises (
  id                 uuid primary key default gen_random_uuid(),
  organization_id    uuid not null,
  receivable_id      uuid not null,
  followup_id        uuid,
  promised_amount    shared.money_amount not null,
  promised_on        date not null,
  status             text not null default 'pending',
  created_by_user_id uuid,
  created_at         timestamptz not null default now(),
  updated_at         timestamptz not null default now(),
  constraint payment_promises_org_id_uk unique (organization_id, id),
  constraint payment_promises_receivable_fk foreign key (organization_id, receivable_id)
    references receivables.receivables (organization_id, id),
  constraint payment_promises_followup_fk foreign key (organization_id, followup_id)
    references receivables.collection_followups (organization_id, id),
  constraint payment_promises_amount_ck check (promised_amount > 0),
  constraint payment_promises_status_ck check (status in ('pending', 'kept', 'broken', 'cancelled'))
);

create index if not exists receivables_customer_idx on receivables.receivables (organization_id, customer_id);
create index if not exists receivables_open_due_idx on receivables.receivables (organization_id, due_on)
  where status in ('open', 'partially_paid');
create index if not exists receivable_adjustments_receivable_idx
  on receivables.receivable_adjustments (organization_id, receivable_id);
create index if not exists payments_customer_idx on receivables.payments (organization_id, customer_id, received_on desc);
create unique index if not exists payment_applications_active_uk
  on receivables.payment_applications (organization_id, payment_id, receivable_id) where reversed_at is null;
create index if not exists payment_applications_receivable_idx
  on receivables.payment_applications (organization_id, receivable_id);
create index if not exists collection_followups_receivable_idx
  on receivables.collection_followups (organization_id, receivable_id, performed_at desc);
create index if not exists collection_followups_next_action_idx
  on receivables.collection_followups (organization_id, next_action_on) where next_action_on is not null;
create index if not exists payment_promises_receivable_idx on receivables.payment_promises (organization_id, receivable_id);
create index if not exists payment_promises_followup_idx on receivables.payment_promises (organization_id, followup_id);
create index if not exists payment_promises_pending_idx on receivables.payment_promises (organization_id, promised_on)
  where status = 'pending';

-- +goose StatementBegin
create or replace function receivables.recalculate_receivable(p_organization_id uuid, p_receivable_id uuid)
 returns void
 language plpgsql
 set search_path to ''
as $function$
declare
  v_original  numeric(18, 5);
  v_debits    numeric(18, 5);
  v_credits   numeric(18, 5);
  v_applied   numeric(18, 5);
  v_cancelled boolean;
  v_balance   numeric(18, 5);
  v_status    text;
begin
  select r.original_amount into v_original
    from receivables.receivables r
   where r.organization_id = p_organization_id and r.id = p_receivable_id
     for update;
  if not found then
    return;
  end if;

  select coalesce(sum(a.amount) filter (where a.adjustment_type = 'debit_note'), 0),
         coalesce(sum(a.amount) filter (where a.adjustment_type <> 'debit_note'), 0),
         coalesce(bool_or(a.adjustment_type = 'cancellation'), false)
    into v_debits, v_credits, v_cancelled
    from receivables.receivable_adjustments a
   where a.organization_id = p_organization_id and a.receivable_id = p_receivable_id;

  select coalesce(sum(pa.amount), 0) into v_applied
    from receivables.payment_applications pa
   where pa.organization_id = p_organization_id
     and pa.receivable_id = p_receivable_id
     and pa.reversed_at is null;

  v_balance := v_original + v_debits - v_credits - v_applied;
  if v_balance < 0 then
    raise exception 'El saldo de la cuenta por cobrar % quedaría negativo (%)', p_receivable_id, v_balance
      using errcode = 'check_violation';
  end if;

  v_status := case
    when v_cancelled   then 'cancelled'
    when v_balance = 0 then 'paid'
    when v_applied > 0 then 'partially_paid'
    else 'open'
  end;

  perform pg_catalog.set_config('receivables.recalculating', 'on', true);
  update receivables.receivables r
     set balance_amount = v_balance,
         status         = v_status,
         settled_at     = case when v_balance = 0 then coalesce(r.settled_at, pg_catalog.now()) end
   where r.organization_id = p_organization_id and r.id = p_receivable_id;
  perform pg_catalog.set_config('receivables.recalculating', 'off', true);
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function receivables.receivables_guard()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
begin
  if tg_op = 'INSERT' then
    new.balance_amount := new.original_amount;
    new.status := 'open';
    new.settled_at := null;
    return new;
  end if;

  if coalesce(pg_catalog.current_setting('receivables.recalculating', true), 'off') <> 'on'
     and (new.balance_amount, new.status, new.settled_at)
         is distinct from (old.balance_amount, old.status, old.settled_at) then
    raise exception 'Saldo y estado de la cuenta % solo cambian por aplicaciones o ajustes', old.id
      using errcode = 'restrict_violation';
  end if;

  if (new.organization_id, new.source_invoice_id, new.source_event_id, new.customer_id,
      new.currency_code, new.original_amount, new.document_number, new.issued_on)
     is distinct from
     (old.organization_id, old.source_invoice_id, old.source_event_id, old.customer_id,
      old.currency_code, old.original_amount, old.document_number, old.issued_on) then
    raise exception 'Los datos de origen de la cuenta % son inmutables', old.id
      using errcode = 'restrict_violation';
  end if;

  return new;
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function receivables.payment_applications_validate()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_payment    receivables.payments%rowtype;
  v_receivable receivables.receivables%rowtype;
  v_applied    numeric(18, 5);
begin
  if tg_op = 'UPDATE' then
    if old.reversed_at is not null then
      raise exception 'La aplicación % ya fue revertida', old.id using errcode = 'restrict_violation';
    end if;
    if (new.organization_id, new.payment_id, new.receivable_id, new.amount, new.applied_at)
       is distinct from (old.organization_id, old.payment_id, old.receivable_id, old.amount, old.applied_at) then
      raise exception 'Una aplicación no se modifica: revierta y cree una nueva' using errcode = 'restrict_violation';
    end if;
  end if;

  select * into v_payment
    from receivables.payments p
   where p.organization_id = new.organization_id and p.id = new.payment_id
     for update;

  if new.reversed_at is null then
    select * into v_receivable
      from receivables.receivables r
     where r.organization_id = new.organization_id and r.id = new.receivable_id;

    if v_payment.status <> 'posted' then
      raise exception 'El pago % está anulado', v_payment.id using errcode = 'check_violation';
    end if;
    if v_receivable.status = 'cancelled' then
      raise exception 'La cuenta % está anulada', v_receivable.id using errcode = 'check_violation';
    end if;
    if v_payment.customer_id <> v_receivable.customer_id then
      raise exception 'El pago y la cuenta pertenecen a clientes distintos' using errcode = 'check_violation';
    end if;
    if v_payment.currency_code <> v_receivable.currency_code then
      raise exception 'El pago (%) y la cuenta (%) están en monedas distintas',
        v_payment.currency_code, v_receivable.currency_code using errcode = 'check_violation';
    end if;

    select coalesce(sum(pa.amount), 0) into v_applied
      from receivables.payment_applications pa
     where pa.organization_id = new.organization_id
       and pa.payment_id = new.payment_id
       and pa.reversed_at is null;
    if v_applied > v_payment.amount then
      raise exception 'Las aplicaciones (%) superan el monto del pago % (%)', v_applied, v_payment.id, v_payment.amount
        using errcode = 'check_violation';
    end if;
  end if;

  perform receivables.recalculate_receivable(new.organization_id, new.receivable_id);
  return null;
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function receivables.receivable_adjustments_apply()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
begin
  perform receivables.recalculate_receivable(new.organization_id, new.receivable_id);
  return null;
end;
$function$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function receivables.payments_guard()
 returns trigger
 language plpgsql
 set search_path to ''
as $function$
declare
  v_has_applications boolean;
  v_has_active       boolean;
begin
  select count(*) > 0, count(*) filter (where pa.reversed_at is null) > 0
    into v_has_applications, v_has_active
    from receivables.payment_applications pa
   where pa.organization_id = old.organization_id and pa.payment_id = old.id;

  if v_has_applications
     and (new.amount, new.customer_id, new.currency_code, new.organization_id)
         is distinct from (old.amount, old.customer_id, old.currency_code, old.organization_id) then
    raise exception 'El pago % tiene aplicaciones: monto, cliente y moneda son inmutables', old.id
      using errcode = 'restrict_violation';
  end if;

  if old.status = 'voided' and new.status <> 'voided' then
    raise exception 'El pago % está anulado', old.id using errcode = 'restrict_violation';
  end if;

  if new.status = 'voided' and old.status = 'posted' and v_has_active then
    raise exception 'Revierta las aplicaciones del pago % antes de anularlo', old.id
      using errcode = 'restrict_violation';
  end if;

  return new;
end;
$function$;
-- +goose StatementEnd

create or replace trigger receivables_guard before insert or update on receivables.receivables
  for each row execute function receivables.receivables_guard();
create or replace trigger receivables_set_updated_at before update on receivables.receivables
  for each row execute function shared.set_updated_at();
create or replace trigger payment_applications_validate after insert or update on receivables.payment_applications
  for each row execute function receivables.payment_applications_validate();
create or replace trigger receivable_adjustments_apply after insert on receivables.receivable_adjustments
  for each row execute function receivables.receivable_adjustments_apply();
create or replace trigger payments_guard before update on receivables.payments
  for each row execute function receivables.payments_guard();
create or replace trigger payments_set_updated_at before update on receivables.payments
  for each row execute function shared.set_updated_at();
create or replace trigger payment_promises_set_updated_at before update on receivables.payment_promises
  for each row execute function shared.set_updated_at();

create or replace view receivables.receivable_aging with (security_invoker = true) as
select organization_id,
       id as receivable_id,
       customer_id,
       customer_legal_name,
       document_number,
       currency_code,
       original_amount,
       balance_amount,
       issued_on,
       due_on,
       greatest(current_date - due_on, 0) as days_overdue,
       case
         when due_on >= current_date then 'current'
         when (current_date - due_on) <= 30 then '1_30'
         when (current_date - due_on) <= 60 then '31_60'
         when (current_date - due_on) <= 90 then '61_90'
         else '90_plus'
       end as aging_bucket
  from receivables.receivables r
 where status in ('open', 'partially_paid');

alter table receivables.receivables            enable row level security;
alter table receivables.receivable_adjustments enable row level security;
alter table receivables.payments               enable row level security;
alter table receivables.payment_applications   enable row level security;
alter table receivables.collection_followups   enable row level security;
alter table receivables.payment_promises       enable row level security;

-- +goose StatementBegin
do $$
declare
  t text;
begin
  foreach t in array array['receivables', 'receivable_adjustments', 'payments', 'payment_applications',
                           'collection_followups', 'payment_promises']
  loop
    if not exists (
      select 1 from pg_catalog.pg_policies
      where schemaname = 'receivables' and tablename = t and policyname = t || '_tenant'
    ) then
      execute format(
        'create policy %I on receivables.%I for all
           using (organization_id = (select shared.current_organization_id()))
           with check (organization_id = (select shared.current_organization_id()))',
        t || '_tenant', t);
    end if;
  end loop;
end $$;
-- +goose StatementEnd

-- Privilegios de receivables_app tal como los dejó 0007_grants. Los ajustes propuestos (docs/decisiones/0002 §3)
-- los aplica database-platform; esta baseline no los adelanta.
grant usage on schema receivables to receivables_app;
grant select, insert, update on receivables.receivables, receivables.payments, receivables.payment_applications
  to receivables_app;
grant select, insert on receivables.receivable_adjustments to receivables_app;
grant select, insert, update, delete on receivables.collection_followups, receivables.payment_promises,
  receivables.receivable_aging to receivables_app;

-- +goose Down
-- La baseline nunca se revierte: el schema preexiste al repo (lo creó database-platform).
select 1;
