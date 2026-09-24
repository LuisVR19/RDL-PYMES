-- Sesión con app.current_organization_id = organización activa (política document_sequences_tenant).

-- name: ListSequences :many
select id, organization_id, document_type, branch_id, prefix, next_number, last_assigned_number, updated_at
from billing.document_sequences
where organization_id = $1
order by document_type, branch_id nulls first;

-- Serializa las operaciones sobre las secuencias de un tipo de documento en la organización: configurar (el prefijo
-- no se repite) y asignar números. Es de transacción: se libera al confirmar o revertir.
-- name: LockSequences :exec
select pg_advisory_xact_lock(hashtextextended('billing.document_sequences:' || sqlc.arg(organization_id)::text
                                              || ':' || sqlc.arg(document_type)::text, 0));

-- name: GetSequenceForUpdate :one
select id, organization_id, document_type, branch_id, prefix, next_number, last_assigned_number, updated_at
from billing.document_sequences
where organization_id = sqlc.arg(organization_id) and document_type = sqlc.arg(document_type)
  and branch_id is not distinct from sqlc.narg(branch_id)::uuid
for update;

-- name: InsertSequence :one
insert into billing.document_sequences (organization_id, document_type, branch_id, prefix, next_number)
values (sqlc.arg(organization_id), sqlc.arg(document_type), sqlc.narg(branch_id), sqlc.arg(prefix), sqlc.arg(next_number))
returning id, organization_id, document_type, branch_id, prefix, next_number, last_assigned_number, updated_at;

-- name: UpdateSequence :one
update billing.document_sequences
set prefix = sqlc.arg(prefix), next_number = sqlc.arg(next_number), last_assigned_number = sqlc.narg(last_assigned_number)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
returning id, organization_id, document_type, branch_id, prefix, next_number, last_assigned_number, updated_at;
