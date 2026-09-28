# 0002 · Propuesta para `database-platform`: logins, lectura de `core` y privilegios

- **Fecha:** 2026-09-24
- **Estado:** Aprobado el 2026-09-24. **§1 y §2 aplicados en dev** el 2026-09-24 a pedido explícito del usuario
  (migraciones `0010_receivables_login_roles` y `0011_receivables_core_read` en `supabase_migrations`). §3 pendiente
  (`database-platform`).
- **Contexto:** informe `0001-estado-inicial-bd.md`, secciones 2.4 y 4.4; hallazgo 17 de `RDL.Contracts/docs/ESTADO.md`.

## 1. Logins (igual que Platform, propuesta 0002 de su repo)

Archivo listo para el SQL Editor: `scripts/dev/0010_receivables_login_roles.sql`. Las contraseñas se asignan después,
fuera de banda, y quedan solo en el `.env` local:

```sql
alter role receivables_api     password '<generada>';
alter role receivables_migrate password '<generada>';
```

- `receivables_api`: `LOGIN INHERIT NOBYPASSRLS`, miembro de `receivables_app`, límite 20 conexiones. Se conecta por
  Supavisor en modo transacción (6543).
- `receivables_migrate`: miembro de `receivables_migrator`, con `SET role = 'receivables_migrator'` para que lo que cree
  goose quede a nombre del dueño del schema. Modo sesión (5432).

## 2. Lectura de `core` (bloquea el incremento 2)

La API tiene que revalidar la membresía en cada petición (igual que Platform: `core.find_user_by_subject` y la consulta
`GetActiveMembership`) y leer `core.organizations.timezone` para las fechas de negocio, **también desde el consumidor**
(sin usuario). Hoy `receivables_app` no tiene ni `USAGE` en `core`.

```sql
grant usage on schema core to receivables_app;
grant select on core.organizations, core.organization_users, core.organization_user_roles, core.users
  to receivables_app;
grant execute on function core.find_user_by_subject(text) to receivables_app;
```

Solo lectura. Las políticas existentes ya acotan lo que se ve:

- `organizations_read`: la organización de la sesión (`app.current_organization_id`) o las del propio usuario activo.
  El consumidor, que solo fija la organización del evento, ve únicamente esa.
- `organization_users_own` / `organization_user_roles_own`: las membresías del propio usuario
  (`app.current_user_id`); `*_tenant`: las de la organización activa.
- `users_self` / `users_org_members`: el propio usuario y los miembros de la organización activa.
- `find_user_by_subject` es `security definer` y devuelve solo la fila del `sub` pedido (ADR 0007 de Platform).

No se pide `core.branches`: las cuentas por cobrar no guardan sucursal. Si la decisión D11 de contratos hace
obligatorio `branchId`, se revisa.

> Nota: `billing_app` tampoco tiene lectura de `organizations` ni de membresías (solo `branches`). La Billing API (P4)
> va a necesitar el mismo `GRANT`; conviene resolverlo junto.

## 3. Ajustes de privilegios (recomendados, no bloquean)

```sql
-- Las gestiones de cobro y las promesas son historial: se cancelan por estado, no se borran.
revoke delete on receivables.collection_followups, receivables.payment_promises from receivables_app;
-- La vista de aging es auto-actualizable; solo se lee.
revoke insert, update, delete on receivables.receivable_aging from receivables_app;
```

Si se prefiere que lo haga el propio repo, los objetos son de `receivables_migrator` y puede ir como migración goose
de esta API. **Decisión R9.**

## 4. Para anotar (sin propuesta todavía)

- `audit.audit_events` → la política `audit_events_read` deja leer los eventos de todos los servicios de la
  organización activa. Para `receivables_app` bastaría con los suyos (`service = current_service()`).
