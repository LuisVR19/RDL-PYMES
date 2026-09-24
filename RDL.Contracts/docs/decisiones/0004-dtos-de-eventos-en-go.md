# 0004 · DTOs de eventos en Go, escritos a mano y verificados contra el schema

- **Fecha:** 2026-09-24 · **Estado:** Aceptado

## Decisión

1. Los DTOs de `pkg/events` se escriben **a mano** (`InvoiceIssuedV1`, `Envelope`, `DocumentLine`...). El sobre va
   embebido, así que en JSON sus campos quedan en el mismo objeto que los del cuerpo.
2. Un **test de contrato** recorre cada schema (con sus `allOf` y `$ref`) y el struct correspondiente por reflexión,
   y falla si:
   - un campo está en uno y no en el otro;
   - un campo `required` tiene `omitempty`, o uno opcional no lo tiene (ni es puntero);
   - un tipo común no se mapea a su tipo Go (`money.json` → `money.Amount`, `uuid.json` → `uuid.UUID`,
     `utc-datetime.json` → `events.Instant`...);
   - aparece un `float` en cualquier DTO.
3. Cada ejemplo válido hace round trip (JSON → struct con `DisallowUnknownFields` → JSON) sin cambios y el resultado
   pasa el schema. Los montos de los ejemplos se verifican con `math/big.Rat`.
4. Los schemas se embeben en el módulo (`contracts.Schemas`) y `events.Validator` los compila. Una API valida contra
   **la misma versión** de los schemas que la de sus DTOs, sin leer archivos en tiempo de ejecución.
5. Un evento nuevo se agrega al `registry` del test y recibe todas las verificaciones. Un cambio incompatible agrega
   `...V2` y no toca `V1`.

## Por qué no generarlos

Los generadores de JSON Schema → Go (por ejemplo `omissis/go-jsonschema`) no producen nuestros tipos (`money.Amount`,
`Instant`, `Date`), tienen problemas con `allOf` + `unevaluatedProperties` y generan nombres que cambian cuando cambia
el schema. Escribirlos a mano con un test que los amarra da structs legibles, con la misma garantía.

## Consecuencias

- `Instant` normaliza la representación: `"...00.250Z"` sale como `"...00.25Z"`. Es el mismo instante; nadie debe
  comparar instantes como strings.
- `DocumentLine` escribe `taxes: []` aunque el slice sea nil, porque el schema exige un arreglo.
- `OutboxRow()` arma la fila de `integration.outbox_messages` (`aggregate_type` = `invoice`), pero no valida: el
  productor llama a `Validator.Validate` antes de escribir.
