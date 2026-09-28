# 0003 · Reutilización del código de Platform API

- **Fecha:** 2026-09-24
- **Estado:** Aceptada (plan aprobado).

## Contexto

El prompt P6 pide reutilizar lo que Platform ya resolvió (tenancy, JWT, transacción con `set_config`, correlación,
Problem Details, idempotencia, auditoría, suite de aislamiento) en lugar de reinventarlo, e importarlo o copiarlo con
un ADR.

## Decisión

**Copiar**, no importar.

- El módulo de Platform se llama `rdl/platform-api`: no es una ruta resolvible fuera de su repo y no tiene tags. Un
  `replace` hacia `../RDL.Platform.API` acoplaría el build de Receivables a todo Platform (sus dependencias y su
  `internal/`, que Go no deja importar de todos modos).
- Lo que se reutiliza es poco y estable, y ya tiene tests.
- Cada archivo copiado lleva en su comentario de paquete la línea
  `// Origen: RDL.Platform.API, copiado sin cambios de comportamiento (...)`. Los cambios se limitan a nombres
  (módulo, login `receivables_api`, `urn:rdl:receivables:problem:`, puerto 8083).

| Copiado en el incremento 1 | Pendiente (incrementos 2, 6, 10) |
|---|---|
| `internal/platform/{config,health,logger,telemetry}`, `pkg/correlation`, `pkg/requestinfo`, `internal/adapters/http/problem`, `internal/adapters/postgres/pool.go` | `pkg/tenancy`, `internal/adapters/auth`, `tx.go`/`txmanager.go`, idempotencia, auditoría, patrón de `tests/isolation` |

## Consecuencias

- Un arreglo en Platform hay que replicarlo aquí a mano. Mitigación: la línea de origen permite encontrar los archivos
  con `grep -r "Origen: RDL.Platform.API"`.
- Cuando infra (P2) publique un módulo de *building blocks*, estos paquetes se reemplazan por el import y se borran.
