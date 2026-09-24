# Catálogo de eventos v1

Los 8 eventos de la tabla 6.2 de la arquitectura. No hay otros (D8): `contractsctl validate` rechaza un evento que no
esté en esa tabla. Fuente verificable: [`asyncapi/asyncapi.yaml`](../../asyncapi/asyncapi.yaml). Sobre común y
reglas de entrega: [convenciones §10](../convenciones.md#10-eventos).

| Evento | Productor | Consumidores | Canal lógico | Agregado (outbox) | Go |
|---|---|---|---|---|---|
| [InvoiceIssued](invoice-issued.md) | billing | fiscal, receivables | `billing.invoice-issued.v1` | `invoice` | `InvoiceIssuedV1` |
| InvoiceCancelled | billing | fiscal, receivables | `billing.invoice-cancelled.v1` | `invoice` | `InvoiceCancelledV1` |
| CreditNoteIssued | billing | fiscal, receivables | `billing.credit-note-issued.v1` | `credit_note` | `CreditNoteIssuedV1` |
| DebitNoteIssued | billing | fiscal, receivables | `billing.debit-note-issued.v1` | `debit_note` | `DebitNoteIssuedV1` |
| ElectronicDocumentAccepted | fiscal | bff, billing, notifications | `fiscal.electronic-document-accepted.v1` | `electronic_document` | `ElectronicDocumentAcceptedV1` |
| ElectronicDocumentRejected | fiscal | bff, billing, notifications | `fiscal.electronic-document-rejected.v1` | `electronic_document` | `ElectronicDocumentRejectedV1` |
| PaymentReceived | receivables | bff, reports | `receivables.payment-received.v1` | `payment` | `PaymentReceivedV1` |
| ReceivableSettled | receivables | bff, reports | `receivables.receivable-settled.v1` | `receivable` | `ReceivableSettledV1` |

Los canales son nombres lógicos; el broker y su mapeo los define infraestructura (P2, mensajería pendiente).

## Resumen por evento

**InvoiceCancelled** — anulación de una factura emitida: `invoiceId`, `invoiceNumber`, `cancelledAt`,
`cancelledByUserId`, `reason` (obligatorio), `currency` y `total`. Sin líneas: el consumidor ya tiene el detalle
por `InvoiceIssued`. Receivables registra un ajuste `cancellation`.

**CreditNoteIssued / DebitNoteIssued** — nota sobre una factura emitida, con la misma estructura de líneas y totales
que `InvoiceIssued` (`schemas/events/parts/issued-note.v1.json`) más `documentId`, `documentNumber`,
`referencedInvoiceId`, `referencedInvoiceNumber` y `referenceReason`. Los montos siempre son positivos: el tipo de
nota dice si resta (crédito) o suma (débito) al saldo. Solo la de débito lleva `dueDate`.

**ElectronicDocumentAccepted / Rejected** — cambio de estado fiscal: `electronicDocumentId`, `sourceType`
(`invoice`, `credit_note`, `debit_note`), `sourceDocumentId`, `sourceDocumentNumber`, `numericKey` (50 dígitos),
`consecutiveNumber` (20 dígitos) y `haciendaStatusMessage`. Aceptado agrega `acceptedAt`; rechazado agrega
`rejectedAt` y `rejectionReason` (Billing marca `requires_correction`).

**PaymentReceived** — pago registrado: `paymentId`, `customerId`, `receivedOn`, `amount` (> 0), `currency`,
`exchangeRate`, `paymentMethodCode`, `reference`, `receivedByUserId` y `applications[]` (puede ir vacío). La suma de
las aplicaciones nunca supera el pago.

**ReceivableSettled** — la cuenta por cobrar llegó a saldo cero: `receivableId`, `sourceInvoiceId`, `customerId`,
`documentNumber`, `currency`, `originalAmount` y `settledAt`.

## Qué verifican las herramientas

| Verificación | Dónde |
|---|---|
| El documento AsyncAPI cumple el meta-schema oficial 3.0.0 | `contractsctl validate` |
| Catálogo = tabla 6.2 (nombres, productor, consumidores); schema y canal con el nombre derivado | `contractsctl validate` |
| Los `const` del schema (`eventType`, `version`, `sourceService`) coinciden con el catálogo | `contractsctl validate` y `go test ./pkg/events` |
| Ejemplos válidos pasan y los inválidos fallan | `contractsctl validate` y `go test ./pkg/events` |
| Structs Go = schemas, sin `float`, round trip exacto, montos que cuadran | `go test ./pkg/events` |

## TODOs

- `TODO(fiscal)`: códigos de referencia de las notas, motivo de anulación codificado, códigos de rechazo de Hacienda y
  composición de la clave numérica y el consecutivo.
- Eventos de Platform (`OrganizationCreated`, `MemberAdded`...): fuera de v1 (D8).
