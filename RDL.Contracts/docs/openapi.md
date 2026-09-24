# OpenAPI

| Documento | API | Estado |
|---|---|---|
| [`openapi/components/common.yaml`](../openapi/components/common.yaml) | todas | Componentes comunes: seguridad, parámetros (`Idempotency-Key`, `X-Correlation-Id`, `limit`, `cursor`), `Problem`, respuestas de error y tipos de datos (los mismos JSON Schema de los eventos) |
| [`openapi/platform.yaml`](../openapi/platform.yaml) | Platform | **Importado tal cual** de `RDL.Platform.API/api/openapi.yaml` (v0.7.0). Es la implementación real |
| [`openapi/billing.yaml`](../openapi/billing.yaml) | Billing | Esqueleto: clientes, productos, facturas y notas, emisión, anulación, numeración |
| [`openapi/fiscal.yaml`](../openapi/fiscal.yaml) | E-Invoice (`fiscal`) | Esqueleto: perfil fiscal y certificado, establecimientos, documentos electrónicos y archivos, reintento, catálogos |
| [`openapi/receivables.yaml`](../openapi/receivables.yaml) | Receivables | Esqueleto: cuentas por cobrar, aging, pagos, aplicaciones y reversos, seguimientos |
| [`openapi/bff-internal.yaml`](../openapi/bff-internal.yaml) | las tres | Rutas internas `/internal/v1/...` que el BFF compone para la vista transversal (arquitectura 2.2) |

"Esqueleto" significa recursos, rutas, estados y errores principales; los campos se completan en el repo de cada API
y vuelven aquí por PR.

## Qué verifica `contractsctl validate`

1. Cada documento cumple el **meta-schema oficial de OpenAPI 3.1** (vendorizado, sin red).
2. **Todos los `$ref` resuelven**, también los que apuntan a otro archivo (`components/common.yaml`,
   `../schemas/...`). Un `$ref` a una URL externa es un error.
3. Toda operación tiene **`operationId` único** en su documento.
4. Cada **comando de las máquinas de estado** (`POST /v1/invoices/{id}/issue`...) existe como operación en el
   OpenAPI de la API dueña.
5. Los hallazgos sobre `platform.yaml` salen como **avisos para Platform**: el documento no se retoca aquí.

## Hallazgos para Platform (alinear en `RDL.Platform.API`)

| Hallazgo | Propuesta |
|---|---|
| `GET /healthz` y `GET /readyz` sin `operationId` | Agregar `getLiveness` y `getReadiness` |
| Define su propio `Problem`, `IdempotencyKey`, `CorrelationId` y `bearerAuth` | Referenciar `components/common.yaml` cuando Platform importe el módulo de contratos; hoy son equivalentes |
| `Problem.type` sin `pattern` | El común exige `urn:rdl:<servicio>:problem:<código>`; los tipos de Platform ya cumplen |
| Los problem types viven en la descripción | Registrarlos en `problems/platform.yaml` (incremento 8) |
| `identificationTypeCode` sin catálogo | `TODO(fiscal)` D9 |

## Decisiones

- Rutas sin `organizationId` (D7): Billing, fiscal y Receivables usan `/v1/<recurso>`; Platform mantiene
  `/v1/organizations/current/...`.
- Las rutas internas del BFF llevan el JWT del usuario: la tenancy es la misma que en las rutas públicas.
- ADR [0005](decisiones/0005-validacion-de-openapi.md): por qué el meta-schema oficial en lugar de `kin-openapi`.
