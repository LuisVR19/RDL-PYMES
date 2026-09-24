# 0007 · Reglas fiscales tomadas del borrador de los Anexos v4.4

- **Fecha:** 2026-09-24 · **Estado:** Aceptado, **provisional hasta revalidar con la versión oficial**
- **Fuente:** `RDL PYMES/docs/Hacienda/DGT-R-000-2024DisposicionesTecnicasDeComprobantesElectronicosCP.pdf`, borrador de
  consulta pública (septiembre 2024) de la resolución que se publicó como **MH-DGT-RES-0027-2024**, con sus Anexos 1
  (estructuras), 2 (firma) y 3 (API). La versión 4.4 es obligatoria desde el 1 de septiembre de 2025.

## Decisiones aplicadas en v0.2.0

1. **Redondeo (resuelve D2).** Anexo 1, "Especificaciones técnicas": los decimales se separan con punto, sin separador
   de miles, y se redondea mirando el decimal siguiente (menor que 5 no cambia; 5 o más sube). Montos Decimal 18,5,
   cantidades 16,3. Se aplica a 5 decimales en cada campo; el total del comprobante coincide con la suma de las líneas.
   Implementado en `money.RoundHalfUp` / `money.Round5`.
2. **Exoneración en puntos de tarifa.** Anexo 1, campos "Tarifa exonerada" (Decimal 4,2, expresada en puntos: 13 % es
   `13`) y "Monto del Impuesto Exonerado" (= tarifa exonerada × Subtotal). v0.1.0 tenía `percentage` como porcentaje
   **del impuesto**, que daba el mismo monto con otro significado y habría hecho rechazar los comprobantes. Se reemplaza
   por `exoneratedRate` (tipo común `TaxRate`) y cambia la fórmula.
3. **`grossAmount` en la línea.** Anexo 1, campo "MontoTotal" (= cantidad × precio unitario), obligatorio en el
   comprobante. La línea del evento no lo traía; ahora es obligatorio y `subtotal = grossAmount − discount`.

Son cambios incompatibles en `document-line.v1` y `exoneration.v1`. Se hacen en la misma versión de schema porque
v0.1.0 no se publicó ni tiene consumidores (convenciones §11 permite cambios incompatibles en `v0` con aviso en el
CHANGELOG). A partir del primer tag, un cambio así exige `v2`.

## Hallazgos registrados, sin aplicar (requieren decisión)

| Hallazgo | Fuente | Impacto |
|---|---|---|
| **Un comprobante válido no se anula**: se corrige con nota de crédito o débito; el código de referencia 01 es "Anula documento de referencia" | Resolución, art. 9; Anexo 1, Nota 9 | `InvoiceCancelled` y la anulación de la máquina de estados de la factura. Decisión 2 del prompt P5 |
| Un comprobante **rechazado por Hacienda no tiene validez** y no lleva nota; se sustituye (tipo de documento de referencia 10) | Art. 9; Nota 10 y su pie 28 | Flujo de corrección de rechazados; el evento no trae la referencia al sustituido |
| **Consecutivo** de 20 dígitos: establecimiento (3; 001 = casa matriz), terminal (5), tipo de comprobante (2) y número (10), por establecimiento o terminal | Anexo 1, Nota 3 | Informa D11 (sucursal ↔ establecimiento ↔ terminal); el alcance final lo decide fiscal |
| **Clave** de 50 dígitos: 506 + `ddmmaa` + identificación del emisor rellenada a 12 (Nota 4.1) + consecutivo + situación (1 normal, 2 contingencia, 3 sin internet) + código de seguridad de 8 | Anexo 1, Nota 3 | Glosario actualizado; lo implementa fiscal |
| Catálogos completos: tipos de comprobante, identificación, condición de venta, medios de pago, impuestos, tarifas de IVA, referencia, exoneración, unidades de medida, instituciones | Anexo 1, Notas 4 a 23 | Siguen como `FiscalCode` (formato) hasta cargarlos en `fiscal.*` desde la versión oficial |
| Hasta 5 descuentos por línea, cada uno sobre la base menos el anterior, con código de descuento (Nota 19) | Anexo 1, campo "Descuento" | El evento tiene un solo descuento por línea; suficiente para V1, anotado para `v2` |
| Márgenes de error para sistemas de 2 decimales (desde la v4.2), sin monto definido | Anexo 1, control de cambios | Fiscal lo confirma con la versión oficial o con el ambiente de pruebas |

## Consecuencias para otros repos

- **Billing** y **fiscal** tienen la columna `exoneration_percentage numeric(7,4)` en `invoice_line_taxes` y
  `electronic_document_line_taxes`. Con v0.2.0 debe guardar **puntos de tarifa** (4,2): hay que renombrarla o
  documentarla con migraciones expand → contract en cada API.
- Toda regla de esta lista se revalida cuando llegue la versión oficial de los anexos.
