# Convenciones comunes

Ley común de todas las APIs (Platform, Billing, fiscal, Receivables) y del BFF. Lo que aquí se fija lo verifican
`contractsctl validate` y `contractsctl lint`. Las decisiones y sus fuentes están en
[`decisiones/0001-inventario-y-brechas.md`](decisiones/0001-inventario-y-brechas.md). Cuando la base ya impone algo
(dominios de `shared`, `CHECK`), manda la base.

## 1. Identificadores

- **UUID v4**, en minúsculas y con guiones (`3f2b8c1e-...`). En JSON Schema: `type: string, format: uuid`.
- Los genera el servicio dueño (`gen_random_uuid()` o `uuid.New()`). Excepción aceptada: Platform deriva el id de
  una organización nueva con UUID v5 a partir de la `Idempotency-Key`, para que el reintento cree la misma.
- Un id nunca codifica información de negocio. Los números visibles (factura `FAC-0001`) y los fiscales
  (consecutivo, clave numérica) son campos aparte.

## 2. Fechas y horas

| Caso | Formato | Ejemplo |
|---|---|---|
| Instante | RFC 3339 **en UTC con `Z`** (`format: date-time`) | `2026-09-24T15:04:05Z` |
| Fecha de negocio sin hora (vencimiento, fecha de pago) | `YYYY-MM-DD` (`format: date`) | `2026-10-24` |

- En la base: `timestamptz` para instantes y `date` para fechas de negocio.
- Una fecha de negocio se interpreta en la **zona horaria de la organización** (`core.organizations.timezone`, por
  defecto `America/Costa_Rica`). Ejemplo: una factura emitida el 24 a las 23:30 de Costa Rica tiene fecha de negocio
  24, aunque en UTC ya sea el 25.
- Se permiten fracciones de segundo. Se rechaza un instante con desfase distinto de `Z` (`-06:00`): la presentación en
  hora local es trabajo del cliente.

## 3. Dinero y números decimales

**Nunca `number` en JSON ni `float` en Go.** Todo valor decimal viaja como **string** con el formato de la tabla,
igual que en la base (`shared.*`).

| Tipo | Base | Patrón JSON | Ejemplos válidos |
|---|---|---|---|
| `Money` | `numeric(18,5)`, `>= 0` | `^(0\|[1-9][0-9]{0,12})(\.[0-9]{1,5})?$` | `"0"`, `"11300"`, `"1300.50"`, `"0.00001"` |
| `ExchangeRate` | `numeric(18,5)`, `> 0` | igual que `Money`, distinto de cero | `"1"`, `"512.34"` |
| `Quantity` | `numeric(16,3)`, `> 0` | `^(0\|[1-9][0-9]{0,12})(\.[0-9]{1,3})?$`, distinto de cero | `"1"`, `"2.5"`, `"0.125"` |
| `Percentage` | `numeric(7,4)`, `0..100` | `^(100(\.0{1,4})?\|[1-9]?[0-9](\.[0-9]{1,4})?)$` | `"13"`, `"0.5"`, `"100"` |
| `CurrencyCode` | `char(3)` | `^[A-Z]{3}$` (ISO 4217) | `"CRC"`, `"USD"` |

- Sin signo: los montos **nunca son negativos** (`money_amount >= 0`). Un descuento, una nota de crédito o un ajuste
  se expresan con su tipo, no con el signo.
- Sin separador de miles, sin exponente y con `.` como separador decimal. Se aceptan ceros a la derecha
  (`"1300.50"`), no a la izquierda (`"01300"`).
- La moneda va siempre en un campo aparte, junto al monto o en el encabezado del documento.
- **Redondeo: pendiente (decisión D2).** Billing calcula y fiscal valida. La escala máxima es 5 decimales, como la
  base, pero el modo y el paso de redondeo dependen de la especificación de Hacienda. `TODO(fiscal)`.

## 4. Nombres

| Dónde | Estilo | Ejemplo |
|---|---|---|
| Campos JSON | `camelCase` | `organizationId`, `totalAmount` |
| Columnas de la base | `snake_case` | `organization_id` |
| Valores de enumeraciones (estados, tipos) | `snake_case` en minúsculas, igual que los `CHECK` | `partially_paid`, `credit_note` |
| Eventos | `PascalCase`, entidad + verbo en pasado | `InvoiceIssued` |
| Archivos de schema | `kebab-case.vN.json` | `invoice-issued.v1.json` |
| Servicios (campos de máquina) | `platform`, `billing`, `fiscal`, `receivables` | `sourceService: "billing"` |

"E-Invoice API" es el nombre del servicio en la documentación. En todo campo de máquina se llama `fiscal` (D4).

## 5. Tenancy

- La organización activa sale **solo** del claim `org_id` del JWT verificado (Supabase Auth, hook de Platform) más
  la membresía revalidada en `core`. Nunca de un body, una query, un header ni la ruta.
- Rutas: Platform usa `/v1/organizations/current/...` porque su recurso es la organización. Las demás APIs usan
  `/v1/<recurso>` (`/v1/invoices`) y operan sobre la organización del token (D7).
- Un recurso de otra organización responde **404** (no se revela que existe). Un rol insuficiente dentro de la propia
  responde **403**. Un estado que no permite la operación responde **409**.
- Todo evento lleva `organizationId`, y el consumidor lo usa como tenant de su transacción.

