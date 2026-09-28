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

-- name: FindReceivableByInvoice :one
select id, customer_id, currency_code::text as currency_code, original_amount::text as original_amount
from receivables.receivables
where organization_id = sqlc.arg(organization_id) and source_invoice_id = sqlc.arg(source_invoice_id);

-- balance_amount y status no se envían: receivables_guard fija saldo = original y status = open en el alta.
-- original_amount viaja como texto y se convierte en la base: nunca pasa por float.
-- name: InsertReceivable :exec
insert into receivables.receivables (
  id, organization_id, source_invoice_id, source_event_id, customer_id, customer_identification_number,
  customer_legal_name, document_number, currency_code, original_amount, balance_amount, issued_on, due_on,
  sale_condition_code
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(source_invoice_id), sqlc.arg(source_event_id),
  sqlc.arg(customer_id), sqlc.arg(customer_identification_number), sqlc.arg(customer_legal_name),
  sqlc.arg(document_number), sqlc.arg(currency_code)::text, sqlc.arg(original_amount)::text::numeric,
  sqlc.arg(original_amount)::text::numeric, sqlc.arg(issued_on), sqlc.arg(due_on), sqlc.arg(sale_condition_code)
);

-- name: GetReceivable :one
select id, source_invoice_id, customer_id, customer_legal_name, document_number,
       currency_code::text as currency_code,
       trim_scale(original_amount)::text as original_amount,
       trim_scale(balance_amount)::text as balance_amount,
       issued_on, due_on, status, settled_at, created_at
from receivables.receivables
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);

-- name: GetReceivableByInvoice :one
select id, source_invoice_id, customer_id, customer_legal_name, document_number,
       currency_code::text as currency_code,
       trim_scale(original_amount)::text as original_amount,
       trim_scale(balance_amount)::text as balance_amount,
       issued_on, due_on, status, settled_at, created_at
from receivables.receivables
where organization_id = sqlc.arg(organization_id) and source_invoice_id = sqlc.arg(source_invoice_id);

-- name: ListAdjustmentViews :many
select id, adjustment_type, trim_scale(amount)::text as amount, source_document_id, reason, created_at
from receivables.receivable_adjustments
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by created_at, id;

-- name: ListApplicationViewsForReceivable :many
select id, payment_id, receivable_id, trim_scale(amount)::text as amount, applied_at, reversed_at, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by applied_at, id;

-- Aging: saldo cobrable agrupado por moneda y vencimiento; el tramo lo decide internal/domain/aging con asOf
-- (fecha de negocio de la organización). No usa la vista receivable_aging, que calcula con CURRENT_DATE en UTC.
-- name: AgingByDueDate :many
select currency_code::text as currency_code, due_on, sum(balance_amount)::text as balance
from receivables.receivables
where organization_id = sqlc.arg(organization_id)
  and status in ('open', 'partially_paid')
  and (sqlc.narg(currency)::text is null or currency_code = sqlc.narg(currency)::text)
group by currency_code, due_on
order by currency_code, due_on;
