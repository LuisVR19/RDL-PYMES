-- Sesión con app.current_organization_id (política organizations_read).

-- name: GetOrganizationTimezone :one
select timezone from core.organizations where id = $1;
