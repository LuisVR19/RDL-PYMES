# 0002 · Dinero y decimales como string

- **Fecha:** 2026-09-24 · **Estado:** Aceptado

## Decisión

1. Montos, tipos de cambio, cantidades y porcentajes viajan en JSON como **string decimal**, con la misma precisión
   que los dominios de `shared` (ver `docs/convenciones.md` §3). Nunca `number`.
2. En Go, `pkg/events/money` tiene un tipo por concepto (`Amount`, `ExchangeRate`, `Quantity`, `Percentage`,
   `Currency`) que **guarda el string validado**, no un número. `UnmarshalJSON` rechaza un JSON number.
3. El paquete **no hace aritmética**. Cada API calcula con la librería decimal que elija (Billing propone
   `shopspring/decimal` o `cockroachdb/apd` en su propio ADR) y convierte con `Parse*` / `String()`.
4. Los patrones de Go y de JSON Schema son el mismo texto. Un test compara las constantes con `schemas/common/*.json`
   y otro aplica a los tipos Go los mismos ejemplos válidos e inválidos que al schema.

## Por qué

- `number` en JSON se lee como `float64` en casi todos los lenguajes: `0.1 + 0.2` y montos de 13 dígitos pierden
  precisión en silencio. Con string, el consumidor decide cómo parsear sin perder nada.
- Guardar el string evita elegir por todas las APIs una librería decimal y su semántica de redondeo, que todavía no
  está decidida (D2). Un contrato no necesita sumar: necesita transportar sin pérdida.
- El round trip es exacto: `"1300.50"` sale como `"1300.50"`, así que un hash o una firma sobre el payload no cambian.

## Consecuencias

- `"1300.5"` y `"1300.50"` son el mismo monto con distinta representación. Comparar montos es trabajo de la librería
  decimal de cada API, no de este paquete.
- Montos siempre `>= 0`: el signo nunca se usa para expresar descuentos, notas de crédito o ajustes.
- El valor cero de `Amount` y `Percentage` es `"0"`; el de `Quantity`, `ExchangeRate` y `Currency` no se puede
  serializar (su dominio no admite cero ni vacío).
