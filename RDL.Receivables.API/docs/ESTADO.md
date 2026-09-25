# Estado del proyecto · Receivables API

**Última actualización:** 2026-09-25
**Punto de corte:** los 10 incrementos del plan terminados y probados contra dev. Nada está commiteado todavía. Queda la
lista de TODOs del final para revisar en equipo.

## Decisiones aprobadas (2026-09-24)

Informe de brechas (ADR 0001), propuesta para `database-platform` (ADR 0002) y el plan completo (`docs/PLAN.md`),
incluidas R1 a R11 con las opciones recomendadas:
R1 la base sigue al contrato (migración de `recalculate_receivable`) · R2/R3 revertir aplicaciones al anular o al
acreditar de más (lo liberado queda disponible en el pago) · R4 la nota de débito conserva el `due_on` de la factura
y sobre una cuenta `cancelled` va a dead letter · R5 sin castigo en V1 · R6 tramos `current, 1_30, 31_60, 61_90,
90_plus` · R7 sin evento de pago anulado · R8 promesas `pending → kept | broken | cancelled` · R9 las revocaciones las
aplica `database-platform` · R10 el endpoint interno admite además `biller` · R11 nuevo problem type
`customer-mismatch`.

## Incrementos

| # | Incremento | Estado |
|---|---|---|
| 1 | Esqueleto, config, health, OTel, logins y baseline | ✅ |
| 2 | JWT, TenantContext, `GET /v1/receivables` | ✅ |
| 3 | Dominio de saldos, agregados, migraciones 00003/00004 | ✅ (00003/00004 aplicadas en dev) |
| 4 | Consumidor: inbox, dead letter, reintentos, `cmd/consumer`, `cmd/replay`, `InvoiceIssued` | ✅ |
| 5 | `GET /internal/v1/receivables/by-invoice/{invoiceId}` | ✅ |
| 6 | Pagos y aplicaciones, `PaymentReceived`, `ReceivableSettled`, idempotencia, concurrencia | ✅ |
| 7 | Reverso de aplicación y anulación de pago | ✅ |
| 8 | `CreditNoteIssued`, `DebitNoteIssued`, `InvoiceCancelled` (R2 a R4) | ✅ |
| 9 | Aging, detalle de la cuenta, seguimientos y promesas | ✅ |
| 10 | Suite de aislamiento, E2E, autorrevisión, documentación, TODOs | ✅ |

## Qué hay

- **Dominio** (`internal/domain`): `receivable` (saldo y estado derivados con la misma fórmula que la base, R1),
  `payment`, `settlement` (reglas entre los dos: cliente, moneda, disponible, saldo, reversiones de R2/R3), `aging`
  (R6), `collection` (R8), `amount` (forma de `numeric(18,5)`, sin redondeo), `civil` (fecha de negocio), `permission`.
  La cuenta tiene id propio, distinto del de la factura (antes lo compartía y la PK global habría chocado entre
  organizaciones).
- **Casos de uso** (`internal/app`): un struct por caso. Comandos idempotentes por `Idempotency-Key` (24 h, en
  `integration.idempotency_keys`), con audit event y outbox en la misma transacción; bloqueo en orden estable (pagos
  y después cuentas, por id, ADR 0006); después de escribir se compara lo que dejaron los triggers con el agregado.
- **Consumidor** (ADR 0005): `HandleEvent` + `ProcessEvent` (inbox, reintentos 6× con backoff de 1 s a 32 s, dead letter
  en otra transacción), `cmd/consumer` (health en 8084, métricas OTel), `cmd/replay`. Consume `InvoiceIssued`,
  `CreditNoteIssued`, `DebitNoteIssued` e `InvoiceCancelled`; una nota que llega antes que su factura se reintenta.
- **HTTP**: todos los endpoints de `api/openapi.yaml` (un test lo compara con el router), Problem Details con los tipos
  de contratos más `customer-mismatch`, 405 por método (se cambió el router copiado de Platform para que
  `/v1/receivables/aging` no choque con `/v1/receivables/{id}`).
- **Postgres**: ledger de agregados, lecturas, outbox (DTOs de contratos validados contra el schema antes de
  insertar), idempotencia, cobranza, inbox, dead letters y auditoría. Los ids de un `any(...)` viajan como `text[]`:
  con `QueryExecModeExec` (Supavisor) pgx no codifica un `[]uuid.UUID` sin tipo.
- **Migraciones**: 00001 baseline (marcada), 00002 índices, 00003 guard con `pg_trigger_depth() > 1`, 00004
  `recalculate_receivable` según R1. Las cuatro aplicadas en dev.
- **ADRs**: 0001 brechas, 0002 propuesta a database-platform, 0003 reutilización de Platform, 0004 decimal, 0005
  consumo de eventos, 0006 bloqueo y concurrencia, 0007 tramos del aging, 0008 suites contra dev.

## Verificación (2026-09-25)

- `go build`, `go vet` (también con `-tags=integration`), `golangci-lint --build-tags=integration` (sin issues propios;
  ver CRLF abajo) y `go test ./...`: dominio 82–100 % (aging, amount, civil, collection y permission al 100 %),
  casos de uso 84 %, eventos 86 %, HTTP 67 %.
