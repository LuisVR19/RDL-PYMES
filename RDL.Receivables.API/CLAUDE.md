# Receivables API (cuentas por cobrar y cobranza) — Go

Dueña de `receivables`: cuentas por cobrar, pagos, aplicaciones, ajustes, aging, seguimientos y promesas de pago.
Consume `InvoiceIssued`, `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled`; produce `PaymentReceived` y
`ReceivableSettled`. Contexto: `docs/contexto/`. Plan y decisiones aprobadas: `docs/PLAN.md`. Decisiones:
`docs/decisiones/` (leer 0001 antes de tocar la base). Handoff: `docs/ESTADO.md`.

# Reglas de plataforma (no modificar sin PR en contracts)
- Este servicio SOLO escribe en el schema `receivables` (más `audit.audit_events` e `integration.*` vía sus políticas).
  Nunca generes INSERT/UPDATE/DELETE ni migraciones sobre core, billing, fiscal o subscriptions.
  Lo que haga falta en `shared`, `audit`, `integration` o en privilegios va como propuesta a `database-platform`.
- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas (organization_id, id).
  Toda consulta filtra por el TenantContext, nunca por un valor recibido en body, query o header.
- Montos: numeric (`shared.money_amount`) y `shopspring/decimal` en Go. **Prohibido `float`**, `real` o `double precision`.
- Fechas en UTC (timestamptz). Fechas de negocio (`issued_on`, `due_on`, `received_on`, `asOf`) en la zona de la
  organización (`core.organizations.timezone`), nunca calculadas en UTC.
- Cambios que emiten eventos: entidad + outbox en la MISMA transacción. No inventar eventos (no hay "pago anulado").
- Cada operación sensible genera un audit event con correlationId, en la misma transacción que el cambio.

# Reglas de este repo
- **El saldo y el estado solo cambian dentro de los agregados** (`internal/domain/receivable`, `payment`). Ningún
  handler ni consumidor escribe `status` o `balance_amount`; en la base los recalculan los triggers.
- **Las aplicaciones, pagos y ajustes se revierten o se anulan con motivo; nunca se borran ni se editan.**
- **El tenant de un evento sale de su `organizationId`**; el de una petición HTTP, solo del `org_id` del JWT verificado
  más la membresía activa revalidada en `core`.
- Bloqueos en orden estable: primero pagos (por id), después cuentas (por id), el mismo orden que los triggers.
- Toda operación con tenant corre en `TxManager.WithinTenantTx` (o `WithinServiceTx` en el consumidor), que fija
  `app.current_organization_id` (y `app.current_user_id`) con `set_config(..., true)`. Nunca `SET` de sesión.
- Recurso de otra organización → 404. Rol insuficiente → 403. Estado que no permite la operación → 409.
- La matriz de permisos vive solo en `internal/domain/permission`.
- Migraciones: goose en `migrations/`, historial en `receivables.goose_db_version`, patrón expand → migrate → contract.
  Mostrar el SQL antes de aplicarlo; solo dev y local.
- Endpoints nuevos: registrarlos en `internal/wiring`, documentarlos en `api/openapi.yaml` (un test compara el
  router con el OpenAPI) y agregar su caso cruzado en `tests/isolation`. Eventos nuevos: `events.NewDecoder` y
  `wiring.EventProcessor`.
- No debilites un test de aislamiento ni desactives RLS para que algo pase: reporta y propone la migración.
- Nunca uses la service_role key, producción, ni datos reales. No se inventa un broker (transporte: `TODO(P2)`).
- Las decisiones abiertas del repo de contratos no se resuelven aquí: `TODO` y a `docs/ESTADO.md`.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation
```
Luego revisa tu diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente,
tenant de un evento tomado de otro lado, float para montos, timestamp sin zona, fechas de negocio en UTC, saldos que
puedan quedar negativos, efectos duplicados al reprocesar un evento, secretos y errores internos expuestos.
