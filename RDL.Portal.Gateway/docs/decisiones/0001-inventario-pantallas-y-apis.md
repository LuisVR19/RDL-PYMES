# 0001 · Inventario de pantallas y APIs

**Estado:** propuesta, pendiente de aprobación.
**Fecha:** 2026-09-24.
**Alcance:** Paso 1 del prompt P7. Es el mapa entre lo que el portal muestra y quién es dueño de cada dato.

Fuentes: `RDL.Web.Portal/src/app/screens.ts` (registro de las 36 pantallas, con ruta y permiso),
`RDL.Web.Portal/design/pantallas.md` y los OpenAPI de `RDL.Contracts`.

---

## 1. Qué existe hoy

| API | Repo | Estado |
|---|---|---|
| Platform (`:8080`) | `RDL.Platform.API` | Desplegable. P3 completo |
| Billing (`:8081`) | `RDL.Billing.API` | Desplegable. F2 y F3 |
| E-Invoice / fiscal (`:8082`) | — | **No existe.** Solo el esqueleto `openapi/fiscal.yaml` |
| Receivables (`:8083`) | — | **No existe.** Solo el esqueleto `openapi/receivables.yaml` |

De ahí sale la decisión central de este incremento: el Portal Gateway se construye **con las cuatro APIs
declaradas** y solo dos conectadas. Ver [ADR 0002](0002-tabla-de-rutas-y-headers.md).

## 2. Mapa de pantallas → APIs

Las 36 pantallas del portal, por módulo. «Transversal» son diálogos y overlays, no rutas.

| # | Pantalla | Módulo | Dato principal | APIs |
|---|---|---|---|---|
| 1 | Iniciar sesión · Recuperar contraseña | A | — | Supabase Auth (directo, no pasa por el gateway) |
| 2 | Selector de organización | A | Membresías | Platform |
| 3 | Crear organización | A | Organización | Platform |
| 4 | Aceptar invitación | A | Invitación | Platform |
| 5 | Cambio de organización | Transversal | Organización activa | Platform |
| 6 | Inicio | B | Resumen del negocio | **Composición** (Billing + fiscal + Receivables) |
| 7 | Clientes | C | Clientes | Billing |
| 8 | Nuevo · Editar cliente | C | Cliente | Billing |
| 9 | Ficha del cliente | C | Cliente + su cartera | **Composición** (Billing + Receivables) |
| 10 | Productos y servicios | C | Productos | Billing |
| 11 | Nuevo · Editar producto | C | Producto + CABYS | Billing + fiscal (catálogo) |
| 12 | Documentos | C | Facturas + estado fiscal + saldo | **Composición** (listado, incremento 5) |
| 13 | Nueva factura · Editar borrador | C | Borrador | Billing |
| 14 | Emitir | Transversal | Emisión | Billing |
| 15 | Detalle de factura | C | Total + Hacienda + saldo | **Composición** ← la del ejemplo de arquitectura 2.2 |
| 16 | Nota de crédito · débito | C | Nota | Billing (F5) |
| 17 | Anular factura | Transversal | Anulación | Billing (F5) |
| 18 | Configuración fiscal | D | Perfil y certificado | fiscal |
| 19 | Establecimientos y terminales | D | Establecimientos | fiscal |
| 20 | Bandeja | D | Documentos electrónicos | fiscal |
| 21 | Documento electrónico | D | Documento + XML | fiscal |
| 22 | Cuentas por cobrar | E | Cuentas | Receivables |
| 23 | Aging | E | Aging | Receivables |
| 24 | Detalle de cuenta | E | Cuenta + gestiones | Receivables |
| 25 | Pagos | E | Pagos | Receivables |
| 26 | Registrar pago | E | Pago + aplicación | Receivables |
| 27 | Detalle de pago | E | Pago | Receivables |
| 28 | Organización | F | Organización + numeración | Platform + Billing |
| 29 | Sucursales | F | Sucursales | Platform |
| 30 | Usuarios y roles | F | Miembros | Platform |
| 31 | Invitaciones | F | Invitaciones | Platform |
| 32 | Exportar auditoría | F | Audit | **Sin dueño definido** — ver §5 |
| 33 | Mi perfil | F | Usuario | Platform |
| 34 | Notificaciones | Transversal | Eventos | **Tiempo real** (incremento 6) |
| 35 | Pantallas de sistema | Transversal | — | — |
| 36 | Atajos de teclado | Transversal | — | — |

