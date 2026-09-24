# Contracts

Fuente única de verdad de lo que se prometen las APIs del SaaS de facturación electrónica para PYMES de Costa Rica:
**Platform**, **Billing**, **E-Invoice** (servicio `fiscal`), **Receivables** y el **BFF**.

Contiene dos cosas:

1. **Contratos**: glosario, convenciones, ownership de schemas, máquinas de estado, eventos (AsyncAPI 3 +
   JSON Schema 2020-12), OpenAPI 3.1 y registro de tipos de error.
2. **Herramientas en Go** que los hacen verificables:
   - `contractsctl`, un CLI que valida todo, aplica las convenciones y detecta cambios incompatibles;
   - `pkg/events`, los DTOs de los eventos y su validador, que las APIs importan.

Versión: **v0.2.0** (hito H1 · Contratos v1, con las reglas del borrador de los Anexos v4.4 de Hacienda). Estado y pendientes: [`docs/ESTADO.md`](docs/ESTADO.md).

---

## Qué hay en cada carpeta

| Carpeta | Contenido |
|---|---|
| [`docs/`](docs/) | [Glosario](docs/glosario.md), [convenciones](docs/convenciones.md), [ownership](docs/ownership.md), [catálogo de eventos](docs/eventos/README.md), [máquinas de estado](docs/maquinas-de-estado/), [OpenAPI](docs/openapi.md), [decisiones (ADR)](docs/decisiones/) y contexto (arquitectura y planning) |
| [`schemas/common/`](schemas/common/) | Tipos comunes: `Money`, `Quantity`, `Percentage`, `ExchangeRate`, `CurrencyCode`, `Uuid`, `UtcDateTime`, `BusinessDate`, `CabysCode`, `FiscalCode`, `DocumentType`, `Identification`, `CustomerSnapshot`, `ServiceName` |
| [`schemas/events/`](schemas/events/) | Sobre común y un JSON Schema por evento y versión (`invoice-issued.v1.json`...). En `parts/`, las piezas que comparten |
| [`asyncapi/`](asyncapi/asyncapi.yaml) | Catálogo de los 8 eventos v1: canales, productores y consumidores |
| [`openapi/`](openapi/) | Componentes comunes, Platform (importado), esqueletos de Billing, fiscal y Receivables, y rutas internas del BFF |
| [`state-machines/`](state-machines/) | Factura y notas, documento electrónico, cuenta por cobrar y pago, en YAML legible por máquina |
| [`ownership/`](ownership/ownership.yaml) | Qué servicio escribe y lee cada schema de la base |
| [`problems/`](problems/) | Tipos de error (`urn:rdl:<servicio>:problem:<código>`) de cada servicio |
| [`examples/`](examples/) | Ejemplos válidos e inválidos de cada tipo común y de cada evento |
| `cmd/contractsctl`, `internal/` | El CLI, con arquitectura hexagonal (dominio sin infraestructura, casos de uso, adapters) |
| [`pkg/events`](pkg/events/) | DTOs Go de los eventos, `Validator` sobre los schemas embebidos y tipos decimales (`pkg/events/money`) |

## Uso rápido

Requisito: Go 1.27+.

```sh
go run ./cmd/contractsctl validate       # artefactos bien formados y coherentes entre sí
go run ./cmd/contractsctl lint           # convenciones (dinero como string, fechas UTC, tenancy...)
go run ./cmd/contractsctl breaking -base ../contracts-v0.1.0   # cambios incompatibles contra una versión publicada
go run ./cmd/contractsctl diagram state-machines/invoice.yaml   # diagrama Mermaid de una máquina de estado
go test ./...
golangci-lint run ./...
```

Todos los comandos aceptan `-root <dir>` (por defecto `.`) y `validate`, `lint` y `breaking` aceptan `-format json`
para el CI. Códigos de salida: `0` sin errores, `1` hay hallazgos de error, `2` uso incorrecto o fallo al leer.
Los avisos (`warning`) no hacen fallar: hoy son los hallazgos del OpenAPI importado de Platform.

**Antes de abrir un PR:**

```sh
go build ./... && go vet ./... && golangci-lint run ./... && go test ./... \
  && go run ./cmd/contractsctl validate && go run ./cmd/contractsctl lint
```

Con `make`: `make check` (build, test, lint, validate y contracts-lint) y `make breaking BASE=<dir>`.

## Qué verifica cada comando

**`validate`**
- Ownership: ninguna API escribe en el schema de otra; `audit` solo admite insert; servicios y roles de la base.
- JSON Schema: dialecto 2020-12, `$id` igual a la ruta del archivo, `$ref` solo a archivos del repo.
- Ejemplos: los válidos pasan y los inválidos fallan.
- AsyncAPI: cumple el meta-schema oficial 3.0 y el catálogo es igual a la tabla 6.2 de la arquitectura (ningún
  evento inventado; productor, consumidores, schema y canal coherentes).
