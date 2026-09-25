# 0006 · Bloqueo y concurrencia en pagos y aplicaciones

- **Fecha:** 2026-09-25
- **Estado:** Aceptada (docs/PLAN.md §2.3).

## Contexto

Dos aplicaciones simultáneas a la misma cuenta no pueden dejar el saldo negativo, y un pago no puede aplicarse por
encima de su monto. Los triggers de la base ya bloquean en el orden **pago → cuenta**
(`payment_applications_validate` hace `FOR UPDATE` del pago y `recalculate_receivable` de la cuenta).

## Decisión

1. **El agregado decide bajo bloqueo.** Cada comando abre `WithinTenantTx` (o `WithinServiceTx`), bloquea con
   `SELECT … FOR UPDATE` y recién entonces carga el agregado y valida. Así el saldo que ve el dominio es el vigente.
2. **Orden estable:** primero todos los pagos (por id), después todas las cuentas (por id) — `app.lockAll`. Es el
   mismo orden de los triggers, así que dos transacciones sobre filas en común se esperan en vez de cruzarse.
   - Aplicar: pago, cuenta. Revertir: se busca la aplicación y se bloquea su pago y su cuenta.
   - Anular un pago: el pago y después sus cuentas con aplicaciones vigentes.
   - Registrar un pago con aplicaciones: el pago es nuevo (nadie más lo ve); se bloquean sus cuentas.
   - Nota de crédito o anulación de factura: se leen sin bloquear los pagos con aplicaciones vigentes, se bloquean
     esos pagos y después la cuenta; si el conjunto cambió entre la lectura y el bloqueo, error transitorio y el
     consumidor reintenta.
3. **La base es la última defensa y además se verifica.** Al cargar, `LockReceivable` exige que el saldo y el estado
   de la fila coincidan con los que deriva el agregado; después de escribir, `settle` relee lo que dejaron los triggers
   y lo compara. Una diferencia (`ErrInconsistentState`) hace rollback y responde 500 sin detalle: nunca debería pasar
   porque la fórmula es la misma (00004), pero si pasa no se confirma nada.
4. Aislamiento `READ COMMITTED` (el de Supavisor/pgx por defecto): los `FOR UPDATE` bastan; no hace falta
   `SERIALIZABLE` ni reintentos por serialización en la API.

## Consecuencias

- `tests/e2e` (`TestConcurrentApplicationsNeverOverdraw`) lanza dos aplicaciones del 60 % del saldo a la vez contra
  dev: exactamente una entra (201) y la otra recibe `application-exceeds-balance` (422); el saldo final es el 40 %.
- Los tests de casos de uso comprueban el orden de bloqueo registrado por el store en memoria.
- Ningún handler ni consumidor escribe `status` o `balance_amount`; el guard de la base (00003) lo impide aunque se
  intente.
