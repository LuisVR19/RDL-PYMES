-- Todas corren con app.current_organization_id = la organización en cuestión (políticas organizations_*).

-- name: InsertOrganization :one
insert into core.organizations (id, legal_name, trade_name, identification_type_code, identification_number, email, phone, timezone)
values (sqlc.arg(id), sqlc.arg(legal_name), nullif(sqlc.arg(trade_name)::text, ''), sqlc.arg(identification_type_code),
        sqlc.arg(identification_number), sqlc.arg(email), nullif(sqlc.arg(phone)::text, ''), sqlc.arg(timezone))
returning id, legal_name, coalesce(trade_name, '')::text as trade_name, identification_type_code, identification_number,
          email, coalesce(phone, '')::text as phone, timezone, default_currency_code::text as default_currency_code,
          status, created_at, updated_at;

-- name: GetOrganization :one
select id, legal_name, coalesce(trade_name, '')::text as trade_name, identification_type_code, identification_number,
       email, coalesce(phone, '')::text as phone, timezone, default_currency_code::text as default_currency_code,
       status, created_at, updated_at
from core.organizations where id = $1;

-- name: GetOrganizationForUpdate :one
select id, legal_name, coalesce(trade_name, '')::text as trade_name, identification_type_code, identification_number,
       email, coalesce(phone, '')::text as phone, timezone, default_currency_code::text as default_currency_code,
       status, created_at, updated_at
from core.organizations where id = $1
for update;

-- name: UpdateOrganization :one
update core.organizations
set trade_name = nullif(sqlc.arg(trade_name)::text, ''), email = sqlc.arg(email),
    phone = nullif(sqlc.arg(phone)::text, ''), timezone = sqlc.arg(timezone)
where id = sqlc.arg(id)
returning id, legal_name, coalesce(trade_name, '')::text as trade_name, identification_type_code, identification_number,
          email, coalesce(phone, '')::text as phone, timezone, default_currency_code::text as default_currency_code,
          status, created_at, updated_at;
