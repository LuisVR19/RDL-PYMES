-- service fijo: la política audit_events_insert exige service = shared.current_service().
-- payload viaja como texto (ADR 0004 de Platform). Sin RETURNING: no hace falta leer lo insertado.
-- name: InsertAuditEvent :exec
insert into audit.audit_events (
  organization_id, service, actor_type, actor_user_id, action, entity_type, entity_id,
  correlation_id, ip_address, user_agent, payload
) values (
  sqlc.narg(organization_id), 'receivables', sqlc.arg(actor_type), sqlc.narg(actor_user_id), sqlc.arg(action),
  sqlc.arg(entity_type), sqlc.narg(entity_id), sqlc.arg(correlation_id), sqlc.narg(ip_address)::inet,
  sqlc.narg(user_agent), sqlc.arg(payload)::text::jsonb
);
