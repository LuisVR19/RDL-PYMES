-- Carga y persistencia de los agregados (ADR 0006). Toda consulta filtra por organization_id además de RLS.
-- Los montos viajan como texto: nunca pasan por float. Saldo y estado nunca se escriben: los recalcula la base.

-- Bloquea la cuenta. El orden de bloqueo lo decide el caso de uso: pagos por id y después cuentas por id.
-- name: LockReceivable :one
select id, source_invoice_id, customer_id, customer_legal_name, document_number,
       currency_code::text as currency_code, original_amount::text as original_amount,
       balance_amount::text as balance_amount, issued_on, due_on, status, settled_at, created_at
from receivables.receivables
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
for update;

-- name: ListReceivableAdjustments :many
select id, adjustment_type, amount::text as amount, source_document_id, source_event_id, reason,
       created_by_user_id, created_at
from receivables.receivable_adjustments
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by created_at, id;

-- name: ListReceivableApplications :many
select id, payment_id, receivable_id, amount::text as amount, applied_at, applied_by_user_id,
       reversed_at, reversed_by_user_id, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id)
order by applied_at, id;

-- Pagos con aplicaciones vigentes en la cuenta, sin bloquear: sirve para bloquearlos antes que la cuenta.
-- name: ListActivePaymentIDsForReceivable :many
select distinct payment_id
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and receivable_id = sqlc.arg(receivable_id) and reversed_at is null
order by payment_id;

-- name: LockPayment :one
select id, customer_id, received_on, amount::text as amount, currency_code::text as currency_code,
       exchange_rate::text as exchange_rate, payment_method_code, reference, notes, status, void_reason, voided_at,
       received_by_user_id, created_at
from receivables.payments
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
for update;

-- name: ListPaymentApplications :many
select id, payment_id, receivable_id, amount::text as amount, applied_at, applied_by_user_id,
       reversed_at, reversed_by_user_id, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and payment_id = sqlc.arg(payment_id)
order by applied_at, id;

-- name: GetApplication :one
select id, payment_id, receivable_id, amount::text as amount, applied_at, applied_by_user_id,
       reversed_at, reversed_by_user_id, reversal_reason
from receivables.payment_applications
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);

-- name: InsertPayment :exec
insert into receivables.payments (
  id, organization_id, customer_id, received_on, amount, currency_code, exchange_rate, payment_method_code,
  reference, notes, received_by_user_id
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(customer_id), sqlc.arg(received_on),
  sqlc.arg(amount)::text::numeric, sqlc.arg(currency_code)::text, sqlc.arg(exchange_rate)::text::numeric,
  sqlc.arg(payment_method_code), sqlc.narg(reference), sqlc.narg(notes), sqlc.narg(received_by_user_id)
);

-- payment_applications_validate revisa cliente, moneda, pago anulado, cuenta anulada y suma aplicada, y recalcula.
-- name: InsertApplication :exec
insert into receivables.payment_applications (
  id, organization_id, payment_id, receivable_id, amount, applied_at, applied_by_user_id
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(payment_id), sqlc.arg(receivable_id),
  sqlc.arg(amount)::text::numeric, sqlc.arg(applied_at), sqlc.narg(applied_by_user_id)
);

-- Una aplicación solo se revierte una vez (reversed_at is null) y nunca se borra.
-- name: ReverseApplication :execrows
update receivables.payment_applications
set reversed_at = sqlc.arg(reversed_at), reversed_by_user_id = sqlc.narg(reversed_by_user_id),
    reversal_reason = sqlc.arg(reversal_reason)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and reversed_at is null;

-- payments_guard exige que no queden aplicaciones vigentes.
-- name: VoidPayment :execrows
update receivables.payments
set status = 'voided', void_reason = sqlc.arg(void_reason), voided_at = sqlc.arg(voided_at)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and status = 'posted';

-- receivable_adjustments_apply recalcula saldo y estado.
-- name: InsertAdjustment :exec
insert into receivables.receivable_adjustments (
  id, organization_id, receivable_id, adjustment_type, source_document_id, source_event_id, amount, reason,
  created_by_user_id
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(receivable_id), sqlc.arg(adjustment_type),
  sqlc.narg(source_document_id), sqlc.narg(source_event_id), sqlc.arg(amount)::text::numeric, sqlc.arg(reason),
  sqlc.narg(created_by_user_id)
);

-- Lo que dejaron los triggers, para compararlo con el agregado.
-- name: GetReceivableState :one
select balance_amount::text as balance_amount, status, settled_at
from receivables.receivables
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id);
