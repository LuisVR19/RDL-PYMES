# Estado del proyecto · Portal Gateway

Handoff entre sesiones. Se actualiza al cerrar cada incremento.

**Última actualización:** 2026-09-25
**Punto de corte:** incrementos 1 a 4 del prompt P7 y la parte del 8 que no depende de otros (aislamiento y
e2e contra Platform real). Tiempo real y read model quedan como TODO documentados, por decisión explícita de
alcance.

## Qué está hecho

| # | Incremento | Estado |
|---|---|---|
| 1 | Esqueleto: config validada, logger JSON, OpenTelemetry, `/healthz`, `/readyz`, Dockerfile, Makefile | ✅ |
| 2 | Verificación del JWT (JWKS de Supabase) y propagación de identidad, correlación y trazas | ✅ |
| 3 | Tabla de rutas y paso directo de las cuatro APIs (55 rutas) | ✅ |
| 4 | Vista transversal `GET /portal/v1/invoices/{id}/overview` con degradación | ✅ |
| 5 | Listados enriquecidos sin N+1 | 🟡 hecho contra las rutas por lote **propuestas** (sin publicar) en contratos |
| 6 | Notificaciones en tiempo real | ⛔ bloqueado (P2) |
| 7 | Read model | ⛔ decisión pendiente |
| 8 | Endurecimiento: tests de aislamiento entre organizaciones, e2e contra dev | 🟡 Platform ✅ · Billing sin correr · Docker sin construir |

**Verificación al cierre:** `go build`, `go vet` y `golangci-lint run` (0 issues) limpios;
`go test ./...` en verde (90 pruebas). Prueba de humo del binario: arranca, `/healthz` 200, `/readyz` 200
`degraded` con Platform y Billing apagados y el JWKS real respondiendo, 401 sin token, 404 en ruta no
declarada. Nada ha corrido todavía contra las APIs reales (ver «Sin verificar»).

> ⚠️ **`go test -race` no corre en esta máquina:** el detector de carreras necesita cgo y no hay compilador de
> C en el `PATH`. Afecta igual a `make test` de Platform y de Billing, que lo usan. Se corrió `go test` sin
> `-race`. Para habilitarlo hace falta instalar un toolchain de C (por ejemplo MSYS2 o WinLibs) y que `gcc`
> quede en el `PATH`.

### Incremento 5 · listado de documentos sin N+1 (2026-09-25)

- `GET /portal/v1/invoices` pasa de paso directo a **composición**: la página de Billing (`GET /v1/invoices`), más
  el estado fiscal (`POST /internal/v1/electronic-documents/by-source`) y el saldo
  (`POST /internal/v1/receivables/by-invoice`) de todas sus filas, **una llamada por API por página**. Los dos
  lotes van en paralelo después de Billing (necesitan sus ids); una página vacía no los llama.
- Cada fila trae `invoice` (sin líneas), `fiscal` y `receivable` con la misma `availability` del `overview`
  (ADR 0004): el lote trae la factura → `available`; respondió sin ella → `absent`; falló o su API no está →
  `unavailable` en todas las filas. Si falla Billing, su error tal cual (`writePrimaryError`, compartido con el
  `overview`).
- Solo viajan a Billing los filtros de `routes.InvoiceListParams` (los de su `GET /v1/invoices`): una
  organización no puede colarse ni por la query.
- Pruebas: dominio (unión por fila), caso de uso (una llamada por lote con los ids de la página, degradación,
  página vacía) y router real con APIs falsas: **25 facturas → exactamente 1 llamada por API** (probado en
  negativo: con una llamada por fila la prueba falla), filtros, degradación, lote que falla, error de Billing.
  `tests/isolation` y el e2e cubren el listado (sin Billing, el e2e comprueba el 502). **E2E 32/32 y
  aislamiento en verde contra Platform real.**
- **Sin verificar contra APIs reales:** Billing no arranca aquí, y E-Invoice y Receivables no existen. Las rutas
  por lote son la propuesta sin publicar de contratos: si cambian al aprobarse, se ajustan `readers.go` y sus
  pruebas.

### Vista transversal contra la ruta interna de Billing (2026-09-25)

Billing ya implementa `GET /internal/v1/invoices/{id}/summary`. `BILLING_SUMMARY_SOURCE` pasa a `internal` por
defecto: el `overview` ya no trae la factura con sus líneas. `public` queda solo como respaldo para una Billing
anterior. Pruebas nuevas: la vista lee la ruta del contrato con el token del usuario, y el respaldo sigue
funcionando. **Sin probar contra Billing real** (no arranca en esta máquina).

### Incremento 8 · lo hecho el 2026-09-25

- **`tests/isolation`** (tag `integration`, `make test-isolation`): router real en proceso, verificador real
  contra el JWKS de dev, tokens reales de los dos usuarios de prueba y Platform corriendo en local. Cada corrida
  crea una organización nueva por usuario, a través del gateway, y comprueba que A no ve ni toca nada de B:
  organización actual, membresías, `organizationId` por query y `X-Organization-Id` ignorados, activar una
  organización ajena (404), sucursal e invitación ajenas (404 al leer, editar y revocar; ausentes de los
  listados), 30 peticiones concurrentes intercaladas sin cruzar identidades, y token real con la firma alterada
  cortado en el gateway. **8/8 en verde contra Platform real.** Los 2 casos de Billing (cliente ajeno; borrador
  ajeno, su historial y su `overview`) se saltan: Billing no tiene `.env` en esta máquina.
