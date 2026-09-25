# Estado de Billing API

Handoff entre sesiones. Se actualiza al cerrar cada incremento.

## Última actualización: 2026-09-25 — incremento 9 (endurecimiento). F2 y F3 implementadas.

### Conflicto con contratos v0.2.0 resuelto (2026-09-25)
- Luis Valverde publicó en `main` contratos **v0.2.0** con la misma decisión de D2 y de `exoneratedRate`, tomada del
  borrador de los Anexos v4.4 (su ADR 0007), más `money.TaxRate`, `money.Round5` y el campo obligatorio
  **`grossAmount`** (MontoTotal) en cada línea del evento. Mis cambios locales equivalentes en contratos chocaban con
  los suyos: se descartaron y `RDL.Contracts` quedó idéntico a `main`.
- Billing se adaptó a v0.2.0: tarifa exonerada como `money.TaxRate` (el tipo garantiza el formato 4,2),
  `grossAmount` en `InvoiceIssued` (`Line.Gross()` = subtotal + descuento), y un test que prueba que `money.Round`
  coincide con `money.Round5` del contrato (ejemplos del Anexo + 5000 casos aleatorios).
- Sugerencia para contratos (no aplicada): un ejemplo de `InvoiceIssued` que sí necesite redondeo, con un caso de mitad
  exacta; hoy los ejemplos están elegidos para no necesitarlo.

### Ruta interna para el Portal Gateway (2026-09-25)
- **`GET /internal/v1/invoices/{id}/summary`** (`getInvoiceSummary` de `openapi/bff-internal.yaml` del contrato),
  implementada tal cual el contrato, sin desviaciones: `id`, `documentType`, `number` (null en borrador),
  `status`, `requiresCorrection`, `customerLegalName` (del snapshot; se omite en borrador, igual que
  `customerSnapshot` en el detalle), `currency`, `total`.
- Misma cadena que `/v1`: token del usuario + membresía revalidada, `InvoicesRead` (todos los roles), ajeno → 404.
  Que solo se alcance por la red interna es tarea del despliegue (documentado en `router.go` y `api/openapi.yaml`).
- `InvoiceRepository.GetHeader`: la misma consulta generada `GetInvoice`, **sin cargar líneas**. No hubo que
  tocar `queries/` ni correr sqlc (que no está instalado en esta máquina).
- Pruebas: forma exacta de la respuesta (borrador y emitida), autenticación y 405/404 en la ruta interna, todos
  los roles leen, ajeno → `ErrNotFound`; caso cruzado agregado en `tests/isolation` (criterio 1) y `GetHeader`
  en la integración de los adapters. **Las dos últimas no corrieron: no hay `.env` de Billing en esta máquina.**
- `api/openapi.yaml` → 0.9.0 con la ruta.
- Los `.go` estaban en CRLF por el checkout (`core.autocrlf=true`) y `golangci-lint` los marcaba; se
  normalizaron a LF (git no ve diferencia).

### Hecho en el incremento 9
- Suite de aislamiento `tests/isolation` con los 6 criterios, sobre el router real, la membresía real en core y
  `postgres.SavepointTxManager` (cada prueba en una transacción revertida: no deja datos). Fixtures en
  `scripts/dev/0011_billing_isolation_fixtures.sql`. **Todavía no corrió de verdad: faltan los fixtures en dev.**
- E2E `scripts/dev/e2e.sh` (login real en Supabase Auth, cliente, producto, borrador, emisión, outbox con
  `scripts/dev/outboxcheck`, snapshots, aislamiento con un segundo usuario, token forjado). **Sin correr: falta
  `.e2e.local` con un usuario de prueba.**
- `wiring.Deps` recibe `app.TxManager` (la suite inyecta el de savepoints).
- Autorrevisión del código: sin consultas a billing sin `organization_id`, sin escrituras fuera de
  billing/audit/integration, sin `float`, sin `timestamp` sin zona, sin secretos en el repo, 500 sin detalle interno,
  todo `customerId`/`productId`/`branchId` validado en la base.
- Cobertura: dominio 78–100 % (money 78 % por helpers de test), casos de uso 85 %, handlers 84 %, eventos 89 %.
- README final.

## Definición de terminado (prompt P4)