- Tests de propiedades con `rapid` de las 5 invariantes, con reenvíos de eventos y verificación de que toda operación
  válida se acepta; tests de transiciones generados desde `receivable.yaml` y `payment.yaml`.
- Contra dev con el login `receivables_api`:
  - `make test-isolation` (6 criterios + bypass del guard): verde. Organizaciones de prueba de Platform
    `f404f95e-…` (A) y `add6ee45-…` (B), en el `.env` local.
  - `make test-e2e`: verde (factura → pago que la cancela con `PaymentReceived` y `ReceivableSettled` en el outbox con
    el mismo `correlationId` → repetición idempotente → anulación con el saldo de vuelta; aplicaciones y reverso con
    los problem types del contrato; dos aplicaciones simultáneas del 60 % → exactamente una entra; notas, anulación de
    factura y reprocesos; aging, seguimiento y promesa).
  - `make test-integration`: el guard rechaza el `UPDATE` directo de saldo y estado con el GUC fijado.
  - `cmd/api` y `cmd/consumer` levantados: `/readyz` 200 (base, JWKS); endpoints sin token → 401.
- Binarios estáticos para Linux (`api`, `consumer`, `replay`, `migrate`) compilan. **La imagen Docker no se construyó**:
  Docker Desktop no estaba corriendo en esta máquina.

## Datos que quedaron en dev

La app no puede borrar (sin `DELETE` en cuentas, pagos ni aplicaciones), así que quedaron datos ficticios:
- organización de los ejemplos de contratos (`304b6c9e-…`, no existe en `core`): 3 cuentas de `cmd/replay` y 20 dead
  letters de los ejemplos inválidos;
- organizaciones de prueba A y B: las cuentas, pagos, seguimientos y promesas que crean las suites en cada corrida.
Si molestan, las limpia `database-platform`.

## TODOs para revisar en equipo

**Transporte (P2)**
- `TODO(P2)`: transporte de eventos. El consumidor usa `events.NoSource`; el adapter real implementa `events.Source`
  (ack solo si `handle` devuelve nil). Tampoco hay publicador del outbox: los eventos quedan en
  `integration.outbox_messages` sin `published_at`.
- `TODO(P2)`: el pipeline asume el repo solo, pero el build necesita `../RDL.Contracts` (replace) y los tests leen sus
  ejemplos: hace falta el monorepo o el tag de contratos.

**Repo de contratos (PRs, no se resuelven aquí)**
- Crear el tag `v0.1.0` y quitar el `replace`.
- `customer-mismatch` (422) en `problems/receivables.yaml` (R11); ya se usa.
- `openapi/receivables.yaml`: incorporar lo que completa `api/openapi.yaml` (promesas de pago, campos de FollowUp,
  Payment con aplicaciones, detalle de la cuenta con aplicaciones y ajustes).
- `state-machines/receivable.yaml`: faltan `paid → open` (revertir la única aplicación, R1), `paid → cancelled` (R2) y
  `paid → partially_paid` por `CreditNoteIssued` (R3). Están permitidas en `pendingInContract` y en
  `TestTransitionsPendingInContract`.
- Sin evento de pago anulado (R7): el BFF se entera por la API.
- Factura de total 0 (exonerada por completo): `original_amount > 0` en la base; hoy va a dead letter.
- Factura anulada después de acreditarla por completo: no hay saldo que anular (la base no admite un ajuste de 0); hoy
  va a dead letter.
- Nota de débito con vencimiento propio (R4): se conserva el de la factura y el de la nota queda en el audit.

**database-platform**
- Incorporar a su repo las migraciones 0010 y 0011 aplicadas desde aquí, y las revocaciones de R9 (ADR 0002 §3).
- Catálogos `fiscal.payment_methods` y `fiscal.sale_conditions` vacíos: `paymentMethodCode` solo se valida por formato.

**Decisiones de producto**
- Tipo de cambio: se guarda el que manda el cliente (1 por defecto). Aplicaciones entre monedas: fuera de V1.
- El `customerId` de un pago no se valida contra un maestro de clientes (Receivables no lo tiene); una aplicación exige
  el mismo cliente que la cuenta.
- `settled_at` se llena cuando el saldo llega a 0, también en `cancelled` (comportamiento de la base, sin cambio).
- Aging con `asOf` pasado: reclasifica los saldos actuales, no reconstruye el histórico.
- Seguimientos y promesas se listan sin paginación (las 200 más recientes).
- Dead letters: se resuelven a mano (`resolved_at`) y se reenvían con `cmd/replay`; no hay herramienta en V1.

**Pendiente de verificar**
- Construir la imagen Docker (`make docker`) con Docker Desktop encendido.
- Probar con un JWT real de Supabase (hook de `org_id`): el verificador está cubierto con un JWKS propio y las suites
  simulan solo la firma.
- La baseline 00001 nunca se ejecutó desde cero (no hay base local).
- CRLF: con `core.autocrlf=true` los `.go` quedan con CRLF en Windows y `golangci-lint` (gofmt) los marca aunque en el
  índice estén en LF. Propuesta: `.gitattributes` con `*.go text eol=lf`.
