# Prompt · P0 Contracts (contratos y acuerdos) en Go

> **Cómo usarlo**
> 1. Crea el repo `RDL.Contracts` vacío (junto a `RDL.Platform.API`) y copia en `docs/contexto/` el documento de arquitectura y el planning (los mismos `.md` de `RDL.Platform.API/docs/contexto/`).
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano y en equipo **todo** lo que sea glosario, máquinas de estado, montos y campos fiscales: este repo es un acuerdo entre personas, no solo código.

---

## Rol y objetivo

Eres un ingeniero de software senior en Go, con experiencia en diseño de APIs y sistemas orientados a eventos. Vas a construir el repositorio de **contratos** de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. Es la **fuente única de verdad** de lo que las APIs (Platform, Billing, E-Invoice, Receivables) y el BFF se prometen entre sí: glosario, máquinas de estado, convenciones, eventos y OpenAPI. Si este repo es ambiguo o se rompe sin avisar, las APIs se desalinean en silencio y el error aparece en producción, en un documento fiscal. Por eso la precisión y la compatibilidad pesan más que la cantidad.

El repo tiene dos partes:
1. **Artefactos de contrato** (Markdown, AsyncAPI 3, JSON Schema 2020-12, OpenAPI 3.1): lo que se acuerda.
2. **Herramientas en Go** que los hacen verificables: un CLI que valida todo, detecta cambios incompatibles contra la versión publicada y un paquete Go de DTOs de eventos que las APIs importan.

**Entregable:** el repositorio `RDL.Contracts` con el alcance de la Fase 0 del planning (hito **H1 Contratos v1**). Debe validar en CI, tener el CLI `contractsctl` con tests, el paquete `pkg/events` versionado y la documentación lista para aprobarla los tres desarrolladores.

## Contexto que debes leer primero

- `docs/contexto/arquitectura-v1.md`: secciones **2.1** (fronteras de escritura), **3** (responsabilidad de cada API), **4.4** (snapshots), **5** (multiempresa), **6.1 a 6.4** (flujo, catálogo de eventos, `InvoiceIssued`, confiabilidad), **7.3** (auditoría) y **11** (decisiones pendientes).
- `docs/contexto/planning-v1.md`: sección **P0 Contratos y acuerdos**, "Decisiones que bloquean" y "Cómo trabajamos con Claude Code".
- **Platform API ya existe** en `<<ruta a RDL.Platform.API>>`. Es la referencia de lo que ya está decidido y funcionando. Lee:
  - `api/openapi.yaml`: el OpenAPI real de Platform (se importa a este repo, no se reescribe);
  - `docs/decisiones/*.md` y `docs/ESTADO.md` (sección "TODOs de contrato": son preguntas que este repo debe responder);
  - `internal/adapters/http/problem/` y `internal/adapters/http/errors.go`: los `type` de Problem Details en uso;
  - `pkg/correlation` y la idempotencia de `internal/app`: cómo se usan hoy `X-Correlation-Id` e `Idempotency-Key`;
  - `CLAUDE.md` y la estructura de carpetas: **la arquitectura de este repo debe seguir el mismo estilo**.
- Especificación oficial de Hacienda (comprobantes electrónicos, versión vigente) en `<<ruta o URL de la especificación>>`, si está disponible. **Si no está, ningún campo fiscal se inventa** (ver Paso 3).

## Datos del entorno

- Ruta del repo: `<<C:\Projects\Personal\RDL PYMES\RDL.Contracts>>`.
- Módulo Go (debe ser importable desde las otras APIs): `<<ej. bitbucket.org/rdl/contracts o github.com/rdl/contracts>>`.
- Hosting del repo y CI: `<<Bitbucket Pipelines / GitHub Actions>>`.
- Proyecto Supabase (dev), **solo lectura** para alinear tipos: `<<project-ref>>`. Nunca producción. Este repo **no migra ni escribe** en la base.
- Versión de Go: `<<1.27+>>` (la misma que Platform).

## Paso 1 · Inventario de lo que ya está decidido (obligatorio)

