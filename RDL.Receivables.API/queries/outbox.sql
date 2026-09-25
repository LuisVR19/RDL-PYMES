-- source_service fijo: la política outbox_messages_service exige que coincida con shared.current_service().
-- payload como texto (ADR 0004 de Platform).
-- name: InsertOutboxMessage :exec
insert into integration.outbox_messages (
  id, source_service, organization_id, event_type, event_version, aggregate_type, aggregate_id, correlation_id,
  payload, occurred_at
) values (
  sqlc.arg(id), 'receivables', sqlc.arg(organization_id), sqlc.arg(event_type), sqlc.arg(event_version),
  sqlc.arg(aggregate_type), sqlc.arg(aggregate_id), sqlc.arg(correlation_id), sqlc.arg(payload)::text::jsonb,
  sqlc.arg(occurred_at)
);