- **`scripts/dev/e2e.sh`** (`make e2e`), contra el binario corriendo: salud, 401, paso directo a Platform,
  `X-Correlation-Id` (se respeta si es UUID, se reemplaza si no; se verificó en el log de Platform que llega el
  mismo), tenant que no se deja elegir, problema de Platform reenviado sin tocar, superficie cerrada (ruta no
  declarada, `/internal`, `/v1` sin `/portal`, 405, sin `Server`), 503 `upstream-not-configured` de fiscal,
  Billing caída → 502 `upstream-unavailable` también en el `overview`, y CORS. **31/31 en verde.** Con Billing
  arriba, el script cambia a comprobar el dato (clientes 200, `overview` inexistente 404).
- `make lint` ahora incluye `--build-tags=integration`, para que la suite también pase el linter.
- **Fines de línea:** con `core.autocrlf=true`, los `.go` quedan en CRLF al hacer checkout y `golangci-lint`
  (gofmt) los marca todos. Se normalizaron a LF; git no ve diferencia. Para que no vuelva a pasar, conviene un
  `.gitattributes` con `*.go text eol=lf` (no se agregó: lo decide quien publica el repo).

## Decisiones tomadas

- **ADR 0001** · Inventario de pantallas y APIs: el mapa de las 36 pantallas del portal a sus APIs.
- **ADR 0002** · Tabla de rutas, listas blancas de headers y **las cuatro APIs declaradas desde el día uno**.
  `/readyz` distingue crítico (JWKS → 503) de degradable (una API caída → 200 `degraded`).
- **ADR 0003** · Clientes escritos a mano; reintentos solo en lecturas; sin caché de negocio.
- **ADR 0004** · Degradación con un campo `availability` **aparte** del `status` de cada API.

## Pendiente de confirmación del equipo

- **¿Un reintento en los lotes por POST?** Son lecturas sin efectos, pero la regla del `CLAUDE.md` dice «solo
   GET, un reintento», así que hoy **no se reintentan**: si fallan, esa parte sale `unavailable`. Si el equipo
   lo permite, es poner `Retryable` en `postReadJSON` y rearmar el cuerpo en cada intento.
- **El «saldo en esta página» del diseño** es una suma de montos: el gateway no suma (regla del repo). La hace el
   portal con `big.js`, o se le pide a Receivables un total por lote.

1. **ADR 0004 · la forma de la vista.** El prompt P7 sugiere `fiscal: { status: "unavailable" }`; se
   implementó `fiscal: { availability: "unavailable" }` porque lo otro metería un estado inventado en la
   máquina de estados del documento electrónico, y el contrato manda sobre el prompt. **Si el equipo prefiere
   la forma del prompt, el cambio es de una línea** en `internal/adapters/http/overview.go`.
2. **`problems/portal-gateway.yaml`** y 3. **las rutas por lote** (ADR 0001 §4) ya están en el repo de contratos
   como cambio **sin publicar** (CHANGELOG, 2026-09-25): faltan las 2 aprobaciones. Nombres finales:
   `POST /internal/v1/electronic-documents/by-source` y `POST /internal/v1/receivables/by-invoice`.
4. **La forma de la vista transversal** debería quedar en un `openapi/portal-gateway.yaml` del repo de
   contratos; hoy solo vive en `api/openapi.yaml` de este repo.

## Bloqueos, con su dueño

| Bloqueo | Qué desbloquea | Dueño |
|---|---|---|
| Aprobar las rutas por lote (ya propuestas en contratos, sin publicar) | Dar por definitivo el incremento 5 | **Equipo (2 aprobaciones)** |
| Transporte de eventos sin decidir | Incremento 6 (notificaciones) | **P2** |
| Sin schema ni rol para este servicio; el `CHECK` de `integration` no incluye `portal-gateway` | Incremento 7 (read model) | **database-platform** |
| E-Invoice y Receivables no existen | Módulos D (4 pantallas) y E (6 pantallas) del portal | **P5 y P6** |
| Nadie expone `audit.audit_events` | Pantalla 32 · Exportar auditoría | **Equipo** |

## Sin verificar todavía

- **Billing a través del gateway.** El aislamiento y el e2e ya corren contra Platform real, pero Billing no
  arranca en esta máquina (falta su `.env` con la contraseña de `billing_api`). Cuando esté arriba en `:8081`,
  `make test-isolation` y `make e2e` la cubren sin tocar nada: sus casos hoy se saltan o comprueban la
  degradación. Queda pendiente con ella el `overview` de una factura **emitida** con token real (paso 6 del prompt).
- **La imagen Docker no se ha construido** (Docker no está instalado).

## TODOs en el código

| Dónde | Qué |
|---|---|
| `internal/domain/routes/routes.go` | Confirmar rutas y campos de fiscal y receivables cuando existan sus repos |
| `internal/adapters/http/problem/problem.go` | Registrar los problem types en el repo de contratos |
| `cmd/gateway/main.go`, `Makefile`, `Dockerfile` | `replay` y el cierre ordenado de conexiones SSE (incremento 6) |
| `api/openapi.yaml` | Enriquecer `GET /portal/v1/invoices` en el incremento 5 |

## Siguiente paso recomendado

**Levantar Billing** (su `.env` y los tres pasos manuales de `docs/CHECKPOINTS.md` §4) y correr aquí
`make test-isolation` y `make e2e`: es lo único que falta para cerrar la verificación de punta a punta del
paso directo y del `overview` con la ruta interna.

Después, del lado del gateway solo queda lo bloqueado: notificaciones (P2), read model (almacenamiento) y la
imagen Docker. Lo que más rinde ahora es **cablear el portal** contra `/portal/v1`, incluido este listado.
