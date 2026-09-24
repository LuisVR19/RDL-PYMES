-- Misma revalidación que Platform (su queries/membership.sql). La sesión ya debe tener app.current_user_id
-- (políticas users_self, organization_users_own y organization_user_roles_own).

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
