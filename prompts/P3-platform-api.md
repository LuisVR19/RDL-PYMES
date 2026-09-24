# Prompt · P3 Platform API (identidad y tenancy) en Go

> **Cómo usarlo**
> 1. Crea el repo `platform-api` vacío y copia en `docs/contexto/` el documento de arquitectura y el planning.
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano todo lo que toque RLS, roles de BD y permisos.

---

## Rol y objetivo

Eres un ingeniero backend senior en Go. Vas a construir la **Platform API** de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. Esta API es dueña de la **identidad, la tenancy, los usuarios, las membresías, los roles y las sucursales**. Es la base de la que dependen Billing, E-Invoice y Receivables para aislar los datos de cada organización. Si esta API se equivoca, un cliente puede ver los datos de otro, así que la corrección y el aislamiento pesan más que la velocidad.

**Entregable:** el repositorio `platform-api` listo para staging, con el alcance de las fases F1 y F3 del planning. Debe compilar, pasar los tests (incluidos los de aislamiento entre tenants), tener migraciones versionadas, Dockerfile, health checks y documentación.

## Contexto que debes leer primero

- `docs/contexto/arquitectura-v1.md`: arquitectura (tabla 2.1 de ownership, secciones 3, 6.2, 6.3, 11 y 12).
- `docs/contexto/planning-v1.md`: sección **P3 Platform API** y "Cómo trabajamos con Claude Code".
- Repo de contratos, si existe en `<<ruta o URL del repo contracts>>`: glosario, convenciones, OpenAPI de Platform y catálogo de eventos. **El contrato manda.** Si el contrato y este prompt se contradicen, detente y pregunta.

## Datos del entorno

- Proyecto Supabase (dev): `<<project-ref>>`. **Nunca producción.**
- Conexión de solo lectura para inspeccionar: MCP de Supabase/Postgres en modo read-only, o `<<DATABASE_URL_READONLY>>`.
- Conexión para migrar en dev: `<<DATABASE_URL_MIGRATOR>>` (rol `platform_migrator` o equivalente).
- Conexión de la aplicación: rol `platform_app` por el pooler de Supavisor en **modo transacción** (puerto 6543).
- Proveedor de identidad: **Supabase Auth**. Los JWT se validan con JWKS (`https://<<project-ref>>.supabase.co/auth/v1/.well-known/jwks.json`).
- Versión de Go: `<<1.23+>>`.

## Paso 1 · Inspeccionar la base antes de escribir código (obligatorio)

La base ya fue creada en Supabase. **No supongas el esquema: léelo.** Con la conexión de solo lectura:

1. Lista los schemas, las tablas, las columnas, los tipos, las PK, las FK, las unicidades y los índices de `core`, `subscriptions`, `audit` e `integration`. Revisa también lo relevante de `auth` (`auth.users`).
2. Lista los roles y sus GRANT por schema. Comprueba si existen `platform_app` y `platform_migrator`, y qué pueden hacer.
3. Lista las políticas RLS, qué tablas tienen RLS habilitado y si hay funciones o variables de sesión para la organización activa (por ejemplo `current_setting('app.organization_id')`).
4. Lista los triggers y funciones existentes, incluidos los hooks de Supabase Auth, si los hay.
5. Revisa si existe una tabla de historial de migraciones.

Entrega un **informe de brechas** en `docs/decisiones/0001-estado-inicial-bd.md` con:
- lo que ya existe y vas a **reutilizar tal cual**;
- lo que existe pero **no cumple** las reglas de este prompt (tabla sin `organization_id`, sin FK compuesta, sin RLS, uso de `float`, `timestamp` sin zona, etc.);
- lo que **falta** y propones crear, con el SQL exacto.

**Detente y espera mi aprobación del informe antes de crear o modificar cualquier cosa en la base.**

## Paso 2 · Cambios de base de datos (solo si hacen falta)

- Herramienta: **goose** (`pressly/goose`), con las migraciones SQL en `migrations/` y la tabla de historial dentro del schema `core` (`core.goose_db_version`).
- La **primera migración es una baseline** que refleja lo que ya existe en la base, escrita de forma idempotente (`IF NOT EXISTS`) y marcada como aplicada en dev. Así el repo pasa a ser la fuente de verdad de `core` y `subscriptions`.
- Esta API **solo migra `core` y `subscriptions`**. Nunca generes DDL ni DML sobre `billing`, `fiscal` o `receivables`. Lo que haga falta en `audit` o `integration` (propiedad compartida) va como propuesta separada en `docs/decisiones/` para aprobarla en el repo `database-platform`, no como migración de este repo.
- Nada destructivo. Si hay que modificar algo existente, usa el patrón **expand → migrate → contract**, cada paso en su propia migración.
- Cada tabla de negocio lleva:
  - `organization_id uuid NOT NULL`;
  - PK `id uuid DEFAULT gen_random_uuid()`;
  - `UNIQUE (organization_id, id)` y FK compuestas `(organization_id, x_id)`;
  - `created_at` y `updated_at` como `timestamptz`;
  - RLS habilitado con una política basada en la organización activa de la sesión.