Los contratos no parten de cero: Platform y la base ya fijaron varias cosas. **No las supongas: léelas.**

1. **Base de datos** (MCP de Supabase en solo lectura):
   - dominios y tipos compartidos en `shared` (por ejemplo `shared.money_amount`, `shared.currency_code`, `shared.sha256_hex`): precisión y escala de los montos;
   - columnas de `integration.outbox_messages` e `integration.inbox_messages`: el sobre (envelope) de los eventos tiene que caber ahí sin transformaciones raras;
   - catálogo `core.roles`, estados posibles de `core.organizations`, `core.organization_users` e `core.invitations`;
   - tablas de `billing`, `fiscal` y `receivables` que ya existan, con sus columnas de estado y sus `CHECK`: son la mejor pista de las máquinas de estado.
2. **Platform API**: endpoints, esquemas, `type` de Problem Details, headers, paginación por cursor, formato de fechas y roles.
3. **Documento de arquitectura**: catálogo de los 8 eventos, contrato mínimo de `InvoiceIssued` y reglas de confiabilidad.

Entrega un **informe** en `docs/decisiones/0001-inventario-y-brechas.md` con:
- lo que ya está decidido y este repo solo **formaliza** (con la fuente: archivo, tabla o sección);
- las **contradicciones** entre fuentes (por ejemplo, un rol con otro nombre en la base y en el documento, o una precisión de montos distinta);
- lo que **falta decidir**, con una propuesta y quién debe aprobarla.

**Detente y espera mi aprobación del informe antes de escribir artefactos.**

## Paso 2 · Documentos de acuerdo (`docs/`)

Todo en español, corto y verificable. Cada documento termina con una lista de **TODOs y preguntas abiertas**.

1. **`docs/glosario.md`**: organización vs cliente final, usuario vs miembro, sucursal vs establecimiento/terminal fiscal, factura (comercial) vs documento electrónico (fiscal), nota de crédito/débito, cuenta por cobrar, pago, aplicación de pago, snapshot, tenant. Para cada término: definición, dueño (API) y lo que **no** es.
2. **`docs/maquinas-de-estado/`**: `invoice.md`, `electronic-document.md`, `receivable.md` (y `payment.md` si hace falta). Para cada una:
   - estados, transiciones permitidas, **quién la dispara** (comando HTTP, evento o worker) y qué evento emite;
   - diagrama Mermaid `stateDiagram-v2`;
   - la misma máquina en `state-machines/<entidad>.yaml`, legible por máquina, para que las APIs generen tests a partir de ella.
   Recuerda (sección 6.1): una factura puede estar `ISSUED` mientras su documento fiscal está `PROCESSING`, `ACCEPTED` o `REJECTED`. Son máquinas distintas.
3. **`docs/ownership.md`** + **`ownership/ownership.yaml`**: matriz de la tabla 2.1 (qué API escribe y lee cada schema) en formato verificable. Las APIs podrán usar el YAML en sus tests de permisos.
4. **`docs/convenciones.md`**: la ley común. Mínimo:
   - **IDs:** UUID (v4 o v7; decide y justifica), en minúsculas.
   - **Fechas:** RFC 3339 / ISO-8601 **en UTC con `Z`**. Fechas sin hora (vencimientos) como `YYYY-MM-DD` y en qué zona se interpretan.
   - **Dinero:** **string decimal**, nunca `number`, con la precisión acordada con la base (`shared.money_amount`). Moneda ISO 4217 en campo aparte. Define redondeo y en qué punto del cálculo se aplica (o déjalo como decisión pendiente si depende de "dónde vive el cálculo de impuestos").
   - **Errores:** Problem Details (RFC 9457), `application/problem+json`, con `type` estable. Formaliza la convención que ya usa Platform (`urn:rdl:<servicio>:problem:<código>`) y un registro de tipos por servicio en `problems/<servicio>.yaml`.
   - **Idempotencia:** header `Idempotency-Key` en todo comando `POST`; misma clave y mismo cuerpo → misma respuesta; misma clave y otro cuerpo → 422. Vigencia de las claves.
   - **Correlación:** `X-Correlation-Id` (UUID), propagado a logs, trazas, eventos y auditoría.
   - **Tenancy:** la organización sale del token (`org_id`), nunca del body, la query, un header ni la ruta. Rutas `/v1/organizations/current/...`. Recurso de otra organización → 404; rol insuficiente → 403.
   - **Paginación:** `limit` (1–100) y `cursor` opaco; respuesta con `items` y `nextCursor`.
   - **Nombres:** `camelCase` en JSON, `snake_case` en la base, eventos en `PascalCase` en pasado (`InvoiceIssued`).
   - **Versionado:** SemVer del repo, versión mayor en la ruta HTTP (`/v1`), versión entera por evento (`InvoiceIssued` v1, v2...).

