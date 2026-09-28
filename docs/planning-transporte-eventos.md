# Planning · Transporte de eventos (P2)

**Fecha:** 2026-09-28 · **Estado:** propuesta para decidir · **Destraba:** bloqueo 6 de `docs/CHECKPOINTS.md`

Este documento explica qué falta para que los eventos viajen entre servicios, qué opciones hay, cuál se
recomienda y en qué orden se construye. Todo lo que dice «hoy» está verificado en los repos al 2026-09-28.

---

## 1. El problema en una frase

Los servicios **ya escriben** sus eventos en el outbox y Receivables **ya sabe** procesarlos, pero **nada los
lleva** de un lado al otro. Hoy una factura emitida en Billing no crea su cuenta por cobrar sola: hay que
reenviarla a mano con `cmd/replay`.

Consecuencias visibles hoy:

- Emitir, acreditar, debitar o anular una factura **no mueve el saldo** en Cobranza.
- Las pantallas 22–27 del portal solo muestran lo que entró por `cmd/replay` o por las suites.
- No hay notificaciones en tiempo real (incremento 6 del gateway) ni read model (incremento 7).
- E-Invoice (P5), cuando exista, no tendrá de dónde recibir `InvoiceIssued`.

## 2. Qué ya está hecho (y no se toca)

| Pieza | Dónde | Estado |
|---|---|---|
| Sobre común y 8 eventos con JSON Schema | `RDL.Contracts` (`asyncapi/`, `schemas/events/`, `pkg/events`) | ✅ v0.2.0 |
| Canales con nombre (`billing.invoice-issued.v1`, …) | `asyncapi/asyncapi.yaml` (sin `servers`: la infraestructura es P2) | ✅ |
| Outbox en la misma transacción, validado contra el schema | Billing (4 eventos), Receivables (2 eventos) | ✅ |
| Tabla `integration.outbox_messages` con `published_at`, `publish_attempts`, `next_attempt_at`, `last_error` | database-platform | ✅ pensada para un publicador por sondeo |
| RLS del outbox por `source_service = shared.current_service()` (no por organización) | database-platform | ✅ cada servicio solo ve y publica **lo suyo** |
| Consumidor con inbox (`ON CONFLICT DO NOTHING`), reintentos 6× con backoff, dead letter, tenant del evento | Receivables `cmd/consumer` (ADR 0005) | ✅ probado con `cmd/replay` |
| Puerto de la fuente: `events.Source.Run(ctx, handle)`, ack solo si `handle` devuelve `nil` | Receivables `internal/adapters/events` | ✅ hoy `NoSource` |
| Reprocesar es inocuo (invariante 4) | tests + `cmd/replay` dos veces sobre dev | ✅ |

**Lo que falta son exactamente tres piezas:** el **publicador** (lee el outbox y entrega), el **medio** (dónde
esperan los mensajes) y el **adaptador de la fuente** en cada consumidor.

```
 Billing API ──tx──► billing.invoices + integration.outbox_messages
                                   │
                          ┌────────┴────────┐
                          │  PUBLICADOR (1) │  lee lo suyo (RLS), entrega, marca published_at
                          └────────┬────────┘
                                   ▼
                          ┌─────────────────┐
                          │    MEDIO (2)    │  una cola por consumidor
                          └──┬───────────┬──┘
                             ▼           ▼
              ┌────────────────────┐   ┌────────────────────┐
              │ Receivables        │   │ E-Invoice (P5)     │
              │ Source (3) → inbox │   │ Source (3) → inbox │
              └────────────────────┘   └────────────────────┘
```

## 3. Requisitos que la solución tiene que cumplir

Salen del contrato (convenciones §10), la arquitectura (§6.4 y criterios §12) y las reglas de cada repo.

1. **Al menos una vez.** Ningún evento escrito en el outbox se pierde. Duplicados sí puede haber: el inbox los
   absorbe.
2. **Sin orden garantizado.** Receivables ya lo tolera (una nota que llega antes que su factura se reintenta).
3. **El productor no depende del consumidor.** Billing responde 201 aunque Receivables, E-Invoice o el propio
   transporte estén caídos (criterio 2 de arquitectura).
4. **Fan-out.** `InvoiceIssued` va a fiscal **y** a Receivables. Cada consumidor recibe su copia y la confirma por
   su cuenta.
