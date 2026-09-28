-- Propuesta 0002 (database-platform): roles de login de Receivables API.
-- Ejecutar UNA vez en dev desde el SQL Editor de Supabase (como postgres).
-- Luego asignar las contraseñas, también desde el SQL Editor, sin guardarlas en ningún archivo:
--   alter role receivables_api     password '<generada>';
--   alter role receivables_migrate password '<generada>';

do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'receivables_api') then
    create role receivables_api login inherit nobypassrls connection limit 20;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'receivables_migrate') then
    create role receivables_migrate login inherit nobypassrls connection limit 2;
  end if;
end $$;

grant receivables_app to receivables_api;
grant receivables_migrator to receivables_migrate;
-- Los objetos que cree goose quedan a nombre de receivables_migrator (dueño de receivables).
alter role receivables_migrate set role = 'receivables_migrator';

comment on role receivables_api     is 'Login de Receivables API. Hereda receivables_app. Contraseña fuera de banda.';
comment on role receivables_migrate is 'Login de migraciones goose de Receivables. Ejecuta como receivables_migrator.';