## Paso 3 · Eventos (AsyncAPI 3 + JSON Schema)

- `asyncapi/asyncapi.yaml` (AsyncAPI **3.0**) con los **8 eventos** del catálogo 6.2: `InvoiceIssued`, `InvoiceCancelled`, `CreditNoteIssued`, `DebitNoteIssued`, `ElectronicDocumentAccepted`, `ElectronicDocumentRejected`, `PaymentReceived`, `ReceivableSettled`. Indica productor y consumidores de cada uno.
- Un JSON Schema **2020-12** por evento y versión: `schemas/events/<evento-en-kebab>.v1.json` (por ejemplo `invoice-issued.v1.json`), con `$id` estable y `additionalProperties: false`.
- **Sobre común** en `schemas/events/envelope.v1.json`, referenciado por todos: `eventId`, `eventType`, `version`, `occurredAt`, `correlationId` y `organizationId`, **obligatorios**. Alinéalo con las columnas de `integration.outbox_messages` e `integration.inbox_messages` (Paso 1) y documenta el mapeo campo ↔ columna.
- Tipos reutilizables en `schemas/common/`: `money.json` (string decimal con patrón), `currency-code.json`, `uuid.json`, `utc-datetime.json`, `identification.json` y `customer-snapshot.json`.
- **`InvoiceIssued` v1 completo:** encabezado, `customerSnapshot`, sucursal, moneda, **líneas** (cantidad, unidad, precio unitario, descuentos, impuestos por línea, exoneraciones) y totales. Reglas:
  - montos siempre como string decimal (nunca `number`);
  - los totales se pueden verificar contra las líneas: documenta la fórmula;
  - los **códigos fiscales** (CABYS, tipo y tarifa de impuesto, tipo de exoneración, condición de venta, medio de pago, unidad de medida) van como strings con referencia al catálogo oficial. **No inventes valores, enumeraciones ni reglas:** si la especificación de Hacienda no está en el repo, deja el campo con un `TODO(fiscal)` en la descripción y lístalo al final.
- **Ejemplos:** al menos un ejemplo válido y dos inválidos por evento en `examples/events/<evento>/`. Los inválidos documentan qué debe rechazarse (monto como `number`, falta `organizationId`, fecha sin zona, campo extra).
- **Eventos de Platform:** hoy no están en el catálogo 6.2 (Platform tiene TODOs por esto). **No los agregues**: propónlos en el informe como decisión pendiente (`OrganizationCreated`, `MemberAdded`, etc.).

## Paso 4 · OpenAPI

- `openapi/platform.yaml`: **importa** el `api/openapi.yaml` de Platform tal cual y adáptalo solo a las convenciones comunes (componentes compartidos). Todo cambio que afecte a Platform va a una lista para aplicar en ese repo.
- `openapi/billing.yaml`, `openapi/e-invoice.yaml`, `openapi/receivables.yaml`: **esqueletos** OpenAPI 3.1 con los recursos y rutas principales de la sección 3 (sin detallar todos los campos), usando los componentes comunes.
- `openapi/bff-internal.yaml`: las rutas internas que usará el BFF, si el documento las define. Si no las define, deja el esqueleto con un TODO.
- `openapi/components/`: componentes compartidos por todas las APIs: `Problem`, `ValidationProblem`, headers (`Idempotency-Key`, `X-Correlation-Id`), parámetros de paginación, `Money`, `UtcDateTime` y `bearerAuth`. Las APIs los referencian con `$ref` y no los copian.

