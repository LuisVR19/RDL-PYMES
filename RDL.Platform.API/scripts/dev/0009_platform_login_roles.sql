-- Propuesta 0002 (database-platform): roles de login de Platform API.
-- Ejecutar UNA vez en dev desde el SQL Editor de Supabase (como postgres).
-- Luego asignar las contraseñas, también desde el SQL Editor, sin guardarlas en ningún archivo:
--   alter role platform_api     password '<generada>';
--   alter role platform_migrate password '<generada>';

do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'platform_api') then
    create role platform_api login inherit nobypassrls connection limit 20;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'platform_migrate') then
    create role platform_migrate login inherit nobypassrls connection limit 2;
  end if;
end $$;

grant platform_app to platform_api;
grant platform_migrator to platform_migrate;
-- Los objetos que cree goose quedan a nombre de platform_migrator (dueño de core y de los default privileges).
alter role platform_migrate set role = 'platform_migrator';

comment on role platform_api     is 'Login de Platform API. Hereda platform_app. Contraseña fuera de banda.';
comment on role platform_migrate is 'Login de migraciones goose de Platform. Ejecuta como platform_migrator.';