## 6. Errores (Problem Details)

- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), `Content-Type: application/problem+json`.
- `type` estable: **`urn:rdl:<servicio>:problem:<código-kebab>`** (D5). Ejemplo:
  `urn:rdl:billing:problem:invoice-not-draft`.
- Cada servicio registra sus tipos en `problems/<servicio>.yaml` (código, status HTTP, título, cuándo ocurre).
  Un `type` que no está registrado no se usa.
- Campos: `type`, `title`, `status`, `detail` (opcional, sin datos internos), `instance` (ruta del request) y
  `correlationId`. Los errores de validación agregan `errors: [{field, message}]` con el nombre del campo JSON.
- Un 500 nunca expone el detalle interno (SQL, stack, nombres de constraint). El detalle va al log con el
  `correlationId`.

## 7. Idempotencia

- Todo comando `POST` que crea algo o cambia un estado exige el header **`Idempotency-Key`** (1 a 255 caracteres ASCII
  visibles; se recomienda un UUID). Sin la clave responde 400 `idempotency-key-required`.
- Misma clave y mismo cuerpo, dentro de la vigencia: **la misma respuesta** (mismo status y cuerpo). Misma clave con
  otro cuerpo: **422** `idempotency-key-reused`.
- La clave vale por **organización y servicio** (`integration.idempotency_keys`) durante **24 horas**.
- `PATCH`, `PUT` y `DELETE` son idempotentes por diseño y no exigen la clave.
- Los consumidores de eventos son idempotentes por `eventId` (`integration.inbox_messages`): reprocesar un evento no
  crea nada duplicado (criterio de aceptación 3).

## 8. Correlación y trazas

- Header **`X-Correlation-Id`** (UUID). Si el cliente envía un UUID, se respeta; si envía otro valor o nada, el
  servicio genera uno. Siempre se devuelve en la respuesta.
- Se propaga a los logs, las trazas (OpenTelemetry), `audit.audit_events.correlation_id` y el `correlationId` de todo
  evento que el request produzca. Un consumidor usa el `correlationId` del evento en todo lo que haga a partir de él.

## 9. Paginación

- Parámetros: `limit` (1–100, por defecto 20) y `cursor` (opaco; el cliente no lo interpreta).
- Respuesta: `{ "items": [...], "nextCursor": "..." | null }`. `nextCursor: null` significa que no hay más.
- Orden estable definido por cada endpoint (por ejemplo, `createdAt` descendente y luego `id`).
- Un `limit` fuera de rango responde 422.

## 10. Eventos

- Sobre común obligatorio (schema `schemas/events/envelope.v1.json`): `eventId`, `eventType`, `version`,
  `occurredAt`, `correlationId`, `organizationId` y `sourceService` (D3). El cuerpo de cada evento va en el mismo
  objeto, junto a esos campos.
- Mapeo al outbox: `eventId` → `id`, `eventType` → `event_type`, `version` → `event_version`,
  `occurredAt` → `occurred_at`, `correlationId` → `correlation_id`, `organizationId` → `organization_id`,
  `sourceService` → `source_service`; el objeto completo → `payload`.
- La entidad y su mensaje de outbox se escriben en la **misma transacción**. El worker publica; la API no espera.
- Entrega **al menos una vez**: el consumidor deduplica por `eventId`.
- Un evento lleva **snapshots** de lo que necesita el consumidor (cliente, líneas, montos). El consumidor no vuelve a
  consultar al productor para completar datos.
- `additionalProperties: false` en todos los schemas: un campo desconocido es un error, no se ignora.

## 11. Versionado y compatibilidad

- **Repo:** SemVer (`vX.Y.Z`). Mientras sea `v0`, se permiten cambios incompatibles con versión menor, avisando en
  el `CHANGELOG.md`.
- **HTTP:** versión mayor en la ruta (`/v1`). Dentro de `v1` solo hay cambios compatibles.
- **Eventos:** versión entera por evento (`InvoiceIssued` v1, v2...). Un schema publicado es inmutable salvo cambios
  compatibles. Un cambio incompatible crea `vN+1`, que convive con `vN` hasta que ningún consumidor use la anterior.

| Cambio | ¿Compatible? |
|---|---|
| Agregar un campo opcional | Sí |
| Agregar un valor a un enum | **No** para eventos (el consumidor puede no conocerlo); sí en una respuesta HTTP documentada como extensible |
| Agregar un campo obligatorio | No |
| Quitar o renombrar un campo | No |
| Cambiar el tipo, el formato o endurecer un patrón | No |
| Quitar un valor de un enum | No |

`contractsctl breaking` aplica esta tabla contra la última versión publicada.

## 12. Roles

`owner`, `admin`, `biller`, `collector`, `accountant`, `read_only` (catálogo `core.roles`). V1: un rol por
membresía. Cada API tiene su matriz de permisos en un solo lugar de su dominio; el rol efectivo sale de la membresía
revalidada, no del claim `org_roles` (que puede estar desactualizado).

## TODOs y preguntas abiertas

- `TODO(fiscal)` D2: modo y paso de redondeo, según la especificación de Hacienda.
- `TODO(fiscal)` D9: catálogo de tipos de identificación y formato del número por tipo.
- ¿Enums extensibles en respuestas HTTP? Se propone documentarlo por campo en cada OpenAPI.
