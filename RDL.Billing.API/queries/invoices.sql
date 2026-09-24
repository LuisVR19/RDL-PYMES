-- Sesión con app.current_organization_id = organización activa (políticas *_tenant de billing). Los montos viajan
-- como texto en ambos sentidos. La inmutabilidad de lo emitido la garantizan además invoices_guard y
-- guard_draft_children.

-- name: InsertInvoice :one
insert into billing.invoices (organization_id, document_type, status, branch_id, customer_id, sale_condition_code,
                              credit_term_days, currency_code, exchange_rate, notes, created_by_user_id,
                              subtotal_amount, discount_amount, tax_amount, exoneration_amount, total_amount)
values (sqlc.arg(organization_id), sqlc.arg(document_type), 'draft', sqlc.narg(branch_id), sqlc.arg(customer_id),
        sqlc.arg(sale_condition_code), sqlc.narg(credit_term_days), sqlc.arg(currency_code)::text,
        sqlc.arg(exchange_rate)::text::numeric, nullif(sqlc.arg(notes)::text, ''), sqlc.arg(created_by_user_id),
        sqlc.arg(subtotal)::text::numeric, sqlc.arg(discount)::text::numeric, sqlc.arg(tax)::text::numeric,
        sqlc.arg(exoneration)::text::numeric, sqlc.arg(total)::text::numeric)
returning id, created_at, updated_at;

-- name: UpdateInvoiceDraft :one
update billing.invoices
set branch_id = sqlc.narg(branch_id), customer_id = sqlc.arg(customer_id),
    sale_condition_code = sqlc.arg(sale_condition_code), credit_term_days = sqlc.narg(credit_term_days),
    currency_code = sqlc.arg(currency_code)::text, exchange_rate = sqlc.arg(exchange_rate)::text::numeric,
    notes = nullif(sqlc.arg(notes)::text, ''),
    subtotal_amount = sqlc.arg(subtotal)::text::numeric, discount_amount = sqlc.arg(discount)::text::numeric,
    tax_amount = sqlc.arg(tax)::text::numeric, exoneration_amount = sqlc.arg(exoneration)::text::numeric,
    total_amount = sqlc.arg(total)::text::numeric
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and status = 'draft'
returning updated_at;

-- name: DeleteInvoiceDraft :execrows
delete from billing.invoices where organization_id = $1 and id = $2 and status = 'draft';

-- name: GetInvoice :one
select id, organization_id, document_type, coalesce(number, '')::text as number, status, branch_id, customer_id,
       customer_identification_type_code, customer_identification_number, customer_legal_name, customer_email,
       customer_phone, customer_address, issued_at, coalesce(due_date::text, '')::text as due_date, sale_condition_code, credit_term_days,
       currency_code::text as currency_code, exchange_rate::text as exchange_rate,
       subtotal_amount::text as subtotal, discount_amount::text as discount, tax_amount::text as tax,
       exoneration_amount::text as exoneration, total_amount::text as total, coalesce(notes, '')::text as notes,
       referenced_invoice_id, coalesce(reference_reason, '')::text as reference_reason, requires_correction,
       coalesce(fiscal_rejection_reason, '')::text as fiscal_rejection_reason, created_by_user_id, issued_by_user_id,
       created_at, updated_at
from billing.invoices where organization_id = $1 and id = $2;

-- name: GetInvoiceForUpdate :one
select id, organization_id, document_type, coalesce(number, '')::text as number, status, branch_id, customer_id,
       customer_identification_type_code, customer_identification_number, customer_legal_name, customer_email,
       customer_phone, customer_address, issued_at, coalesce(due_date::text, '')::text as due_date, sale_condition_code, credit_term_days,
       currency_code::text as currency_code, exchange_rate::text as exchange_rate,
       subtotal_amount::text as subtotal, discount_amount::text as discount, tax_amount::text as tax,
       exoneration_amount::text as exoneration, total_amount::text as total, coalesce(notes, '')::text as notes,
       referenced_invoice_id, coalesce(reference_reason, '')::text as reference_reason, requires_correction,
       coalesce(fiscal_rejection_reason, '')::text as fiscal_rejection_reason, created_by_user_id, issued_by_user_id,
       created_at, updated_at
from billing.invoices where organization_id = $1 and id = $2
for update;

-- Orden estable por (created_at, id). Fechas de emisión en la zona de la organización (fecha de negocio).
-- name: ListInvoices :many
select id, organization_id, document_type, coalesce(number, '')::text as number, status, branch_id, customer_id,
       customer_identification_type_code, customer_identification_number, customer_legal_name, customer_email,
       customer_phone, customer_address, issued_at, coalesce(due_date::text, '')::text as due_date, sale_condition_code, credit_term_days,
       currency_code::text as currency_code, exchange_rate::text as exchange_rate,
       subtotal_amount::text as subtotal, discount_amount::text as discount, tax_amount::text as tax,
       exoneration_amount::text as exoneration, total_amount::text as total, coalesce(notes, '')::text as notes,
       referenced_invoice_id, coalesce(reference_reason, '')::text as reference_reason, requires_correction,
       coalesce(fiscal_rejection_reason, '')::text as fiscal_rejection_reason, created_by_user_id, issued_by_user_id,
       created_at, updated_at
