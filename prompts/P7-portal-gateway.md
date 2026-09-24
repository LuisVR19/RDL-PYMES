# Prompt · P7 Portal Gateway (BFF del portal web) en Go

> **Cómo usarlo**
> 1. Crea el repo `RDL.Portal.Gateway` vacío (junto a `RDL.Platform.API`, `RDL.Contracts` y las APIs de dominio) y copia en `docs/contexto/` el documento de arquitectura y el planning (los mismos `.md` de `RDL.Platform.API/docs/contexto/`).
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano todo lo que toque tokens, propagación de headers, caché y notificaciones en tiempo real: un error aquí mezcla datos de dos empresas en una misma pantalla.

---

## Rol y objetivo

Eres un ingeniero backend senior en Go. Vas a construir el **Portal Gateway**, el BFF (*Backend For Frontend*) del portal web de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. El Portal Gateway es el **único punto de entrada del portal**: recibe al usuario, reenvía sus peticiones a las APIs dueñas (Platform, Billing, E-Invoice/`fiscal` y Receivables) y **compone** lo que una pantalla necesita de varias de ellas. El ejemplo de la arquitectura (2.2) es la vista transversal de una factura:

```
Factura FE00100034
Total: ₡113 000  |  Hacienda: Aceptada  |  Saldo: ₡63 000
```

El total es de Billing, el estado fiscal de E-Invoice y el saldo de Receivables. El Portal Gateway los junta **sin crear dependencias entre dominios** (criterio de aceptación 9).

**Lo que el Portal Gateway no es** (planning P7, donde se le llama BFF): no tiene reglas de negocio, no escribe en los schemas de ninguna API y no calcula montos ni impuestos. Si una pantalla necesita una regla, esa regla pertenece a una API. Es un mesero: pide, junta y entrega; no cocina.

**Entregable:** el repositorio `RDL.Portal.Gateway` listo para staging, con el alcance del planning P7 (semanas 6 a 20): esqueleto con propagación de identidad y correlación, vista transversal por composición, listados paginados, notificaciones en tiempo real del estado fiscal y, si se aprueba su almacenamiento, un read model propio. Debe compilar, pasar los tests (incluidos los de aislamiento entre tenants), tener Dockerfile, health checks y documentación.

## Contexto que debes leer primero

- `docs/contexto/arquitectura-v1.md`: secciones **2.1** (fronteras), **2.2** (vista transversal), **5.2** (identidad del tenant), **6.2** (eventos Accepted, Rejected, PaymentReceived y ReceivableSettled que consume), **8.2** (observabilidad: trazas BFF → API → worker) y **12** (criterios 1, 2 y 9).
- `docs/contexto/planning-v1.md`: sección **P7 BFF** (este servicio) y "Cómo trabajamos con Claude Code".
- **Repo de contratos** en `<<ruta a RDL.Contracts>>`. **El contrato manda**; si contradice este prompt, detente y pregunta. Lee:
  - `docs/convenciones.md` (tenancy, errores, idempotencia, correlación, paginación) y `docs/glosario.md`;
  - los OpenAPI de cada API (`openapi/platform.yaml`, `billing.yaml`, `fiscal.yaml`, `receivables.yaml`) y `openapi/bff-internal.yaml` (las rutas internas que este servicio compone; el nombre del archivo es anterior a este prompt);
  - `openapi/components/common.yaml` (Problem, headers, paginación);
  - `docs/eventos/README.md` y el paquete `pkg/events` (DTOs y `Validator`), para las notificaciones y el read model;
  - `state-machines/*.yaml` (estados que el portal muestra) y `docs/ESTADO.md` (pendientes abiertos).
