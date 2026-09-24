# 0005 · Validación de OpenAPI 3.1 y AsyncAPI 3.0 con sus meta-schemas oficiales

- **Fecha:** 2026-09-24 · **Estado:** Aceptado (desviación del prompt P0, que proponía `kin-openapi`)

## Decisión

1. Los documentos OpenAPI se validan contra el **meta-schema oficial de OpenAPI 3.1**
   (`https://spec.openapis.org/oas/3.1/schema/2022-10-07`) y los AsyncAPI contra el de **AsyncAPI 3.0.0**. Ambos se
   vendorizan en `internal/adapters/jsonschema/metaschemas/` y se aplican con el mismo validador JSON Schema de los
   eventos (`santhosh-tekuri/jsonschema/v6`).
2. El meta-schema no mira dentro de los Schema Objects; esa parte ya la cubre `SchemasCheck`, porque los tipos de
   datos son los JSON Schema del repo. Además, `contractsctl` resuelve cada `$ref` (también entre archivos) y
   verifica `operationId` y los comandos de las máquinas de estado.
3. Un documento **importado** (hoy `openapi/platform.yaml`) no se retoca: sus hallazgos salen como avisos
   dirigidos a la API dueña.

## Por qué no `kin-openapi`

- Todos los documentos son OpenAPI **3.1** (el de Platform ya lo era), y el soporte de 3.1 en `kin-openapi` es
  parcial: no entiende bien `type: [string, 'null']`, `$ref` con hermanos ni el dialecto 2020-12.
- El meta-schema oficial es la definición normativa de la especificación y usa el mismo motor que el resto del repo:
  una dependencia menos y mensajes de error iguales en todo `contractsctl`.

## Consecuencias

- El diff de compatibilidad de OpenAPI (incremento 8) se decide aparte: `oasdiff` soporta 3.1 con limitaciones, que
  se evaluarán allí.
- Para actualizar un meta-schema se reemplaza el archivo vendorizado y se corren los tests.
