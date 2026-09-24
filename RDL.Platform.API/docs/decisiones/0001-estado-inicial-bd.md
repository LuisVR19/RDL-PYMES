# 0001 · Estado inicial de la base de datos (informe de brechas)

- **Fecha:** 2026-09-23
- **Estado:** Propuesto — pendiente de aprobación. No se ha modificado nada en la base.
- **Fuente:** inspección en solo lectura del proyecto Supabase vía MCP (`information_schema`, `pg_catalog`, `pg_policies`, `supabase_migrations.schema_migrations`).
- **Referencias:** `docs/contexto/arquitectura-v1.md` (2.1, 4, 5, 6.2, 7.3, 11, 12), `docs/contexto/planning-v1.md` (P1, P3), prompt P3.

## 1. Resumen

La base ya tiene casi todo lo que la Platform API necesita. El trabajo de base de datos se reduce a:

1. una **baseline** goose de `core` y `subscriptions` (idempotente, sin cambios reales);
2. **tres migraciones pequeñas** en `core`: organización activa, hook de Supabase Auth e invariante de owner;
3. una **propuesta a `database-platform`** con los roles de login (`docs/decisiones/0002-propuesta-database-platform.md`).

Además, el prompt P3 tiene varios supuestos que **no coinciden con la base real**. En esos casos manda la base (sección 4).

## 2. Inventario

### 2.1 Historial de migraciones

`supabase_migrations.schema_migrations`, aplicadas por `database-platform`:

| Versión | Nombre | Contenido relevante para Platform |
|---|---|---|
| 20260923195950 | 0001_bootstrap | Roles `*_app` / `*_migrator` (NOLOGIN), schemas, dominios `shared.*`, funciones `shared.current_*` |
| 20260923200025 | 0002_core_subscriptions | Todas las tablas de `core` y `subscriptions`, RLS, seed de `core.roles` (ejecutado con `set role platform_migrator`) |
| 20260923200118–200302 | 0003–0005 | `fiscal`, `billing`, `receivables` (fuera de alcance) |
| 20260923200303 | 0006_audit_integration | `audit.audit_events`, `integration.*` |
| 20260923200317 | 0007_grants | GRANTs por rol y default privileges |
| 20260923200411 | 0008_catalog_policies | Parte las políticas `plans_manage` en insert/update/delete |

No existe `core.goose_db_version`. La base no tiene datos: 0 organizaciones, 0 usuarios, 0 `auth.users`. Solo `core.roles` tiene filas (6).

### 2.2 Schemas y dueños

| Schema | Dueño | `platform_app` | `platform_migrator` |
|---|---|---|---|
| core | platform_migrator | USAGE; CRUD en tablas (solo SELECT en `roles`) | dueño |
| subscriptions | platform_migrator | USAGE; CRUD | dueño |
| audit | postgres | USAGE; SELECT e INSERT en `audit_events` | — |
| integration | postgres | USAGE; SELECT, INSERT y UPDATE (más DELETE en `idempotency_keys`) | — |
| shared | postgres | USAGE; EXECUTE en funciones | USAGE; EXECUTE |
| billing / fiscal / receivables | *_migrator | **sin USAGE** ✅ | sin USAGE ✅ |
| auth | supabase_auth_admin | sin USAGE | sin USAGE |

Default privileges: todo lo que `platform_migrator` cree en `core` o `subscriptions` otorga `arwd` a `platform_app`. Las funciones nuevas **no** son ejecutables por `public`.

### 2.3 Tablas de `core` y `subscriptions`

