-- Salvo FindInvitationByTokenHash, todas corren en la organización de la invitación (política invitations_tenant).

-- name: InsertInvitation :one
insert into core.invitations (organization_id, email, role_code, token_hash, expires_at, invited_by_user_id)
values (sqlc.arg(organization_id), sqlc.arg(email), sqlc.arg(role_code), sqlc.arg(token_hash)::text,
        sqlc.arg(expires_at), sqlc.arg(invited_by_user_id))
returning id, created_at;

-- name: ListInvitations :many
select id, organization_id, email, role_code, status, expires_at, invited_by_user_id, accepted_by_user_id, accepted_at, created_at
from core.invitations
where organization_id = sqlc.arg(organization_id)
  and (sqlc.narg(status)::text is null or status = sqlc.narg(status)::text)
  and (sqlc.narg(after_created_at)::timestamptz is null
       or (created_at, id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid))
order by created_at, id
limit sqlc.arg(page_size);

-- name: GetInvitationForUpdate :one
select id, organization_id, email, role_code, status, expires_at, invited_by_user_id, accepted_by_user_id, accepted_at, created_at
from core.invitations
where organization_id = $1 and id = $2
for update;

-- Sesión del invitado (app.current_user_id): la política invitations_invitee solo deja ver las invitaciones
-- dirigidas a su email. Un token válido de otra persona no devuelve nada.
-- name: FindInvitationByTokenHash :one
select id, organization_id, email, role_code, status, expires_at, invited_by_user_id, accepted_by_user_id, accepted_at, created_at
from core.invitations
where token_hash = sqlc.arg(token_hash)::text;

-- name: GetInvitationByTokenHashForUpdate :one
select id, organization_id, email, role_code, status, expires_at, invited_by_user_id, accepted_by_user_id, accepted_at, created_at
from core.invitations
where organization_id = sqlc.arg(organization_id) and token_hash = sqlc.arg(token_hash)::text
for update;

-- name: RevokeInvitation :exec
update core.invitations set status = 'revoked'
where organization_id = $1 and id = $2 and status = 'pending';

-- name: AcceptInvitation :exec
update core.invitations set status = 'accepted', accepted_at = now(), accepted_by_user_id = sqlc.arg(user_id)
where organization_id = sqlc.arg(organization_id) and id = sqlc.arg(id) and status = 'pending';

-- Las pendientes vencidas se marcan al volver a invitar al mismo email (libera invitations_pending_email_uk).
-- name: ExpirePendingInvitationForEmail :exec
update core.invitations set status = 'expired'
where organization_id = $1 and lower(email) = lower(sqlc.arg(email)::text) and status = 'pending' and expires_at <= now();

-- name: IsActiveMemberEmail :one
select exists (
  select 1 from core.organization_users ou join core.users u on u.id = ou.user_id
  where ou.organization_id = $1 and lower(u.email) = lower(sqlc.arg(email)::text) and ou.status = 'active'
);
