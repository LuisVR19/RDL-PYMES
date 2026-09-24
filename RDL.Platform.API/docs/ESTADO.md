# Estado del proyecto · Platform API

**Última actualización:** 2026-09-24
**Punto de corte:** terminados los incrementos 1 a 8. El alcance del prompt P3 está completo; quedan las decisiones abiertas y los TODOs de contrato.

## Qué está hecho

| # | Incremento | Estado | Endpoints |
|---|---|---|---|
| 1 | Esqueleto, config, health, OTel | ✅ | `GET /healthz`, `GET /readyz` |
| 2 | JWT (JWKS de Supabase) + TenantContext | ✅ | middleware de `/v1/*` y `/v1/organizations/current*` |
| 3 | Me, membresías, organización activa | ✅ | `GET /v1/me`, `GET /v1/me/memberships`, `PUT /v1/me/active-organization` |
| 4 | Organizaciones | ✅ | `POST /v1/organizations`, `GET`/`PATCH /v1/organizations/current` |
| 5 | Miembros y roles | ✅ | `GET /v1/organizations/current/users`, `PATCH .../users/{userId}` |
| 6 | Invitaciones | ✅ | `POST`/`GET .../invitations`, `DELETE .../invitations/{id}`, `POST /v1/invitations/{token}/accept` |
| 7 | Sucursales | ✅ | `GET`/`POST .../branches`, `GET`/`PATCH .../branches/{id}` |
| 8 | Endurecimiento: RLS de `core.users`, suite de aislamiento, autorrevisión | ✅ | — (migraciones 00005/00006, `tests/isolation`) |

Verificación al cierre:
- `go build`, `go vet` y `golangci-lint --build-tags=integration` (0 issues) limpios, y `go test ./...` en verde.
- **Suite de aislamiento: 6/6** (`make test-isolation`, unos 100 s contra dev).
- **E2E contra Supabase dev: 69/69** (`scripts/dev/e2e.sh`, con dos usuarios reales).
- Advisors de seguridad de Supabase: solo avisa que la protección de contraseñas filtradas de Auth está apagada (configuración del Dashboard, fuera de este repo).

## Base de datos (proyecto dev `dzlsnsstuqpxvwegeqcy`)

- Migraciones goose **00001 a 00006 aplicadas** (00005/00006: RLS de `core.users`, expand y contract, ADR 0007); el historial está en `core.goose_db_version`. `go run ./cmd/migrate status` las muestra.
- Roles de login `platform_api` y `platform_migrate` creados (migración `0009_platform_login_roles` en `supabase_migrations`, propuesta 0002). Las contraseñas las asignó el usuario y están solo en `.env`.
- Custom Access Token Hook `core.custom_access_token_hook` **activado** en el Dashboard. Emite `org_id` y `org_roles`.
- La confirmación de email de Supabase Auth está **desactivada** en dev para las pruebas.
- Datos de prueba: usuarios `usuario.e2e@rdlpymes.com` y `usuario.e2e2@rdlpymes.com` (credenciales en `.e2e.local`, ignorado por git) y varias organizaciones `E2E <n> S.A.` creadas por el script.

## Cómo retomar

```sh
cd RDL.Platform.API
go run ./cmd/migrate status                    # confirma conexión y migraciones
go run ./cmd/api                               # terminal 1
bash scripts/dev/e2e.sh http://127.0.0.1:8080  # terminal 2: debe dar 69/69
go test -count=1 -tags=integration ./tests/isolation/...   # 6/6, no hace falta la API corriendo
```

## Incremento 8 (hecho el 2026-09-24)

1. **`core.users`:** se eliminó `users_platform` (`using (true)`). Ahora hay `users_self`, `users_self_update` y `users_org_members`. La búsqueda por `sub` y el alta pasan por `core.find_user_by_subject` y `core.provision_user` (`security definer`). Se aplicó 00005 (expand), después el código y después 00006 (contract), con `migrate up-by-one`.
2. **Suite de aislamiento** en `tests/isolation`, con los 6 criterios. Corre el router real (`internal/wiring`) con un verificador de JWT de prueba. ⚠ No usa testcontainers (ADR 0007): el schema lo crea `database-platform`.
3. **Autorrevisión:** todas las consultas filtran por el tenant del contexto o se apoyan en RLS del propio usuario (`FindInvitationByTokenHash`, por diseño). Las escrituras solo van a `core`, `audit.audit_events` e `integration.idempotency_keys`. No hay `float` (el TTL de idempotencia pasó de `float64` a segundos enteros), todas las fechas son `timestamptz` y los 500 no exponen detalle. `.env` y `.e2e.local` están ignorados por git.
4. README, CLAUDE.md y ADR 0007 actualizados.

Datos que deja la suite en dev: organizaciones `Isolation … S.A.` y usuarios `iso-*@isolation.test`, sin cuenta en Auth.

## Decisiones abiertas para revisar en equipo

- **Solo un owner gestiona owners** (un admin no asigna, degrada ni suspende owners). La regla la agregué yo; ver `membership.PlanChange` y ADR 0006.
- **Invitaciones** (ADR 0006): un reintento del alta devuelve la invitación sin token (solo se guarda el hash); el token viaja en la URL (definido por el prompt); todavía no se envía email.
- **IP para la auditoría:** se toma de la conexión, no de `X-Forwarded-For`. Hay que definirlo cuando exista el balanceador.
- **Membresías suspendidas:** no aparecen en `GET /v1/me/memberships`, porque RLS tampoco deja ver esas organizaciones.

## TODOs de contrato (no se inventaron)

- Catálogo de `identificationTypeCode` y formato del número por tipo.
- Eventos de Platform (`OrganizationCreated`, `MemberAdded`, invitaciones): no están en el catálogo 6.2, así que no se publican en el outbox.
- OpenAPI del repo `contracts`: `api/openapi.yaml` es local; hay que alinearlo cuando exista.
- Convención de `type` de Problem Details (`urn:rdl:platform:problem:*`).
- Envío de invitaciones por email (servicio de notificaciones).

## Notas técnicas que conviene recordar

- Con Supavisor en modo transacción, pgx usa `QueryExecModeExec`. Por eso los parámetros `jsonb` se envían como **texto** (`sqlc.arg(x)::text::jsonb`): un `[]byte` viaja como `bytea` y la base lo rechaza.
- `postgres` es miembro de todos los `*_app`, pero no de `platform_api`. Las pruebas SQL con MCP usan `set_config('role','platform_app',true)` y terminan con `raise exception` para forzar el rollback.
- En modo automático de Claude Code se bloquean la generación de credenciales y el sondeo de hosts. Las contraseñas y el host del pooler los pone el usuario en `.env`.
