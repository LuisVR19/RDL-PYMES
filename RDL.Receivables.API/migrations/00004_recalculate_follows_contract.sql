-- Migrate: recalculate_receivable deriva el estado como state-machines/receivable.yaml (decisión R1, opción A;
-- docs/decisiones/0001-estado-inicial-bd.md §4.1). Antes: partially_paid si había aplicaciones vigentes. Ahora, igual
-- que receivable.Derive en Go (si cambia una, cambia la otra):
--   cancelled si hay ajuste de anulación; paid si saldo = 0; open si saldo = original + débitos; si no, partially_paid.
-- El saldo no cambia de fórmula. Solo reemplaza la función; las tablas no tienen filas en dev, así que no hace falta
-- recalcular cuentas existentes.

-- +goose Up
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
    when v_cancelled                        then 'cancelled'
    when v_balance = 0                      then 'paid'
    when v_balance = v_original + v_debits then 'open'
    else 'partially_paid'
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

-- +goose Down
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