| Criterio | Estado |
|---|---|
| Informe de brechas aprobado, logins creados, migraciones en dev con baseline | ✅ 00001 y 00002 aplicadas · ⏳ 00003 espera `grant usage on schema core to billing_migrator` |
| Endpoints de F2 y F3 implementados, en OpenAPI y con tests | ✅ |
| Emite con E-Invoice apagada (criterio 2) | ✅ la emisión solo escribe en la base y el outbox |
| Cambiar cliente o producto no altera lo emitido ni su evento (criterio 5) | ✅ unit + integración contra dev |
| Tests de propiedades del cálculo en verde | ✅ 5 invariantes, referencia exacta, mutaciones detectadas |
| `InvoiceIssued` v1 en el outbox, misma transacción, validado contra el schema | ✅ unit + integración contra dev |
| Suite de aislamiento en verde con `make test-isolation`; E2E en verde | ⏳ escritas; faltan los fixtures (script 0011) y `.e2e.local` |
| Audit en cada operación sensible; idempotencia en cada comando; correlationId de punta a punta | ✅ |
| `golangci-lint` limpio, cobertura alta, imagen Docker, health checks | ✅ lint, cobertura, health · ⏳ Docker sin verificar (no está instalado) |
| Lista final de TODOs | ✅ abajo |

## Para correr en dev (una vez, como `postgres`, desde el SQL Editor)

1. `grant usage on schema core to billing_migrator;` → luego `make migrate-up` (aplica `00003_branch_fk`).
2. `scripts/dev/0011_billing_isolation_fixtures.sql` → luego `make test-isolation`.
3. Para el E2E: `.e2e.local` con `E2E_EMAIL`/`E2E_PASSWORD` (y opcional `E2E_EMAIL2`/`E2E_PASSWORD2`) de usuarios de
   prueba con organización activa (se crea con el E2E de Platform) → `make run` y `bash scripts/dev/e2e.sh`.

## TODOs para revisar en equipo

### Fiscal (`TODO(fiscal)`: no se inventan)
- Cargar los catálogos `fiscal.*` (hoy vacíos): tarifas de impuesto (sin ellas, una línea con impuesto responde 422),
  tipos de impuesto, unidades, condiciones de venta, medios de pago, tipos de identificación y de exoneración, CABYS.
  Billing solo valida formato hasta entonces.
- Margen de error de Hacienda para diferencias de redondeo (no aplica a Billing: todo cuadra exacto).
- Base imponible de impuestos que no son IVA o que se calculan sobre otro impuesto; impuesto asumido por el emisor.
- Hasta 5 descuentos encadenados por línea (hoy uno).
- Qué condiciones de venta exigen plazo de crédito; formato del número de identificación por tipo (D9).
- Dirección del snapshot con provincia, cantón y distrito (hoy texto libre).

### Contrato (repo de contratos)
- Publicar el tag `v0.2.0` de contratos (ADR 0007 de Luis Valverde: reglas del borrador de los Anexos v4.4,
  provisionales hasta la versión oficial); después quitar el
  `replace` de `go.mod` y el contexto especial del Dockerfile.
- `ProductPatch` para `PATCH /v1/products/{id}` (ADR 0004 §3).
- Completar el esqueleto de Billing: campos de `Invoice` que la API ya devuelve (`saleConditionCode`, `creditTermDays`,
  `exchangeRate`, `notes`, `updatedAt`), `InvoiceDraftPatch`, líneas libres sin `productId`, filtros
  `issuedFrom`/`issuedTo`, `used` en `DocumentSequence`.

### Decisiones de producto
- Matriz de permisos (propuesta del prompt).
- Idempotencia: el reintento responde el estado **actual** del recurso (como Platform), no el cuerpo original
  (convenciones §7 dice "la misma respuesta").
- `branchId` obligatorio en los eventos (D11) y relación sucursal ↔ establecimiento fiscal.

### Técnicos
- Verificar `make docker` en una máquina con Docker.
- Renombrar `invoice_line_taxes.exoneration_percentage` → `exonerated_rate` (opcional, expand → contract).
- CHECKs de longitud en las columnas `text` de billing (hoy los valida la app).
- Consolidar `pkg/tenancy` y compañía en un módulo *building-blocks* versionado y borrar las copias (ADR 0003).

### Fuera de alcance (F4 y F5; el diseño ya los admite)
- F4: condiciones de venta y medios de pago completos, exoneraciones de entrada, campos del XML, consumir
  `ElectronicDocumentRejected` para `requires_correction`.
- F5: notas de crédito y débito, anulación con motivo, `CreditNoteIssued`, `DebitNoteIssued`, `InvoiceCancelled`.

### Hecho en el incremento 8
- `Invoice.Issue` (dominio): solo `draft`, al menos una línea, cada línea cuadra y los totales son la suma exacta de
  las líneas; snapshot del cliente, número, `issuedAt`, vencimiento. `DueDate` = emisión + plazo.
- `POST /v1/invoices/{id}/issue` (`app.IssueInvoice`): una transacción con revalidación de cliente, sucursal y
  productos; número bajo candado; transición, historial, audit e `InvoiceIssued` v1 validado contra el schema del
  contrato en el outbox. Sin llamadas a otros servicios. Idempotente. ADR 0007.
