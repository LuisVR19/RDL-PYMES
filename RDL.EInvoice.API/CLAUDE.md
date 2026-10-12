# E-Invoice API (servicio `fiscal`, comprobantes electrónicos ante Hacienda) — Go

Dueña de `fiscal`: configuración fiscal por organización, catálogos (CABYS y los de Hacienda), establecimientos,
terminales, consecutivos y clave, XML 4.4, firma XAdES-EPES, envío y consulta ante Hacienda, contingencia e historial.
Consume `InvoiceIssued`, `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled`; produce
`ElectronicDocumentAccepted` y `ElectronicDocumentRejected`. Plan y decisiones aprobadas: `docs/PLAN.md`. Informes:
`docs/decisiones/0001` (fuentes de Hacienda), `0002` (base; leerlo antes de tocar migraciones), `0003` (mapeo evento →
XML). Detalle de campos: `docs/analisis/`. Handoff: `docs/ESTADO.md`. Contexto: `docs/contexto/`.

# Reglas de plataforma (no modificar sin PR en contracts)
- Este servicio SOLO escribe en el schema `fiscal` (más `audit.audit_events` e `integration.*` vía sus políticas).
  Nunca generes INSERT/UPDATE/DELETE ni migraciones sobre core, billing, receivables o subscriptions. Lo que haga
  falta en `core`, `shared`, `audit`, `integration` o en roles va como propuesta a `database-platform`.
- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas (organization_id, id).
  Toda consulta filtra por el TenantContext, nunca por un valor recibido en body, query o header.
- Montos: numeric (`shared.money_amount`) y `shopspring/decimal` en Go. **Prohibido `float`**, `real` o
  `double precision` (lo verifica `internal/archtest`).
- Fechas en UTC (timestamptz). La `FechaEmision` del XML es la hora de Costa Rica del instante de generación.
- Cambios que emiten eventos: entidad + outbox en la MISMA transacción, validados contra su JSON Schema.
- Cada operación sensible (configuración, certificado, emisión, reintento) genera un audit event con correlationId.

# Reglas de este repo
- **`docs/hacienda/` es la fuente de verdad fiscal y es de solo lectura.** Toda regla fiscal cita su sección en un
  comentario (`// Anexo 1, pág. 19`, `// Nota 3`, `// Art. 9 (FUENTE: borrador sept-2024)`). Un archivo nuevo ahí se
  registra en `docs/hacienda/manifest.sha256` (lo verifica `internal/archtest`).
- **No se inventan códigos, catálogos, campos, tarifas ni reglas de Hacienda.** Si no está en `docs/hacienda/`, es un
  `TODO(fiscal)` y va a `docs/ESTADO.md`. Ante una diferencia entre el anexo y el XSD, manda el XSD.
- **El XML valida contra el XSD oficial antes de firmarse.** **Lo firmado no se regenera:** un reintento envía el
  mismo XML. El consecutivo y la clave se asignan una sola vez, bajo bloqueo, mientras el documento está en
  `processing` (el trigger `electronic_documents_guard` lo impone en la base).
- **Ningún secreto fuera del `SecretStore`:** el `.p12`, su PIN y las credenciales de Hacienda solo existen en memoria
  al firmar o autenticar. Nunca en disco en claro, logs, audit events, respuestas, errores ni tests (el certificado
  de los tests es uno generado en `tests/fixtures/`).
- **Solo el ambiente de pruebas de Hacienda.** `HACIENDA_ENVIRONMENT` solo admite `sandbox`; la configuración rechaza
  cualquier otro valor. **El certificado de una organización nunca firma un documento de otra.**
- **El tenant de un evento sale de su `organizationId`**; el de una petición HTTP, solo del `org_id` del JWT verificado
  más la membresía activa revalidada en `core`.
- Toda operación con tenant corre en `TxManager.WithinTenantTx` (o `WithinServiceTx` en el consumidor y el worker),
  que fija `app.current_organization_id` con `set_config(..., true)`. Nunca `SET` de sesión.
- Recurso de otra organización → 404. Rol insuficiente → 403. Estado que no permite la operación → 409.
- La matriz de permisos vive solo en `internal/domain/permission`.
- Migraciones: goose en `migrations/`, historial en `fiscal.goose_db_version`, patrón expand → migrate → contract.
  Mostrar el SQL antes de aplicarlo; solo dev y local. Los catálogos se cargan con `cmd/import-catalogs` desde
  `docs/hacienda/`, con versión y sha256 en `fiscal.catalog_imports`; nunca a mano ni desde memoria.
- No debilites un test de aislamiento ni desactives RLS para que algo pase: reporta y propone la migración.
- Nunca uses la service_role key, producción ni datos reales. No se inventa un broker (transporte: `TODO(P2)`).
- Las decisiones abiertas del repo de contratos no se resuelven aquí: `TODO` y a `docs/ESTADO.md`.

# Antes de terminar cada tarea
```
go build ./... && go vet ./... && golangci-lint run --build-tags=integration ./... && go test ./... && make test-isolation
```
Luego revisa tu diff buscando: reglas fiscales sin cita, códigos o campos inventados, XML que no pase el XSD, secretos
en cualquier salida, un certificado usado fuera de su organización, consecutivos que puedan repetirse, uso del
ambiente de producción, `float`, consultas sin tenant y escrituras a schemas ajenos.
