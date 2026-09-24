# Estado del proyecto · Contracts

**Última actualización:** 2026-09-24
**Punto de corte:** terminados los 9 incrementos del prompt P0. Versión **v0.1.0** lista en el `CHANGELOG`; **el tag
lo crea quien publique el repo** (en esta máquina no se usa git).

## Verificación al cierre

- `go build`, `go vet` y `golangci-lint` (0 issues) limpios; `go test ./...` en verde en 18 paquetes. Cobertura: dominio
  entre 90 y 100 %, casos de uso 83 %, `pkg/events` 88 %.
- `contractsctl validate`: OK, con 2 avisos (hallazgos del OpenAPI importado de Platform).
- `contractsctl lint`: OK. `contractsctl breaking -base .`: OK.
- Cada regla se probó también en negativo sobre copias del repo con errores sembrados.

## Definición de terminado (prompt P0)

- [x] Informe de inventario aprobado (ADR 0001) y decisiones D1–D13 con propuesta y responsable.
- [x] Glosario, convenciones, ownership y máquinas de estado (Invoice, ElectronicDocument, Receivable y Payment) en
  formato verificable.
- [x] AsyncAPI 3 con los 8 eventos, un JSON Schema por evento con el sobre común, e `InvoiceIssued` v1 completo con
  líneas, descuentos, impuestos y exoneraciones (con `TODO(fiscal)` donde falta la especificación).
- [x] OpenAPI con componentes comunes, Platform importado y esqueletos de Billing, fiscal, Receivables y rutas del BFF.
- [x] `contractsctl validate`, `lint` y `breaking` funcionando y en el pipeline.
- [x] `pkg/events` sin `float`, con tests de contrato struct ↔ schema. **Pendiente:** crear el tag `v0.1.0` al publicar.
- [x] `golangci-lint` limpio y cobertura alta en dominio y casos de uso.
- [x] Lista final de pendientes (abajo).

## Desviaciones del prompt (todas con ADR)

| Desviación | Por qué | ADR |
|---|---|---|
| OpenAPI y AsyncAPI se validan con sus meta-schemas oficiales, no con `kin-openapi` | Todo es OpenAPI 3.1 y el soporte de 3.1 en `kin-openapi` es parcial | 0005 |
| `breaking` compara contra un directorio base, no con `git show` | Sin dependencia de git; se prueba con `fstest.MapFS`. El pipeline arma la base con el último tag | 0006 |
| Sin `oasdiff` | Soporte parcial de 3.1; los OpenAPI todavía son esqueletos | 0006 |
| El OpenAPI de E-Invoice se llama `fiscal.yaml` | Decisión D4: `fiscal` en todo nombre de máquina | 0001 |
| Tag `v0.1.0` no creado | El usuario pidió no usar git | — |

## Pendientes para revisar en equipo

### Fiscal y Hacienda (sin especificación en el repo)

1. **Catálogos:** tipos de identificación (y formato del número por tipo), tipos y tarifas de impuesto, unidades de
   medida, condiciones de venta, medios de pago, tipos de documento de exoneración y tipos de comprobante. Las tablas
   `fiscal.*` existen pero están vacías. Hoy los códigos solo se validan por formato (`FiscalCode`).
2. **Redondeo (D2):** modo y paso de redondeo. Afecta las fórmulas de `docs/eventos/invoice-issued.md`.
3. **Fórmulas de totales:** subtotal después de descuentos, impuesto antes de exoneración y exoneración como porcentaje
   del impuesto. Es una propuesta que sigue las columnas de la base; si Hacienda define otra cosa, se cambia en `v2`.
4. **Transiciones del documento electrónico (C3):** son una propuesta; los 7 estados sí salen de la base.
5. Código de referencia de las notas, código de motivo de anulación, códigos de rechazo de Hacienda y composición de
   la clave numérica y el consecutivo.
6. ¿El comprobante exige provincia, cantón y distrito del cliente? Hoy el snapshot tiene la dirección en texto.
7. **D11:** relación sucursal ↔ establecimiento/terminal y alcance de los consecutivos. Sin valor por defecto
   (fiscal + contabilidad). Relacionado: ¿`branchId` debe ser obligatorio en los eventos?
8. Campos del perfil de contribuyente (`openapi/fiscal.yaml`).

### Reglas de negocio abiertas (equipo)

9. ¿Se pueden anular notas de crédito o débito? No hay evento para eso.
10. ¿Cuándo se limpia `requires_correction`? Propuesta: al emitir la nota que corrige.
11. Receivables: ¿el castigo (`write_off`) deja la cuenta en `paid` o en `cancelled`? ¿Qué pasa al anular una factura
    ya pagada o parcialmente pagada (¿saldo a favor?)? Nota de débito sobre una cuenta pagada: propuesta, reabrirla.
12. No hay evento de **pago anulado**: el BFF y reportes no se enteran por eventos.
13. ¿Un documento que pasa mucho tiempo en `error` o `contingency` emite algún evento?
14. Rangos del aging (propuesta 0-30, 31-60, 61-90, +90).
15. Eventos de Platform (`OrganizationCreated`, `MemberAdded`...): fuera de v1 (D8).
16. Campos que agregué y el documento no menciona: `total` y `currency` en `InvoiceCancelled`, y `sourceService` en el
    sobre (D3, aprobado).

### Para otros repos

17. **database-platform:** `receivables_app` no tiene `USAGE` en `core`, pero necesita revalidar la membresía.
    `billing.invoices.branch_id` y `billing.document_sequences.branch_id` no tienen FK hacia `core.branches`.
18. **Platform:** agregar `operationId` a `/healthz` y `/readyz`; cuando importe este módulo, referenciar
    `components/common.yaml` y `problems/platform.yaml`; su TODO de `identificationTypeCode` se resuelve con el
    punto 1. Ver `docs/openapi.md`.
19. **Billing / fiscal / Receivables:** confirmar los problem types propuestos en `problems/*.yaml` y completar los
    esqueletos OpenAPI al implementar.
20. **BFF:** la vista transversal y su read model se documentan en su repo (P7).

### Del propio repo

21. **D1:** la ruta del módulo `bitbucket.org/rdl/contracts` es provisional.
22. Verificación opcional de los grants reales de la base contra `ownership.yaml`, para el CI de cada API.
23. Diff de OpenAPI más fino (cuerpos y respuestas) cuando dejen de ser esqueletos.
24. Comprobar que los diagramas Mermaid se ven bien en Bitbucket (usan `{id}` y `⇒` en las etiquetas).

## Cómo retomar

```sh
go test ./... && go run ./cmd/contractsctl validate && go run ./cmd/contractsctl lint
```

Leer primero `docs/decisiones/0001-inventario-y-brechas.md` y este archivo.