- **Platform API** en `<<ruta a RDL.Platform.API>>`: la referencia de arquitectura. Reutiliza `internal/adapters/auth` (verificación de JWT con JWKS de Supabase), `pkg/correlation`, el formato de Problem Details, `internal/wiring`, la estructura de `cmd/` y su `CLAUDE.md`. Lee sus ADR 0003 (organización activa) y 0005 (router).
- Si ya existen `RDL.Billing.API`, `RDL.EInvoice.API` o `RDL.Receivables.API`, léelos para conocer sus rutas reales; si no, trabaja contra los OpenAPI del repo de contratos con servidores falsos (`httptest`).

## Datos del entorno

- URLs de las APIs en dev: Platform `<<http://localhost:8080>>`, Billing `<<http://localhost:8081>>`, fiscal `<<http://localhost:8082>>`, Receivables `<<http://localhost:8083>>`. El Portal Gateway escucha en `<<:8090>>`.
- Proveedor de identidad: **Supabase Auth**. JWKS: `https://<<project-ref>>.supabase.co/auth/v1/.well-known/jwks.json`. El JWT trae `sub`, `org_id` y `org_roles`.
- Origen del portal web (CORS): `<<http://localhost:5173>>`.
- Módulo del repo de contratos: `<<ej. bitbucket.org/rdl/contracts>>` en la versión `<<vX.Y.Z>>`.
- Transporte de eventos (P2): `<<broker o cola decidido, o "pendiente">>`.
- Almacenamiento del read model: `<<decisión pendiente>>` (ver Paso 5).
- Versión de Go: `<<1.27+>>`.
- **El Portal Gateway no tiene credenciales de base de datos en V1.** Nunca uses la `service_role` key.

## Paso 1 · Inventario de lo que el portal necesita (obligatorio)

Antes de escribir código, arma el mapa entre pantallas y APIs:

1. Lista las pantallas del portal del planning P8 (facturación, configuración fiscal, cobranza, administración) y, para cada una, qué datos muestra y de qué API sale cada dato.
2. Recorre los OpenAPI del repo de contratos y clasifica cada operación que el portal usará:
   - **paso directo** (una pantalla, una API: crear cliente, emitir factura, registrar pago);
   - **composición** (una pantalla, varias APIs: vista transversal de factura, listado de facturas con estado fiscal y saldo, ficha de cliente con su cartera);
   - **tiempo real** (cambios de estado fiscal, pagos).
3. Detecta los **N+1**: un listado de 50 facturas no puede hacer 100 llamadas internas. Hoy `openapi/bff-internal.yaml` solo tiene rutas de a una factura. Propón las rutas internas **por lote** que hacen falta (por ejemplo, estados fiscales y saldos de una lista de ids, con un máximo de ids por llamada) como **PR al repo de contratos**; no las inventes solo aquí.
4. Revisa qué eventos necesita el Portal Gateway (`ElectronicDocumentAccepted`, `ElectronicDocumentRejected`, `PaymentReceived`, `ReceivableSettled`) y cómo le llegarían (el transporte es de P2).

Entrega un **informe** en `docs/decisiones/0001-inventario-pantallas-y-apis.md` con el mapa de pantallas y datos, la clasificación de operaciones, las rutas internas por lote propuestas para el repo de contratos, lo que falta en los OpenAPI y las decisiones abiertas.

**Detente y espera mi aprobación del informe antes de implementar.**

## Paso 2 · Identidad, tenancy y propagación

**Regla de oro:** el Portal Gateway **nunca decide ni fabrica el tenant**. La organización activa viaja en el JWT del usuario y cada API la revalida contra su base.

