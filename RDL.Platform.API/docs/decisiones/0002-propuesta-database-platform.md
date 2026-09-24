# 0002 · Propuesta para `database-platform`: roles de login de Platform

- **Fecha:** 2026-09-23
- **Estado:** Propuesto. **No se aplica desde este repo.** Los roles, `audit` e `integration` son propiedad compartida y requieren un PR en `database-platform` con dos aprobaciones.

## Problema

`platform_app` y `platform_migrator` se crearon como `NOLOGIN` (`0001_bootstrap`). La API no puede conectarse con ellos. Conectarse como `postgres` no es opción: es miembro de todos los `*_app`, tiene `BYPASSRLS` y haría que `shared.current_service()` devuelva `'platform'` para cualquier servicio.

## Propuesta

```sql
-- Aplicación: hereda los privilegios de platform_app, sin BYPASSRLS.
create role platform_api login inherit nobypassrls connection limit 20;
grant platform_app to platform_api;

-- Migraciones: los objetos deben quedar a nombre de platform_migrator
-- (dueño de core/subscriptions y de los default privileges).
create role platform_migrate login inherit nobypassrls connection limit 2;
grant platform_migrator to platform_migrate;
alter role platform_migrate set role = 'platform_migrator';
```

- Las contraseñas se asignan fuera de banda (`alter role … password …` desde el gestor de secretos) y **nunca** se versionan.
- La conexión por Supavisor usa el formato `platform_api.<project-ref>`. La app va por el puerto 6543 (modo transacción) y las migraciones por el 5432 o por conexión directa, para que se aplique `set role`.
- `shared.current_service()` sigue funcionando: `pg_has_role('platform_api', 'platform_app', 'member')` es verdadero.
- Los tests de aislamiento se conectan como `platform_api`. Si mañana alguien le da `BYPASSRLS` a este rol, deben fallar.

## Opcional (solo si no se aprueba el id determinista, pregunta P2 de 0001)

```sql
-- expand: permitir claves de idempotencia previas al tenant
alter table integration.idempotency_keys alter column organization_id drop not null;
-- la PK no admite nulos: habría que pasar a PK sintética + unicidad con nulls not distinct (PG15+)
```

Se desaconseja: cambia la PK de una tabla compartida por las cuatro APIs. La opción `uuidv5` de 0001 §5.3 resuelve el caso sin tocar `integration`.