5. **Tenancy.** El tenant de un evento sale **solo** de su `organizationId`. El transporte no filtra ni reescribe
   organizaciones, y ningún servicio lee el outbox de otro (RLS actual).
6. **Compatibilidad con Supavisor en modo transacción.** Sin `LISTEN/NOTIFY` ni estado de sesión. El sondeo tiene
   que funcionar solo con SQL por transacción.
7. **Trazabilidad.** El `correlationId` del evento llega al log, la traza OTel y el audit del consumidor (ya lo
   hace Receivables).
8. **Operable.** Métricas de lag, pendientes, reintentos y dead letters, y una forma de reprocesar.
9. **Reemplazable.** El publicador y la fuente van detrás de puertos. Cambiar de medio en V2 cambia adaptadores,
   no dominios.

## 4. Opciones para el medio

| | A · Colas en PostgreSQL (`pgmq` / Supabase Queues) | B · RabbitMQ gestionado | C · Cola de la nube (SQS+SNS, Pub/Sub, Service Bus) | D · NATS JetStream |
|---|---|---|---|---|
| Infraestructura nueva | **Ninguna**: una extensión en la misma base | Un broker (CloudAMQP o propio) | Una cuenta y un servicio de la nube | Un clúster NATS |
| Depende de decisiones abiertas | No | Hosting (P2) | **Nube sin decidir** | Hosting (P2) |
| Con Supavisor (transacción) | ✅ es SQL puro | ✅ fuera de la base | ✅ fuera de la base | ✅ fuera de la base |
| Fan-out | El publicador escribe en una cola por consumidor | Exchange `topic` + una cola por consumidor | Tema + suscripción por consumidor | Subject + consumer durable |
| Dead letter | La propia (`integration.dead_letters`, ya existe) | DLX nativo | DLQ nativa | Max deliveries + stream aparte |
| Secretos nuevos | Ninguno (mismos roles de base) | Usuario y URL del broker | Credenciales de la nube | Credenciales NATS |
| Costo en V1 | ≈ 0 (carga en la base de dev/prod) | Plan pago del broker | Por mensaje (bajo) | Servidor propio |
| Límite práctico | Miles de mensajes por minuto: sobra para una PYME por organización | Alto | Alto | Alto |
| Riesgo | Carga en la base compartida; versión de `pgmq` que ofrezca Supabase | Otra pieza que operar y monitorear | Atarse a una nube antes de elegirla | Equipo sin experiencia, más ops |

**Por qué no alcanza el outbox solo, sin medio.** Un consumidor no puede leer el outbox de otro servicio: la RLS lo
impide a propósito (P6, paso 4). Aflojarla rompería el aislamiento entre servicios. Siempre hace falta que el dueño
del outbox *publique*.

**Por qué no Supabase Realtime para esto.** Realtime no es una cola: si el consumidor está caído, el mensaje se
pierde. Sí sirve para **notificar al navegador** (§8), que es otro problema.

### Recomendación: **A · colas en PostgreSQL con `pgmq`**, detrás de puertos

- Destraba todo **sin decidir la nube ni contratar nada**, que es justo lo que tiene parado a P2.
- Es transaccional con la base que ya usan los servicios. El publicador puede marcar `published_at` y encolar en la
  **misma transacción**, así que entre el outbox y la cola no hay «publicado pero perdido».
- El volumen de V1 (facturas de PYMES) queda muy lejos de los límites de una cola en PostgreSQL.
- La salida a B o C queda abierta: el publicador y `Source` son interfaces. Migrar es escribir dos adaptadores y
  cambiar configuración, no tocar Billing ni Receivables.

**Condición para confirmarla:** que el spike (fase F1) compruebe en dev que `pgmq` está disponible en el proyecto
Supabase y funciona con los roles de login y el pooler en modo transacción. Si falla, la alternativa es la misma
arquitectura con una tabla de entregas propia en `integration` (más SQL, mismo diseño) o, si ya se decidió la
nube, la opción C.

## 5. Diseño propuesto (con la opción A)

### 5.1 Colas

Una cola por **consumidor** (no por evento): `q_receivables`, `q_fiscal` y, cuando exista, `q_portal_gateway`.
Cada consumidor lee solo la suya y el tipo de evento viaja en el mensaje. Es más simple de dar permisos y de
monitorear, y el consumidor ya despacha por `eventType`.