| Tabla | Clave / unicidades | FK | Notas |
|---|---|---|---|
| `core.organizations` | PK `id`; UK `(identification_type_code, identification_number)` | — | `status` ∈ active/suspended/closed; `timezone` por defecto `America/Costa_Rica`; `default_currency_code` `shared.currency_code` |
| `core.users` | PK `id`; UK `(identity_provider, external_subject)`; UK `lower(email)` | — | Global (sin `organization_id`, correcto). `status` ∈ active/disabled. **No tiene `active_organization_id`** |
| `core.organization_users` | PK `id`; UK `(organization_id, id)`; UK `(organization_id, user_id)` | → organizations, → users | `status` ∈ **active/suspended**. **No tiene columna de rol** |
| `core.organization_user_roles` | PK `(organization_id, organization_user_id, role_code)` | **FK compuesta** → organization_users `(organization_id, id)` ON DELETE CASCADE; → roles; → users (granted_by) | **Varios roles por membresía.** Solo tiene `granted_at` (sin `created_at` ni `updated_at`) |
| `core.roles` | PK `code` | — | Catálogo global: `owner, admin, biller, collector, accountant, read_only` |
| `core.invitations` | PK `id`; UK `(organization_id, id)`; UK `token_hash`; UK parcial `(organization_id, lower(email)) WHERE status='pending'` | → organizations, → roles, → users ×2 | `token_hash` `shared.sha256_hex` (64 hex en minúscula); `status` ∈ pending/accepted/revoked/expired; CHECK de coherencia de aceptación |
| `core.branches` | PK `id`; UK `(organization_id, id)`; UK `(organization_id, code)` | → organizations | `is_active` para la baja lógica |
| `subscriptions.*` | `plans`, `organization_subscriptions`, `usage_counters` | FK compuesta en `usage_counters` | Fuera de alcance (F5). Se incluyen en la baseline |

Todas las fechas son `timestamptz` y todos los montos usan `numeric(18,5)` (dominio `shared.money_amount`). **No hay `float` ni `timestamp` sin zona.** Todas las tablas tienen trigger `shared.set_updated_at()`, salvo `roles` y `organization_user_roles`, que no tienen `updated_at`.

### 2.4 Tablas compartidas que usará Platform (sin migrarlas)

- `audit.audit_events`: append-only por trigger (`UPDATE`, `DELETE` y `TRUNCATE` → excepción). Política de INSERT: `service = shared.current_service()`. No tiene columnas `before` ni `after`; hay `payload jsonb`.
- `integration.idempotency_keys`: PK `(service, organization_id, idempotency_key)`, `request_hash sha256_hex`, `response_status`, `response_body`, `expires_at`. **`organization_id NOT NULL`.**
- `integration.outbox_messages`: lista. `organization_id NOT NULL` y `source_service` controlado por RLS.

### 2.5 Mecanismo de sesión RLS (ya definido)

```sql
shared.current_organization_id() -> current_setting('app.current_organization_id', true)::uuid
shared.current_user_id()         -> current_setting('app.current_user_id', true)::uuid   -- core.users.id, no el sub
shared.current_service()         -> 'platform' si current_user es miembro de platform_app
```

Políticas de `core`:

| Tabla | Política | Regla |
|---|---|---|
| organizations | read | `id = org actual` **o** membresía activa del usuario actual |
| organizations | insert | `true` |
| organizations | update | `id = org actual` |
| users | users_platform | `true` (ALL) |
| organization_users | tenant (ALL) / own (SELECT) | org actual / `user_id = usuario actual` |
| organization_user_roles | tenant (ALL) / own (SELECT) | org actual / la membresía es del usuario actual |
| invitations | tenant (ALL) / invitee (SELECT) | org actual / `lower(email)` = email del usuario actual |
| branches | tenant (ALL) | org actual |
| roles | read | `true` |

RLS está **habilitado, pero no forzado**. Los dueños de las tablas (`platform_migrator`, `postgres`) lo saltan y `platform_app` no, que es lo correcto.

### 2.6 Funciones, triggers y hooks

- Triggers en `core`: solo `*_set_updated_at`.
- **No existe Custom Access Token Hook** ni ninguna función `*hook*`.
- No hay trigger sobre `auth.users`.

## 3. Lo que se reutiliza tal cual

