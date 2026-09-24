# Billing API (operación comercial) — Go

Dueña del schema `billing`: clientes, productos, facturas y sus líneas, cálculo de montos, numeración visible y
estados comerciales. Produce `InvoiceIssued` (y en F5 las notas y la anulación).
Contexto: `docs/contexto/` (arquitectura y planning). Decisiones: `docs/decisiones/` (leer 0001 antes de tocar la base).
Contratos: `../RDL.Contracts` (glosario, convenciones, máquina de estados de Invoice, OpenAPI de Billing, schemas de
eventos). **El contrato manda**: si este archivo o un prompt lo contradicen, detente y pregunta.

# Reglas de plataforma (no modificar sin PR en contracts)
- Este servicio SOLO escribe en el schema `billing` (más `audit.audit_events` e `integration.*` vía sus políticas).
  Nunca generes INSERT/UPDATE/DELETE ni migraciones sobre core, fiscal, receivables o subscriptions.
  Lo que haga falta en `shared`, `audit`, `integration` o `core` va como propuesta a `database-platform`
  (ver `docs/decisiones/0002-propuesta-database-platform.md`), no como migración aquí.
- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas (organization_id, id).
  Toda consulta filtra por el TenantContext, nunca por un valor recibido en body, query o header.
- Montos, cantidades, tasas y tipo de cambio: dominios `shared.*` (numeric) y decimal exacto en Go. **Prohibido `float`**
  (`float32`, `float64`, `real`, `double precision`).
- Fechas en UTC (timestamptz). `due_date` e `issueDate` son fechas de negocio en la zona de la organización.
- Cambios que emiten eventos: entidad + outbox en la MISMA transacción, evento validado contra su JSON Schema.
  La emisión nunca llama a otro servicio: responde 201 aunque E-Invoice esté caída.
- Cada operación sensible genera un audit event con correlationId, en la misma transacción que el cambio.

# Reglas de este repo
- La organización activa sale SOLO del `org_id` del JWT verificado + membresía activa revalidada en BD.
- pgx corre en `QueryExecModeExec` (Supavisor): los parámetros `jsonb` y los arreglos (`uuid[]`) viajan como texto y
  se convierten en SQL (`::text::jsonb`, `::text[]::uuid[]`). Los montos también: `::text::numeric` al escribir y
  `::text` al leer, nunca un tipo numérico del driver.
- Toda operación con tenant corre en `TxManager.WithinTenantTx`, que fija `app.current_organization_id` y
  `app.current_user_id` con `set_config(..., true)`. Nunca `SET` de sesión (Supavisor en modo transacción).
- `branch_id`, `customer_id` y `product_id` del cliente se validan contra la organización activa. De otra organización → 404.
- Recurso de otra organización → 404. Rol insuficiente → 403. Estado que no permite la operación → 409.
- El cálculo de montos vive SOLO en `internal/domain/invoice` (funciones puras). El redondeo, SOLO en `money.Round`
  (D2, ADR 0007 de contratos: 5 decimales, mitad hacia arriba, cada campo de línea). Nunca `decimal.Div`: dividir
  entre 100 es `Shift(-2)` (`money.Percent`).
- La exoneración guarda la **tarifa exonerada** en puntos (6.5 de un IVA de 13), no un % del impuesto.
- Una factura emitida no se edita. La base también lo impide (`invoices_guard`, `guard_draft_children`): no los esquives.
- La matriz de permisos vive solo en `internal/domain/permission`. Nada de `if role == ...` en handlers.
- No inventes códigos, catálogos ni reglas fiscales: valida el formato y deja `TODO(fiscal)`.
- Problem Details con `type` = `urn:rdl:billing:problem:<código>` registrado en `problems/billing.yaml` del contrato.
- Migraciones: goose en `migrations/`, historial en `billing.goose_db_version`, patrón expand → migrate → contract
  (`go run ./cmd/migrate up-by-one`). Mostrar el SQL al equipo antes de aplicarlo; solo en dev y local.
- Endpoints nuevos: registrarlos en `internal/wiring` y agregar su caso cruzado en `tests/isolation`.
- No debilites un test de aislamiento ni desactives RLS para que algo pase: reporta y propone la migración.
- Nunca uses la service_role key, producción, ni datos o certificados reales.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation
```
Luego revisa tu diff buscando: consultas sin tenant, escrituras a schemas ajenos, confianza en ids del cliente,
float en montos, timestamp sin zona, facturas emitidas modificables, eventos que no validen contra el schema,
secretos y errores internos expuestos.
