# 0002 · Propuesta a database-platform y Platform: logins y lecturas de `core`

- **Fecha:** 2026-09-24
- **Estado:** Propuesto — lo ejecuta `database-platform` (o quien administre la base) como `postgres` en dev.
- **Contexto:** informe 0001, §2.2, §3.1 y §3.2. Este repo solo migra `billing`; nada de esto va como migración suya.

## 1. Logins de Billing

Igual que Platform (`0009_platform_login_roles`). SQL en `scripts/dev/0010_billing_login_roles.sql`.

- `billing_api`: LOGIN, hereda `billing_app`, `connection limit 20`. La app entra por Supavisor en modo transacción (6543).
- `billing_migrate`: LOGIN, hereda `billing_migrator`, `connection limit 2`, `set role = 'billing_migrator'` para que
  todo lo que cree goose sea de `billing_migrator`. Entra en modo sesión (5432).

Las contraseñas se asignan fuera de banda con `alter role … password` y **no se guardan en ningún archivo**.

## 2. Lecturas de `core` para `billing_app`

Necesarias para la revalidación de membresía y el rol efectivo (prompt Paso 3, convenciones §5 y §12) y para la zona
horaria de la fecha de negocio de `InvoiceIssued` (`issueDate`):

Billing reutiliza la consulta de membresía de Platform (`queries/membership.sql`), que parte de
`core.find_user_by_subject()`:

```sql
grant execute on function core.find_user_by_subject(text) to billing_app;
grant select (id, status) on core.users to billing_app;
grant select on core.organization_users, core.organization_user_roles to billing_app;
grant select (id, status, timezone, default_currency_code) on core.organizations to billing_app;
```

Las políticas RLS existentes (`users_self`, `organization_users_own`, `organization_user_roles_own`,
`organizations_read`) ya restringen lo visible. No se pide escritura.

## 3. REFERENCES en `core.branches` para `billing_migrator`

Para las FK compuestas `(organization_id, branch_id) → core.branches (organization_id, id)` (informe 0001, §3.2):

```sql
grant usage on schema core to billing_migrator;  -- sin USAGE, crear la FK falla con "permission denied for schema core"
grant references (organization_id, id) on core.branches to billing_migrator;
```

`USAGE` solo permite nombrar objetos de `core`; no da lectura ni escritura sobre ninguna tabla.

Efecto secundario conocido: con la FK, Platform no podrá borrar físicamente una sucursal usada por Billing. Platform
ya usa baja lógica (`is_active`), así que no cambia nada.