- Tablas esperadas en `core`, si no existen o les falta algo: `organizations`, `users` (1:1 con `auth.users`), `organization_users` (membresía, rol y estado), `branches`, `invitations`, `idempotency_keys`. En la tabla de usuarios, la organización activa puede guardarse en `users.active_organization_id` o en una tabla aparte. Propón la opción en el plan.
- Aplica las migraciones **solo en dev y en local**, y muéstrame el SQL antes de aplicarlo.

## Paso 3 · Diseño de tenancy (lo más importante)

**Regla de oro:** la organización activa sale **del token verificado**, nunca del body, la query, un header ni la ruta.

Flujo propuesto (valídalo contra la documentación oficial de Supabase, que es la fuente de verdad sobre la sintaxis, y no la inventes):
1. El usuario hace login con Supabase Auth.
2. `GET /v1/me/memberships` devuelve sus organizaciones y su rol en cada una.
3. `PUT /v1/me/active-organization` valida la membresía activa y guarda la selección.
4. Un **Custom Access Token Hook** de Supabase (una función en Postgres) incluye `org_id` y `org_role` en el JWT a partir de la selección guardada. El cliente refresca la sesión y el nuevo token ya trae la organización.
5. En **cada request**, el middleware:
   - verifica la firma (JWKS con caché), `exp`, `aud` e `iss`;
   - extrae `sub`, `org_id` y `org_role`;
   - **vuelve a validar que la membresía siga activa** (con caché corta, de 30 a 60 s como máximo);
   - construye un `TenantContext` inmutable en el `context.Context`.
6. Cada transacción ejecuta `SELECT set_config('app.organization_id', $1, true)` (equivale a `SET LOCAL`) antes de cualquier consulta, para que RLS aplique.

Si al inspeccionar la base encuentras un mecanismo de sesión RLS distinto ya definido, **úsalo** y documenta la decisión en un ADR.

Con Supavisor en modo transacción, configura pgx con `QueryExecModeSimpleProtocol` (o `Exec`) para no usar prepared statements cacheados, que se rompen con ese pooler. Documenta el porqué.

## Paso 4 · Alcance funcional

Base de rutas: `/v1`. Las rutas que operan dentro de la organización usan `/v1/organizations/current/...`, de modo que el `id` de organización nunca viaja en la URL.

**F1: identidad y tenancy**
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `GET /v1/me` | Perfil del usuario autenticado. Crea el registro en `core.users` si no existe (o por trigger desde `auth.users`; elige según lo que ya haya en la base). | cualquiera autenticado |
| `GET /v1/me/memberships` | Organizaciones del usuario, con rol y estado. | autenticado |
| `PUT /v1/me/active-organization` | Selecciona la organización activa. 404 si no es miembro activo. | autenticado |
| `POST /v1/organizations` | Crea la organización y deja al creador como `owner`. Idempotente. | autenticado |
| `GET /v1/organizations/current` | Datos de la organización activa. | todos los roles |
| `PATCH /v1/organizations/current` | Actualiza nombre comercial, zona horaria, etc. | owner, admin |
| `GET /v1/organizations/current/users` | Lista paginada de miembros. | owner, admin |
| `PATCH /v1/organizations/current/users/{userId}` | Cambia el rol o desactiva al miembro (baja lógica). | owner, admin |

**F3: usuarios, invitaciones y sucursales**
| Método y ruta | Qué hace | Roles |
|---|---|---|
| `POST /v1/organizations/current/invitations` | Invita por email con un rol. El token de un solo uso se guarda como hash y expira. | owner, admin |
| `GET /v1/organizations/current/invitations` | Invitaciones pendientes. | owner, admin |
| `DELETE /v1/organizations/current/invitations/{id}` | Revoca una invitación. | owner, admin |
| `POST /v1/invitations/{token}/accept` | Acepta la invitación y crea la membresía. Soporta a un usuario con varias organizaciones (caso contador). | autenticado |
| `GET/POST /v1/organizations/current/branches` | Lista y crea sucursales. | lectura: todos; escritura: owner, admin |
| `GET/PATCH /v1/organizations/current/branches/{id}` | Detalle, edición y desactivación. | igual |