### 5.2 Enrutamiento: quién recibe qué

Sale del AsyncAPI (fuente única de verdad). Hoy el catálogo nombra consumidores por evento. Se propone agregar las
operaciones `receive` por servicio y que `contractsctl` genere la tabla de rutas que usa el publicador:

| Evento | Productor | Colas destino |
|---|---|---|
| `InvoiceIssued` v1 | billing | `q_receivables`, `q_fiscal` |
| `InvoiceCancelled` v1 | billing | `q_receivables`, `q_fiscal` |
| `CreditNoteIssued` v1 | billing | `q_receivables`, `q_fiscal` |
| `DebitNoteIssued` v1 | billing | `q_receivables`, `q_fiscal` |
| `ElectronicDocumentAccepted` v1 | fiscal | `q_portal_gateway` (notificaciones) |
| `ElectronicDocumentRejected` v1 | fiscal | `q_portal_gateway` |
| `PaymentReceived` v1 | receivables | `q_portal_gateway` |
| `ReceivableSettled` v1 | receivables | `q_portal_gateway` |

Un evento sin consumidores todavía (los de Receivables mientras no exista el consumidor del gateway) se marca
publicado igual: no se retiene el outbox esperando a alguien que no existe.

`q_fiscal` se crea **desde el día 1** aunque E-Invoice no exista. Así las facturas emitidas antes de P5 quedan
esperando y E-Invoice las procesa al arrancar (con retención suficiente, §9).

### 5.3 Publicador (uno por servicio productor)

Un proceso `cmd/relay` en Billing y otro en Receivables (y en E-Invoice cuando exista), con el **rol de login del
propio servicio**. Por la RLS actual solo ve su outbox. Un ciclo:

1. Abre una transacción y toma un lote:
   `SELECT … FROM integration.outbox_messages WHERE published_at IS NULL AND (next_attempt_at IS NULL OR next_attempt_at <= now()) ORDER BY occurred_at LIMIT 100 FOR UPDATE SKIP LOCKED`.
2. Por cada mensaje, `pgmq.send` a cada cola destino de la tabla de rutas.
3. Marca `published_at = now()` en la **misma transacción** y hace commit.
4. Si falla, rollback. Otra transacción suma `publish_attempts`, deja `last_error` y fija `next_attempt_at` con
   backoff.
5. Duerme el intervalo de sondeo (1 s con lote vacío, 0 s si el lote vino lleno).

Con `SKIP LOCKED` se pueden correr dos réplicas sin publicar doble. Si igual pasara, el inbox lo absorbe.

### 5.4 Fuente en el consumidor

Implementación de `events.Source` sobre `pgmq.read(cola, vt, n)`:

- El *visibility timeout* es **mayor que el tiempo máximo de un mensaje** (`CONSUMER_MESSAGE_TIMEOUT`, 2 min, que ya
  incluye los 6 reintentos internos). Se propone `vt = 5 min`.
- `handle` devuelve `nil` → `pgmq.delete`. Devuelve error → no se borra y reaparece al vencer el `vt`.
- Tope de relecturas (`read_ct`): pasado el tope, la fuente lo manda a `integration.dead_letters` (tabla que ya
  existe) y lo borra de la cola. Es la red de seguridad para un mensaje que tumba el proceso antes de responder.
- La transacción de lectura de la cola es **aparte** de la del efecto: el efecto sigue en `WithinServiceTx` con su
  inbox, exactamente como hoy.

### 5.5 Dónde vive el código común

El publicador y la fuente `pgmq` son iguales en todos los servicios. El planning original (P2) los pone en el
paquete **building-blocks**, que no existe. Hoy `pkg/tenancy`, `pkg/correlation` y compañía están copiados en tres
repos y ya divergieron (CHECKPOINTS §6.9).

**Propuesta:** crear `RDL.BuildingBlocks` (Go, mismo esquema de `replace` que contratos) con `outbox/relay` y
`eventsource/pgmq` para empezar, y mover después los paquetes duplicados. Si el equipo prefiere no abrir un repo
ahora, la alternativa es copiarlos en Billing y Receivables con una nota «Origen:», como ya se hizo con otras
piezas.

