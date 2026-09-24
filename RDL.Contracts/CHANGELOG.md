# Changelog

Todo cambio de contrato se registra aquí. Formato: [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/), versiones [SemVer](https://semver.org/lang/es/).

## [Sin publicar]

(nada todavía)

## [0.2.0] - 2026-09-24

Reglas fiscales del borrador de los Anexos y Estructuras v4.4 de Hacienda (ADR 0007). Provisionales hasta revalidar
con la versión oficial.

### Cambiado (incompatible; permitido en `v0` porque v0.1.0 no se publicó)
- `exoneration.v1`: `percentage` (porcentaje del impuesto) se reemplaza por **`exoneratedRate`** (puntos de tarifa,
  formato 4,2) y el monto pasa a ser `exoneratedRate / 100 × subtotal` de la línea.
- `document-line.v1`: nuevo campo obligatorio **`grossAmount`** (MontoTotal = cantidad × precio) y
  `subtotal = grossAmount − discount`.
- `pkg/events`: `Exoneration.ExoneratedRate` (`money.TaxRate`) y `DocumentLine.GrossAmount`.

### Agregado
- Tipo común `TaxRate` (`schemas/common/tax-rate.json`) y `money.TaxRate`.
- Redondeo oficial: 5 decimales, mitad hacia arriba (resuelve D2). `money.RoundHalfUp` y `money.Round5` con los
  ejemplos del anexo.
- Glosario: composición del consecutivo (20 dígitos) y de la clave numérica (50 dígitos).
- ADR 0007 con las reglas aplicadas y los hallazgos pendientes (anulación por nota de crédito, rechazados, D11).

## [0.1.0] - 2026-09-24

Primera versión: hito H1 · Contratos v1. El tag lo crea quien publique el repo.

### Agregado
- Esqueleto del repo: CLI `contractsctl` (`validate`, `lint`, `version`), Makefile y pipeline de Bitbucket.
- Convenciones comunes (`docs/convenciones.md`), glosario (`docs/glosario.md`) y ownership de schemas (`docs/ownership.md` + `ownership/ownership.yaml`).
- `contractsctl validate` verifica la matriz de ownership: sin escrituras en schemas de otra API, `audit` append only, servicios y roles que la base reconoce.
- Tipos comunes en JSON Schema 2020-12 (`schemas/common/`): `Money`, `ExchangeRate`, `Quantity`, `Percentage`, `CurrencyCode`, `Uuid`, `UtcDateTime`, `BusinessDate`, `ServiceName`, `Identification` y `CustomerSnapshot`.
- Sobre común de eventos `schemas/events/envelope.v1.json` con `sourceService` (D3).
- Suites de ejemplos válidos e inválidos (`examples/common/`), verificadas por `contractsctl validate`.
- `contractsctl validate` compila los schemas (dialecto, `$id` = ruta, `$ref` solo al repo) y verifica los ejemplos.
- `pkg/events/money`: tipos decimales exactos sin `float` (ADR 0002).
- `InvoiceIssued` v1 completo (`schemas/events/invoice-issued.v1.json`): líneas con snapshot del producto, impuestos por línea, exoneraciones, medios de pago y totales. Partes reutilizables para las notas en `schemas/events/parts/`. Tipos comunes `CabysCode` y `FiscalCode`. Códigos fiscales como `TODO(fiscal)`.
- `docs/eventos/invoice-issued.md`: campos, origen en Billing y fórmulas de los montos.
- `pkg/events`: `Envelope`, `InvoiceIssuedV1` y sus partes, `Instant` (siempre UTC con Z), `Date`, `OutboxRow()` y `Validator` sobre los schemas embebidos (`contracts.Schemas`). Tests de contrato struct ↔ schema, sin `float`, round trip de ejemplos y aritmética exacta (ADR 0004).
- Suites de ejemplos: los inválidos pueden escribirse como un válido más un JSON Patch (`base` + `patch`).
- Los otros 7 eventos del catálogo 6.2 (`InvoiceCancelled`, `CreditNoteIssued`, `DebitNoteIssued`, `ElectronicDocumentAccepted`, `ElectronicDocumentRejected`, `PaymentReceived`, `ReceivableSettled`) con schemas, partes (`issued-note`, `electronic-document`), ejemplos y DTOs Go. Tipo común `DocumentType`.
- `asyncapi/asyncapi.yaml` (AsyncAPI 3.0): canales lógicos, operaciones de envío y recepción, y mensajes con `x-rdl-version`, `x-rdl-producer` y `x-rdl-consumers`.
- `contractsctl validate` valida el AsyncAPI contra el meta-schema oficial 3.0.0 (vendorizado) y el catálogo contra la tabla 6.2: ningún evento fuera de ella, productor, consumidores, schema, canal y `const` coherentes.
- `pkg/events`: interfaz `Event`, `Spec` por evento, `Catalog` y `ToOutboxRow` genérico.
- `docs/eventos/README.md`: catálogo con productor, consumidores, canal y agregado.
- Máquinas de estado de factura/notas, documento electrónico, cuenta por cobrar y pago (`state-machines/*.yaml` + `docs/maquinas-de-estado/*.md`). Estados = `CHECK` de la base; transiciones con disparador tipado (comando, evento consumido o worker) y evento emitido. Las del documento electrónico son propuesta para fiscal (C3).
- `contractsctl validate` verifica las máquinas: estados alcanzables, finales sin salida, sin callejones, comandos `MÉTODO /v1/...`, el dueño emite solo lo que produce y reacciona solo a lo que consume, y el diagrama del doc coincide con el YAML.
- `contractsctl diagram state-machines/<entidad>.yaml` imprime el bloque Mermaid.
- OpenAPI 3.1: componentes comunes (`openapi/components/common.yaml`), Platform importado tal cual (`openapi/platform.yaml`), esqueletos de Billing, fiscal (E-Invoice) y Receivables, y rutas internas para el BFF (`openapi/bff-internal.yaml`). Ver `docs/openapi.md`.
- `contractsctl validate` valida los OpenAPI contra el meta-schema oficial 3.1 (vendorizado), resuelve todos los `$ref` (también entre archivos), exige `operationId` único y verifica que cada comando de las máquinas de estado exista en el OpenAPI de su dueño. Los hallazgos de documentos importados salen como avisos (ADR 0005).
- `contractsctl lint`: dinero y decimales nunca como `number`/`integer`, instantes y fechas con su tipo, camelCase, objetos cerrados (salvo `x-rdl-base`), reglas de eventos, rutas versionadas, sin parámetros de tenant, `Idempotency-Key` en los POST y registro de problem types (ADR 0006).
- Registro de problem types por servicio (`problems/*.yaml`): Platform tomado de su código; Billing, fiscal y Receivables como propuesta.
- `contractsctl breaking -base <dir>`: cambios incompatibles en schemas, operaciones OpenAPI y catálogo de eventos contra la versión publicada. El pipeline arma la base con el último tag (ADR 0006).
- README definitivo, `docs/ESTADO.md` con la lista de pendientes y `Example` compilable de `pkg/events`.
- Informe de inventario y brechas (`docs/decisiones/0001-inventario-y-brechas.md`), aprobado.
