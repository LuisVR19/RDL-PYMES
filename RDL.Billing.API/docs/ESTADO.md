# Estado de Billing API

Handoff entre sesiones. Se actualiza al cerrar cada incremento.

## Última actualización: 2026-09-27 — **F5 hecho**: notas de crédito y débito y anulación, probadas en dev.

### F5 · notas y anulación (2026-09-27)
- **Sin migraciones**: la baseline ya tenía `referenced_invoice_id`, `reference_reason`, las columnas de anulación,
  `invoices_reference_ck`, `invoices_cancelled_ck` y `invoices_guard` (deja pasar solo issued → cancelled).
- Dominio: `Header` lleva la referencia de la nota (obligatoria con motivo de hasta 500 caracteres en las notas,
  prohibida en la factura); `ApplyHeader` no deja cambiar el tipo ni la factura referenciada, sí el motivo.
  `Invoice.Cancel`: solo una factura emitida (`ErrNotIssued`, `ErrNoteNotCancellable`), motivo obligatorio.
- Casos de uso: la referencia se valida al crear, al editar (si cambian cliente o moneda) y **otra vez al emitir**.
  Inexistente, de otra organización, borrador, otra nota u otro cliente → 422 `invalid-reference`; factura anulada
  → 409 `invoice-not-issued`; otra moneda → 422 en `currency`. `CancelInvoice` (owner, admin, permiso nuevo
  `invoices.cancel`): anulación, historial con el motivo, audit `invoice.cancelled` e `InvoiceCancelled` en una
  transacción; idempotente.
- Eventos: `CreditNoteIssued`, `DebitNoteIssued` (con su `dueDate` = emisión + plazo) e `InvoiceCancelled` v1,
  validados contra su schema antes del outbox. La numeración de cada tipo es independiente (primera NC: 00000001).
- HTTP: `POST /v1/invoices/{id}/cancel`; POST y PATCH aceptan `referencedInvoiceId`/`referenceReason`; el
  `Invoice` devuelve la referencia y la anulación. `api/openapi.yaml` → 0.10.0; propuesta en contratos (CHANGELOG
  `[Sin publicar]`).
- Verificación: build, vet, **golangci-lint 0**, `go test -race` · **integración** con `TestNotesAndCancelEndToEndSQL`
  (notas, anulación, eventos validados y el trigger que impide cambiar el motivo) · **aislamiento 6/6** con los casos
  cruzados de anular y de una nota que referencia un documento de otra organización · **E2E 33/33** · **en vivo** por
  el portal y el gateway: nota de crédito guardada, recargada con su motivo, emitida (00000001) y la factura 00000004
  anulada; en el outbox de dev quedaron `CreditNoteIssued` (con `referencedInvoiceNumber` 00000004) e
  `InvoiceCancelled`.
- El E2E necesita `python3`: en esta máquina el alias de la Microsoft Store lo tapa; se corrió con un envoltorio a
  `%LOCALAPPDATA%\Programs\Python\Python312\python.exe`.

### Para decidir en equipo (F5)
- **Misma moneda que la factura** en la nota: regla nueva del portal/Billing, no del contrato (Receivables no convierte).
- **Tope de lo acreditado**: Billing no impide que las notas de crédito sumen más que la factura. ¿Se limita?
- **Anular una factura con notas emitidas**: hoy se permite. ¿Se bloquea?
- **¿Se anulan notas?** (TODO(fiscal) de la máquina de estados): hoy 409 `conflict`.
- **Anulación ante Hacienda** (ESTADO §3 de contratos): Billing solo emite `InvoiceCancelled`; si la nota de crédito
  código 01 la genera fiscal desde el evento, falta decidirlo.
- **Código de referencia de Hacienda** en las notas: hoy solo el motivo en texto (TODO(fiscal)).

## 2026-09-27 — P4 cerrado en dev: migración 00003, aislamiento 6/6, integración 11/11, E2E 33/33.

### Cierre en dev (2026-09-27)
- SQL Editor (Luis): `grant usage on schema core to billing_migrator`, contraseñas nuevas de `billing_api` y
  `billing_migrate` (las del `.env` de esta máquina) y fixtures `0011`.
