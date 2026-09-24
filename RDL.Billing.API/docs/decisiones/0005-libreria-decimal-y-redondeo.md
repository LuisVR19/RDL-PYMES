# 0005 · Librería decimal y redondeo

- **Fecha:** 2026-09-25 · **Estado:** Aceptado
- **Regla de negocio:** D2, decidida en el ADR 0007 del repo de contratos v0.2.0 (reglas del borrador de los Anexos
  v4.4): 5 decimales, mitad hacia arriba mirando el sexto decimal, cada campo calculado de la línea; totales como sumas
  exactas. El contrato trae `money.Round5` como referencia; `money.Round` de Billing se prueba equivalente.

## Decisión: `github.com/shopspring/decimal`

| | shopspring/decimal | cockroachdb/apd |
|---|---|---|
| Suma, resta, multiplicación | exactas (big.Int + exponente) | exactas dentro de la precisión del `Context` |
| Redondeo a 5 decimales, mitad hacia arriba | `Round(5)`: trunca a 6 decimales y suma 5 → **mira el sexto decimal**, igual que el Anexo. Es "mitad lejos de cero", idéntico a mitad hacia arriba porque los montos nunca son negativos | `Context{Rounding: RoundHalfUp}.Quantize`, más explícito |
| API | valores inmutables, métodos encadenables | punteros, `Context` y `Condition` en cada operación |
| Riesgo | `Div` redondea a 16 dígitos por defecto | ninguno si se configura bien el contexto |

Se elige shopspring porque el cálculo de Billing **no divide**: `× tarifa / 100` es `Mul` + `Shift(-2)` (correr la
coma), que es exacto. Sin división, la ventaja de apd (contexto explícito) no aporta y su API hace el código más
largo y fácil de equivocar. Para que nadie use `Div` por accidente, todo pasa por `internal/domain/money`:

- `money.Round` es el **único** redondeo del sistema (hace pánico con un negativo: sería un error de cálculo);
- `money.Percent(base, tarifa) = Round(base × tarifa / 100)` con `Shift(-2)`;
- `money.Dec` / `money.ToAmount` convierten desde y hacia los tipos del contrato; `ToAmount` falla si el valor no cabe
  en `shared.money_amount` o tiene más de 5 decimales sin redondear.

Los montos nunca son `float` (tests de `internal/archtest`) y viajan como texto hasta la base (ADR 0004).

## Cálculo

`internal/domain/invoice/calculation.go`, funciones puras: `CalculateLine` (línea → impuestos → exoneraciones) y
`CalculateTotals` (sumas). Validaciones del cálculo: descuento ≤ monto bruto, a lo sumo un impuesto por tipo, tarifa
exonerada con formato 4,2 y ≤ tarifa del impuesto, todo monto dentro de 13 enteros.

## Cómo se verifica

- Casos de ejemplo, incluidos los de la mitad exacta (`0.000005 → 0.00001`, `0.000025 → 0.00003`, donde el
  redondeo bancario daría otra cosa).
- **Los ejemplos válidos de `InvoiceIssued` del repo de contratos se recalculan** y deben salir idénticos, línea por
  línea y en los totales (incluido el que necesita redondeo).
- Tests basados en propiedades (`pgregory.net/rapid`) con las 5 invariantes del planning, contra una referencia
  racional exacta (`math/big.Rat`).
- Mutaciones comprobadas a mano: cambiar `Round` por truncar o por redondeo bancario hace fallar los tests.

## Pendiente

- `TODO(fiscal)`: base imponible de impuestos que no son IVA, impuesto asumido por el emisor y hasta 5 descuentos
  encadenados por línea (hoy uno). Ninguno está en el modelo v1.