1. **Verificación del token en la entrada:** firma con JWKS (con caché), `exp`, `aud` e `iss`, reutilizando el adapter de Platform. Un token inválido se corta en el Portal Gateway (401) y nunca llega a las APIs.
2. **Reenvío:** a cada API se le pasa **el mismo** `Authorization: Bearer <token del usuario>`. Prohibido usar un token de servicio o credenciales propias para leer datos de un usuario.
3. **No hay atajos de tenancy:** el Portal Gateway no agrega `organizationId` a ninguna ruta, query, header ni body; no lo lee del cliente; y no revalida la membresía por su cuenta (no tiene acceso a `core`): lo hace cada API.
4. **Headers que se propagan:** `Authorization`, `X-Correlation-Id` (se genera si falta y se devuelve siempre), `Idempotency-Key` (tal cual, en los comandos), `Accept-Language` y el contexto de trazas W3C (`traceparent`, `tracestate`) vía OpenTelemetry. Cualquier otro header del cliente se descarta por defecto (lista blanca).
5. **Errores:** los Problem Details de una API se reenvían **sin cambios** (mismo status y mismo `type`). Los errores propios del Portal Gateway (API caída, timeout) usan `urn:rdl:portal-gateway:problem:<código>`; registra esos tipos en un `problems/portal-gateway.yaml` propuesto como PR al repo de contratos.
6. **CORS** solo para el origen del portal; sin cookies de sesión (el token viaja en el header).

## Paso 3 · Alcance funcional

Rutas públicas bajo `/portal/v1/...`. Una sola tabla de rutas declara, por cada ruta, su tipo (paso directo, composición o tiempo real), la API destino y los headers permitidos: **nada de reverse proxy abierto** que reenvíe cualquier ruta.

**Semana 6: esqueleto**
- Config, logger, OpenTelemetry (trazas gateway → API), `GET /healthz`, `GET /readyz` (JWKS y las cuatro APIs responden), Dockerfile y pipeline.
- Propagación de identidad y correlación funcionando contra `GET /v1/me` de Platform.

**Semanas 7 a 10: paso directo, composición y listados**

| Tipo | Ejemplos | Regla |
|---|---|---|
| Paso directo | Perfil y membresías (Platform), clientes, productos, facturas y emisión (Billing), configuración fiscal (fiscal), pagos (Receivables) | Solo reenvía. Mismo método, mismo cuerpo, mismos errores. **Sin reintentos automáticos en comandos**: el cliente reintenta con la misma `Idempotency-Key` |
| Composición | `GET /portal/v1/invoices/{id}/overview`: documento de Billing + estado fiscal + saldo | Llamadas en paralelo con timeout por API |
| Listados | `GET /portal/v1/invoices`: página de Billing enriquecida con estado fiscal y saldo | Una llamada por API y por página (rutas por lote del Paso 1), paginación por cursor de Billing |

**Degradación:** si una API secundaria falla o no responde a tiempo, la composición **no falla entera**. El overview devuelve lo que sí llegó y marca la parte faltante (por ejemplo `fiscal: { status: "unavailable" }`). Así el portal sigue mostrando la factura aunque E-Invoice esté caída, en línea con el criterio 2. Si falla la API principal (Billing en una vista de factura), sí se responde el error de Billing.

**Semanas 11 a 16: notificaciones en tiempo real**
- Endpoint de suscripción (propón SSE o WebSocket en un ADR; SSE es suficiente si solo el servidor empuja): `GET /portal/v1/notifications`.
- Fuente: eventos `ElectronicDocumentAccepted`, `ElectronicDocumentRejected`, `PaymentReceived` y `ReceivableSettled`, validados con `events.Validator` del repo de contratos.
- **Aislamiento estricto:** una conexión recibe solo eventos cuyo `organizationId` es el `org_id` del token con el que se abrió. Al abrirla, el Portal Gateway valida la membresía llamando a Platform con el token del usuario (Platform revalida contra su base). La conexión se cierra al vencer el token; el cliente reconecta con uno nuevo.
- **Transporte:** es de P2 y no está decidido. **No inventes un broker.** Detrás de un puerto (`EventSource`), entrega el caso de uso, un adapter de desarrollo que reproduzca los ejemplos de `examples/events/` del repo de contratos y un `TODO(P2)` donde se conectará el real.
- **Base de datos:** la tabla `integration.inbox_messages` solo acepta los servicios `platform`, `billing`, `fiscal` y `receivables` (`CHECK` de la base), y el Portal Gateway no tiene rol de base. En esta fase no uses inbox en base: deduplica en memoria por `eventId` (las notificaciones son avisos de pantalla, no efectos de negocio) y documenta la decisión.

