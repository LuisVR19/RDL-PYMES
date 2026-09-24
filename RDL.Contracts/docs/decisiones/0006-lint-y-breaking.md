# 0006 · `contractsctl lint` y `contractsctl breaking`

- **Fecha:** 2026-09-24 · **Estado:** Aceptado (con desviaciones del prompt P0, anotadas abajo)

## `lint`: convenciones como reglas

Reglas puras en `internal/domain/convention`, aplicadas a `schemas/**`, `openapi/**` y `problems/*.yaml`:

| Regla | Qué exige | Convención |
|---|---|---|
| `no-number` | Ningún `type: number` (los decimales son string) | §3 |
| `decimal-as-integer` | Un campo con nombre de monto (`*Amount`, `total`, `tax`, `*Price`, `*Rate`...) no es `integer` | §3 |
| `instant-format` / `date-format` | `*At` es `UtcDateTime`; `*Date` y `*On` son `BusinessDate` | §2 |
| `camel-case` | Nombres de campo en camelCase | §4 |
| `closed-object` | Todo objeto de `schemas/` se cierra, salvo los schemas base marcados `x-rdl-base: true` | §10 |
| `event-envelope` / `event-closed` / `event-const` | Un evento compone el sobre, cierra con `unevaluatedProperties: false` y fija `eventType`, `version` y `sourceService` | §10 |
| `openapi-path-version` | Rutas bajo `/v1/` o `/internal/v1/` | §11 |
| `openapi-tenant-param` | Ningún parámetro `organizationId` / `tenant` | §5 |
| `openapi-idempotency` | Todo `POST` público lleva `Idempotency-Key` | §7 |
| `problem-*` | `problems/<servicio>.yaml` existe para los 4 servicios, con códigos kebab-case únicos, status 4xx/5xx, `title` y `when`, y los tipos comunes | §6 |

`x-rdl-base` es una anotación: JSON Schema 2020-12 ignora las palabras clave desconocidas, así que no cambia la
validación.

## `breaking`: contra un directorio base, sin git

- `contractsctl breaking -base <dir>` compara el repo con **un directorio que contiene la versión publicada** (el
  último tag extraído). **Desviación:** el prompt proponía leer la base con `git show <ref>:<path>`. Un directorio
  deja al CLI sin depender de git (se puede comparar contra un tarball, un artefacto de CI o una copia local) y lo
  hace probable con `fstest.MapFS`. El pipeline arma la base con `git archive` del último tag.
- Reglas (`internal/domain/compat`, tabla §11), conservadoras:
  - **Schemas:** quitar un campo o un archivo, agregar un `required`, cambiar `type`, `$ref`, `const`, `format`,
    `pattern` o el cierre del objeto, endurecer un límite, y quitar **o agregar** un valor de `enum` (los eventos no
    son extensibles). Agregar un campo opcional o aflojar un límite es compatible.
  - **OpenAPI:** quitar una operación, volver obligatorio un parámetro (también los que llegan por `$ref` a
    `components/common.yaml`) o el cuerpo. **Desviación:** no se usa `oasdiff`, que tiene soporte parcial de 3.1 y
    agregaría una dependencia grande para esqueletos. Cuando los OpenAPI dejen de ser esqueletos, se reevalúa.
  - **Catálogo:** quitar un evento del AsyncAPI.
- La salida sugiere el camino correcto: un evento incompatible se publica como `vN+1` en un archivo nuevo.
