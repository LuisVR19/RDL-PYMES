-- Migrate: receivables_guard deja de confiar solo en el GUC receivables.recalculating
-- (docs/decisiones/0001-estado-inicial-bd.md §4.2). Cualquier sesión puede fijar ese GUC con set_config, así que un
-- UPDATE directo de receivables_app podía escribir un saldo arbitrario. El recálculo legítimo siempre corre dentro de
-- un trigger (payment_applications_validate o receivable_adjustments_apply → recalculate_receivable → UPDATE), así
-- que el guard exige además pg_trigger_depth() > 1; un UPDATE directo de la app corre a profundidad 1.
-- Solo reemplaza la función: no cambia datos, tablas ni triggers.

-- +goose Up
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

  if (coalesce(pg_catalog.current_setting('receivables.recalculating', true), 'off') <> 'on'
      or pg_catalog.pg_trigger_depth() < 2)
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

-- +goose Down
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
