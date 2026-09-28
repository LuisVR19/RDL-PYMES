-- Seguimientos y promesas de pago. Son historial de cobranza: se crean y una promesa cambia de estado; no se borran.

-- name: InsertFollowUp :exec
insert into receivables.collection_followups (
  id, organization_id, receivable_id, followup_type, notes, performed_at, performed_by_user_id, next_action_on
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(receivable_id), sqlc.arg(followup_type), sqlc.arg(notes),
  sqlc.arg(performed_at), sqlc.narg(performed_by_user_id), sqlc.narg(next_action_on)
);

-- name: GetFollowUp :one
select id, receivable_id, followup_type, notes, performed_at, performed_by_user_id, next_action_on, created_at
from receivables.collection_followups
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);

-- name: ListFollowUps :many
select id, receivable_id, followup_type, notes, performed_at, performed_by_user_id, next_action_on, created_at
from receivables.collection_followups
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by performed_at desc, id desc
limit 200;

-- name: InsertPromise :exec
insert into receivables.payment_promises (
  id, organization_id, receivable_id, followup_id, promised_amount, promised_on, created_by_user_id
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(receivable_id), sqlc.narg(followup_id),
  sqlc.arg(promised_amount)::text::numeric, sqlc.arg(promised_on), sqlc.narg(created_by_user_id)
);

-- name: GetPromise :one
select id, receivable_id, followup_id, trim_scale(promised_amount)::text as promised_amount, promised_on, status,
       created_by_user_id, created_at, updated_at
from receivables.payment_promises
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);

-- name: LockPromise :one
select id, receivable_id, followup_id, trim_scale(promised_amount)::text as promised_amount, promised_on, status,
       created_by_user_id, created_at, updated_at
from receivables.payment_promises
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
for update;

-- name: ListPromises :many
select id, receivable_id, followup_id, trim_scale(promised_amount)::text as promised_amount, promised_on, status,
       created_by_user_id, created_at, updated_at
from receivables.payment_promises
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by promised_on desc, id desc
limit 200;

-- name: UpdatePromiseStatus :execrows
update receivables.payment_promises
set status = sqlc.arg(status)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and status = 'pending';
