# Estado del proyecto · Portal Gateway

Handoff entre sesiones. Se actualiza al cerrar cada incremento.

**Última actualización:** 2026-09-24
**Punto de corte:** incrementos 1 a 4 del prompt P7 (esqueleto, identidad, paso directo y vista transversal
con degradación). Tiempo real y read model quedan como TODO documentados, por decisión explícita de alcance.

## Qué está hecho

| # | Incremento | Estado |
|---|---|---|
| 1 | Esqueleto: config validada, logger JSON, OpenTelemetry, `/healthz`, `/readyz`, Dockerfile, Makefile | ✅ |
| 2 | Verificación del JWT (JWKS de Supabase) y propagación de identidad, correlación y trazas | ✅ |
| 3 | Tabla de rutas y paso directo de las cuatro APIs (55 rutas) | ✅ |
| 4 | Vista transversal `GET /portal/v1/invoices/{id}/overview` con degradación | ✅ |
| 5 | Listados enriquecidos sin N+1 | ⛔ bloqueado (ver abajo) |
| 6 | Notificaciones en tiempo real | ⛔ bloqueado (P2) |
| 7 | Read model | ⛔ decisión pendiente |
| 8 | Endurecimiento: tests de aislamiento entre organizaciones, e2e contra dev | ⏳ pendiente |

**Verificación al cierre:** `go build`, `go vet` y `golangci-lint run` (0 issues) limpios;
`go test ./...` en verde (90 pruebas). Prueba de humo del binario: arranca, `/healthz` 200, `/readyz` 200
`degraded` con Platform y Billing apagados y el JWKS real respondiendo, 401 sin token, 404 en ruta no
declarada. Nada ha corrido todavía contra las APIs reales (ver «Sin verificar»).

> ⚠️ **`go test -race` no corre en esta máquina:** el detector de carreras necesita cgo y no hay compilador de
> C en el `PATH`. Afecta igual a `make test` de Platform y de Billing, que lo usan. Se corrió `go test` sin
> `-race`. Para habilitarlo hace falta instalar un toolchain de C (por ejemplo MSYS2 o WinLibs) y que `gcc`
> quede en el `PATH`.

## Decisiones tomadas

- **ADR 0001** · Inventario de pantallas y APIs: el mapa de las 36 pantallas del portal a sus APIs.
- **ADR 0002** · Tabla de rutas, listas blancas de headers y **las cuatro APIs declaradas desde el día uno**.
  `/readyz` distingue crítico (JWKS → 503) de degradable (una API caída → 200 `degraded`).
- **ADR 0003** · Clientes escritos a mano; reintentos solo en lecturas; sin caché de negocio.
- **ADR 0004** · Degradación con un campo `availability` **aparte** del `status` de cada API.

## Pendiente de confirmación del equipo

1. **ADR 0004 · la forma de la vista.** El prompt P7 sugiere `fiscal: { status: "unavailable" }`; se
   implementó `fiscal: { availability: "unavailable" }` porque lo otro metería un estado inventado en la
   máquina de estados del documento electrónico, y el contrato manda sobre el prompt. **Si el equipo prefiere
   la forma del prompt, el cambio es de una línea** en `internal/adapters/http/overview.go`.
2. **`problems/portal-gateway.yaml`** está propuesto en `docs/propuestas/`: falta el PR al repo de contratos
   (2 aprobaciones + entrada en el CHANGELOG).
3. **Rutas por lote** para los listados (ADR 0001 §4): falta el mismo PR.
4. **La forma de la vista transversal** debería quedar en un `openapi/portal-gateway.yaml` del repo de
   contratos; hoy solo vive en `api/openapi.yaml` de este repo.

## Bloqueos, con su dueño

| Bloqueo | Qué desbloquea | Dueño |
|---|---|---|
| Billing no implementa `GET /internal/v1/invoices/{id}/summary` (es esqueleto en `bff-internal.yaml`) | La vista transversal definitiva y, con las rutas por lote, el incremento 5 | **Billing (P4)** |
| Rutas por lote de estados fiscales y saldos | Incremento 5 (listados sin N+1) | **Contracts (PR)** |
| Transporte de eventos sin decidir | Incremento 6 (notificaciones) | **P2** |
| Sin schema ni rol para este servicio; el `CHECK` de `integration` no incluye `portal-gateway` | Incremento 7 (read model) | **database-platform** |
| E-Invoice y Receivables no existen | Módulos D (4 pantallas) y E (6 pantallas) del portal | **P5 y P6** |
| Nadie expone `audit.audit_events` | Pantalla 32 · Exportar auditoría | **Equipo** |

## Sin verificar todavía

- **No ha corrido contra las APIs reales.** Toda la suite usa servidores falsos (`httptest`). Falta levantar
  Platform y Billing y probar el paso directo y el `overview` con un token real de Supabase dev.
- **Falta la suite de aislamiento** (`tests/isolation`, carpeta creada y vacía): dos usuarios de
  organizaciones distintas contra el router real. Hoy el aislamiento se apoya en que el gateway no decide la
  organización y cada API la revalida, y eso está probado con APIs falsas, no de punta a punta.
- **Falta el e2e** (`scripts/dev/e2e.sh`, como el de Platform y Billing).
- **La imagen Docker no se ha construido.**

## TODOs en el código

| Dónde | Qué |
|---|---|
| `internal/adapters/downstream/readers.go` | `SummaryPublic` es temporal: pasar a `internal` cuando Billing exponga la ruta |
| `internal/domain/routes/routes.go` | Confirmar rutas y campos de fiscal y receivables cuando existan sus repos |
| `internal/adapters/http/problem/problem.go` | Registrar los problem types en el repo de contratos |
| `cmd/gateway/main.go`, `Makefile`, `Dockerfile` | `replay` y el cierre ordenado de conexiones SSE (incremento 6) |
| `api/openapi.yaml` | Enriquecer `GET /portal/v1/invoices` en el incremento 5 |

## Siguiente paso recomendado

**Implementar `GET /internal/v1/invoices/{id}/summary` en Billing.** Es lo que más desbloquea: deja la vista
transversal en su forma definitiva y es el primer paso de los listados enriquecidos, que es lo que de verdad
le quita los datos simulados a `RDL.Web.Portal`.
