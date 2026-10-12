-- Propuesta a database-platform (docs/decisiones/0002-estado-inicial-bd.md §6, aprobada el 2026-10-12).
-- Para el SQL Editor del proyecto de DEV (dzlsnsstuqpxvwegeqcy), como 0010_receivables_login_roles y
-- 0011_receivables_core_read. Nunca en producción. Idempotente.

-- 1 · Logins. Las contraseñas se asignan después, fuera de banda, y quedan solo en el .env local:
--   alter role fiscal_api     password '<generada>';
--   alter role fiscal_migrate password '<generada>';
do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'fiscal_api') then
    create role fiscal_api login inherit nobypassrls connection limit 20 in role fiscal_app;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'fiscal_migrate') then
    create role fiscal_migrate login inherit nobypassrls connection limit 2 in role fiscal_migrator;
  end if;
end $$;
-- Lo que cree goose queda a nombre del dueño del schema.
alter role fiscal_migrate set role = 'fiscal_migrator';

-- 2 · Revalidar la membresía en cada petición y leer la zona horaria de la organización (bloquea el incremento 2).
grant select on core.organizations, core.organization_users, core.organization_user_roles, core.users to fiscal_app;
grant execute on function core.find_user_by_subject(text) to fiscal_app;

-- 3 · Decisión 4: FK real establecimiento → sucursal (la agrega una migración de este repo después de este grant).
grant usage on schema core to fiscal_migrator;
grant references on core.branches to fiscal_migrator;
