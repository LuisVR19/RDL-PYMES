# 0007 · Emisión y mapeo a `InvoiceIssued` v1

- **Fecha:** 2026-09-25 · **Estado:** Aceptado
- **Contrato:** `schemas/events/invoice-issued.v1.json` y `docs/eventos/invoice-issued.md` del repo de contratos
  (con el ADR 0007 de contratos: redondeo y `exoneratedRate`).

## Una transacción, sin llamadas a otros servicios

`POST /v1/invoices/{id}/issue` (`app.IssueInvoice`) hace todo en **una** transacción con la sesión RLS del tenant:

1. reserva la `Idempotency-Key` (un reintento de una emisión confirmada responde el mismo 201, no 409);
2. bloquea el borrador (`FOR UPDATE`); si no es `draft` → 409 `invoice-not-draft`; sin líneas → 422;
3. revalida en la base: cliente activo (422 `customer-inactive`), sucursal activa, productos activos;
4. fija `issuedAt` (reloj del servidor, en microsegundos como `timestamptz`) y la fecha de negocio en la zona de la
   organización (`core.organizations.timezone`): una emisión a las 23:30 de Costa Rica es del mismo día aunque en UTC
   ya sea el siguiente;
5. asigna el número visible bajo candado (ADR 0006);
6. `Invoice.Issue`: verifica que cada línea cuadre y que los totales sean la suma de las líneas, copia el snapshot del
   cliente y calcula el vencimiento (informe 0001 §4.4). Las líneas se congelan tal como estaban en el borrador;
7. escribe la transición (el `WHERE status = 'draft'` y el trigger `invoices_guard` son la segunda y tercera defensa),
   el historial `draft → issued`, el audit `invoice.issued` y el evento en el outbox.

No llama a E-Invoice, Receivables ni a nadie: emite aunque E-Invoice esté caída (criterio 2). Publicar el outbox es
del worker de infraestructura (P2).

## Mapeo y validación del evento

- El DTO es `events.InvoiceIssuedV1` del módulo de contratos: **se importa, no se copia**.
- `internal/adapters/events` traduce el agregado al DTO. Los montos se copian tal como quedaron en la factura (el
  evento no recalcula). `occurredAt = issuedAt`; `correlationId` = el `X-Correlation-Id` del request.
- Antes de escribir, el payload se **valida contra el JSON Schema embebido en el módulo de contratos** (la misma versión
  que el DTO). Si no valida, la emisión falla entera: no queda factura emitida sin evento ni evento inválido.
- La fila del outbox la arma `events.ToOutboxRow` del contrato (convenciones §10) y el payload viaja como texto
  (`::text::jsonb`, ADR 0004 de Platform).

## Cómo se verifica

- Contrato del evento: facturas con varias líneas, descuentos, varios impuestos, exoneración y líneas sin impuesto
  validan contra el schema; también cualquier factura aleatoria que el dominio puede calcular (`rapid`). Un evento roto
  no produce fila.
- Casos de uso con fakes transaccionales: camino feliz, reintento con la misma clave, 409, 404, cliente o producto
  inactivos, sin líneas, **outbox que falla → nada emitido y el número vuelve a quedar libre**, números consecutivos,
  secuencia de sucursal, vencimiento con crédito y criterio 5.
- Integración contra dev con `billing_api` (`TestIssueEndToEndSQL`): el mismo código de producción en savepoints de una
  transacción que se revierte al final (así no quedan eventos de prueba que el worker publicaría ni audit permanente).
  Verifica factura, historial, secuencia y outbox en la base, que el payload guardado valida, que el reintento no
  duplica el evento, el criterio 5 y el 409 de una segunda emisión.

## Pendiente

- `paymentMethods` del evento: los medios de pago son F4; hoy no se envían (el campo es opcional).
- `TODO(fiscal)`: qué condiciones de venta exigen plazo de crédito (`fiscal.sale_conditions`, vacío).