- Todas las tablas de `core` (sección 2.3) y sus unicidades y FK compuestas.
- El mecanismo de sesión `app.current_organization_id` + `app.current_user_id` (sección 2.5), en lugar del `app.organization_id` del prompt. Cada transacción ejecutará:
  ```sql
  select set_config('app.current_organization_id', $1, true),
         set_config('app.current_user_id',         $2, true);
  ```
- El catálogo `core.roles` y sus códigos.
- `audit.audit_events` para la auditoría. El antes y el después van en `payload` como `{"before":…, "after":…, "reason":…}`.
- `integration.idempotency_keys` para la idempotencia. **No se crea `core.idempotency_keys`.**
- `integration.outbox_messages`: el catálogo de eventos (arquitectura 6.2) **no define eventos de Platform**, así que no se escribe outbox. Queda como TODO.
- `organization_users.status = 'suspended'` como "desactivar miembro" (baja lógica).
- `branches.is_active` para la baja lógica de sucursales.

## 4. Diferencias entre el prompt P3 y la base (manda la base)

| # | El prompt asume | La base tiene | Decisión propuesta |
|---|---|---|---|
| 1 | `app.organization_id` | `app.current_organization_id` + `app.current_user_id` | Usar lo existente (ADR 0003) |
| 2 | Rol `billing_clerk` | `biller` | Usar `biller` |
| 3 | Un rol por membresía | Tabla `organization_user_roles` (N roles) | La API expone `roles: []` (mínimo 1). Ver pregunta P1 |
| 4 | `core.users` 1:1 con `auth.users` por id | Enlace por `(identity_provider, external_subject)` con id propio | `identity_provider = 'supabase'`, `external_subject = sub`. Alta perezosa en `GET /v1/me` (no se puede poner un trigger en `auth.users`: el schema es de Supabase) |
| 5 | `core.idempotency_keys` | `integration.idempotency_keys` con `organization_id NOT NULL` | Reutilizar. Para `POST /v1/organizations` ver sección 5.4 |
| 6 | Membresía "desactivada" | `status = 'suspended'` | Reutilizar `suspended` |
| 7 | Historial `core.goose_db_version` | Historial en `supabase_migrations` | Baseline goose idempotente. Desde ahí, `core` y `subscriptions` solo se migran desde este repo |
| 8 | Conexión como `platform_app` | `platform_app` es NOLOGIN | Hace falta un rol de login miembro de `platform_app` → propuesta a `database-platform` (0002) |

## 5. Lo que no cumple o genera riesgo

1. **`users_platform` = `true` (ALL).** Cualquier sesión de `platform_app` lee y modifica todos los usuarios, sin importar el tenant. El riesgo es acotado porque solo `platform_app` tiene GRANT sobre `core.users`, pero un bug en la API podría exponer emails de otras organizaciones. **Propuesta:** no bloquearlo en F1. Endurecerlo en el incremento 8 con una política `id = current_user_id() OR miembro de la org actual` y una función `security definer` para resolver el `sub` → `core.users` en el login. Se deja como decisión abierta.
2. **`organizations_insert` = `true`.** Es aceptable: crear una organización es una acción de cualquier autenticado. Hay una consecuencia técnica: `INSERT … RETURNING` fallaría, porque la política de SELECT exige org actual o membresía. **Solución en la aplicación:** el id se genera en Go y se fija `app.current_organization_id = <id nuevo>` dentro de la transacción de alta, antes del INSERT.
3. **`idempotency_keys.organization_id NOT NULL`** impide guardar la clave de `POST /v1/organizations`, porque la organización todavía no existe. **Solución propuesta (sin tocar `integration`):** el id de la nueva organización es **determinista**, `uuidv5(namespace_platform, core.users.id || ':' || Idempotency-Key)`. La fila de idempotencia se guarda con ese `organization_id`. Un reintento produce el mismo id y encuentra la respuesta guardada. Otro usuario con la misma clave produce otro id. La alternativa es que `database-platform` permita `organization_id` nulo para comandos previos al tenant (ver 0002, opcional).
4. **`organization_user_roles`** no tiene `created_at` ni `updated_at`. Se acepta: sus filas son inmutables (se insertan y se borran), y `granted_at` + `audit_events` dan la trazabilidad. No se propone cambio.
5. **`audit_events.organization_id` admite nulos.** Se acepta para eventos globales (por ejemplo, el alta de un usuario). Platform siempre lo llenará cuando haya tenant.
6. **`organizations.identification_type_code` no tiene FK ni validación** contra `fiscal.identification_types`. Es correcto no cruzar schemas. La API validará solo formato no vacío y dejará un **TODO de contrato** con los códigos válidos (no se inventan).
7. **`postgres` es miembro de todos los `*_app`.** Por eso `shared.current_service()` devuelve `'platform'` para `postgres`. Los tests de aislamiento **nunca** deben correr como `postgres`: usarán el rol de login real.
8. **JWKS:** `…/auth/v1/.well-known/jwks.json` solo sirve claves si el proyecto usa *JWT Signing Keys* asimétricas. Con el secreto HS256 legado, el JWKS está vacío. **Hay que confirmarlo en Dashboard → Settings → JWT Keys** (pregunta P4).

