-- Custom Access Token Hook de Supabase Auth (docs/decisiones/0001-estado-inicial-bd.md §6.3).
-- security definer con dueño platform_migrator (dueño de las tablas, no pasa por RLS):
-- supabase_auth_admin solo necesita EXECUTE, no GRANTs sobre tablas de core.
-- Activación manual: Dashboard → Authentication → Hooks → Customize Access Token.

-- +goose Up
-- +goose StatementBegin
create or replace function core.custom_access_token_hook(event jsonb)
returns jsonb
language plpgsql
stable
security definer
set search_path = ''
as $$
declare
  v_claims jsonb := event -> 'claims';
  v_org    uuid;
  v_roles  jsonb;
begin
  select ou.organization_id,
         coalesce((select jsonb_agg(r.role_code order by r.role_code)
                     from core.organization_user_roles r
                    where r.organization_id = ou.organization_id
                      and r.organization_user_id = ou.id), '[]'::jsonb)
    into v_org, v_roles
    from core.users u
    join core.organization_users ou
      on ou.organization_id = u.active_organization_id
     and ou.user_id = u.id
     and ou.status = 'active'
    join core.organizations o
      on o.id = ou.organization_id
     and o.status = 'active'
   where u.identity_provider = 'supabase'
     and u.external_subject = event ->> 'user_id'
     and u.status = 'active';

  if v_org is null then
    v_claims := v_claims - 'org_id' - 'org_roles';
  else
    v_claims := v_claims || jsonb_build_object('org_id', v_org, 'org_roles', v_roles);
  end if;

  return jsonb_build_object('claims', v_claims);
end;
$$;
-- +goose StatementEnd

revoke execute on function core.custom_access_token_hook(jsonb) from public, anon, authenticated, platform_app;
grant usage on schema core to supabase_auth_admin;
grant execute on function core.custom_access_token_hook(jsonb) to supabase_auth_admin;

-- +goose Down
drop function if exists core.custom_access_token_hook(jsonb);
revoke usage on schema core from supabase_auth_admin;