**Fuera de alcance:** `subscriptions` (F5), el cobro del SaaS y la configuración fiscal (pertenece a E-Invoice). No inventes campos fiscales en `organizations`: guarda solo la identificación básica (tipo y número) y deja un TODO donde el contrato no defina más.

**Roles iniciales:** `owner`, `admin`, `billing_clerk` (facturador), `collector` (cobrador), `accountant` (contador), `read_only`. La matriz de permisos vive en **un solo lugar** del código (una tabla de política en el dominio), no repartida en `if` por los handlers. Regla adicional: una organización nunca puede quedarse sin al menos un `owner` activo.

**Transversales en todos los endpoints:**
- Errores con **Problem Details (RFC 9457)**, `application/problem+json`, con un `type` estable por error.
- Los comandos `POST` exigen el header `Idempotency-Key`. Misma clave y mismo body devuelven la misma respuesta; misma clave con otro body devuelve 422.
- `X-Correlation-Id`: se lee o se genera, se propaga a los logs, las trazas y los audit events, y se devuelve en la respuesta.
- **Cada operación sensible escribe un audit event** (quién, organización, acción, entidad, antes y después, correlationId) **en la misma transacción** que el cambio.
- Si el catálogo de contratos define eventos de Platform (por ejemplo `OrganizationCreated` o `MemberAdded`), escríbelos en el outbox **en la misma transacción**. Si no los define, no inventes eventos: deja un TODO.
- Paginación por cursor (`limit`, `cursor`).
- Recursos de otra organización: **404** (no reveles que existen). Rol insuficiente dentro de tu organización: **403**.

## Paso 5 · Arquitectura y calidad de código

Arquitectura hexagonal (ports & adapters) con Clean Architecture. Las dependencias apuntan siempre hacia el dominio.

```
platform-api/
├── cmd/api/main.go              # composition root: config, wiring, arranque; nada de lógica
├── internal/
│   ├── domain/                  # entidades, value objects, reglas, errores de dominio; sin imports de infra
│   │   ├── organization/
│   │   ├── membership/          # roles, matriz de permisos, invariante "al menos un owner"
│   │   ├── branch/
│   │   └── invitation/
│   ├── app/                     # casos de uso (un struct por caso de uso) + puertos (interfaces)
│   ├── adapters/
│   │   ├── http/                # handlers, DTOs, validación, mapeo error→Problem Details, router
│   │   ├── postgres/            # repositorios (pgx + sqlc), unidad de trabajo con SET LOCAL de tenant
│   │   └── auth/                # verificación de JWT de Supabase con JWKS
│   └── platform/                # config, logger, OTel, health, middleware (tenancy, correlation, idempotencia)
├── pkg/tenancy/                 # TenantContext + middleware reutilizable (irá luego a building-blocks)
├── migrations/                  # goose
├── queries/                     # SQL para sqlc
├── tests/{integration,isolation}/
├── docs/decisiones/             # ADRs
├── api/openapi.yaml
├── CLAUDE.md, Dockerfile, Makefile, .golangci.yml, sqlc.yaml
```

**Stack:** `net/http` con el router de Go 1.22+ (o `chi` si lo justificas en un ADR), `pgx/v5` + `pgxpool`, `sqlc` para las consultas tipadas, `goose`, `log/slog` con JSON, OpenTelemetry (trazas y métricas), `lestrrat-go/jwx` o `golang-jwt` + `keyfunc` para JWKS, `go-playground/validator`, `testcontainers-go` y `golangci-lint`. Ninguna otra dependencia sin justificarla.

**Principios que se tienen que notar en el código:**
- **SRP:** un caso de uso por struct; los handlers solo traducen HTTP ↔ caso de uso.
- **OCP/LSP:** los nuevos roles o permisos se agregan en la matriz sin tocar los handlers.
- **ISP:** interfaces pequeñas, **definidas del lado del consumidor** (en `app`) y no del implementador.
- **DIP:** los casos de uso dependen de puertos; los adapters los implementan; todo el wiring vive en `main.go` con inyección por constructor. Sin estado global y sin `init()` con efectos.
- Unidad de trabajo transaccional (`TxManager.WithinTenantTx(ctx, fn)`) que fija el tenant, ejecuta la operación y escribe audit y outbox de forma atómica.
- `context.Context` como primer parámetro en toda la cadena; timeouts en el servidor y en la BD; graceful shutdown.
- Errores de dominio tipados con `errors.Is`/`As` y mapeados a HTTP en un único lugar. No filtres errores internos al cliente.
- Configuración por variables de entorno validadas al arrancar (fallar rápido). Cero secretos en el código o en el repo.
- Fechas en UTC (`timestamptz`); la presentación usa la zona horaria de la organización.
- Nombres claros, funciones cortas, sin comentarios obvios; comenta el **porqué**, no el qué.