### 5.6 Permisos (propuesta a database-platform)

Nada de esto se crea desde un servicio (ownership: `integration` y las extensiones son de database-platform):

- Habilitar `pgmq` en dev (y después en los demás ambientes).
- Crear `q_receivables`, `q_fiscal` y `q_portal_gateway`.
- Dar a `billing_api` `send` hacia `q_receivables` y `q_fiscal`. Dar a `receivables_api` `send` hacia
  `q_portal_gateway` y `read`/`delete` sobre `q_receivables`. Dar al login de E-Invoice (miembro de `fiscal_app`), cuando exista, `send` hacia
  `q_portal_gateway` y `read`/`delete` sobre `q_fiscal`.
- Nadie lee una cola ajena. Se verifica con un test de aislamiento nuevo por servicio.

## 6. Plan por fases

Estimaciones en días de trabajo con Claude Code, contando pruebas y documentación. ⛔ = necesita a alguien fuera
del agente.

| Fase | Qué | Entregable verificable | Días | Depende de |
|---|---|---|---|---|
| **F0** | Decisión | ADR 0008 en `RDL.Contracts/docs/decisiones` con la opción elegida; `servers` y operaciones `receive` en el AsyncAPI (propuesta, **⛔ 2 aprobaciones**) | 1 | Este documento |
| **F1** | Spike en dev | Script que prueba `pgmq` con los roles de login reales y el pooler en modo transacción; mide latencia y 1 000 mensajes; informe | 1 | ⛔ database-platform habilita `pgmq` en dev |
| **F2** | Base | Propuesta SQL a database-platform (colas, grants, test de que nadie lee colas ajenas) y su aplicación en dev | 1 | F1 · ⛔ aplicar en dev |
| **F3** | Código común | `RDL.BuildingBlocks` (o copia) con `outbox/relay` y `eventsource/pgmq`, más tests de integración contra dev (lote, SKIP LOCKED, backoff, vt, tope de relecturas) | 3 | F2 |
| **F4** | Billing publica | `cmd/relay` en Billing, métricas OTel (pendientes, lag, fallas), `/healthz`/`/readyz`, Dockerfile; tabla de rutas generada por `contractsctl` | 2 | F3 |
| **F5** | Receivables consume | Fuente `pgmq` en `cmd/consumer` en lugar de `NoSource`; **E2E real**: emitir en Billing → la cuenta aparece sola en Receivables y en el portal (pantalla 22), con el mismo `correlationId` en los tres logs | 2 | F4 |
| **F6** | Receivables publica | `cmd/relay` en Receivables para `PaymentReceived` y `ReceivableSettled` (sin consumidores todavía: se marcan publicados) | 1 | F3 |
| **F7** | Operación | Retención del outbox y de las colas (archivo o purga), alertas sobre dead letters y lag, runbook de reproceso con `cmd/replay`, pruebas de caos (matar relay y consumer en medio de un lote) | 2 | F5 |
| **F8** | Notificaciones | Incremento 6 del gateway según la decisión de §8 | 3–4 | F6 · ⛔ decisión §8 |

**Total hasta que Billing y Receivables hablen solos (F0–F5): unos 10 días de trabajo** más los tiempos de
aprobación y de database-platform. F6 y F7 suman 3 días. F8 depende de su decisión.

E-Invoice (P5) se engancha después sin trabajo de transporte: su cola ya existe y solo implementa la fuente.

## 7. Criterios de aceptación

1. Emitir una factura con Receivables **apagado** responde 201. Al encender Receivables, la cuenta aparece sin
   intervención.
2. Apagar el publicador de Billing no pierde nada: al volver, publica lo pendiente en orden de `occurred_at`.
3. Matar el consumidor en medio de un mensaje: el mensaje reaparece al vencer el `vt` y el efecto ocurre **una sola
   vez** (inbox).
4. Dos réplicas del publicador en paralelo no generan efectos duplicados (en el peor caso duplican el mensaje y el
   inbox lo absorbe).
5. Un evento inválido termina en `integration.dead_letters` sin bloquear a los que vienen detrás.
6. Latencia del outbox a la cuenta creada: p95 menor a 5 s en dev.
7. Ningún rol lee una cola que no es suya (test de aislamiento nuevo). El publicador nunca fija
   `app.current_organization_id`.