## 6. Lo que falta y propongo crear

Las migraciones son goose en `migrations/` y la tabla de historial es `core.goose_db_version`. Todas corren como `platform_migrator`: sin cambios destructivos y solo en `core`.

### 6.1 `00001_baseline.sql`

Reproduce **literalmente** `0002_core_subscriptions` + la parte de `subscriptions` de `0008_catalog_policies`, con guardas:

- `create table if not exists`, `create index if not exists`;
- políticas y triggers dentro de `do $$ … if not exists (select 1 from pg_policies/pg_trigger …) $$`;
- `insert into core.roles … on conflict (code) do nothing`.

Sobre la base dev actual es un **no-op**: se ejecuta y queda registrada en `core.goose_db_version`. En `testcontainers` (base vacía con `supabase/postgres`) crea todo. Para eso los tests aplicarán antes un *fixture* con el bootstrap de `database-platform` (roles, `shared`, `audit` e `integration`), que este repo **no** versiona como migración.

### 6.2 `00002_users_active_organization.sql`

```sql
-- +goose Up
alter table core.users
  add column if not exists active_organization_id uuid references core.organizations (id);
create index if not exists users_active_organization_idx on core.users (active_organization_id);
comment on column core.users.active_organization_id is
  'Organización elegida por el usuario. El hook de Auth la revalida contra una membresía activa antes de emitir org_id.';

-- +goose Down
drop index if exists core.users_active_organization_idx;
alter table core.users drop column if exists active_organization_id;
```

**Por qué una columna y no otra tabla:** la relación es 1:1 con el usuario, se lee en cada emisión de token y ya existe el trigger de `updated_at`. Guardarla aquí no concede acceso: el hook y el middleware revalidan la membresía.

### 6.3 `00003_custom_access_token_hook.sql`

```sql
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
```

- **`security definer`** con dueño `platform_migrator`, que es dueño de las tablas y por tanto no pasa por RLS. Así `supabase_auth_admin` no necesita GRANT sobre ninguna tabla de `core` ni políticas nuevas. Es la variante que muestra la guía oficial de Auth Hooks (`security definer` + `search_path = ''`).
- Si no hay membresía activa, el token sale **sin** `org_id`. Las rutas `/current` responden 403 con `type` `…/no-active-organization`.
- **Paso manual (lo haces tú):** Dashboard → Authentication → Hooks → *Customize Access Token* → `core.custom_access_token_hook`.
- Los claims del token son **informativos**. El middleware revalida membresía y roles contra la BD en cada request (caché de 30 s por `(user, org)`), así que suspender a un miembro surte efecto sin esperar a que expire el JWT.

### 6.4 `00004_owner_invariant.sql`