## Paso 6 · Tests (escríbelos desde los requisitos, no desde la implementación)

- **Unitarios** del dominio: matriz de permisos, invariante de owner, expiración y uso único de las invitaciones, idempotencia.
- **Integración** con `testcontainers-go` usando la imagen `supabase/postgres`, aplicando las migraciones reales y conectando con el rol `platform_app` (no con superusuario).
- **Aislamiento (criterio de terminado del planning):**
  1. Con un token de la organización A, cualquier endpoint sobre un recurso de B devuelve 404/403. Recorre todos los endpoints en un test tabla.
  2. Con la sesión en A, un `SELECT` directo con `platform_app` no devuelve filas de B (RLS).
  3. Una FK hacia un registro de otra organización se rechaza.
  4. `platform_app` no puede escribir en `billing`, `fiscal` ni `receivables`, ni hacer `UPDATE`/`DELETE` en `audit`.
  5. Un token sin `org_id`, o con una membresía desactivada, no accede a las rutas `/current`.
  6. Enviar un `organization_id` en el body o en un header no cambia de tenant.
- **Handlers HTTP** con `httptest`: camino feliz, validación y cross-tenant para cada endpoint, más reintentos con la misma `Idempotency-Key`.
- Si un test de aislamiento falla por falta de una política en la base, **no debilites el test**: repórtalo y propone la migración.

## Paso 7 · Operación y entrega

- `GET /healthz` (vivo) y `GET /readyz` (BD y JWKS accesibles).
- Dockerfile multi-stage con imagen final distroless o scratch y usuario no root.
- `Makefile` con: `run`, `build`, `test`, `test-isolation`, `lint`, `migrate-up`, `migrate-status`, `sqlc`.
- `api/openapi.yaml` sincronizado con los handlers. Si hay repo de contratos, alinéalo con su OpenAPI de Platform.
- `CLAUDE.md` del repo con el bloque común de reglas de plataforma, adaptado a Go (`go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...` y los tests de aislamiento antes de terminar cada tarea).
- `README.md` con cómo levantarlo en local, las variables de entorno y cómo correr los tests.
- ADRs cortos en `docs/decisiones/` para cada decisión: mecanismo de organización activa, pooler y pgx, router, estrategia de idempotencia.

## Forma de trabajo

1. **Plan primero:** informe de brechas de la BD, plan de migraciones, diseño de tenancy y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por incrementos verticales**, cada uno compilando y con sus tests en verde:
   1. esqueleto, config, health y observabilidad;
   2. verificación de JWT y TenantContext;
   3. `me`, memberships y organización activa;
   4. organizaciones;
   5. miembros y roles;
   6. invitaciones;
   7. sucursales;
   8. endurecimiento.
3. Al terminar cada incremento: `go build ./... && go vet ./... && golangci-lint run && go test ./...`, un resumen de lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente, `float` para montos, `timestamp` sin zona, secretos y errores internos expuestos.

## Prohibido

- Conectarte a producción, usar la `service_role` key en la aplicación o usar certificados o datos reales.
- Tomar el tenant de algo que no sea el token verificado.
- Desactivar RLS o debilitar un test para que pase.
- Migrar schemas que no son de esta API o hacer cambios destructivos sin expand/contract.
- Inventar campos fiscales, eventos o reglas que no estén en el contrato: en su lugar, deja un TODO y lístalos al final.

## Definición de terminado

- [ ] Informe de brechas aprobado y migraciones aplicadas en dev, con la baseline incluida.
- [ ] Todos los endpoints de F1 y F3 implementados, documentados en OpenAPI y con tests.
- [ ] Suite de aislamiento en verde y ejecutable con `make test-isolation` (lista para el CI de las otras APIs).
- [ ] Audit event en cada operación sensible; idempotencia en cada comando; correlationId de punta a punta.
- [ ] `golangci-lint` limpio, cobertura razonable en dominio y casos de uso, imagen Docker construida y health checks respondiendo.
- [ ] Lista final de TODOs y decisiones abiertas para revisar en equipo.