## Paso 5 · Herramientas en Go: arquitectura y calidad de código

Misma arquitectura que Platform API: hexagonal (ports & adapters) con Clean Architecture, dependencias hacia el dominio.

```
RDL.Contracts/
├── cmd/contractsctl/main.go      # composition root del CLI: flags, wiring, salida; nada de lógica
├── internal/
│   ├── domain/                   # sin imports de infraestructura
│   │   ├── schema/               # modelo simplificado de un schema (propiedades, tipos, required, enum)
│   │   ├── compat/               # reglas de compatibilidad: qué cambio rompe y cuál no (funciones puras)
│   │   ├── catalog/              # catálogo de eventos: nombre, versión, productor, consumidores; invariantes
│   │   ├── statemachine/         # estados, transiciones, invariantes (sin estados huérfanos, inicial único...)
│   │   └── convention/           # reglas verificables: money como string, fechas con formato, camelCase...
│   ├── app/                      # casos de uso (un struct por caso) + puertos (interfaces pequeñas)
│   │   ├── validate.go           # ValidateAll: schemas, ejemplos, AsyncAPI, OpenAPI, máquinas de estado
│   │   ├── breaking.go           # CheckBreaking: compara contra la versión base (tag o rama)
│   │   └── lint.go               # LintConventions: aplica domain/convention a todos los artefactos
│   ├── adapters/
│   │   ├── fs/                   # lee artefactos del disco (o de un fs.FS para tests)
│   │   ├── git/                  # obtiene la versión base de un archivo (git show <ref>:<path>)
│   │   ├── jsonschema/           # validación con santhosh-tekuri/jsonschema/v6
│   │   ├── openapi/              # carga y validación con kin-openapi; diff con oasdiff
│   │   └── report/               # salida legible en texto y JSON para CI
│   └── platform/                 # config y logger del CLI
├── pkg/
│   ├── events/                   # DTOs Go de los eventos + Envelope; lo importan las APIs
│   │   └── money/                # tipo Money (decimal como string, sin float) con Marshal/Unmarshal
│   └── problem/                  # tipos y constructores de Problem Details comunes
├── asyncapi/  schemas/  openapi/  examples/  state-machines/  ownership/  problems/
├── docs/{contexto,decisiones}/   # contexto, ADRs e informe de brechas
├── CHANGELOG.md, CLAUDE.md, README.md, Makefile, .golangci.yml, go.mod
```

**Comandos del CLI:**
| Comando | Qué hace |
|---|---|
| `contractsctl validate` | Valida todo: JSON Schemas bien formados y con `$ref` resolubles; ejemplos válidos pasan y los inválidos fallan; AsyncAPI y OpenAPI contra su meta-schema; máquinas de estado consistentes; el ownership sin huecos. |
| `contractsctl lint` | Convenciones: ningún monto como `number`, fechas con `format: date-time`, `additionalProperties: false`, `camelCase`, sobre común en cada evento, `type` de Problem con el formato acordado. |
| `contractsctl breaking --base <ref>` | Compara con la versión publicada (tag o `main`). **Falla** si un schema publicado (`vN`) cambia de forma incompatible: quitar o renombrar un campo, agregar un `required`, cambiar un tipo, achicar un enum, endurecer un patrón. Agregar un campo opcional es compatible. Para OpenAPI usa `oasdiff`. |
| `contractsctl version` | Versión del repo y de cada evento. |

**Paquete `pkg/events`:**
- Un struct por evento y versión (`InvoiceIssuedV1`) más `Envelope`, con tags JSON iguales al schema.
- `Money` es un tipo propio basado en string decimal (o `shopspring/decimal` si lo justificas en un ADR). **Prohibido `float32` o `float64`** en cualquier DTO.
- Fechas como `time.Time` serializadas en UTC con `Z`.
- Los DTOs se escriben a mano, **y un test de contrato garantiza que coinciden con el schema**: cada propiedad del schema tiene su campo y viceversa, los `required` no son `omitempty`, y el ejemplo válido hace round trip (JSON → struct → JSON) sin perder nada. Si propones generarlos (por ejemplo con `omissis/go-jsonschema`), justifícalo en un ADR.
- Es un módulo versionado con tags SemVer (`vX.Y.Z`). Un cambio incompatible en un evento **no** modifica `V1`: agrega `V2`.

