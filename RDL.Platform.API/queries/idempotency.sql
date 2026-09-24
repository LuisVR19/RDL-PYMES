-- Sesión con app.current_organization_id = organización del comando (política idempotency_keys_service).

-- name: DeleteExpiredIdempotencyKey :exec
delete from integration.idempotency_keys
where service = 'platform' and organization_id = $1 and idempotency_key = $2 and expires_at < now();

-- Si otra transacción ya reservó la clave, este insert espera a que confirme y luego no devuelve fila.
-- name: ClaimIdempotencyKey :one
insert into integration.idempotency_keys (service, organization_id, idempotency_key, request_hash, expires_at)
values ('platform', $1, $2, sqlc.arg(request_hash)::text, now() + make_interval(secs => sqlc.arg(ttl_seconds)::bigint))
on conflict (service, organization_id, idempotency_key) do nothing
returning request_hash::text;

-- name: GetIdempotencyKey :one
select request_hash::text as request_hash, response_status, response_body
from integration.idempotency_keys
where service = 'platform' and organization_id = $1 and idempotency_key = $2;

-- name: CompleteIdempotencyKey :exec
update integration.idempotency_keys
set response_status = sqlc.arg(response_status), response_body = sqlc.arg(response_body)::text::jsonb
where service = 'platform' and organization_id = sqlc.arg(organization_id) and idempotency_key = sqlc.arg(idempotency_key);
