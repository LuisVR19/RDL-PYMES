-- Evento en la misma transacción que la entidad (convenciones §10). Política outbox_messages_service:
-- source_service = shared.current_service() = 'billing'. El payload viaja como texto (jsonb con QueryExecModeExec).
-- Sin RETURNING: la API solo escribe; publicar es trabajo del worker (P2).
-- name: InsertOutboxMessage :exec
insert into integration.outbox_messages (id, source_service, organization_id, event_type, event_version, aggregate_type,
                                         aggregate_id, correlation_id, payload, occurred_at)
values (sqlc.arg(id), sqlc.arg(source_service), sqlc.arg(organization_id), sqlc.arg(event_type), sqlc.arg(event_version),
        sqlc.arg(aggregate_type), sqlc.arg(aggregate_id), sqlc.arg(correlation_id), sqlc.arg(payload)::text::jsonb,
        sqlc.arg(occurred_at));
