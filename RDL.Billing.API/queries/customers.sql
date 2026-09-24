-- Sesión con app.current_organization_id = organización activa (política customers_tenant). El filtro explícito por
-- organization_id se mantiene además de RLS: defensa en profundidad y uso del índice.
-- Todas las lecturas devuelven las mismas columnas, en el mismo orden (ver toCustomer).

-- name: InsertCustomer :one
insert into billing.customers (organization_id, identification_type_code, identification_number, legal_name,
                               trade_name, email, phone, address_details, is_active, created_by_user_id)
values (sqlc.arg(organization_id), sqlc.arg(identification_type_code), sqlc.arg(identification_number),
        sqlc.arg(legal_name), nullif(sqlc.arg(trade_name)::text, ''), nullif(sqlc.arg(email)::text, ''),
        nullif(sqlc.arg(phone)::text, ''), nullif(sqlc.arg(address)::text, ''), true, sqlc.arg(created_by_user_id))
returning id, organization_id, identification_type_code, identification_number, legal_name,
          coalesce(trade_name, '')::text as trade_name, coalesce(email, '')::text as email,
          coalesce(phone, '')::text as phone, coalesce(address_details, '')::text as address,
          is_active, created_by_user_id, created_at, updated_at;

-- name: ListCustomers :many
select id, organization_id, identification_type_code, identification_number, legal_name,
       coalesce(trade_name, '')::text as trade_name, coalesce(email, '')::text as email,
       coalesce(phone, '')::text as phone, coalesce(address_details, '')::text as address,
       is_active, created_by_user_id, created_at, updated_at
from billing.customers
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(is_active)::boolean is null or is_active = sqlc.narg(is_active)::boolean)
  and (sqlc.narg(search)::text is null
       or legal_name ilike '%' || sqlc.narg(search)::text || '%' escape '\'
       or trade_name ilike '%' || sqlc.narg(search)::text || '%' escape '\'
       or identification_number ilike '%' || sqlc.narg(search)::text || '%' escape '\')
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at, id
limit sqlc.arg(page_size);

-- name: GetCustomer :one
select id, organization_id, identification_type_code, identification_number, legal_name,
       coalesce(trade_name, '')::text as trade_name, coalesce(email, '')::text as email,
       coalesce(phone, '')::text as phone, coalesce(address_details, '')::text as address,
       is_active, created_by_user_id, created_at, updated_at
from billing.customers where organization_id = $1 and id = $2;

-- name: GetCustomerForUpdate :one
select id, organization_id, identification_type_code, identification_number, legal_name,
       coalesce(trade_name, '')::text as trade_name, coalesce(email, '')::text as email,
       coalesce(phone, '')::text as phone, coalesce(address_details, '')::text as address,
       is_active, created_by_user_id, created_at, updated_at
from billing.customers where organization_id = $1 and id = $2
for update;

-- name: UpdateCustomer :one
update billing.customers
set legal_name = sqlc.arg(legal_name), trade_name = nullif(sqlc.arg(trade_name)::text, ''),
    email = nullif(sqlc.arg(email)::text, ''), phone = nullif(sqlc.arg(phone)::text, ''),
    address_details = nullif(sqlc.arg(address)::text, ''), is_active = sqlc.arg(is_active)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
returning id, organization_id, identification_type_code, identification_number, legal_name,
          coalesce(trade_name, '')::text as trade_name, coalesce(email, '')::text as email,
          coalesce(phone, '')::text as phone, coalesce(address_details, '')::text as address,
          is_active, created_by_user_id, created_at, updated_at;
