-- payload viaja como texto: con QueryExecModeExec pgx enviaría un []byte como bytea, no como JSON (ADR 0004).
-- Sin RETURNING: la política de SELECT de audit_events exige organización y un evento global no la tiene.
-- name: InsertAuditEvent :exec
insert into audit.audit_events (
  organization_id, service, actor_type, actor_user_id, action, entity_type, entity_id,
  correlation_id, ip_address, user_agent, payload
) values (
  sqlc.narg(organization_id), 'platform', sqlc.arg(actor_type), sqlc.narg(actor_user_id), sqlc.arg(action),
  sqlc.arg(entity_type), sqlc.narg(entity_id), sqlc.arg(correlation_id), sqlc.narg(ip_address)::inet,
  sqlc.narg(user_agent), sqlc.arg(payload)::text::jsonb
);