- Máquinas de estado: estados alcanzables, finales sin salida, disparadores tipados, eventos coherentes con el
  catálogo y diagrama del doc sincronizado con el YAML.
- OpenAPI: cumple el meta-schema oficial 3.1, todos los `$ref` resuelven, `operationId` único y cada comando de las
  máquinas de estado existe en el OpenAPI de su dueño.

**`lint`**: las reglas de [ADR 0006](docs/decisiones/0006-lint-y-breaking.md): sin `number` para dinero, fechas con
su tipo, camelCase, objetos cerrados, sobre en cada evento, rutas `/v1/`, sin parámetros de organización,
`Idempotency-Key` en los POST y registro de tipos de error completo.

**`breaking`**: compara con un directorio que contiene la versión publicada (el último tag extraído). Falla si un
schema publicado cambia de forma incompatible, si desaparece una operación o un evento, o si un parámetro pasa a ser
obligatorio.

## Cómo proponer un cambio de contrato

1. Rama y PR. Todo cambio de contrato necesita **2 aprobaciones** (planning P0).
2. Cambie el artefacto (schema, OpenAPI, máquina de estado...), sus ejemplos y su doc. Si cambió una máquina de
   estado, regenere su diagrama con `contractsctl diagram` y péguelo en su doc.
3. Si el cambio afecta a un DTO de `pkg/events`, cambie el struct: el test de contrato compara struct y schema campo
   por campo y falla si no coinciden.
4. Agregue una entrada en [`CHANGELOG.md`](CHANGELOG.md).
5. El CI corre `validate`, `lint` y `breaking` contra el último tag.

**Nunca se inventan** campos fiscales, códigos de Hacienda, eventos ni estados: se deja un `TODO(fiscal)` o `TODO` y
se agrega a la lista de pendientes de `docs/ESTADO.md`.

## Cómo versionar un evento

- Un schema publicado es **inmutable salvo cambios compatibles** (agregar un campo opcional).
- Un cambio incompatible crea `schemas/events/<evento>.v2.json`, su suite de ejemplos, su mensaje en
  `asyncapi.yaml` (`x-rdl-version: 2`) y un struct `...V2` en `pkg/events`. **`V1` no se toca**: ambas versiones
  conviven hasta que ningún consumidor use la anterior.
- `contractsctl breaking` detecta el cambio incompatible y sugiere este camino.

## Cómo lo usa una API

```sh
go get bitbucket.org/rdl/contracts@v0.1.0
```

```go
import (
    "bitbucket.org/rdl/contracts/pkg/events"
    "bitbucket.org/rdl/contracts/pkg/events/money"
)

e := events.InvoiceIssuedV1{
    Envelope:  events.NewHeader(events.InvoiceIssuedSpec, orgID, correlationID, events.NewInstant(issuedAt)),
    InvoiceID: invoice.ID,
    Currency:  money.MustCurrency("CRC"),
    // ...
}
row, err := events.ToOutboxRow(e)                   // fila para integration.outbox_messages
v, _ := events.DefaultValidator()
err = v.Validate(events.InvoiceIssuedSpec.SchemaFile, row.Payload) // validar antes de escribir
```

- Los montos son `money.Amount` (string decimal, nunca `float`). Cada API calcula con su propia librería decimal y
  convierte con `money.ParseAmount` / `String()`.
- `events.Instant` siempre se serializa en UTC con `Z`; `events.Date` es la fecha de negocio (`events.DateIn(t, loc)`
  la calcula en la zona de la organización).
- Los schemas van embebidos en el módulo: el `Validator` usa exactamente la versión que se importó.

La ruta del módulo (`bitbucket.org/rdl/contracts`) es provisional (decisión D1 del informe 0001). Si cambia, se
cambia en `go.mod` y en los imports.

## Publicar una versión

1. `CHANGELOG.md` con la sección de la versión y `make check` en verde.
2. Crear el tag `vX.Y.Z` y publicarlo (`make release VERSION=vX.Y.Z` lo crea; el push lo hace quien publica).
3. Las APIs actualizan con `go get bitbucket.org/rdl/contracts@vX.Y.Z`.

## Documentación relacionada

- [`docs/ESTADO.md`](docs/ESTADO.md): estado, definición de terminado y **lista de pendientes para revisar en equipo**.
- [`docs/decisiones/`](docs/decisiones/): ADR 0001 (inventario y decisiones D1–D13) a 0006.
- [`CLAUDE.md`](CLAUDE.md): reglas para trabajar en este repo con Claude Code.
