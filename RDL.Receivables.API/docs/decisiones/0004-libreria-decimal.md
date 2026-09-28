# 0004 · Librería decimal: shopspring/decimal

- **Fecha:** 2026-09-25
- **Estado:** Aceptada (docs/PLAN.md §2.2).

## Contexto

Los montos son `shared.money_amount` (`numeric(18,5)`). El prompt prohíbe `float` y pide elegir entre
`shopspring/decimal` y `cockroachdb/apd` con un ADR. Receivables solo suma, resta y compara: no calcula impuestos, no
divide ni redondea (aging sin conversión entre monedas, R6).

## Decisión

- **`github.com/shopspring/decimal`**: API simple (`Add`, `Sub`, `Cmp`), precisión arbitraria, sin contexto de
  redondeo que configurar. `apd` sobra para un dominio sin divisiones.
- En los bordes, los montos viajan como **texto**: el JSON con `money.ParseAmount` de contratos (patrón de
  `schemas/common/money.json`), la base con `::text` en las consultas y `::text::numeric` en los inserts. Ningún monto
  pasa por `float64` ni por `numeric` de pgx.
- `internal/domain/amount.Positive` valida la forma de `numeric(18,5)` (> 0, 13 enteros, 5 decimales) y **rechaza**
  un sexto decimal en vez de redondearlo: redondear un monto es una decisión de negocio.
- Las respuestas usan `trim_scale` o `decimal.String()`: `"1300.5"` y no `"1300.50000"` (mismo valor, el formato Money
  admite ambos).

## Consecuencias

- Los tests de propiedades generan montos con 0 a 5 decimales y comparan con `Equal`, nunca con `==` de strings.
- Si algún día hay aplicaciones entre monedas, el tipo de cambio y el redondeo son una decisión nueva (prompt P6).