from billing.invoices
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(document_type)::text is null or document_type = sqlc.narg(document_type)::text)
  and (sqlc.narg(status)::text is null or status = sqlc.narg(status)::text)
  and (sqlc.narg(customer_id)::uuid is null or customer_id = sqlc.narg(customer_id)::uuid)
  and (sqlc.narg(requires_correction)::boolean is null or requires_correction = sqlc.narg(requires_correction)::boolean)
  and (sqlc.narg(issued_from)::date is null
       or (issued_at at time zone sqlc.arg(timezone)::text)::date >= sqlc.narg(issued_from)::date)
  and (sqlc.narg(issued_to)::date is null
       or (issued_at at time zone sqlc.arg(timezone)::text)::date <= sqlc.narg(issued_to)::date)
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at, id
limit sqlc.arg(page_size);

-- name: InsertInvoiceLine :one
insert into billing.invoice_lines (organization_id, invoice_id, line_number, product_id, product_code, cabys_code,
                                   description, unit_of_measure_code, is_service, quantity, unit_price,
                                   discount_amount, discount_reason, subtotal_amount, tax_amount, total_amount)
values (sqlc.arg(organization_id), sqlc.arg(invoice_id), sqlc.arg(line_number), sqlc.narg(product_id),
        nullif(sqlc.arg(product_code)::text, ''), sqlc.arg(cabys_code)::text, sqlc.arg(description),
        sqlc.arg(unit_of_measure_code), sqlc.arg(is_service), sqlc.arg(quantity)::text::numeric,
        sqlc.arg(unit_price)::text::numeric, sqlc.arg(discount)::text::numeric,
        nullif(sqlc.arg(discount_reason)::text, ''), sqlc.arg(subtotal)::text::numeric,
        sqlc.arg(tax)::text::numeric, sqlc.arg(total)::text::numeric)
returning id;

-- name: InsertInvoiceLineTax :exec
insert into billing.invoice_line_taxes (organization_id, invoice_line_id, tax_type_code, tax_rate_code, rate,
                                        taxable_base, tax_amount)
values (sqlc.arg(organization_id), sqlc.arg(invoice_line_id), sqlc.arg(tax_type_code),
        nullif(sqlc.arg(tax_rate_code)::text, ''), sqlc.arg(rate)::text::numeric,
        sqlc.arg(taxable_base)::text::numeric, sqlc.arg(tax_amount)::text::numeric);

-- Borra las líneas del borrador; sus impuestos caen por ON DELETE CASCADE. guard_draft_children lo impide si no es draft.
-- name: DeleteInvoiceLines :exec
delete from billing.invoice_lines where organization_id = $1 and invoice_id = $2;

-- Líneas de varios documentos a la vez (ids como text[]: QueryExecModeExec).
-- name: ListInvoiceLines :many
select id, invoice_id, line_number, product_id, coalesce(product_code, '')::text as product_code,
       cabys_code::text as cabys_code, description, unit_of_measure_code, is_service, quantity::text as quantity,
       unit_price::text as unit_price, discount_amount::text as discount,
       coalesce(discount_reason, '')::text as discount_reason, subtotal_amount::text as subtotal,
       tax_amount::text as tax, total_amount::text as total
from billing.invoice_lines
where organization_id = sqlc.arg(organization_id) and invoice_id = any(sqlc.arg(invoice_ids)::text[]::uuid[])
order by invoice_id, line_number;

-- name: ListInvoiceLineTaxes :many
select t.invoice_line_id, t.tax_type_code, coalesce(t.tax_rate_code, '')::text as tax_rate_code, t.rate::text as rate,
       t.taxable_base::text as taxable_base, t.tax_amount::text as tax_amount,
       t.exoneration_document_type_code, t.exoneration_document_number, t.exoneration_institution,
       t.exoneration_issued_at, coalesce(t.exoneration_percentage::text, '')::text as exoneration_rate,
       coalesce(t.exoneration_amount::text, '')::text as exoneration_amount
from billing.invoice_line_taxes t
join billing.invoice_lines l on l.organization_id = t.organization_id and l.id = t.invoice_line_id
where t.organization_id = sqlc.arg(organization_id) and l.invoice_id = any(sqlc.arg(invoice_ids)::text[]::uuid[])
order by t.invoice_line_id, t.tax_type_code;

-- name: ListInvoiceStatusHistory :many
select from_status, to_status, coalesce(reason, '')::text as reason, changed_by_user_id, changed_at
from billing.invoice_status_history
where organization_id = $1 and invoice_id = $2
order by changed_at, id;

-- draft → issued con los snapshots. El WHERE status = 'draft' y el trigger invoices_guard impiden emitir dos veces.
-- name: IssueInvoice :one
update billing.invoices
set status = 'issued', number = sqlc.arg(number)::text, issued_at = sqlc.arg(issued_at)::timestamptz,
    issued_by_user_id = sqlc.arg(issued_by_user_id)::uuid, due_date = sqlc.arg(due_date)::text::date,
    customer_identification_type_code = sqlc.arg(customer_identification_type_code)::text,
    customer_identification_number = sqlc.arg(customer_identification_number)::text,
    customer_legal_name = sqlc.arg(customer_legal_name)::text, customer_email = nullif(sqlc.arg(customer_email)::text, ''),
    customer_phone = nullif(sqlc.arg(customer_phone)::text, ''), customer_address = nullif(sqlc.arg(customer_address)::text, '')
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and status = 'draft'
returning updated_at;

-- name: InsertStatusChange :exec
insert into billing.invoice_status_history (organization_id, invoice_id, from_status, to_status, reason, changed_by_user_id, changed_at)
values (sqlc.arg(organization_id), sqlc.arg(invoice_id), sqlc.narg(from_status), sqlc.arg(to_status),
        nullif(sqlc.arg(reason)::text, ''), sqlc.narg(changed_by_user_id), sqlc.arg(changed_at));
