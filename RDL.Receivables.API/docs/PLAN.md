# Plan · Receivables API (prompt P6)

- **Fecha:** 2026-09-24 · **Estado:** **aprobado** el 2026-09-24, incluidas las propuestas de R1 a R11.
- Informe de brechas: `decisiones/0001-estado-inicial-bd.md`. Propuesta para `database-platform`:
  `decisiones/0002-propuesta-database-platform.md` (logins y `GRANT` sobre `core`).

## 1. Datos del entorno

| Dato | Valor |
|---|---|
| Proyecto Supabase dev | `dzlsnsstuqpxvwegeqcy` (nunca producción) |
| Go | 1.27.1 (igual que Platform) |
| Contratos | `bitbucket.org/rdl/contracts` v0.1.0. **El tag no existe todavía**: `go.mod` usa `replace bitbucket.org/rdl/contracts => ../RDL.Contracts` hasta que se publique. El Dockerfile se construye con el monorepo como contexto |
| Puerto HTTP | 8083 (servidor del esqueleto OpenAPI de contratos) |
| Transporte de eventos (P2) | **pendiente**: puerto `EventSource` + `TODO(P2)` |
| Host del pooler | **falta**: lo pones tú en `.env` (`DB_POOLER_HOST`), igual que en Platform |

## 2. Arquitectura

La del prompt (hexagonal, igual que Platform). Decisiones de diseño:

### 2.1 Reutilización de Platform (ADR 0003)

El módulo de Platform es `rdl/platform-api`: no se puede importar desde otro repo. Se **copian** `pkg/tenancy`,
`pkg/correlation`, `pkg/requestinfo`, `internal/adapters/auth`, `tx.go`/`txmanager.go`, Problem Details, idempotencia y
el patrón de `tests/isolation`, con una cabecera que indica el origen. Cuando exista un módulo compartido (P2,
*building blocks*) se reemplaza por el import.

### 2.2 Dominio

- **`receivable.Receivable`** (agregado): original, débitos, créditos, anulación, aplicaciones vigentes. Métodos
  `ApplyPayment`, `ReverseApplication`, `AddCreditNote`, `AddDebitNote`, `Cancel`. El saldo y el estado se derivan
  **dentro** del agregado con la misma fórmula que `recalculate_receivable` (según R1); un método que dejaría el saldo
  negativo devuelve un error de dominio tipado. Cada operación devuelve si la cuenta **pasó a `paid`** (para emitir
  `ReceivableSettled`).
- **`payment.Payment`** (agregado): monto, moneda, cliente, aplicaciones vigentes. `Apply`, `ReverseApplication`,
  `Void` (revierte todas). Invariante: suma de vigentes ≤ monto.
- **Aplicar un pago** es un servicio de dominio que opera sobre los dos agregados, porque la regla cruza ambos
  (moneda, cliente, saldo, disponible del pago).
- **`aging`**: funciones puras sobre `date` (tipo `civil.Date` propio, sin hora): días de atraso = `asOf − due_on`,
  tramos (R6). "Hoy" se calcula con `time.Now().In(tz)` de la organización.
- **`collection`**: seguimientos y promesas (transiciones de R8).
- **`permission`**: la matriz, en un solo lugar (tabla de la sección 4).
- Decimales con **`shopspring/decimal`** (ADR 0004): solo suma, resta y comparación; ninguna división ni redondeo.
  Conversión en los bordes con `money.ParseAmount` / `String` del repo de contratos.

### 2.3 Base y transacciones

- La base **recalcula** saldo y estado en triggers. El agregado decide y valida primero (para responder con el problem
  type correcto); la base es la última defensa. Los tests de integración comparan lo que devuelve el agregado con lo
  que queda en la base.
- **Concurrencia (ADR 0006):** en cada transacción se bloquean con `SELECT … FOR UPDATE` primero **los pagos** (en
  orden de `id`) y después **las cuentas** (en orden de `id`), el mismo orden que usan los triggers (pago → cuenta).
  Anular un pago bloquea el pago y después todas sus cuentas; anular una factura bloquea los pagos que tienen
  aplicaciones vigentes en esa cuenta y después la cuenta.