**Semanas 15 a 20: read model propio** (bloqueado por una decisión)
- Objetivo del planning: listados y reportes que no le peguen a las tres APIs en cada carga, alimentados por eventos.
- **Contradicción a resolver antes de empezar:** el planning dice que el BFF "no escribe en la base", pero un read model necesita dónde guardarse. Hoy no existe un schema ni un rol para este servicio, y los `CHECK` de `integration` no incluyen `portal-gateway`. Presenta las opciones en el informe (schema `portal` propio en la misma instancia con rol `portal_gateway_app`, lo que exige cambios en `database-platform`; un almacén separado; o posponerlo) y **espera mi decisión**. No crees schemas ni roles por tu cuenta.
- Si se aprueba, el read model es desechable: se reconstruye reprocesando eventos y nunca es fuente de verdad (un dato que decide algo se consulta en la API dueña).

**Fuera de alcance:** reglas de negocio, cálculos de montos o impuestos, escrituras en los schemas de dominio, la app móvil (usará este mismo gateway después de V1) y la consola interna.

## Paso 4 · Resiliencia y rendimiento

- **Timeouts** por API (configurables) y un presupuesto total por petición. Ninguna llamada interna sin timeout.
- **Reintentos** solo en lecturas idempotentes (`GET`), con backoff corto y como máximo uno o dos. **Nunca** en comandos.
- **Concurrencia acotada** en las composiciones (`errgroup` con límite) y cancelación por `context` cuando el cliente se va.
- **Caché:** en V1, ninguna de datos de negocio. Si se propone alguna, la clave incluye organización **y** usuario, tiene TTL corto y va con un ADR. El JWKS sí se cachea.
- **Clientes HTTP** con pool de conexiones y `otelhttp`. Propón en un ADR si los clientes se escriben a mano o se generan desde los OpenAPI del repo de contratos (por ejemplo con `oapi-codegen`); en cualquier caso, un test verifica que las respuestas de las APIs cumplen sus schemas.
- **Límite de tamaño** del cuerpo en la entrada, igual que Platform.

## Paso 5 · Arquitectura y calidad de código

La misma arquitectura hexagonal (ports & adapters) con Clean Architecture que Platform, aunque aquí el dominio es fino: la lógica es de composición, no de negocio.

```
RDL.Portal.Gateway/
├── cmd/
│   ├── gateway/main.go          # composition root: config, wiring, arranque
│   └── replay/main.go           # adapter de desarrollo: reproduce eventos para las notificaciones
├── internal/
│   ├── domain/                  # sin imports de infraestructura
│   │   ├── view/                # modelos de vista (InvoiceOverview, InvoiceRow...) y reglas de degradación
│   │   ├── routes/              # tabla declarativa de rutas: tipo, API destino, headers permitidos
│   │   └── notification/        # filtro por organización, deduplicación por eventId
│   ├── app/                     # casos de uso (componer overview, listar facturas, suscribirse) + puertos por API
│   ├── adapters/
│   │   ├── http/                # handlers, router, CORS, Problem Details, SSE/WebSocket
│   │   ├── downstream/          # clientes de Platform, Billing, fiscal y Receivables; propagación de headers
│   │   ├── auth/                # verificación de JWT (igual que Platform)
│   │   └── events/              # decodificación y validación con pkg/events; EventSource de desarrollo
│   ├── wiring/                  # arma handlers y clientes; lo usan cmd/* y los tests
│   └── platform/                # config (.env), logger, OTel, health
├── pkg/correlation/             # reutilizado de Platform
├── tests/{integration,isolation}/
├── docs/{contexto,decisiones}/  docs/ESTADO.md
├── api/openapi.yaml             # rutas públicas /portal/v1/...
├── CLAUDE.md, README.md, Dockerfile, Makefile, .golangci.yml
```