**Stack:** Go estándar, `santhosh-tekuri/jsonschema/v6` (JSON Schema 2020-12), `getkin/kin-openapi`, `oasdiff/oasdiff`, `gopkg.in/yaml.v3` (o `goccy/go-yaml`), `log/slog` y `golangci-lint`. AsyncAPI 3 se valida contra su meta-schema oficial (vendorizado en `internal/adapters/jsonschema/metaschemas/`) con el mismo validador. Ninguna otra dependencia sin justificarla.

**Principios que se tienen que notar en el código** (los mismos de Platform):
- **SRP:** un caso de uso por struct; `main.go` solo parsea flags, arma dependencias y traduce el resultado a código de salida.
- **OCP:** una regla nueva de convención o de compatibilidad se agrega como una función más en su lista, sin tocar los casos de uso.
- **ISP y DIP:** interfaces pequeñas **definidas del lado del consumidor** (en `app`), implementadas por los adapters, inyectadas por constructor. Sin estado global ni `init()` con efectos.
- El dominio no sabe de archivos, git ni librerías de JSON Schema: trabaja sobre su propio modelo.
- Errores tipados con `errors.Is`/`As`. Cada hallazgo del CLI dice **archivo, ruta JSON (pointer) y regla**, para que se corrija sin adivinar.
- `context.Context` como primer parámetro; salida determinista (orden estable) para que el CI sea reproducible.
- Nombres claros, funciones cortas, sin comentarios obvios; comenta el **porqué**, no el qué.

## Paso 6 · Tests (escríbelos desde las reglas, no desde la implementación)

- **Dominio** con tablas: cada regla de `compat` con un caso compatible y uno incompatible (quitar campo, agregar `required`, cambiar tipo, achicar enum, ampliar enum, agregar opcional, cambiar `format`); cada invariante de `statemachine` y de `convention`.
- **Casos de uso** con `fstest.MapFS` y fakes de los puertos (sin disco ni git reales).
- **Artefactos reales** (el test más importante): `go test ./...` valida todos los schemas, ejemplos, AsyncAPI, OpenAPI y máquinas de estado del repo. Si alguien rompe un contrato, falla el test, no solo el CI.
- **Contrato de `pkg/events`:** struct ↔ schema y round trip de ejemplos (Paso 5). Un test que falle si aparece un `float` en cualquier DTO (por reflexión).
- **`breaking` de punta a punta:** un repo git temporal con una versión base y un cambio incompatible → el comando termina con código distinto de 0 y reporta el campo.
- **Platform:** el OpenAPI importado de Platform pasa `validate` y `lint`. Si no pasa, **no lo cambies para que pase**: repórtalo como hallazgo para Platform.

## Paso 7 · CI y entrega

- Pipeline (`<<Bitbucket Pipelines / GitHub Actions>>`) en cada PR: `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test ./...`, `contractsctl validate`, `contractsctl lint` y `contractsctl breaking --base <último tag>`.
- Un PR que cambie un contrato publicado de forma incompatible **falla** salvo que agregue una versión nueva del evento o suba la versión mayor.
- Al hacer merge a `main`: tag SemVer y publicación del módulo Go (`pkg/events`, `pkg/problem`). `CHANGELOG.md` con cada cambio de contrato.
- `Makefile` con: `build`, `test`, `lint`, `validate`, `contracts-lint`, `breaking`, `release`.
- `CLAUDE.md` con el bloque común de reglas de plataforma (el mismo de Platform), más las reglas propias de este repo (ver abajo), y el comando de cierre: `go build ./... && go vet ./... && golangci-lint run && go test ./... && go run ./cmd/contractsctl validate && go run ./cmd/contractsctl lint`.
- `README.md` con: qué es cada carpeta, cómo validar en local, cómo proponer un cambio de contrato, cómo versionar un evento y cómo lo importa una API (`go get <módulo>@vX.Y.Z`).
- ADRs cortos en `docs/decisiones/` por cada decisión: formato de dinero y precisión, UUID v4 vs v7, convención de `type` de Problem, DTOs a mano vs generados, política de versionado y compatibilidad.

