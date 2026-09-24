# 0003 · Identidad y composición de los JSON Schema

- **Fecha:** 2026-09-24 · **Estado:** Aceptado

## Decisión

1. **Dialecto único:** JSON Schema 2020-12. `contractsctl validate` rechaza otro `$schema`.
2. **`$id` = ruta del archivo** bajo la base `https://contracts.rdl.invalid/`
   (`schemas/common/money.json` → `https://contracts.rdl.invalid/schemas/common/money.json`). El TLD `.invalid`
   (RFC 2606) no existe, así que nada se resuelve por red; el validador carga todos los schemas del repo y bloquea
   cualquier `$ref` externo.
3. Los `$ref` son **relativos** (`../common/uuid.json`): significan lo mismo leyendo el repo que en el validador.
4. **Formatos como aserción:** `uuid`, `date-time`, `date` y `email` se validan (en 2020-12 son solo anotaciones por
   defecto). Además, `uuid` y `utc-datetime` tienen `pattern` (minúsculas; sufijo `Z`), porque `format` solo no exige
   eso.
5. **Sobre de eventos:** `envelope.v1.json` es un schema base, abierto. Cada evento hace
   `allOf: [{ "$ref": "envelope.v1.json" }]`, declara sus campos y cierra con **`unevaluatedProperties: false`**.
   `additionalProperties: false` no sirve con `allOf` (rechazaría los campos del sobre). Los objetos que no componen
   (tipos comunes, líneas) usan `additionalProperties: false`.
6. **Ejemplos como suites:** `examples/**/*.json` con `{schema, valid[], invalid[{value, why}]}`. Todo ejemplo válido
   debe pasar y todo inválido debe fallar. Un inválido que pasa significa que el schema es demasiado permisivo.

## Por qué

- Un `$id` que refleja la ruta evita la clase de error "el `$ref` apunta a otro archivo del que parece".
- Las suites de ejemplos documentan con casos concretos qué se rechaza (monto como number, fecha sin `Z`, campo
  extra) y se verifican solas en cada `go test`.
