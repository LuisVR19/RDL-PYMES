-- Sesión con app.current_organization_id = organización activa (políticas *_tenant).

-- Paginación por cursor sobre (joined_at, id): estable aunque entren miembros nuevos entre páginas.
-- name: ListMembers :many
select ou.id as membership_id, u.id as user_id, u.email, u.full_name, ou.status, ou.joined_at,
       coalesce(array_agg(r.role_code order by r.role_code) filter (where r.role_code is not null), '{}')::text[] as roles
from core.organization_users ou
join core.users u on u.id = ou.user_id
left join core.organization_user_roles r
  on r.organization_id = ou.organization_id and r.organization_user_id = ou.id
where ou.organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(status)::text is null or ou.status = sqlc.narg(status)::text)
  and (sqlc.narg(after_joined_at)::timestamptz is null
       or (ou.joined_at, ou.id) > (sqlc.narg(after_joined_at)::timestamptz, sqlc.narg(after_id)::uuid))
group by ou.id, u.id, u.email, u.full_name, ou.status, ou.joined_at
order by ou.joined_at, ou.id
limit sqlc.arg(page_size);

-- Bloquea la membresía hasta el fin de la transacción: dos admins cambiando al mismo miembro se serializan.
-- name: GetMemberForUpdate :one
select ou.id as membership_id, u.id as user_id, u.external_subject, u.email, u.full_name, ou.status, ou.joined_at,
       coalesce((select array_agg(r.role_code order by r.role_code)
                   from core.organization_user_roles r
                  where r.organization_id = ou.organization_id and r.organization_user_id = ou.id), '{}')::text[] as roles
from core.organization_users ou
join core.users u on u.id = ou.user_id
where ou.organization_id = sqlc.arg(organization_id) and ou.user_id = sqlc.arg(user_id)
for update of ou;

-- name: CountActiveOwners :one
select count(*)::int
from core.organization_users ou
join core.organization_user_roles r
  on r.organization_id = ou.organization_id and r.organization_user_id = ou.id and r.role_code = 'owner'
where ou.organization_id = $1 and ou.status = 'active';

-- name: UpdateMembershipStatus :exec
update core.organization_users set status = sqlc.arg(status)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(membership_id);

-- name: DeleteMembershipRoles :exec
delete from core.organization_user_roles
where organization_id = $1 and organization_user_id = $2;

-- name: FindMembershipByUser :one
select id, status from core.organization_users where organization_id = $1 and user_id = $2;