**Cobertura con lo que existe hoy:** módulos A (5), C (13) y F (6) = **24 pantallas** con datos reales, más
Inicio (6) y Detalle de factura (15) parcialmente. Los módulos D (4) y E (6) quedan declarados pero sin backend.

## 3. Clasificación de las operaciones

- **Paso directo (54 rutas):** una pantalla, una API. Es todo lo de la tabla de `internal/domain/routes`.
- **Composición (1 implementada, 3 pendientes):**
  - `GET /portal/v1/invoices/{id}/overview` — **implementada** en este incremento.
  - `GET /portal/v1/invoices` enriquecido (pantalla 12) — incremento 5, bloqueada por §4.
  - `GET /portal/v1/customers/{id}/overview` (pantalla 9, cliente + cartera) — incremento 5.
  - `GET /portal/v1/home` (pantalla 6) — incremento 5; hay que decidir qué resume.
- **Tiempo real (1):** `GET /portal/v1/notifications` — incremento 6, bloqueado por el transporte de P2.

## 4. N+1: rutas por lote que hacen falta en el repo de contratos

`openapi/bff-internal.yaml` solo tiene rutas **de a una factura**. Una página de 50 facturas haría 100 llamadas
internas. Propuesta para el repo de contratos (PR aparte, con sus 2 aprobaciones y su entrada en el CHANGELOG):

```yaml
POST /internal/v1/electronic-documents/statuses:by-source   # fiscal
POST /internal/v1/receivables/balances:by-invoice           # receivables
  body: { ids: [uuid], }   # máximo 100 por llamada
  200:  { items: [ {sourceDocumentId|invoiceId, ...} ] }     # los ids que no existen simplemente no vienen
```

`POST` y no `GET` porque la lista de ids no cabe cómodamente en una query. Son lecturas: sin `Idempotency-Key`
y sin efectos.

**Además, para la vista transversal:** Billing tiene declarada `GET /internal/v1/invoices/{id}/summary` en
`bff-internal.yaml`, pero **no la implementa**. Mientras tanto el gateway deriva el resumen del detalle público
(`BILLING_SUMMARY_SOURCE=public`). Ver [ADR 0004](0004-degradacion-y-disponibilidad.md).

## 5. Decisiones abiertas

| # | Asunto | Propuesta | Quién decide |
|---|---|---|---|
| D1 | **Pantalla 32 · Exportar auditoría.** `audit.audit_events` es transversal y ninguna API lo expone hoy | Que Platform exponga `GET /v1/audit-events` (es la dueña de la identidad) | Equipo + `database-platform` |
| D2 | **Read model del BFF.** El planning dice que el BFF «no escribe en la base», pero un read model necesita dónde vivir. No hay schema ni rol, y el `CHECK` de `integration` no incluye `portal-gateway` | **Posponer.** Con las rutas por lote de §4, los listados no necesitan read model en V1 | Luis Valverde |
| D3 | **Transporte de eventos (P2).** Sin broker decidido no hay notificaciones reales | Entregar el caso de uso tras un puerto `EventSource` + adapter de desarrollo, con `TODO(P2)` | P2 |
| D4 | **SSE o WebSocket** para `/portal/v1/notifications` | SSE: solo el servidor empuja | Equipo |
| D5 | **Deduplicación de notificaciones.** `integration.inbox_messages` no admite el servicio `portal-gateway` y el gateway no tiene rol de base | En memoria por `eventId`: son avisos de pantalla, no efectos de negocio | Equipo |
| D6 | **Pantalla 6 · Inicio.** No está definido qué resume ni de dónde sale cada número | Definirlo con el diseño antes del incremento 5 | Equipo |

## 6. Lo que este incremento NO hace

Listados enriquecidos (necesitan §4), notificaciones en tiempo real (D3) y read model (D2).
Estado al día: [`docs/ESTADO.md`](../ESTADO.md).
