-- Sesión con app.current_organization_id = organización activa (política branches_tenant).

-- name: InsertBranch :one
insert into core.branches (organization_id, code, name, address, phone, email)
values (sqlc.arg(organization_id), sqlc.arg(code), sqlc.arg(name), nullif(sqlc.arg(address)::text, ''),
        nullif(sqlc.arg(phone)::text, ''), nullif(sqlc.arg(email)::text, ''))
returning id, organization_id, code, name, coalesce(address, '')::text as address, coalesce(phone, '')::text as phone,
          coalesce(email, '')::text as email, is_active, created_at, updated_at;

-- name: ListBranches :many
select id, organization_id, code, name, coalesce(address, '')::text as address, coalesce(phone, '')::text as phone,
       coalesce(email, '')::text as email, is_active, created_at, updated_at
from core.branches
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(is_active)::boolean is null or is_active = sqlc.narg(is_active)::boolean)
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at, id
limit sqlc.arg(page_size);

-- name: GetBranch :one
select id, organization_id, code, name, coalesce(address, '')::text as address, coalesce(phone, '')::text as phone,
       coalesce(email, '')::text as email, is_active, created_at, updated_at
from core.branches where organization_id = $1 and id = $2;

-- name: GetBranchForUpdate :one
select id, organization_id, code, name, coalesce(address, '')::text as address, coalesce(phone, '')::text as phone,
       coalesce(email, '')::text as email, is_active, created_at, updated_at
from core.branches where organization_id = $1 and id = $2
for update;

-- name: UpdateBranch :one
update core.branches
set name = sqlc.arg(name), address = nullif(sqlc.arg(address)::text, ''), phone = nullif(sqlc.arg(phone)::text, ''),
    email = nullif(sqlc.arg(email)::text, ''), is_active = sqlc.arg(is_active)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id)
returning id, organization_id, code, name, coalesce(address, '')::text as address, coalesce(phone, '')::text as phone,
          coalesce(email, '')::text as email, is_active, created_at, updated_at;