- `internal/adapters/events`: mapeo a `events.InvoiceIssuedV1` (importado del contrato) y validación con su JSON Schema.
- Tests: dominio, contrato del evento (+ `rapid`), casos de uso (incluido outbox que falla → nada emitido), handlers
  e integración `TestIssueEndToEndSQL` contra dev (savepoints en una transacción revertida). Base dev limpia después.
- Migraciones: `00002_sequence_last_assigned` **aplicada** en dev. `00003_branch_fk` **pendiente**: falla con
  "permission denied for schema core"; falta `grant usage on schema core to billing_migrator` (agregado al script
  `scripts/dev/0010_billing_login_roles.sql` y a la propuesta 0002).

### Hecho en el incremento 7
- Numeración: `GET/PUT /v1/document-sequences`, prefijo + 8 dígitos, sin prefijos repetidos por tipo, ADR 0006.

### Hecho en el incremento 7
- Dominio `numbering`: formato prefijo + 8 dígitos, `Configure` (solo sin uso, sin prefijo repetido en el tipo),
  `Assign` (para la emisión) y `Choose` (sucursal → organización → nueva). ADR 0006 (concurrencia y huecos).
- `GET /v1/document-sequences` y `PUT /v1/document-sequences/{documentType}` (owner, admin): 409 `sequence-in-use`,
  409 `conflict` por prefijo repetido, sucursal validada contra la organización. Candado asesor por
  (organización, tipo) + `FOR UPDATE`. Audit `sequence.configured`.
- Tests: dominio, casos de uso, handlers. `TestSequencesSQL` (integración) espera las migraciones.

### Hecho en el incremento 6
- Facturas en borrador: crear, editar encabezado, reemplazar líneas, descartar, listar, detalle, historial.

### Hecho en el incremento 6
- Decisiones §3.3 y §4.2–4.5 del informe 0001 **aprobadas** (tasa desde `fiscal.tax_rates`, snapshot de línea al
  escribir el borrador, historial solo de transiciones, vencimiento, numeración).
- Agregado `invoice.Invoice`: `NewDraft`, `ApplyHeader`, `ReplaceLines` (recalcula con `CalculateLine`/`Totals`),
  `CanDiscard`; solo `draft` se edita (`ErrNotDraft` → 409 `invoice-not-draft`). Tipo de cambio 1 en la moneda local.
- Endpoints: `POST/GET /v1/invoices`, `GET/PATCH/DELETE /v1/invoices/{id}`, `PUT /{id}/lines`, `GET /{id}/history`.
  Cliente, sucursal y productos validados contra la organización activa (ajenos → 404; cliente inactivo → 422
  `customer-inactive`; sucursal o producto inactivos → 422). Snapshot y tasas desde la base, nunca del cuerpo.
- `PATCH` distingue ausente de `null` (`branchId`, `creditTermDays`). `PUT /lines` valida el arreglo campo a campo.
- Lecturas de `core.organizations`, `core.branches` y `fiscal.tax_rates` (adapter `catalog`).
- Tests: agregado, casos de uso (idempotencia, referencias ajenas, monedas, tasa desconocida, roles, emitido
  inmutable, descarte, 404 cruzado) y handlers. SQL real en transacción revertida, incluido que **la base impide
  modificar un documento emitido** aunque la app lo intente. Base dev: 0 filas de Billing después.

### Alcance acotado (para revisar / PR al contrato)
- Solo `documentType: invoice`; las notas son F5.
- Solo líneas con `productId`. Las líneas libres necesitan campos (CABYS, descripción, unidad, impuestos) que el
  esqueleto del contrato no define todavía.
- Sin exoneraciones de entrada (F4); el cálculo ya las soporta.
- El `Invoice` de la API agrega `saleConditionCode`, `creditTermDays`, `exchangeRate`, `notes`, `updatedAt`, que el
  esqueleto del contrato no tiene.

### Hecho en el incremento 5
- D2 decidido y contratos actualizados (ADR 0007 de contratos); `shopspring/decimal`, `money.Round`, cálculo puro y
  tests de propiedades.

### Hecho en el incremento 4
- Productos con impuestos, montos exactos del contrato, PATCH parcial, 409 `product-code-taken`, tests anti-float.

### Hecho en el incremento 3
- Clientes completos: `POST` idempotente, `GET /{id}`, `PATCH` con baja lógica, 409
  `customer-identification-taken`, audit en la misma transacción.

### Hecho antes
- Incremento 2: JWT, TenantContext, revalidación de membresía, `TxManager`, matriz de permisos, `GET /v1/customers`.
- Incremento 1: esqueleto (config, logger, OTel, health, Problem Details, correlación, Dockerfile, Makefile, lint),
  `cmd/migrate` y baseline aplicada en dev (`00001`).
- Paso 1: informe de brechas (0001), propuesta a database-platform (0002) y logins creados en dev.