**Reglas propias para el `CLAUDE.md` de este repo:**
- Un schema publicado (`vN` con tag) es **inmutable** salvo cambios compatibles. Lo incompatible va en `vN+1`.
- Montos como string decimal; ningún `number` para dinero ni `float` en Go.
- No se inventan campos fiscales, eventos ni estados: TODO y a la lista de pendientes.
- Todo cambio de contrato necesita 2 aprobaciones (planning P0) y una entrada en `CHANGELOG.md`.
- Este repo no se conecta a producción ni escribe en ninguna base.

## Forma de trabajo

1. **Plan primero:** informe de inventario y brechas (Paso 1), lista de decisiones pendientes con propuesta y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por incrementos verticales**, cada uno compilando y con sus tests en verde:
   1. esqueleto del repo, `go.mod`, CLI vacío, `Makefile`, lint y CI;
   2. convenciones, glosario y ownership (docs + YAML) y `contractsctl validate` para el ownership;
   3. tipos comunes, sobre de eventos y `pkg/events/money`;
   4. `InvoiceIssued` v1 completo (schema, ejemplos, DTO, test de contrato);
   5. los otros 7 eventos y `asyncapi.yaml`;
   6. máquinas de estado (docs, YAML y validación);
   7. OpenAPI: componentes comunes, Platform importado y esqueletos de Billing, E-Invoice, Receivables y BFF;
   8. `contractsctl lint` y `contractsctl breaking`;
   9. endurecimiento: autorrevisión, README, CHANGELOG, primer tag `v0.1.0` y lista de TODOs.
3. Al terminar cada incremento: `go build ./... && go vet ./... && golangci-lint run && go test ./...`, un resumen de lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: montos como `number` o `float`, fechas sin zona, campos fiscales inventados, eventos o estados que no estén en el documento, contradicciones con lo que ya hace Platform y schemas sin `additionalProperties: false`.

## Prohibido

- Inventar campos fiscales, códigos, enumeraciones de Hacienda, eventos o estados que no estén en el documento o en la especificación: deja un TODO y lístalo al final.
- Usar `number` para dinero en los schemas o `float32`/`float64` en Go.
- Modificar de forma incompatible un schema ya publicado en lugar de crear una versión nueva.
- Cambiar el OpenAPI de Platform para que pase las validaciones sin reportarlo: Platform es la implementación real.
- Conectarte a producción, escribir en la base o usar datos o certificados reales.

## Definición de terminado

- [ ] Informe de inventario aprobado y decisiones pendientes listadas con propuesta y responsable.
- [ ] Glosario, convenciones, ownership y máquinas de estado de Invoice, ElectronicDocument y Receivable escritos y en formato verificable.
- [ ] AsyncAPI 3 con los 8 eventos, un JSON Schema por evento con el sobre común, y `InvoiceIssued` v1 completo con líneas, descuentos, impuestos y exoneraciones (con TODOs donde falte la especificación).
- [ ] OpenAPI con componentes comunes, Platform importado y esqueletos de Billing, E-Invoice, Receivables y BFF.
- [ ] `contractsctl validate`, `lint` y `breaking` funcionando y en el CI; un cambio incompatible sin versión nueva hace fallar el pipeline.
- [ ] `pkg/events` publicado con tag `v0.1.0`, sin `float`, con los tests de contrato struct ↔ schema en verde.
- [ ] `golangci-lint` limpio y cobertura razonable en dominio y casos de uso.
- [ ] Lista final de TODOs (fiscales, de Platform y de decisiones) para revisar en equipo. Con esto se cumple el hito **H1 Contratos v1**.
