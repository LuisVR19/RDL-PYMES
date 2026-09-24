# 0004 · Productos: montos exactos, módulo de contratos y PATCH parcial

- **Fecha:** 2026-09-24 · **Estado:** Aceptado (el punto 3 es una propuesta al repo de contratos)

## 1. Montos: los tipos del contrato, como texto de punta a punta

El precio del producto es el primer monto de Billing. Se usan los tipos de `bitbucket.org/rdl/contracts/pkg/events/money`
(`Amount`, `Currency`), reexportados desde `internal/domain/money`:

- son strings decimales validados con los mismos patrones que `schemas/common/*.json` y los dominios `shared.*`
  (13 enteros, 5 decimales, sin signo); un JSON `number` se rechaza (400);
- hacia la base viajan como texto (`::text::numeric` al escribir, `::text` al leer): nunca pasan por `float` ni por un
  tipo numérico del driver. La prueba de integración hace ida y vuelta con `9999999999999.99999`;
- `money.CanonicalAmount` quita los ceros decimales a la derecha: la base devuelve 5 decimales (`1300.50000`) y el
  cliente puede mandar cualquier cantidad. Con una sola forma, un PATCH con el mismo precio no es un cambio, el hash
  de idempotencia no depende de cómo se escribió y la API responde siempre `1300.5`.

Estos tipos **no hacen aritmética**. La librería decimal para el cálculo (`shopspring/decimal` o `cockroachdb/apd`) se
elige en el incremento 5 junto con la regla de redondeo (D2, pendiente); ese ADR la justifica.

Dos tests lo vigilan: `internal/archtest` falla si aparece `float32`/`float64` en el código de producción o
`real`/`double precision`/`float` en el SQL, y por reflexión si un struct del dominio o un DTO contiene un float.

## 2. Módulo de contratos con `replace`

`go.mod` requiere `bitbucket.org/rdl/contracts v0.2.0` con `replace => ../RDL.Contracts`: el módulo no tiene tag
publicado. Consecuencias:

- `make docker` construye con la carpeta padre como contexto; `Dockerfile.dockerignore` deja entrar solo
  `RDL.Billing.API/` y `RDL.Contracts/` y excluye `.env`, llaves y binarios.
- **TODO(contracts):** publicar el tag `v0.2.0` y quitar el `replace` y el contexto especial del Dockerfile.

## 3. `PATCH /v1/products/{id}` acepta cambios parciales

El contrato define el cuerpo de este PATCH como `ProductInput` (todos los campos obligatorios): para desactivar un
producto habría que reenviarlo entero. El prompt pide "edita o desactiva", y clientes ya usa `CustomerPatch` parcial.

Billing acepta un cuerpo parcial (campo ausente = no cambia; `taxes` presente reemplaza la lista, `[]` la vacía). Todo
`ProductInput` válido sigue funcionando igual, así que ningún cliente del contrato se rompe.
**Propuesta de PR al contrato:** agregar `ProductPatch` (todos opcionales, como `CustomerPatch`) y usarlo en este PATCH.

## 4. Impuestos del producto

Solo códigos (`taxTypeCode`, `taxRateCode`), validados con el patrón `FiscalCode` del contrato, a lo sumo uno por
tipo (unicidad de la base) y ordenados por tipo. La tasa no se guarda en el producto: se resolverá al armar la línea
desde `fiscal.tax_rates` (informe 0001 §4.2, pendiente de aprobación; el catálogo sigue vacío). `TODO(fiscal)`:
validar CABYS, unidades y tipos/tarifas contra sus catálogos cuando fiscal los cargue.
