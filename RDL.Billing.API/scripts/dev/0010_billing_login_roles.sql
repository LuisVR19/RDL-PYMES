-- Propuesta 0002 (database-platform): roles de login de Billing API y lecturas de core.
-- Ejecutar UNA vez en dev desde el SQL Editor de Supabase (como postgres).
-- Luego asignar las contraseñas, también desde el SQL Editor, sin guardarlas en ningún archivo:
--   alter role billing_api     password '<generada>';
--   alter role billing_migrate password '<generada>';

do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'billing_api') then
    create role billing_api login inherit nobypassrls connection limit 20;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'billing_migrate') then
    create role billing_migrate login inherit nobypassrls connection limit 2;
  end if;
end $$;

grant billing_app to billing_api;
grant billing_migrator to billing_migrate;
-- Los objetos que cree goose quedan a nombre de billing_migrator (dueño de billing).
alter role billing_migrate set role = 'billing_migrator';

comment on role billing_api     is 'Login de Billing API. Hereda billing_app. Contraseña fuera de banda.';
comment on role billing_migrate is 'Login de migraciones goose de Billing. Ejecuta como billing_migrator.';

-- Revalidación de membresía, rol efectivo y zona horaria de la organización (informe 0001, §3.1).
grant execute on function core.find_user_by_subject(text) to billing_app;
grant select (id, status) on core.users to billing_app;
grant select on core.organization_users, core.organization_user_roles to billing_app;
grant select (id, status, timezone, default_currency_code) on core.organizations to billing_app;

-- FK compuestas hacia core.branches (informe 0001, §3.2). Crear una FK exige USAGE en el schema además de REFERENCES.
grant usage on schema core to billing_migrator;
grant references (organization_id, id) on core.branches to billing_migrator;
