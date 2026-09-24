-- core.users no es visible sin sesión de usuario (políticas users_self / users_org_members, migración 00005).
-- La búsqueda por sujeto y el alta pasan por funciones security definer.

-- name: FindUserBySubject :one
select id, external_subject, email, full_name, status, active_organization_id, created_at
from core.find_user_by_subject(sqlc.arg(external_subject)::text);

-- Alta idempotente: ante llamadas concurrentes del mismo sujeto, solo una inserta; las demás no devuelven fila
-- y el caso de uso relee. Un email ya usado por otra identidad sigue fallando por users_email_uk (409).
-- name: InsertUserIfAbsent :one
select id, external_subject, email, full_name, status, active_organization_id, created_at
from core.provision_user(sqlc.arg(external_subject)::text, sqlc.arg(email)::text, sqlc.arg(full_name)::text);

-- Requiere la sesión del propio usuario (users_self_update).
-- name: SetUserActiveOrganization :execrows
update core.users set active_organization_id = sqlc.arg(organization_id)
where id = sqlc.arg(user_id);
