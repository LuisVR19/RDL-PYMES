-- Sesión con app.current_organization_id = organización activa (políticas products_tenant y product_taxes_tenant).
-- Los montos viajan como texto en ambos sentidos: nunca pasan por float ni por un tipo numérico del driver.

-- name: InsertProduct :one
insert into billing.products (organization_id, code, description, cabys_code, unit_of_measure_code, unit_price,
                              currency_code, is_service, is_active)
values (sqlc.arg(organization_id), sqlc.arg(code), sqlc.arg(description), sqlc.arg(cabys_code)::text,
        sqlc.arg(unit_of_measure_code), sqlc.arg(unit_price)::text::numeric, sqlc.arg(currency_code)::text,
        sqlc.arg(is_service), sqlc.arg(is_active))
returning id, organization_id, code, description, cabys_code::text as cabys_code, unit_of_measure_code,
          unit_price::text as unit_price, currency_code::text as currency_code, is_service, is_active,
          created_at, updated_at;

-- name: ListProducts :many
select id, organization_id, code, description, cabys_code::text as cabys_code, unit_of_measure_code,
       unit_price::text as unit_price, currency_code::text as currency_code, is_service, is_active,
       created_at, updated_at
from billing.products
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(is_active)::boolean is null or is_active = sqlc.narg(is_active)::boolean)
  and (sqlc.narg(search)::text is null
       or code ilike '%' || sqlc.narg(search)::text || '%' escape '\'
       or description ilike '%' || sqlc.narg(search)::text || '%' escape '\')
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at, id
limit sqlc.arg(page_size);

-- name: GetProduct :one
select id, organization_id, code, description, cabys_code::text as cabys_code, unit_of_measure_code,
       unit_price::text as unit_price, currency_code::text as currency_code, is_service, is_active,
       created_at, updated_at
from billing.products where organization_id = $1 and id = $2;

-- name: GetProductForUpdate :one
select id, organization_id, code, description, cabys_code::text as cabys_code, unit_of_measure_code,
       unit_price::text as unit_price, currency_code::text as currency_code, is_service, is_active,
       created_at, updated_at
from billing.products where organization_id = $1 and id = $2
for update;

-- name: UpdateProduct :one
update billing.products
set code = sqlc.arg(code), description = sqlc.arg(description), cabys_code = sqlc.arg(cabys_code)::text,
    unit_of_measure_code = sqlc.arg(unit_of_measure_code), unit_price = sqlc.arg(unit_price)::text::numeric,
    currency_code = sqlc.arg(currency_code)::text, is_service = sqlc.arg(is_service), is_active = sqlc.arg(is_active)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
returning id, organization_id, code, description, cabys_code::text as cabys_code, unit_of_measure_code,
          unit_price::text as unit_price, currency_code::text as currency_code, is_service, is_active,
          created_at, updated_at;

-- name: InsertProductTax :exec
insert into billing.product_taxes (organization_id, product_id, tax_type_code, tax_rate_code)
values ($1, $2, $3, $4);

-- name: DeleteProductTaxes :exec
delete from billing.product_taxes where organization_id = $1 and product_id = $2;

-- Impuestos de varios productos a la vez: una consulta por página, no una por producto.
-- Los ids viajan como text[]: con QueryExecModeExec (Supavisor) pgx no sabe codificar un []uuid.UUID sin describir
-- el statement, igual que un jsonb viaja como texto (ADR 0004 de Platform).
-- name: ListProductTaxes :many
select product_id, tax_type_code, tax_rate_code
from billing.product_taxes
where organization_id = sqlc.arg(organization_id) and product_id = any(sqlc.arg(product_ids)::text[]::uuid[])
order by product_id, tax_type_code;