- `TxManager.WithinTenantTx` (HTTP: organización + usuario) y `WithinServiceTx(org)` (consumidor: solo la organización
  del evento, sin usuario). Los dos escriben entidad, audit, inbox y outbox en la misma transacción.
- Con Supavisor: `QueryExecModeExec`, sin caché de statements y `jsonb` como texto (ADR 0004 de Platform).

### 2.4 Consumo de eventos (ADR 0005)

```
EventSource (puerto) ──► HandleEvent(ctx, payload []byte)
                           1. events.Validator por eventType → inválido: dead letter (sin reintentos)
                           2. WithinServiceTx(organizationId del sobre)
                           3. INSERT inbox (consumer_service='receivables') ON CONFLICT DO NOTHING
                              → 0 filas: duplicado, commit y fin
                           4. efecto + audit (actor_type='service', correlationId del evento) + outbox; processed_at
                         fallo transitorio → reintento con backoff; al agotarlos → dead letter
```

- **Errores permanentes** (payload inválido, moneda o cliente distinto de la cuenta, regla abierta no resuelta): dead
  letter directo con el motivo.
- **Errores transitorios** (base caída, serialización, deadlock) y **"la factura todavía no existe"** (una nota llegó
  antes que su `InvoiceIssued`; la entrega es al menos una vez y sin orden garantizado): reintento. Propuesta: 6
  intentos con backoff exponencial de 1 s a 32 s y jitter; después, dead letter.
- La dead letter se escribe en **otra** transacción (la del efecto ya hizo rollback).
- `cmd/consumer`: proceso aparte con health (`/healthz`, `/readyz`) y métricas OTel (procesados, duplicados,
  reintentos, dead letters, antigüedad del último mensaje). Hasta P2 usa un `EventSource` vacío con `TODO(P2)`.
- `cmd/replay` (y `make replay FILE=…`): lee un evento suelto o un archivo de ejemplos de contratos (`valid[]`) y lo
  pasa por el mismo `HandleEvent`.

## 3. Decisiones que necesito de ti (el prompt pide no resolverlas solo)

| # | Tema | Propuesta |
|---|---|---|
| **R1** | Estado derivado: base vs contrato (0001 §4.1) | **A:** migración en `receivables` para que la base siga el contrato (`open` = saldo igual a original + débitos) |
| **R2** | Anulación de una factura con pagos aplicados (parcial o total) | Al recibir `InvoiceCancelled`: revertir las aplicaciones vigentes con motivo "Factura anulada", registrar el ajuste `cancellation` por el saldo que queda y dejar la cuenta en `cancelled`. Lo que se había aplicado vuelve a estar **disponible en el pago** (ese es el "saldo a favor", sin tabla nueva) y se puede aplicar a otra factura. Sin evento nuevo |
| **R3** | Nota de crédito mayor que el saldo | Igual que R2: revertir aplicaciones (las más recientes primero) solo hasta donde haga falta. Si ni así alcanza (la nota supera el total adeudado), dead letter con el motivo |
| **R4** | Nota de débito: vencimiento y cuenta `cancelled` | Se suma a la cuenta de la factura y se **conserva el `due_on` de la factura** (el `dueDate` de la nota queda en el audit; `TODO` en contratos). Sobre una cuenta `cancelled`: dead letter |
| **R5** | Castigo (`write_off`) | Fuera de alcance en V1: no hay endpoint. Si se agrega, la base ya lo cuenta como crédito y deja la cuenta en `paid` |
| **R6** | Tramos del aging | `current` (no vencida; vence hoy = no atrasada), `1_30`, `31_60`, `61_90`, `90_plus`, los mismos códigos de la vista existente. Respuesta por moneda y tramo; sin conversión entre monedas |
| **R7** | Pago anulado sin evento | No se inventa. El BFF lo ve por la API. `TODO` para el catálogo |
| **R8** | Promesas de pago | `pending → kept | broken | cancelled`; los tres son finales. "Cumplida" la marca el usuario (sin automatismo en V1) |
| **R9** | Privilegios de más (`DELETE` en seguimientos y promesas, escritura en la vista) | Que los revoque `database-platform` (0002 §3) |
| **R10** | ¿Quién llama al endpoint interno del BFF? | Mismo JWT del usuario (así lo define `bff-internal.yaml`) y roles de lectura **más `biller`**, porque la vista transversal de facturación muestra el saldo |
| **R11** | Pago con aplicaciones de otro cliente | La base lo prohíbe. Se agrega `customer-mismatch` (422) a `problems/receivables.yaml` por PR a contratos |

