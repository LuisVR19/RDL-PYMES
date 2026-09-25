-- Sesión con app.current_organization_id (políticas *_tenant). El filtro por organización es explícito además
-- de RLS: la consulta no depende solo de la política para quedar acotada al tenant.

-- Paginación por cursor sobre (created_at, id) descendente: índices receivables_created_idx y
-- receivables_status_created_idx (migración 00002). trim_scale quita los ceros de numeric(18,5): "1300.5", no
-- "1300.50000" (mismo valor, el formato Money del contrato admite ambos).
-- Vencida = todavía cobrable (open, partially_paid) y due_on anterior al día de negocio de la organización.
-- name: ListReceivables :many
select id, source_invoice_id, customer_id, customer_legal_name, document_number,
       currency_code::text as currency_code,
       trim_scale(original_amount)::text as original_amount,
       trim_scale(balance_amount)::text as balance_amount,
       issued_on, due_on, status, settled_at, created_at
from receivables.receivables
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(status)::text is null or status = sqlc.narg(status)::text)
  and (sqlc.narg(customer_id)::uuid is null or customer_id = sqlc.narg(customer_id)::uuid)
  and (sqlc.narg(overdue)::boolean is null
       or (status in ('open', 'partially_paid') and due_on < sqlc.narg(today)::date) = sqlc.narg(overdue)::boolean)
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at desc, id desc
limit sqlc.arg(page_size);