- `.env` creado: pooler `aws-0-us-east-2.pooler.supabase.com` (el `.env.example` sugiere us-east-1, que no es).
- Migración **00003_branch_fk aplicada**; `migrate status` → 00001–00003 aplicadas.
- `go vet` + `go test -race` en verde (por primera vez con `-race`: gcc de WinLibs instalado) · `golangci-lint` 0
  issues tras normalizar a LF los 90 `.go` que el checkout dejó en CRLF.
- **Aislamiento 6/6** (36 subpruebas, ninguna saltada).
- **Integración 11/11** con `TEST_MEMBER_ORG=b0000000-0000-4000-8000-00000000000a` y
  `TEST_MEMBER_SUBJECT=billing-iso-owner-a` (org A de los fixtures): sin ellas se saltan la emisión completa y la
  membresía activa. La emisión corre en una transacción revertida; no deja datos.
- Contraseñas nuevas de `usuario.e2e@…` / `usuario.e2e2@…` (SQL Editor, `auth.users`), en los `.e2e.local` de
  Billing, Platform y el gateway.
- **E2E 33/33** contra la API local. La primera corrida dio 31/32 con una falsa fuga: `usuario.e2e2` tenía como
  activa la organización de `usuario.e2e` (aceptó su invitación en la prueba del portal del 2026-09-25, rol
  collector), así que ver la factura era lo correcto. Se cambió su organización activa a una propia con
  `PUT /v1/me/active-organization` (→ 404 en la factura ajena) y `scripts/dev/e2e.sh` ahora verifica esa
  precondición antes del caso de aislamiento. Datos confirmados en dev: facturas 00000001 y 00000002 de la
  organización `5a0a4fd1-…`.


### OpenAPI de Billing en contratos (2026-09-26)
- `RDL.Contracts/openapi/billing.yaml` refleja lo implementado en F2/F3 (ProductPatch, InvoiceDraftPatch, campos de
  Invoice y Product, DocumentSequenceInput/`used`, issuedFrom/issuedTo, respuestas de error). Problem types
  confirmados, salvo los de F5.
- Corrección aquí: las líneas de la respuesta de factura traen `grossAmount` (document-line.v1 lo exige desde v0.2.0);
  `api/openapi.yaml` pasa a 0.9.1.

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
| Suite de aislamiento en verde con `make test-isolation`; E2E en verde | ✅ aislamiento 6/6 · E2E 33/33 (2026-09-27) |
| Audit en cada operación sensible; idempotencia en cada comando; correlationId de punta a punta | ✅ |
| `golangci-lint` limpio, cobertura alta, imagen Docker, health checks | ✅ lint, cobertura, health · ⏳ Docker sin verificar (no está instalado) |
| Lista final de TODOs | ✅ abajo |

## Para correr en dev (una vez, como `postgres`, desde el SQL Editor)

1. ~~`grant usage on schema core to billing_migrator;` → `make migrate-up`~~ ✅ 2026-09-27.
2. ~~`scripts/dev/0011_billing_isolation_fixtures.sql` → `make test-isolation`~~ ✅ 2026-09-27.
3. ✅ 2026-09-27. Para el E2E: `.e2e.local` con `E2E_EMAIL`/`E2E_PASSWORD` (y opcional `E2E_EMAIL2`/`E2E_PASSWORD2`) de usuarios de
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
- OpenAPI de Billing en contratos: **propuesta escrita** en `openapi/billing.yaml` (entrada `[Sin publicar]` del
  CHANGELOG, `breaking` contra v0.2.0 OK), pendiente de las 2 aprobaciones. Queda fuera la línea libre sin `productId`
  (se define al implementarla) y las rutas F5.

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

### Fuera de alcance (F4; el diseño ya lo admite)
- F4: condiciones de venta y medios de pago completos, exoneraciones de entrada, campos del XML, consumir
  `ElectronicDocumentRejected` para `requires_correction`.
- ~~F5~~ hecho el 2026-09-27 (arriba).

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
- ~~Solo `documentType: invoice`~~: las notas llegaron con F5.
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

