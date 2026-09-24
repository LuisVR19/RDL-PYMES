# Platform API (identidad y tenancy) — Go

Dueña de `core` y `subscriptions`: organizaciones, usuarios, membresías, roles, sucursales e invitaciones.
Contexto: `docs/contexto/` (arquitectura y planning). Decisiones: `docs/decisiones/` (leer 0001 y 0003 antes de tocar tenancy).

# Reglas de plataforma (no modificar sin PR en contracts)
- Este servicio SOLO escribe en los schemas `core` y `subscriptions` (más `audit.audit_events` e `integration.*`
  vía sus políticas). Nunca generes INSERT/UPDATE/DELETE ni migraciones sobre billing, fiscal o receivables.
  Lo que haga falta en `audit` o `integration` va como propuesta a `database-platform`, no como migración aquí.
- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas (organization_id, id).
  Toda consulta filtra por el TenantContext, nunca por un valor recibido en body, query o header.
- Montos: numeric. Prohibido float o double.
- Fechas en UTC (timestamptz). Presentación en la zona horaria de la organización.
- Cambios que emiten eventos: entidad + outbox en la MISMA transacción. Platform no tiene eventos en el catálogo: no inventarlos.
- Cada operación sensible genera un audit event con correlationId, en la misma transacción que el cambio.

# Reglas de este repo
- La organización activa sale SOLO del `org_id` del JWT verificado + membresía activa revalidada en BD.
- Toda operación con tenant corre en `TxManager.WithinTenantTx`, que fija `app.current_organization_id` y
  `app.current_user_id` con `set_config(..., true)`. Nunca `SET` de sesión (Supavisor en modo transacción).
- Recurso de otra organización → 404. Rol insuficiente dentro de la propia → 403.
- La matriz de permisos vive solo en `internal/domain/membership`. Nada de `if role == ...` en handlers.
- Roles: `owner, admin, biller, collector, accountant, read_only` (catálogo `core.roles`). V1: un rol por membresía,
  la API responde `roles: []`.
- Migraciones: goose en `migrations/`, historial en `core.goose_db_version`, patrón expand → migrate → contract
  (`go run ./cmd/migrate up-by-one` para aplicar solo la fase expand antes de desplegar).
- `core.users` no es visible sin sesión: buscar por `sub` o dar de alta solo con `core.find_user_by_subject` /
  `core.provision_user` (ADR 0007). No agregues políticas amplias a `core.users`.
- Endpoints nuevos: registrarlos en `internal/wiring` y agregar su caso cruzado en `tests/isolation`.
- No debilites un test de aislamiento ni desactives RLS para que algo pase: reporta y propone la migración.
- Nunca uses la service_role key, producción, ni datos o certificados reales.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation
```
Luego revisa tu diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente,
float para montos, timestamp sin zona, secretos y errores internos expuestos.
