-- La sesión ya debe tener app.current_user_id (políticas organization_users_own / organization_user_roles_own).

-- name: FindActiveUserIDBySubject :one
select id from core.find_user_by_subject(sqlc.arg(external_subject)::text) where status = 'active';

-- name: GetActiveMembership :one
select u.id as user_id,
       coalesce(array_agg(r.role_code order by r.role_code) filter (where r.role_code is not null), '{}')::text[] as roles
from core.users u
join core.organization_users ou
  on ou.user_id = u.id and ou.organization_id = sqlc.arg(organization_id) and ou.status = 'active'
join core.organizations o
  on o.id = ou.organization_id and o.status = 'active'
left join core.organization_user_roles r
  on r.organization_id = ou.organization_id and r.organization_user_id = ou.id
where u.id = sqlc.arg(user_id)
group by u.id;

-- name: ListActiveMembershipsForUser :many
select ou.organization_id,
       o.legal_name,
       coalesce(o.trade_name, '')::text as trade_name,
       o.status as organization_status,
       ou.status,
       ou.joined_at,
       coalesce(array_agg(r.role_code order by r.role_code) filter (where r.role_code is not null), '{}')::text[] as roles
from core.organization_users ou
join core.organizations o on o.id = ou.organization_id
left join core.organization_user_roles r
  on r.organization_id = ou.organization_id and r.organization_user_id = ou.id
where ou.user_id = sqlc.arg(user_id) and ou.status = 'active'
group by ou.organization_id, o.legal_name, o.trade_name, o.status, ou.status, ou.joined_at
order by o.legal_name, ou.organization_id;

-- name: InsertMembership :one
insert into core.organization_users (organization_id, user_id) values ($1, $2) returning id;

-- name: InsertMembershipRole :exec
insert into core.organization_user_roles (organization_id, organization_user_id, role_code, granted_by_user_id)
values ($1, $2, $3, $4);
