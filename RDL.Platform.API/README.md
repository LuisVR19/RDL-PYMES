# Platform API

Servicio de **identidad y tenancy** del SaaS de facturación electrónica para PYMES. Gestiona organizaciones (tenants),
usuarios, membresías, roles, sucursales e invitaciones. Los demás servicios (facturación, fiscal, cobros) confían
en lo que esta API decide: quién es el usuario, en qué organización trabaja y qué rol tiene.

- Lenguaje: Go 1.27, `net/http` estándar (sin framework), pgx v5 y sqlc.
- Base de datos: Postgres de Supabase. Esta API es dueña de los schemas `core` y `subscriptions`.
- Autenticación: JWT de Supabase Auth (ES256, verificado con el JWKS del proyecto).
- Observabilidad: logs JSON (`slog`) con `correlationId` y OpenTelemetry (OTLP, opcional).

---

## Índice

1. [Cómo funciona](#cómo-funciona)
2. [Endpoints](#endpoints)
3. [Roles y permisos](#roles-y-permisos)
4. [Levantar la API por primera vez](#levantar-la-api-por-primera-vez)
5. [Comandos del día a día](#comandos-del-día-a-día)
6. [Probar la API a mano](#probar-la-api-a-mano)
7. [Pruebas](#pruebas)
8. [Variables de entorno](#variables-de-entorno)
9. [Estructura del código](#estructura-del-código)
10. [Problemas comunes](#problemas-comunes)
11. [Documentación relacionada](#documentación-relacionada)

---

## Cómo funciona

### Organización activa (tenant)

1. El usuario inicia sesión en **Supabase Auth** y recibe un JWT.
2. El *Custom Access Token Hook* (`core.custom_access_token_hook`) agrega al token el claim `org_id` (su organización
   activa) y `org_roles`.
3. En cada request la API:
   - verifica la firma, la expiración, la audiencia y el emisor del JWT;
   - toma la organización **solo** del `org_id` del token. Nunca de un body, query o header;
   - revalida en la base que la membresía siga activa (con una caché corta). Así, suspender a alguien surte efecto de
     inmediato, sin esperar a que venza su token.
4. Para cambiar de organización, el cliente llama a `PUT /v1/me/active-organization` y **refresca el token**, así el
   hook emite el `org_id` nuevo.

### Aislamiento en la base (RLS)

- Toda consulta corre en una transacción que fija `app.current_organization_id` y `app.current_user_id` con
  `set_config(..., true)`, que dura solo lo que dura la transacción (compatible con Supavisor en modo transacción).
- Las políticas RLS de Postgres filtran por esas variables. Aunque el código tuviera un bug, la base no devuelve
  filas de otra organización.
- La API se conecta con el login `platform_api`, que hereda `platform_app`: sin `BYPASSRLS` y sin ser dueño de las
  tablas.
- Un recurso de otra organización responde **404**. Un rol insuficiente dentro de la propia responde **403**.

### Convenciones de la API

- **Errores:** [Problem Details](https://www.rfc-editor.org/rfc/rfc9457) (`application/problem+json`) con `type` del
  estilo `urn:rdl:platform:problem:last-owner`. Los 500 nunca exponen el detalle interno, que queda en el log.
- **Idempotencia:** los POST que crean algo exigen el header `Idempotency-Key`. Misma clave y mismo cuerpo devuelven
  la misma respuesta. Misma clave con otro cuerpo devuelve 422.
- **Paginación:** listados con `?limit=` (1–100) y `nextCursor` opaco.
- **Correlación:** cada respuesta trae `X-Correlation-Id`. Si el cliente envía un UUID, se respeta; cualquier otro valor
  se reemplaza.
- **Auditoría:** cada operación sensible escribe en `audit.audit_events` dentro de la misma transacción del cambio.
- **Fechas:** siempre UTC (`timestamptz`). La zona horaria de la organización se usa solo para presentar datos.

El contrato completo está en [`api/openapi.yaml`](api/openapi.yaml).

---

## Endpoints

| Método | Ruta | Qué hace | Quién |
|---|---|---|---|
| GET | `/healthz` | El proceso está vivo | público |
| GET | `/readyz` | La base y el JWKS responden (503 si no) | público |
| GET | `/v1/me` | Perfil del usuario. La primera vez lo da de alta en `core.users` | autenticado |
| GET | `/v1/me/memberships` | Organizaciones donde tiene membresía activa | autenticado |
| PUT | `/v1/me/active-organization` | Elige la organización activa (luego hay que refrescar el token) | autenticado |
| POST | `/v1/organizations` | Crea una organización; el creador queda como `owner`. Requiere `Idempotency-Key` | autenticado |
| POST | `/v1/invitations/{token}/accept` | Acepta una invitación dirigida a su email. Requiere `Idempotency-Key` | autenticado |
| GET | `/v1/organizations/current` | Datos de la organización activa | todos los roles |
| PATCH | `/v1/organizations/current` | Edita contacto, nombre comercial y zona horaria | owner, admin |
| GET | `/v1/organizations/current/users` | Lista los miembros | owner, admin |
| PATCH | `/v1/organizations/current/users/{userId}` | Cambia el rol o suspende/reactiva a un miembro | owner, admin |
| POST | `/v1/organizations/current/invitations` | Invita por email con un rol. Requiere `Idempotency-Key` | owner, admin |
| GET | `/v1/organizations/current/invitations` | Lista invitaciones (`?status=pending`, etc.) | owner, admin |
| DELETE | `/v1/organizations/current/invitations/{id}` | Revoca una invitación (idempotente) | owner, admin |
| GET | `/v1/organizations/current/branches` | Lista sucursales (`?active=true/false`) | todos los roles |
| POST | `/v1/organizations/current/branches` | Crea una sucursal. Requiere `Idempotency-Key` | owner, admin |
| GET | `/v1/organizations/current/branches/{id}` | Detalle de una sucursal | todos los roles |
| PATCH | `/v1/organizations/current/branches/{id}` | Edita o desactiva una sucursal | owner, admin |

Las rutas `/v1/organizations/current/*` exigen que el token traiga `org_id` y que la membresía esté activa. Si no:
- sin `org_id` → 403 `no-active-organization`;
- con la membresía suspendida → 403 `membership-inactive`.

---

## Roles y permisos

Roles del catálogo `core.roles`: `owner`, `admin`, `biller`, `collector`, `accountant`, `read_only`. En V1 cada
membresía tiene **un** rol.

| Permiso | owner | admin | biller, collector, accountant, read_only |
|---|:-:|:-:|:-:|
| Ver organización y sucursales | ✅ | ✅ | ✅ |
| Editar organización | ✅ | ✅ | — |
| Ver y gestionar miembros | ✅ | ✅ | — |
| Invitar y revocar invitaciones | ✅ | ✅ | — |
| Crear y editar sucursales | ✅ | ✅ | — |

Reglas adicionales:
- Solo un `owner` asigna, degrada o suspende a otro `owner`, o invita con rol `owner`.
- Una organización nunca se queda sin un owner activo. Lo impiden el dominio y un trigger en la base (409 `last-owner`).

La matriz vive solo en `internal/domain/membership/permissions.go`.

---

## Levantar la API por primera vez

### 1. Requisitos

| Herramienta | Para qué | Obligatoria |
|---|---|---|
| Go 1.27+ | compilar y correr | sí |
| Git Bash (en Windows) | `scripts/dev/e2e.sh` y `make` | para E2E |
| Python 3 | lo usa `e2e.sh` para leer JSON | para E2E |
| `golangci-lint` v2 | lint | recomendado |
| `sqlc` | regenerar `internal/adapters/postgres/db` tras cambiar `queries/` | solo si cambia SQL |
| Docker | construir la imagen | no |

También necesita acceso al proyecto Supabase de **dev** (`dzlsnsstuqpxvwegeqcy`). **Nunca producción.**

### 2. Preparar la base (una sola vez por ambiente)

Si el ambiente ya está preparado (como dev hoy), salte este paso.

1. Crear los logins de la API con [`scripts/dev/0009_platform_login_roles.sql`](scripts/dev/0009_platform_login_roles.sql)
   (SQL Editor del Dashboard) y asignarles contraseña:
   ```sql
   alter role platform_api     password '...';
   alter role platform_migrate password '...';
   ```
2. Aplicar las migraciones (paso 4).
3. Activar el hook: Dashboard → **Authentication → Hooks → Customize Access Token** → `core.custom_access_token_hook`.

### 3. Configurar `.env`

```sh
cp .env.example .env
```

Complete solo tres valores:

| Variable | De dónde sale |
|---|---|
| `DB_POOLER_HOST` | Dashboard → botón **Connect** → pestaña **Session pooler**. Sirve el host (`aws-0-<región>.pooler.supabase.com`) o la cadena completa |
| `DB_PASSWORD` | Contraseña de `platform_api` |
| `MIGRATE_DB_PASSWORD` | Contraseña de `platform_migrate` |

- Las contraseñas van **entre comillas simples** y fuera de la URL, así que pueden tener `@`, `#` o `&`.
- La API y el migrador leen el `.env` solos. Las variables del entorno tienen prioridad sobre el archivo.
- Con eso se arman las conexiones:
  - la API usa `platform_api.<ref>@<host>:6543` (Supavisor en modo transacción);
  - el migrador usa `platform_migrate.<ref>@<host>:5432` (modo sesión).
- `.env` está en `.gitignore`. **Nunca** ponga ahí la `service_role` key.

### 4. Migraciones

```sh
go run ./cmd/migrate status   # debe mostrar 00001 a 00006 como "aplicada"
go run ./cmd/migrate up       # aplica las pendientes
```

### 5. Arrancar

```sh
go run ./cmd/api
```

Queda escuchando en `:8080`. Para comprobarlo:

```sh
curl http://127.0.0.1:8080/healthz   # 200
curl http://127.0.0.1:8080/readyz    # 200 si la base y el JWKS responden
```

---

## Comandos del día a día

| Comando | Qué hace |
|---|---|
| `go run ./cmd/api` | Levanta la API en `:8080` |
| `go run ./cmd/migrate status` | Estado de las migraciones |
| `go run ./cmd/migrate up` | Aplica todas las migraciones pendientes |
| `go run ./cmd/migrate up-by-one` | Aplica solo la siguiente (para expand → desplegar código → contract) |
| `go run ./cmd/migrate down` | Revierte la última (solo en dev) |
| `go build ./...` | Compila todo |
| `go test ./...` | Tests unitarios (no necesitan base) |
| `go test -count=1 -tags=integration ./tests/isolation/...` | Suite de aislamiento entre tenants (necesita base) |
| `golangci-lint run --build-tags=integration ./...` | Lint, incluida la suite |
| `sqlc generate` | Regenera el código de `queries/*.sql` |
| `bash scripts/dev/e2e.sh http://127.0.0.1:8080` | Prueba de punta a punta (API corriendo) |
| `docker build -t platform-api:dev .` | Imagen distroless con `api` y `migrate` |

Hay un `Makefile` con atajos (`make run`, `make build`, `make test`, `make test-isolation`, `make lint`, `make sqlc`,
`make migrate-up`). `make` no entiende las comillas del `.env`; si un target necesita las variables:

```sh
set -a; . ./.env; set +a
```

**Antes de dar por terminado un cambio:**

```sh
go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation
```

---

## Probar la API a mano

Necesita un usuario en Supabase Auth de dev. En dev la confirmación de email está desactivada.

```sh
SUPABASE_URL=https://dzlsnsstuqpxvwegeqcy.supabase.co
KEY=sb_publishable_...   # publishable key del proyecto (Dashboard → Settings → API Keys)

# 1. Login: devuelve access_token y refresh_token
curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=password" \
  -H "apikey: $KEY" -H 'Content-Type: application/json' \
  -d '{"email":"usuario@ejemplo.com","password":"..."}'

TOKEN=...   # access_token

# 2. Perfil (lo da de alta la primera vez)
curl -s http://127.0.0.1:8080/v1/me -H "Authorization: Bearer $TOKEN"

# 3. Crear una organización
curl -s -X POST http://127.0.0.1:8080/v1/organizations \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"legalName":"Mi Empresa S.A.","identificationTypeCode":"02","identificationNumber":"3101123456","email":"admin@miempresa.com"}'

# 4. Refrescar el token para que traiga org_id
curl -s -X POST "$SUPABASE_URL/auth/v1/token?grant_type=refresh_token" \
  -H "apikey: $KEY" -H 'Content-Type: application/json' -d '{"refresh_token":"..."}'

# 5. Usar la organización activa
curl -s http://127.0.0.1:8080/v1/organizations/current -H "Authorization: Bearer $TOKEN"
```

Si la organización creada no queda activa (porque el usuario ya tenía otra), llame a
`PUT /v1/me/active-organization` con `{"organizationId":"..."}` y refresque el token.

---

## Pruebas

### Unitarias

```sh
go test ./...
```

Cubren el dominio, los casos de uso (con fakes), los handlers HTTP, la configuración y el verificador de JWT.
No necesitan base ni red.

### Suite de aislamiento (`tests/isolation`)

```sh
make test-isolation
# o: go test -count=1 -tags=integration ./tests/isolation/...
```

- Corre el router real contra la base de dev con el login `platform_api`. Toma la conexión del `.env`, o de
  `TEST_DATABASE_URL` (+ `TEST_DB_PASSWORD`) si está definida.
- Solo simula la verificación del JWT. Todo lo demás es real, incluida la revalidación de membresía.
- Comprueba que una organización no puede ver ni tocar datos de otra: por endpoints, por SQL directo, por FK
  cruzadas, por escrituras en schemas ajenos, con tokens sin organización o suspendidos, y con `organization_id`
  en body, query o header.
- Tarda unos 2 minutos. No borra datos: deja organizaciones `Isolation … S.A.` en dev.
- Sin base configurada, se salta con un aviso.
- Se niega a correr con un rol que salte RLS.

### Punta a punta (`scripts/dev/e2e.sh`)

Usa usuarios reales de Supabase Auth. Cree `.e2e.local` (ignorado por git):

```sh
E2E_EMAIL='usuario.e2e@rdlpymes.com'
E2E_PASSWORD='...'
E2E_EMAIL2='usuario.e2e2@rdlpymes.com'   # opcional: habilita invitaciones, permisos y suspensión
E2E_PASSWORD2='...'
```

Con la API corriendo en otra terminal:

```sh
bash scripts/dev/e2e.sh http://127.0.0.1:8080
```

Recorre login → `/v1/me` → alta idempotente de organización → refresco del token (el hook emite `org_id`) →
organización activa → membresías → miembros → sucursales → invitaciones → permisos por rol → suspensión → token
forjado. Con los dos usuarios da **69/69** y se puede repetir.

---

## Variables de entorno

Solo `SUPABASE_URL`, `DB_POOLER_HOST` (o `DATABASE_URL`) y las contraseñas son obligatorias. El resto tiene valor por
defecto.

| Variable | Defecto | Descripción |
|---|---|---|
| `SUPABASE_URL` | — | URL del proyecto Supabase (dev) |
| `DB_POOLER_HOST` | — | Host de Supavisor; con él se arma `DATABASE_URL` |
| `DB_PASSWORD` | — | Contraseña de `platform_api` |
| `MIGRATE_DB_PASSWORD` | — | Contraseña de `platform_migrate` |
| `DATABASE_URL` | armada con el pooler | Conexión explícita de la API (reemplaza a `DB_POOLER_HOST`) |
| `MIGRATE_DATABASE_URL` | armada con el pooler | Conexión explícita del migrador |
| `DB_STATEMENT_TIMEOUT` | `5s` | `statement_timeout` de cada conexión |
| `HTTP_ADDR` | `:8080` | Dirección de escucha |
| `HTTP_READ_HEADER_TIMEOUT` / `HTTP_READ_TIMEOUT` / `HTTP_WRITE_TIMEOUT` / `HTTP_IDLE_TIMEOUT` | `5s` / `15s` / `30s` / `60s` | Timeouts del servidor |
| `HTTP_SHUTDOWN_TIMEOUT` | `20s` | Espera del apagado ordenado |
| `AUTH_ISSUER` | `$SUPABASE_URL/auth/v1` | `iss` esperado en el JWT |
| `AUTH_JWKS_URL` | `$AUTH_ISSUER/.well-known/jwks.json` | Claves públicas para verificar el JWT |
| `AUTH_AUDIENCE` | `authenticated` | `aud` esperado |
| `AUTH_JWKS_REFRESH` | `15m` | Cada cuánto se refresca el JWKS |
| `AUTH_MEMBERSHIP_CACHE_TTL` | `30s` | Caché de la revalidación de membresía |
| `APP_ENV` | `dev` | Ambiente (va en los logs) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `OTEL_SERVICE_NAME` | `platform-api` | Nombre del servicio en logs y trazas |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | vacío | Endpoint OTLP. Vacío = no exporta |

---

## Estructura del código

Arquitectura hexagonal: el dominio no conoce la infraestructura.

```
cmd/
  api/            main: config, pool, verificador JWT, servidor HTTP
  migrate/        migrador goose (up, up-by-one, down, status)
internal/
  domain/         reglas de negocio puras: organization, membership (permisos), invitation, branch, user
  app/            casos de uso y puertos (TxManager, repositorios, auditoría)
  adapters/
    http/         handlers, router, Problem Details, validación
    postgres/     repositorios, transacciones con sesión RLS; db/ lo genera sqlc
    auth/         verificación del JWT con JWKS
  wiring/         arma los handlers (lo comparten cmd/api y la suite de aislamiento)
  platform/       config (.env), logger, health, telemetría
pkg/
  tenancy/        Identity, TenantContext, middlewares y caché de membresía
  correlation/    X-Correlation-Id
  requestinfo/    IP y user agent para auditoría
queries/          SQL de sqlc
migrations/       migraciones goose (historial en core.goose_db_version)
sqlc/external.sql objetos de otros dueños, solo para tipar sqlc (nunca se aplica)
api/openapi.yaml  contrato HTTP
tests/isolation/  suite de aislamiento entre tenants (build tag integration)
scripts/dev/      e2e.sh y SQL de roles de login
docs/             contexto, decisiones (ADR) y ESTADO.md
```

### Agregar un endpoint

1. Si hay reglas nuevas, van en `internal/domain`. Los permisos, en `membership/permissions.go`.
2. Caso de uso en `internal/app`, usando `WithinTenantTx` y auditando.
3. SQL en `queries/` y luego `sqlc generate`.
4. Handler en `internal/adapters/http`, registrado en `internal/wiring`.
5. Documentarlo en `api/openapi.yaml`.
6. Agregar su caso cruzado en `tests/isolation` y un paso en `e2e.sh`.

### Reglas que no se rompen

- Este servicio solo escribe en `core`, `subscriptions`, `audit.audit_events` e `integration.*`.
- Toda consulta usa el tenant del contexto, nunca un id enviado por el cliente.
- Montos en `numeric`, nunca `float`. Fechas en `timestamptz`.
- `core.users` solo se busca por `sub` o se da de alta mediante `core.find_user_by_subject` y `core.provision_user`.
- Nunca se desactiva RLS ni se debilita un test de aislamiento para que algo pase.

---

## Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| `DB_POOLER_HOST es obligatoria` | Falta en `.env`. Cópielo del Dashboard → Connect → Session pooler |
| `/readyz` responde 503 | La base o el JWKS no responden. El detalle está en el log de la API |
| `la sesión corre como "..."; se esperaba "platform_migrator"` | El migrador no usa el login `platform_migrate` o falta `alter role platform_migrate set role = 'platform_migrator'` |
| `bind: Solo se permite un uso de cada dirección` | Ya hay una API en `:8080`. En Windows: `Get-Process api \| Stop-Process` (PowerShell), o cambie `HTTP_ADDR` |
| `/v1/organizations/current` → 403 `no-active-organization` | El token no trae `org_id`: elija la organización con `PUT /v1/me/active-organization` y refresque el token |
| 403 `membership-inactive` | La membresía está suspendida o la organización no está activa |
| 401 en todo | Token vencido, de otro proyecto o alterado. Vuelva a iniciar sesión |
| El token no trae `org_id` tras refrescar | El hook no está activado en el Dashboard (Authentication → Hooks) |
| `make` falla leyendo variables | Cargue el `.env` antes: `set -a; . ./.env; set +a` |

---

## Documentación relacionada

- [`docs/ESTADO.md`](docs/ESTADO.md): estado del proyecto, decisiones abiertas y TODOs.
- [`docs/decisiones/`](docs/decisiones/): ADRs 0001–0007 (base de datos, sesión RLS, pooler, router, invitaciones,
  RLS de usuarios y suite de aislamiento).
- [`docs/contexto/`](docs/contexto/): arquitectura y planning de la plataforma.
- [`api/openapi.yaml`](api/openapi.yaml): contrato HTTP.
- [`CLAUDE.md`](CLAUDE.md): reglas para trabajar en el repo con Claude Code.
