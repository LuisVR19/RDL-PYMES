-- Lectura de pagos para la API. Filtro explícito por organización además de RLS.

-- name: GetPayment :one
select id, customer_id, received_on, trim_scale(amount)::text as amount, currency_code::text as currency_code,
       trim_scale(exchange_rate)::text as exchange_rate, payment_method_code, reference, notes, status, void_reason,
       voided_at, received_by_user_id, created_at
from receivables.payments
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);

-- Paginación por cursor sobre (created_at, id) descendente: índice payments_created_idx (00002).
-- name: ListPayments :many
select id, customer_id, received_on, trim_scale(amount)::text as amount, currency_code::text as currency_code,
       trim_scale(exchange_rate)::text as exchange_rate, payment_method_code, reference, notes, status, void_reason,
       voided_at, received_by_user_id, created_at
from receivables.payments
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(customer_id)::uuid is null or customer_id = sqlc.narg(customer_id)::uuid)
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at desc, id desc
limit sqlc.arg(page_size);

-- Los ids viajan como text[]: con QueryExecModeExec (Supavisor) pgx no codifica un []uuid.UUID sin tipo.
-- name: ListApplicationsForPayments :many
select id, payment_id, receivable_id, trim_scale(amount)::text as amount, applied_at, reversed_at, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and payment_id = any(sqlc.arg(payment_ids)::text[]::uuid[])
order by applied_at, id;

-- name: GetApplicationView :one
select id, payment_id, receivable_id, trim_scale(amount)::text as amount, applied_at, reversed_at, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);
