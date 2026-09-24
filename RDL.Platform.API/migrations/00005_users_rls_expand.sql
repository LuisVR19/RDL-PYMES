-- Endurecimiento de core.users, fase expand (decisión P7 de docs/decisiones/0001-estado-inicial-bd.md).
-- Hoy users_platform es `true`: platform_app ve y modifica cualquier usuario. Esta migración agrega el camino
-- nuevo sin quitar el viejo; 00006 (contract) elimina users_platform cuando el código ya no depende de ella.
--
-- - users_self: cada usuario ve y actualiza solo su fila (app.current_user_id).
-- - users_org_members: se ven los usuarios con membresía en la organización activa (listado de miembros,
--   invitaciones). La subconsulta corre bajo la RLS de organization_users (organization_users_tenant).
-- - Sin políticas de INSERT ni DELETE: el alta pasa solo por core.provision_user.
-- - La búsqueda por sujeto ocurre antes de conocer el id del usuario (login, alta perezosa, resolución de
--   membresía), así que la hacen funciones security definer con dueño platform_migrator, igual que el hook.

-- +goose Up
-- +goose StatementBegin
create or replace function core.find_user_by_subject(p_subject text)
returns setof core.users
language sql
stable
security definer
set search_path = ''
as $$
  select u.*
    from core.users u
   where u.identity_provider = 'supabase'
     and u.external_subject = p_subject;
$$;
-- +goose StatementEnd

-- Alta idempotente: ante llamadas concurrentes del mismo sujeto solo una inserta y las demás no reciben fila.
-- Un email ya usado por otra identidad sigue fallando por users_email_uk.
-- +goose StatementBegin
create or replace function core.provision_user(p_subject text, p_email text, p_full_name text)
returns setof core.users
language sql
volatile
security definer
set search_path = ''
as $$
  insert into core.users (identity_provider, external_subject, email, full_name)
  values ('supabase', p_subject, p_email, p_full_name)
  on conflict (identity_provider, external_subject) do nothing
  returning *;
$$;
-- +goose StatementEnd

revoke execute on function core.find_user_by_subject(text) from public, anon, authenticated;
revoke execute on function core.provision_user(text, text, text) from public, anon, authenticated;
grant execute on function core.find_user_by_subject(text) to platform_app;
grant execute on function core.provision_user(text, text, text) to platform_app;

drop policy if exists users_self on core.users;
create policy users_self on core.users
  for select
  using (id = (select shared.current_user_id()));

drop policy if exists users_self_update on core.users;
create policy users_self_update on core.users
  for update
  using (id = (select shared.current_user_id()))
  with check (id = (select shared.current_user_id()));

drop policy if exists users_org_members on core.users;
create policy users_org_members on core.users
  for select
  using (exists (
    select 1
      from core.organization_users ou
     where ou.user_id = users.id
       and ou.organization_id = (select shared.current_organization_id())
  ));

-- +goose Down
drop policy if exists users_org_members on core.users;
drop policy if exists users_self_update on core.users;
drop policy if exists users_self on core.users;
drop function if exists core.provision_user(text, text, text);
drop function if exists core.find_user_by_subject(text);