Es una defensa en profundidad. La regla principal vive en el dominio (`membership`).

```sql
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
      using errcode = 'check_violation';
  end if;
  return null;
end;
$$;
-- +goose StatementEnd

create constraint trigger organization_users_owner_ck
  after insert or update or delete on core.organization_users
  deferrable initially deferred
  for each row execute function core.ensure_active_owner();

create constraint trigger organization_user_roles_owner_ck
  after update or delete on core.organization_user_roles
  deferrable initially deferred
  for each row execute function core.ensure_active_owner();

-- +goose Down
drop trigger if exists organization_user_roles_owner_ck on core.organization_user_roles;
drop trigger if exists organization_users_owner_ck on core.organization_users;
drop function if exists core.ensure_active_owner();
```

Es `deferred` para que el alta (organización → membresía → rol `owner`) y los cambios de rol en varios pasos se evalúen al hacer COMMIT. Corre como invocador, así que bajo RLS solo ve la organización de la sesión, que es la misma de la fila modificada.

## 7. Diseño de tenancy resultante (resumen; detalle en ADRs)

| Flujo | Sesión de BD | Por qué pasa RLS |
|---|---|---|
| `GET /v1/me` (upsert de `core.users`) | solo `current_user_id` tras resolverlo | `users_platform` |
| `GET /v1/me/memberships` | `current_user_id` | `organization_users_own`, `organization_user_roles_own`, `organizations_read` |
| `PUT /v1/me/active-organization` | `current_user_id` | valida membresía activa vía `_own`; escribe `users.active_organization_id` |
| `POST /v1/organizations` | `current_org = uuidv5(...)` + `current_user_id` | `organizations_insert` + políticas `_tenant` sobre el id nuevo |
| Rutas `/v1/organizations/current/...` | `current_org` del **JWT verificado** + `current_user_id` | políticas `_tenant` |
| `POST /v1/invitations/{token}/accept` | 1) `current_user_id` → lee la invitación por `token_hash` (`invitations_invitee`: el email debe coincidir); 2) `current_org = invitation.organization_id` → inserta la membresía y marca la invitación | La autoridad sale del token de un solo uso + el email del usuario autenticado, nunca de un id enviado por el cliente |

La conexión usa pgx con `QueryExecModeSimpleProtocol`, necesario con Supavisor en modo transacción (puerto 6543). Las migraciones usan la conexión directa o el modo sesión (5432).

## 8. Preguntas para aprobar

- **P1.** ¿Roles múltiples por membresía (`roles: []`, como modela la BD) o se restringe V1 a uno solo aunque la tabla permita N?
- **P2.** ¿Se aprueba el id determinista (`uuidv5`) para la idempotencia de `POST /v1/organizations`, o prefieres pedir a `database-platform` que `idempotency_keys.organization_id` admita nulos?
- **P3.** ¿Se aprueba `00004_owner_invariant` (trigger en BD) además de la regla en el dominio?
- **P4.** ¿El proyecto usa *JWT Signing Keys* asimétricas (JWKS con claves)? Si no, hay que migrarlas antes del incremento 2, porque el prompt exige JWKS.
- **P5.** ¿Este proyecto Supabase es **dev**? ¿Cuál es el `project-ref` y qué versión de Go uso (propongo 1.23+)?
- **P6.** ¿Se aprueba la propuesta 0002 de roles de login para enviarla a `database-platform`?
- **P7.** El endurecimiento de `users_platform` (sección 5.1), ¿se hace en el incremento 8 o se deja como decisión abierta del equipo?

## 9. TODOs de contrato (no se inventan)

- Códigos válidos de `identification_type_code` y formato de `identification_number` por tipo.
- Eventos de Platform (`OrganizationCreated`, `MemberAdded`…): no están en el catálogo 6.2, así que no se escriben en el outbox.
- OpenAPI de Platform en el repo `contracts`: no disponible. Se escribe `api/openapi.yaml` local y se alineará cuando exista.
