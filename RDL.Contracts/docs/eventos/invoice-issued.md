# InvoiceIssued v1

- **Schema:** [`schemas/events/invoice-issued.v1.json`](../../schemas/events/invoice-issued.v1.json)
- **Go:** `events.InvoiceIssuedV1` (`pkg/events`)
- **Ejemplos:** [`examples/events/invoice-issued.v1.json`](../../examples/events/invoice-issued.v1.json)
  (3 válidos y 20 inválidos)
- **Productor:** Billing, al emitir una factura (`draft` → `issued`), en la misma transacción que la emisión.
- **Consumidores:** fiscal (crea el documento electrónico en `processing`) y Receivables (crea la cuenta por cobrar).
  Ninguno vuelve a consultar a Billing: el evento trae todo lo necesario. Ambos deduplican por `eventId`.

## Campos

Además del [sobre común](../convenciones.md#10-eventos), con `eventType = "InvoiceIssued"`, `version = 1` y
`sourceService = "billing"`:

| Campo | Tipo | Obl. | Origen en Billing | Notas |
|---|---|:-:|---|---|
| `invoiceId` | Uuid | ✅ | `invoices.id` | `aggregate_id` del outbox |
| `branchId` | Uuid | | `invoices.branch_id` | Falta si la factura no tiene sucursal |
| `invoiceNumber` | string ≤ 50 | ✅ | `invoices.number` | Número visible, no el consecutivo fiscal |
| `issuedAt` | UtcDateTime | ✅ | `invoices.issued_at` | |
| `issueDate` | BusinessDate | ✅ | `issued_at` en la zona de la organización | Receivables la usa como `issued_on` |
| `dueDate` | BusinessDate | ✅ | `invoices.due_date` | De contado = `issueDate`. Nunca anterior a `issueDate` |
| `creditTermDays` | integer ≥ 0 | | `invoices.credit_term_days` | |
| `issuedByUserId` | Uuid | ✅ | `invoices.issued_by_user_id` | |
| `saleConditionCode` | FiscalCode | ✅ | `invoices.sale_condition_code` | `TODO(fiscal)` |
| `currency` | CurrencyCode | ✅ | `invoices.currency_code` | |
| `exchangeRate` | ExchangeRate | ✅ | `invoices.exchange_rate` | 1 en moneda local |
| `customerSnapshot` | CustomerSnapshot | ✅ | `invoices.customer_*` | Copia al emitir |
| `lines[]` | DocumentLine (≥ 1) | ✅ | `invoice_lines` + `invoice_line_taxes` | Snapshot del producto |
| `paymentMethods[]` | `{paymentMethodCode, amount?}` | | `invoice_payment_methods` | Declarados, no son pagos |
| `subtotal`, `discount`, `tax`, `exoneration`, `total` | Money | ✅ | `invoices.*_amount` | Ver fórmulas |
| `notes` | string ≤ 2000 | | `invoices.notes` | |

**Línea** (`schemas/events/parts/document-line.v1.json`): `lineNumber`, `productId?`, `productCode?`, `cabysCode`,
`description`, `unitOfMeasureCode`, `isService`, `quantity`, `unitPrice`, `discount`, `discountReason` (obligatorio
si `discount` ≠ 0), `subtotal`, `tax`, `total` y `taxes[]` (vacío si no lleva impuestos).

**Impuesto** (`parts/line-tax.v1.json`): `taxTypeCode`, `taxRateCode?`, `rate`, `taxableBase`, `amount` y
`exoneration?`. Un tipo de impuesto por línea como máximo (unicidad de la base).

**Exoneración** (`parts/exoneration.v1.json`): `documentTypeCode`, `documentNumber`, `institution`, `issuedAt`,
`percentage` y `amount`. Todos los campos o ninguno, igual que el `CHECK` de la base.

## Fórmulas

Todas las cifras son `Money` (string decimal, ≥ 0). Las igualdades valen **después del redondeo**, cuya regla está
pendiente (D2, `TODO(fiscal)`). Los ejemplos del repo están elegidos para no necesitar redondeo, y un test las
verifica con aritmética racional exacta.

Por línea:

```
subtotal          = quantity × unitPrice − discount
taxes[i].amount   = taxes[i].taxableBase × taxes[i].rate / 100
exoneration.amount = taxes[i].amount × exoneration.percentage / 100      (propuesta; TODO(fiscal))
tax               = Σ taxes[i].amount − Σ taxes[i].exoneration.amount   (impuesto neto)
total             = subtotal + tax
```

Por factura:

```
subtotal    = Σ lines.subtotal            (después de descuentos)
discount    = Σ lines.discount            (informativo)
tax         = Σ lines.taxes[].amount      (antes de exoneraciones)
exoneration = Σ lines.taxes[].exoneration.amount
total       = subtotal + tax − exoneration = Σ lines.total
```

Estas definiciones siguen las columnas de `billing.invoices` e `invoice_lines`. Si la especificación de Hacienda
define los totales de otra forma (por ejemplo, un subtotal antes de descuentos), el cambio se hace en `v2`.

## Reglas para el productor

1. Escribir el evento en `integration.outbox_messages` **en la misma transacción** que la emisión
   (`InvoiceIssuedV1.OutboxRow()` arma la fila) y validarlo antes con `events.DefaultValidator()`.
2. `occurredAt` = `issuedAt` (el hecho de negocio es la emisión).
3. `correlationId` = el `X-Correlation-Id` del request de emisión.
4. Los montos se copian tal como quedaron en la factura; el evento no se recalcula.

## Reglas para los consumidores

1. Deduplicar por `eventId` (`integration.inbox_messages`): reprocesar no crea documentos ni CxC duplicados
   (criterio de aceptación 3).
2. Usar `organizationId` como tenant de la transacción y `correlationId` en todo lo que se derive.
3. Rechazar a la dead letter un payload que no pase el schema; no "arreglarlo".

## TODOs

- `TODO(fiscal)`: catálogos de todos los `*Code`, fórmula de la exoneración, regla de redondeo (D2) y si el
  comprobante necesita provincia, cantón y distrito del cliente.
- ¿`branchId` debería ser obligatorio? La base lo admite `null`; fiscal lo necesitará para el establecimiento (D11).
