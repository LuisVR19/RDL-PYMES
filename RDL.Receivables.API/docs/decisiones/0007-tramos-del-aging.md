# 0007 · Tramos del aging

- **Fecha:** 2026-09-25
- **Estado:** Aceptada (decisión R6, docs/PLAN.md §3).

## Contexto

Contratos dejó los tramos abiertos (propuesta 0-30, 31-60, 61-90, +90). La base ya tiene la vista
`receivables.receivable_aging` con `current, 1_30, 31_60, 61_90, 90_plus`, pero calcula con `CURRENT_DATE` (UTC): entre
las 18:00 y las 24:00 de Costa Rica ya sería "mañana" (ADR 0001 §4.3).

## Decisión

- Tramos `current` (no vencida; **vencer hoy no es atraso**), `1_30`, `31_60`, `61_90`, `90_plus`, sobre los días de
  atraso `asOf − dueOn` (`internal/domain/aging`, funciones puras sobre fechas de calendario).
- `asOf` es una fecha de negocio: por defecto, hoy en `core.organizations.timezone`; nunca la fecha UTC.
- La consulta agrupa el saldo cobrable (`open`, `partially_paid`) por moneda y vencimiento; el tramo lo decide el
  dominio y la suma se hace con decimal exacto. **No se usa la vista.**
- La respuesta trae, por cada moneda presente, sus cinco tramos (en cero si no hay saldo). Sin conversión entre
  monedas. El corte usado va en `X-Aging-As-Of`.
- Un `asOf` pasado reclasifica los saldos **actuales** por vencimiento; no reconstruye el saldo histórico.

## Consecuencias

- Tests de bordes: día 0, 1, 30, 31, 60, 61, 90, 91, cambio de año y bisiesto, y una factura que vence "hoy" a las
  20:00 de Costa Rica.
- La vista `receivable_aging` queda para retirarla en una migración contract cuando se confirme que nadie la usa.
