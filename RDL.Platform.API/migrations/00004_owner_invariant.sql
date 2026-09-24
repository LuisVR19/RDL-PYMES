-- Invariante "toda organización con miembros conserva al menos un owner activo"
-- (docs/decisiones/0001-estado-inicial-bd.md §6.4). Defensa en profundidad: la regla principal vive en el dominio.
-- Diferido para que el alta (organización → membresía → rol) y los cambios de rol en varios pasos se evalúen al COMMIT.

-- +goose Up
-- +goose StatementBegin
create or replace function core.ensure_active_owner()
returns trigger
language plpgsql
set search_path = ''
as $$
declare
  v_org uuid := coalesce(new.organization_id, old.organization_id);
begin
  if exists (select 1 from core.organization_users ou where ou.organization_id = v_org)
     and not exists (
       select 1
         from core.organization_users ou
         join core.organization_user_roles r
           on r.organization_id = ou.organization_id
          and r.organization_user_id = ou.id
          and r.role_code = 'owner'
        where ou.organization_id = v_org
          and ou.status = 'active') then
    raise exception 'La organización % debe conservar al menos un owner activo', v_org
      using errcode = 'check_violation', constraint = 'organization_active_owner_ck';
  end if;
  return null;
end;
$$;
-- +goose StatementEnd

drop trigger if exists organization_users_owner_ck on core.organization_users;
create constraint trigger organization_users_owner_ck
  after insert or update or delete on core.organization_users
  deferrable initially deferred
  for each row execute function core.ensure_active_owner();

drop trigger if exists organization_user_roles_owner_ck on core.organization_user_roles;
create constraint trigger organization_user_roles_owner_ck
  after update or delete on core.organization_user_roles
  deferrable initially deferred
  for each row execute function core.ensure_active_owner();

-- +goose Down
drop trigger if exists organization_user_roles_owner_ck on core.organization_user_roles;
drop trigger if exists organization_users_owner_ck on core.organization_users;
drop function if exists core.ensure_active_owner();