**Stack:** `net/http` con el router de Go 1.22+, la misma librería de JWKS que Platform, `log/slog`, OpenTelemetry (`otelhttp` en entrada y salida), `golang.org/x/sync/errgroup` y el módulo de contratos (`pkg/events`, `Validator`). Ninguna otra dependencia sin justificarla.

**Principios que se tienen que notar en el código:**
- **Sin reglas de negocio:** si aparece un `if` sobre un estado de factura, un rol o un monto para decidir algo, está en el repo equivocado. La única lógica permitida es de presentación y composición (qué API llamar, cómo juntar, cómo degradar).
- **Puertos por API** (`BillingReader`, `FiscalStatusReader`, `BalanceReader`...), definidos del lado del consumidor en `app`, implementados en `adapters/downstream`. Los casos de uso se prueban con fakes.
- **Una sola tabla de rutas:** agregar una pantalla es agregar filas, no escribir otro proxy.
- `context.Context` primero; timeouts; graceful shutdown que cierre ordenadamente las conexiones de notificaciones.
- Errores propios tipados, mapeados a Problem Details en un único lugar; los de las APIs se reenvían sin tocar.
- Configuración por variables de entorno validadas al arrancar; cero secretos en el código.
- Nombres claros, funciones cortas; comenta el **porqué**, no el qué.

## Paso 6 · Tests (escríbelos desde los requisitos, no desde la implementación)

- **Propagación:** con servidores falsos (`httptest`) para las cuatro APIs, cada ruta reenvía exactamente `Authorization`, `X-Correlation-Id`, `Idempotency-Key` y `traceparent`, y descarta el resto. El `X-Correlation-Id` generado por el Portal Gateway llega a todas las APIs de una misma composición.
- **Tenancy:**
  1. un token inválido o vencido se corta en el Portal Gateway (401) y ninguna API recibe la llamada;
  2. el Portal Gateway nunca agrega `organizationId` a una llamada interna, aunque el cliente lo mande en la query, el body o un header;
  3. el token que recibe cada API es el del usuario, byte por byte;
  4. una API que responde 403 o 404 (otra organización, rol insuficiente) se refleja tal cual.
- **Composición y degradación:** overview completo; fiscal caído (la vista sale con el estado fiscal `unavailable`); Receivables lento (se respeta el timeout y la vista sale sin saldo); Billing con 404 (la vista responde 404); el cliente cancela y las llamadas internas se cancelan.
- **Listados sin N+1:** una página de N facturas hace exactamente una llamada por API (se cuenta en el servidor falso).
- **Comandos:** el cuerpo y la `Idempotency-Key` llegan intactos y un 5xx de la API **no** se reintenta.
- **Notificaciones (aislamiento):** dos suscriptores de organizaciones distintas; un evento de A llega solo al de A; un evento duplicado se entrega una vez; un payload inválido se descarta; la conexión se cierra al vencer el token.
- **Contrato:** las respuestas de los servidores falsos salen de los ejemplos y schemas del repo de contratos, y las respuestas públicas del Portal Gateway cumplen su `api/openapi.yaml`.
- **Integración** (build tag `integration`), si las APIs reales están disponibles en dev: login real en Supabase y overview de una factura emitida.

## Paso 7 · Operación y entrega

