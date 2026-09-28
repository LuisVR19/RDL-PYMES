-- Consumo de eventos (ADR 0005). consumer_service fijo: la política inbox_messages_service exige que coincida con
-- shared.current_service() y el login de este servicio solo puede ser 'receivables'.

-- Reclama el evento. 0 filas = duplicado: otro intento ya lo procesó y confirmó.
-- name: ClaimInboxMessage :execrows
insert into integration.inbox_messages (consumer_service, event_id, event_type, organization_id)
values ('receivables', sqlc.arg(event_id), sqlc.arg(event_type), sqlc.arg(organization_id))
on conflict (consumer_service, event_id) do nothing;

-- name: MarkInboxProcessed :exec
update integration.inbox_messages
   set processed_at = now()
 where consumer_service = 'receivables' and event_id = sqlc.arg(event_id);

-- payload como texto (ADR 0004 de Platform: con QueryExecModeExec un []byte viajaría como bytea).
-- name: InsertDeadLetter :exec
insert into integration.dead_letters (
  consumer_service, event_id, event_type, organization_id, payload, error_message, attempts
) values (
  'receivables', sqlc.arg(event_id), sqlc.arg(event_type), sqlc.narg(organization_id),
  sqlc.arg(payload)::text::jsonb, sqlc.arg(error_message), sqlc.arg(attempts)
);