8. El `correlationId` de la petición de emisión aparece en los logs del API de Billing, del publicador, del
   consumidor y en el audit de Receivables.

## 8. Decisión aparte: notificaciones en tiempo real (incremento 6)

El gateway **no tiene base de datos** por regla. Para empujar avisos al navegador («Hacienda aceptó la factura»,
«Pago registrado») hay tres caminos:

| | 8.1 · El gateway consume `q_portal_gateway` y emite SSE | 8.2 · Supabase Realtime Broadcast desde la base | 8.3 · Sin tiempo real en V1 |
|---|---|---|---|
| Cómo | El gateway recibe un rol de login mínimo (solo `read`/`delete` de su cola) y reparte por SSE a las sesiones de cada organización | Un trigger o el consumidor publica en un canal por organización. El portal se suscribe con su JWT y la RLS de `realtime.messages` filtra por `org_id` | El portal refresca al volver a la pantalla (TanStack Query ya lo hace) y la campana queda vacía |
| Rompe una regla | Sí: «gateway sin base de datos». Necesita una excepción aceptada en un ADR | Sí: «el portal consume solo el gateway». Además, un origen nuevo en la CSP | No |
| Riesgo de cruzar organizaciones | Bajo: el filtro está en código y se prueba en `tests/isolation` | Bajo si la RLS de Realtime está bien. Hay que probarla aparte | Ninguno |
| Esfuerzo | 3–4 días | 2–3 días | 0 |

**Recomendación:** empezar con 8.3 mientras E-Invoice no exista (el aviso más valioso, la respuesta de Hacienda,
no tiene todavía quién lo produzca) y decidir entre 8.1 y 8.2 cuando arranque P5. Si se quiere ya el aviso de «pago
registrado», 8.1 es la que respeta mejor la arquitectura (todo pasa por el BFF).

## 9. Riesgos y cómo se cubren

| Riesgo | Mitigación |
|---|---|
| `pgmq` no disponible o con otra versión en Supabase | Es lo primero que se prueba (F1). Plan B: tabla de entregas propia en `integration`, mismo diseño |
| Carga extra en la base compartida | Sondeo con lote y espera (sin consultas en bucle apretado). Métricas de F4. Umbral para pasar a la opción B o C |
| El outbox y las colas crecen sin fin | Retención de F7: purga de lo publicado y confirmado pasado N días (N a definir con legal; el audit ya guarda la traza) |
| Mensaje venenoso que tumba el consumidor | Tope de `read_ct` → dead letter (§5.4) |
| `q_fiscal` se llena durante meses antes de P5 | Sin vencimiento en esa cola hasta que E-Invoice arranque. Monitorear tamaño |
| Orden: nota antes que su factura | Ya cubierto: reintento transitorio en Receivables (ADR 0005) |
| Cambio incompatible de un evento | Ya cubierto: `vN+1` convive con `vN` (convenciones §11). Las rutas se generan por versión |

## 10. Decisiones que se necesitan

| # | Pregunta | Recomendación | Quién |
|---|---|---|---|
| D1 | ¿Qué medio en V1? | A · `pgmq` detrás de puertos | Luis / equipo (ADR 0008) |
| D2 | ¿Dónde vive el código común? | Repo nuevo `RDL.BuildingBlocks` | Luis |
| D3 | ¿Qué camino para las notificaciones? | 8.3 ahora; 8.1 al arrancar P5 | Equipo |
| D4 | ¿Retención del outbox y de las colas? | 30 días después de publicado y confirmado | Legal y equipo |
| D5 | ¿Quién aplica lo de database-platform en dev? | El mismo flujo de las migraciones 0010/0011 | Luis |

## 11. Qué cambia en los documentos al avanzar

- `RDL.Contracts`: ADR 0008, `servers` y operaciones en `asyncapi.yaml`, entrada en `CHANGELOG.md`.
- `docs/CHECKPOINTS.md`: el bloqueo 6 pasa a «en curso» con la fase actual.
- `RDL.Receivables.API/docs/ESTADO.md` y `RDL.Billing.API/docs/ESTADO.md`: `TODO(P2)` resueltos por fase.
- `RDL.Portal.Gateway/docs/ESTADO.md`: incremento 6 destrabado según D3.