- `GET /healthz` (vivo) y `GET /readyz` (JWKS y las cuatro APIs responden; una API caída marca el readiness como degradado con el detalle en el log, no en la respuesta).
- Métricas: latencia y errores por API, composiciones degradadas, conexiones de notificaciones abiertas y eventos entregados.
- Dockerfile multi-stage distroless, usuario no root, con los binarios `gateway` y `replay`.
- `Makefile` con: `run`, `replay`, `build`, `test`, `test-isolation`, `lint`.
- `api/openapi.yaml` con las rutas públicas `/portal/v1/...` usando los componentes comunes del repo de contratos.
- `CLAUDE.md` con el bloque común de reglas de plataforma adaptado al Portal Gateway, más: "sin reglas de negocio", "el token del usuario se reenvía tal cual", "nunca se agrega ni se lee un organizationId", "ninguna ruta fuera de la tabla de rutas", "sin reintentos en comandos", y el comando de cierre `go build ./... && go vet ./... && golangci-lint run && go test ./...`.
- `README.md` con el mismo nivel que el de Platform, incluido cómo levantar el Portal Gateway contra servidores falsos.
- `docs/ESTADO.md` como handoff al cerrar cada sesión.
- ADRs cortos: tabla de rutas y lista blanca de headers, clientes escritos a mano o generados, degradación, SSE o WebSocket, deduplicación de notificaciones, almacenamiento del read model.

## Forma de trabajo

1. **Plan primero:** informe de pantallas y APIs, rutas internas por lote propuestas para el repo de contratos, opciones del read model y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por incrementos verticales**, cada uno compilando y con sus tests en verde:
   1. esqueleto, config, health, observabilidad;
   2. verificación de JWT y propagación de headers contra Platform;
   3. tabla de rutas y paso directo (Platform, Billing, fiscal, Receivables);
   4. vista transversal de factura con degradación;
   5. listados enriquecidos sin N+1 (requiere las rutas por lote aprobadas en el repo de contratos);
   6. notificaciones en tiempo real con aislamiento y adapter de desarrollo;
   7. read model (solo si se aprobó su almacenamiento);
   8. endurecimiento: tests de aislamiento, autorrevisión, README y TODOs.
3. Al terminar cada incremento: `go build ./... && go vet ./... && golangci-lint run && go test ./...`, actualiza `docs/ESTADO.md`, resume lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: reglas de negocio infiltradas, un `organizationId` agregado o leído del cliente, tokens de servicio usados en lugar del del usuario, rutas fuera de la tabla, headers reenviados sin estar en la lista blanca, cachés sin organización y usuario en la clave, llamadas sin timeout, reintentos en comandos, notificaciones que puedan cruzar organizaciones, secretos y errores internos expuestos.

## Prohibido

- Implementar reglas de negocio, calcular montos o impuestos, o decidir un estado.
- Escribir en los schemas de las APIs, conectarse a la base con credenciales de dominio o usar la `service_role` key.
- Fabricar, agregar o leer del cliente un `organizationId`; usar un token de servicio para leer datos de un usuario.
- Un reverse proxy abierto que reenvíe cualquier ruta o cualquier header.
- Reintentar comandos o cachear datos de negocio sin organización y usuario en la clave.
- Crear schemas, roles o valores nuevos en los `CHECK` de la base, o inventar un broker o eventos que no estén en el catálogo: propón y deja un TODO.

## Definición de terminado

- [ ] Informe de pantallas y APIs aprobado; rutas internas por lote y `problems/portal-gateway.yaml` propuestos (y aceptados) en el repo de contratos.
- [ ] Propagación de identidad, correlación y trazas funcionando hacia las cuatro APIs.
- [ ] Paso directo de las operaciones que usa el portal, declarado en una sola tabla de rutas.
- [ ] Vista transversal de factura con degradación cuando una API secundaria falla (criterio 9).
- [ ] Listados paginados enriquecidos sin N+1.
- [ ] Notificaciones en tiempo real aisladas por organización, con adapter de desarrollo y transporte real pendiente de P2.
- [ ] Read model implementado o formalmente pospuesto según la decisión de almacenamiento.
- [ ] Tests de propagación, tenancy, degradación, N+1 y aislamiento de notificaciones en verde.
- [ ] `golangci-lint` limpio, imagen Docker construida y health checks respondiendo.
- [ ] Lista final de TODOs (transporte P2, read model, rutas por lote, decisiones) para revisar en equipo.