## 4. Matriz de permisos (propuesta para revisar en equipo)

| Operación | owner | admin | collector | accountant | read_only | biller |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| Leer cuentas, pagos, aging, seguimientos | ✔ | ✔ | ✔ | ✔ | ✔ | — |
| Saldo por factura (interno BFF) | ✔ | ✔ | ✔ | ✔ | ✔ | ✔ (R10) |
| Registrar pago, aplicar, seguimientos, promesas | ✔ | ✔ | ✔ | — | — | — |
| Anular pago, revertir aplicación | ✔ | ✔ | — | — | — | — |

## 5. Cambios que salen hacia otros repos

- **database-platform:** logins, `GRANT` sobre `core` (bloquea el incremento 2), revocaciones (0002).
- **RDL.Contracts** (PR con `CHANGELOG`): completar `openapi/receivables.yaml` (promesas, detalle con aplicaciones,
  ajustes y seguimientos, paginación, errores), `customer-mismatch` y, si eliges R1-B, `receivable.yaml`.

## 6. Historias, en orden (cada incremento compila, pasa sus tests y pausa para revisión)

1. **Esqueleto:** `go.mod`, config (`.env`), logger, OTel, `/healthz`, `/readyz`, Dockerfile (api, consumer, replay,
   migrate), Makefile, `.golangci.yml`, pipeline, `cmd/migrate` con `up-by-one`, baseline 00001 marcada como aplicada
   e índices 00002 (con los logins creados).
2. **Tenancy:** JWT (JWKS), `TenantContext`, revalidación de membresía en `core` con caché corta, `GET /v1/receivables`
   (vacío). **Requiere el `GRANT` sobre `core`; si no está aplicado, me detengo aquí.**
3. **Dominio de saldos:** agregados, estado derivado (R1), migraciones 00003/00004, tests con `rapid` de las 5
   invariantes y tests de transiciones generados desde los YAML de contratos.
4. **Consumidor:** `HandleEvent`, inbox, dead letter, reintentos, `cmd/consumer`, `cmd/replay`, `InvoiceIssued`.
   Tests con los ejemplos de contratos, duplicados, payload inválido y evento de otra organización.
5. **Interno BFF:** `GET /internal/v1/receivables/by-invoice/{invoiceId}`.
6. **Pagos y aplicaciones:** `POST/GET /v1/payments`, `POST /v1/payment-applications`, `PaymentReceived`,
   `ReceivableSettled`, idempotencia, test de concurrencia.
7. **Reversos y anulación:** `POST /v1/payment-applications/{id}/reverse`, `POST /v1/payments/{id}/void`.
8. **Notas y anulaciones:** consumidores de `CreditNoteIssued`, `DebitNoteIssued`, `InvoiceCancelled` (R2 a R4).
9. **Aging y cobranza:** `GET /v1/receivables/aging`, detalle `GET /v1/receivables/{id}`, seguimientos y promesas.
10. **Endurecimiento:** suite de aislamiento (6 criterios, incluido el bypass de 0001 §4.2), E2E con `cmd/replay`,
    autorrevisión, README, CLAUDE.md, `docs/ESTADO.md` y lista final de TODOs.

## 7. ADRs previstos

0003 reutilización de Platform · 0004 librería decimal · 0005 consumo de eventos (inbox, reintentos, dead letter,
transporte pendiente) · 0006 bloqueo y concurrencia · 0007 tramos del aging · 0008 suite de aislamiento.
