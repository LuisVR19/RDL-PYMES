# 0005 · Consumo de eventos: inbox, reintentos y dead letter

- **Fecha:** 2026-09-25
- **Estado:** Aceptada (docs/PLAN.md §2.4, aprobado el 2026-09-24). Transporte pendiente: `TODO(P2)`.

## Contexto

Receivables consume eventos de Billing (`InvoiceIssued`; `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled`
en el incremento 8). La entrega es **al menos una vez y sin orden garantizado**, y el transporte todavía no existe
(P2). Un saldo mal aplicado por un evento duplicado es el peor error posible de este servicio.

## Decisión

1. **Un proceso aparte**, `cmd/consumer`, con su propio `/healthz` y `/readyz` (`CONSUMER_HTTP_ADDR`, 8084). Escala y
   se apaga por separado de la API. La misma imagen trae `api`, `consumer`, `replay` y `migrate`.
2. **La fuente es un puerto** (`internal/adapters/events.Source`): llama a `handle` por mensaje y lo da por consumido
   solo si devuelve `nil`. Hasta P2 la implementación es `NoSource` (no entrega nada) y los eventos entran con
   `cmd/replay`, que usa exactamente el mismo procesamiento.
3. **Un intento** (`app.HandleEvent`):
   1. valida el payload contra el JSON Schema de `RDL.Contracts` (`pkg/events.Validator`) y comprueba el productor;
   2. abre `WithinServiceTx(organizationId del sobre)`: `app.current_organization_id` sale **del evento**, sin
      usuario;
   3. `INSERT` en `integration.inbox_messages` con `ON CONFLICT DO NOTHING`; 0 filas = duplicado, se confirma sin
      efecto;
   4. efecto + `audit.audit_events` (`actor_type = 'service'`, `correlationId` del evento) + `processed_at`, todo en
      la misma transacción. Si algo falla, el rollback se lleva también el inbox.
4. **Clasificación de errores**:
   - permanentes → dead letter sin reintentos: payload inválido (`ErrInvalidEvent`), tipo o versión sin consumidor
     (`ErrUnsupportedEvent`), regla de negocio (`ErrRejectedEvent`: total 0, factura repetida con otros datos, más
     adelante cuenta anulada, etc.);
   - todo lo demás es transitorio (base caída, serialización, deadlock, unicidad por una carrera, y en el incremento 8
     "la factura todavía no existe"): 6 intentos, backoff exponencial de 1 s a 32 s con *equal jitter*; después,
     dead letter.
5. **La dead letter va en otra transacción** (la del efecto ya hizo rollback). Si no se puede escribir, el mensaje no
   se da por consumido y la fuente lo reentrega; el inbox evita el efecto doble. Un payload que ni siquiera es JSON se
   guarda como `{"unparseable": "<texto>"}` porque la columna es `jsonb`.
6. **Idempotencia por negocio además del inbox**: la misma factura en otro `eventId` (Billing la republicó) no crea
   otra cuenta; con los mismos datos es un no-op y con datos distintos va a dead letter.
7. **Apagado**: el mensaje en curso termina (su contexto no hereda la señal, solo `CONSUMER_MESSAGE_TIMEOUT`, 2 min
   por defecto); después `Run` vuelve.
8. **Métricas OTel**: `receivables.consumer.messages` (por `event_type` y `outcome`: processed, duplicate,
   dead_lettered, failed), `receivables.consumer.attempts` y `receivables.consumer.lag` (segundos desde
   `occurredAt`).

## Consecuencias

- Reprocesar cualquier evento es inocuo (invariante 4): lo prueban los tests con los ejemplos de contratos y
  `cmd/replay` dos veces sobre dev.
- Un tipo nuevo se agrega registrando su traducción en `events.NewDecoder` y su handler en `wiring.EventProcessor`,
  sin tocar los existentes.
- Las dead letters no se reprocesan solas: se resuelven a mano (`resolved_at`, `resolution_notes`) y, si corresponde,
  se reenvían con `cmd/replay`. Una herramienta para eso queda fuera de V1.
- `TODO(P2)`: el adapter del transporte real reemplaza a `NoSource`; debe respetar el contrato de `Source` (ack solo
  con `nil`).
