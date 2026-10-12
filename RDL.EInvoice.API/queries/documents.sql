-- Documentos electrónicos (RLS *_tenant). Montos como texto en los dos sentidos (ADR 0004 de Platform: con
-- QueryExecModeExec nunca un tipo numérico del driver): ::text::numeric al escribir, ::text al leer.

-- name: ExistsDocumentBySource :one
select exists (
  select 1 from fiscal.electronic_documents
   where organization_id = sqlc.arg(organization_id) and source_type = sqlc.arg(source_type)
     and source_document_id = sqlc.arg(source_document_id)
)::boolean;

-- name: InsertElectronicDocument :exec
insert into fiscal.electronic_documents (
  id, organization_id, source_type, source_document_id, source_document_number, source_event_id,
  document_type_code, status, issued_at,
  issuer_identification_type_code, issuer_identification_number, issuer_legal_name, issuer_trade_name,
  issuer_economic_activity_code, issuer_email, issuer_phone, issuer_province_code, issuer_canton_code,
  issuer_district_code, issuer_address_details,
  receiver_identification_type_code, receiver_identification_number, receiver_legal_name, receiver_email,
  receiver_phone, receiver_address,
  sale_condition_code, credit_term_days, currency_code, exchange_rate,
  subtotal_amount, discount_amount, tax_amount, exoneration_amount, total_amount, next_attempt_at
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(source_type), sqlc.arg(source_document_id),
  sqlc.arg(source_document_number), sqlc.arg(source_event_id),
  sqlc.arg(document_type_code), 'processing', sqlc.arg(issued_at),
  sqlc.arg(issuer_identification_type_code), sqlc.arg(issuer_identification_number), sqlc.arg(issuer_legal_name),
  sqlc.narg(issuer_trade_name), sqlc.narg(issuer_economic_activity_code), sqlc.narg(issuer_email),
  sqlc.narg(issuer_phone), sqlc.narg(issuer_province_code), sqlc.narg(issuer_canton_code),
  sqlc.narg(issuer_district_code), sqlc.narg(issuer_address_details),
  sqlc.arg(receiver_identification_type_code), sqlc.arg(receiver_identification_number),
  sqlc.arg(receiver_legal_name), sqlc.narg(receiver_email), sqlc.narg(receiver_phone), sqlc.narg(receiver_address),
  sqlc.arg(sale_condition_code), sqlc.narg(credit_term_days), sqlc.arg(currency_code)::text,
  sqlc.arg(exchange_rate)::text::numeric,
  sqlc.arg(subtotal_amount)::text::numeric, sqlc.arg(discount_amount)::text::numeric,
  sqlc.arg(tax_amount)::text::numeric, sqlc.arg(exoneration_amount)::text::numeric,
  sqlc.arg(total_amount)::text::numeric, now()
);

-- name: InsertDocumentLine :exec
insert into fiscal.electronic_document_lines (
  id, organization_id, electronic_document_id, line_number, cabys_code, product_code, description,
  unit_of_measure_code, quantity, unit_price, gross_amount, discount_amount, discount_reason,
  subtotal_amount, tax_amount, total_amount
) values (
  sqlc.arg(id), sqlc.arg(organization_id), sqlc.arg(electronic_document_id), sqlc.arg(line_number),
  sqlc.arg(cabys_code)::text, sqlc.narg(product_code), sqlc.arg(description), sqlc.arg(unit_of_measure_code),
  sqlc.arg(quantity)::text::numeric, sqlc.arg(unit_price)::text::numeric, sqlc.arg(gross_amount)::text::numeric,
  sqlc.arg(discount_amount)::text::numeric, sqlc.narg(discount_reason), sqlc.arg(subtotal_amount)::text::numeric,
  sqlc.arg(tax_amount)::text::numeric, sqlc.arg(total_amount)::text::numeric
);

-- Exoneración: los 6 campos o ninguno (CHECK electronic_document_line_taxes_exoneration_ck). exoneration_percentage
-- guarda PUNTOS de tarifa (comentario de la columna, migración 00002).
-- name: InsertLineTax :exec
insert into fiscal.electronic_document_line_taxes (
  organization_id, electronic_document_line_id, tax_type_code, tax_rate_code, rate, taxable_base, tax_amount,
  exoneration_document_type_code, exoneration_document_number, exoneration_institution, exoneration_issued_at,
  exoneration_percentage, exoneration_amount
) values (
  sqlc.arg(organization_id), sqlc.arg(electronic_document_line_id), sqlc.arg(tax_type_code), sqlc.narg(tax_rate_code),
  sqlc.arg(rate)::text::numeric, sqlc.arg(taxable_base)::text::numeric, sqlc.arg(tax_amount)::text::numeric,
  sqlc.narg(exoneration_document_type_code), sqlc.narg(exoneration_document_number),
  sqlc.narg(exoneration_institution), sqlc.narg(exoneration_issued_at),
  sqlc.narg(exoneration_percentage)::text::numeric, sqlc.narg(exoneration_amount)::text::numeric
);

-- name: InsertDocumentPaymentMethod :exec
insert into fiscal.electronic_document_payment_methods (
  organization_id, electronic_document_id, position, payment_method_code, amount
) values (
  sqlc.arg(organization_id), sqlc.arg(electronic_document_id), sqlc.arg(position), sqlc.arg(payment_method_code),
  sqlc.narg(amount)::text::numeric
);

-- name: InsertStatusHistory :exec
insert into fiscal.electronic_document_status_history (organization_id, electronic_document_id, from_status, to_status, reason)
values (sqlc.arg(organization_id), sqlc.arg(electronic_document_id), sqlc.narg(from_status), sqlc.arg(to_status), sqlc.narg(reason));

-- name: GetFiscalStatusBySource :one
select id, source_document_id, status, hacienda_status_message
  from fiscal.electronic_documents
 where organization_id = sqlc.arg(organization_id) and source_document_id = sqlc.arg(source_document_id);

-- ids como text[] (ADR 0004 de Platform: arreglos como texto).
-- name: ListFiscalStatusesBySource :many
select id, source_document_id, status, hacienda_status_message
  from fiscal.electronic_documents
 where organization_id = sqlc.arg(organization_id)
   and source_document_id = any (sqlc.arg(source_document_ids)::text[]::uuid[]);
