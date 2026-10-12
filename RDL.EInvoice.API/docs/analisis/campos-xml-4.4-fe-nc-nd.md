# Campos del XML 4.4 · Factura Electrónica, Nota de Crédito y Nota de Débito

Fuente única: **`ANEXOS_Y_ESTRUCTURAS_V4.4.pdf` (oficial, revisión con bitácora al 22/04/2026)**, Anexo 1.
Páginas = «Página N de 99». Condiciones leídas con `pdftotext -raw` (en `-layout` las columnas se corren).

Condición (pág. 16): **1** obligatorio · **2** condicional · **3** opcional · **4** inexistente.
Columnas: **FE / NC / ND** (se omiten FEE, FEC, TE y REP salvo que expliquen algo).
Tipos (pág. 15): String = largo máximo; Decimal «18,5» = 13 enteros + 5 decimales; «16,3» = 13 + 3.
`>` = un nivel de anidamiento (pie 2, pág. 19). **Los nombres exactos del elemento raíz y el namespace salen del XSD**
(Nota 2 solo da «Factura Electrónica», «Nota Crédito Electrónica», «Nota Debito Electrónica» y la URL; el Anexo 2,
pág. 84, usa el XPath `/FacturaElectronica/ds:Signature`). XSD: `https://cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/{facturaElectronica|notaCreditoElectronica|notaDebitoElectronica}` (Nota 2, pág. 65) — **no descargados**.

Diferencias globales FE vs NC/ND (resumen al final, §8).

---

## 1. Encabezado (págs. 18–25)

| Ruta | FE | NC | ND | Tipo / tamaño | Catálogo / regla | Pág. |
|---|---|---|---|---|---|---|
| `Clave` | 1 | 1 | 1 | String 50 fijo | Notas 1, 3 y 4.1. Números **y letras** para personas jurídicas | 18 |
| `ProveedorSistemas` | 1 | 1 | 1 | String 20 | Identificación del proveedor (formato de cédula del emisor); desarrollo propio → la del mismo obligado. Inscrito en RUT, si no → advertencia | 18 |
| `CodigoActividadEmisor` | 1 | 2 | 2 | String 6 | Código del padrón RUT; distinto al inscrito → advertencia; inexistente → rechazo | 18 |
| `CodigoActividadReceptor` | 2 | 2 | 2 | String 6 | Padrón RUT. Pie 1: en FE pasará a obligatorio cuando Hacienda lo comunique | 18 |
| `NumeroConsecutivo` | 1 | 1 | 1 | String 20 fijo, solo números | Notas 1 y 3; se verifica la secuencia | 19 |
| `FechaEmision` | 1 | 1 | 1 | DateTime RFC 3339 §5.6 (`YYYY-MM-DDThh:mi:ss[Z\|±hh:mm]`) | Ni posterior ni anterior a la generación (anterior solo en situación «sin internet»); **debe coincidir con la fecha de la clave** | 19 |
| `Emisor` | 1 | 1 | 1 | ComplexType | | 19 |
| `Emisor>Nombre` | 1 | 1 | 1 | String 100 (mín. 5) | | 19 |
| `Emisor>Identificacion` | 1 | 1 | 1 | ComplexType | | 19 |
| `Emisor>Identificacion>Tipo` | 1 | 1 | 1 | String 2 fijo | Nota 4. 05 y 06 solo en FEC | 19 |
| `Emisor>Identificacion>Numero` | 1 | 1 | 1 | String 20 | Física 9 díg. sin cero inicial; jurídica 10 **caracteres**; DIMEX 11–12 díg.; NITE 10 díg.; sin guiones. Debe estar inscrito | 20 |
| `Emisor>Registrofiscal8707` | 2 | 2 | 2 | String 12 | Obligatorio con CABYS de bebidas alcohólicas (Ley 8707); si falta → advertencia | 20 |
| `Emisor>NombreComercial` | 3 | 3 | 3 | String 80 (mín. 3) | | 21 |
| `Emisor>Ubicacion` | 1 | 2 | 2 | ComplexType | | 21 |
| `Emisor>Ubicacion>Provincia` | 1 | 1 | 1 | String 1 | Notas 14 y 7 (catálogo externo); distinto al RUT → advertencia | 21 |
| `Emisor>Ubicacion>Canton` | 1 | 1 | 1 | String 2 | Notas 14 y 7 | 21 |
| `Emisor>Ubicacion>Distrito` | 1 | 1 | 1 | String 2 | Notas 14 y 7 | 21 |
| `Emisor>Ubicacion>Barrio` | 3 | 2 | 2 | String 50 (mín. 5) | Texto libre | 21 |
| `Emisor>Ubicacion>OtrasSenas` | 1 | 1 | 1 | String 250 (mín. 5) | | 21 |
| `Emisor>OtrasSenasExtranjero` | 4 | 4 | 4 | String 300 | Solo FEC con tipo 05 | 22 |
| `Emisor>Telefono` | 3 | 3 | 3 | ComplexType | | 22 |
| `Emisor>Telefono>CodigoPais` | 1 | 1 | 1 | Integer 3 | | 22 |
| `Emisor>Telefono>NumTelefono` | 1 | 1 | 1 | Integer 20 (mín. 8, salvo especiales como 911) | | 22 |
| `Emisor>CorreoElectronico {1,4}` | 1 | 2 | 2 | String 160 | Regex de la pág. 22; formato inválido → rechazo | 22 |
| `Receptor` | **1** | 2 | 2 | ComplexType | (TE = 2) | 22 |
| `Receptor>Nombre` | 1 | 1 | 1 | String 100 (mín. 3) | | 22 |
| `Receptor>Identificacion` | 1 | 2 | 2 | ComplexType | (TE = 2) | 22 |
| `Receptor>Identificacion>Tipo` | 1 | 1 | 1 | String 2 fijo | Nota 4. 05 en FE solo con condición de venta 12 + referencia al documento de respaldo | 22 |
| `Receptor>Identificacion>Numero` | 1 | 1 | 1 | String 20 | Formatos de la pág. 23. En FE se verifica contra padrones. **En NC y ND deben coincidir con el comprobante original** | 23 |
| `Receptor>NombreComercial` | 3 | 3 | 3 | String 80 | | 23 |
| `Receptor>Ubicacion` | 2 | 2 | 2 | ComplexType | Si tiene domicilio en el país; no aplica a tipo 05 | 23 |
| `Receptor>Ubicacion>Provincia/Canton/Distrito` | 1 | 1 | 1 | String 1 / 2 / 2 | Nota 14 | 23–24 |
| `Receptor>Ubicacion>Barrio` | 3 | 2 | 2 | String 50 | | 24 |
| `Receptor>Ubicacion>OtrasSenas` | 1 | 1 | 1 | String 160 (mín. 5) | | 24 |
| `Receptor>OtrasSenasExtranjero` | 2 | 2 | 2 | String 300 | En FE solo con tipo 05 | 24 |
| `Receptor>Telefono` (+ `CodigoPais`, `NumTelefono`) | 3 (1, 1) | 3 | 3 | igual que emisor | | 24 |
| `Receptor>CorreoElectronico` | 2 | 2 | 2 | String 160 | «cuando el cliente lo requiera» | 24 |
| `CondicionVenta` | 1 | 1 | 1 | String 2 fijo | Notas 5 y 7. 12 solo en FE; 09 y 11 solo en REP | 24 |
| `CondicionVentaOtros` | 2 | 2 | 2 | String 100 (mín. 5) | Obligatorio si `CondicionVenta` = 99 | 25 |
| `PlazoCredito` | 2 | 2 | 2 | Integer 5, **días**, > 0 | Obligatorio con condición 02 (crédito) o 10 | 25 |

> Ya **no existe** `MedioPago` en el encabezado: se trasladó al resumen (bitácora ítem 36, pág. 5). Se eliminó `Fax` (ítem 18).

## 2. Detalle (`DetalleServicio`, págs. 26–47)

| Ruta (bajo `DetalleServicio`) | FE | NC | ND | Tipo / tamaño | Catálogo / regla | Pág. |
|---|---|---|---|---|---|---|
| `DetalleServicio` | 2 | 2 | 2 | ComplexType | Obligatorio salvo cuando solo hay `OtrosCargos` con tipo 04, 08, 09 o 10 de la Nota 16 | 26 |
| `>LineaDetalle {1,1000}` | 1 | 1 | 1 | ComplexType | | 26 |
| `>>NumeroLinea` | 1 | 1 | 1 | PositiveInteger 1–1000 | | 26 |
| `>>PartidaArancelaria` | 4 | 2 | 2 | String 12 | Solo FEE (y NC/ND que modifiquen FEE) | 26 |
| `>>CodigoCABYS` | **1** | 2 | 2 | String 13 | Nota 17 (CABYS del BCCR). En NC/ND obligatorio si el original tiene CABYS; no si la referencia es anterior a 01/12/2020 o es NC/ND **financiera** (Nota 9 códigos 09 y 10). Con exoneración, el código debe estar en la autorización | 26 |
| `>>CodigoComercial {0,5}` | 2 | 2 | 2 | ComplexType | Obligatorio con CABYS de surtidos | 27 |
| `>>>Tipo` | 1 | 1 | 1 | String 2 fijo | Nota 12 (03 para surtidos) | 27 |
| `>>>Codigo` | 1 | 1 | 1 | String 20 | SKU/GTIN para surtidos | 27 |
| `>>Cantidad` | 1 | 1 | 1 | Decimal 16,3, > 0 | | 27 |
| `>>UnidadMedida` | 1 | 1 | 1 | String 15 | Nota 15 (sensible a mayúsculas) | 27 |
| `>>TipoTransaccion` | 2 | 2 | 2 | String 2 | Nota 22; obligatorio para tratamientos tributarios específicos (pie 5) | 28 |
| `>>UnidadMedidaComercial` | 3 | 3 | 3 | String 20 | | 28 |
| `>>Detalle` | 1 | 1 | 1 | String 200 (mín. 3) | | 28 |
| `>>NumeroVINoSerie {1,1000}` | 2 | 2 | 2 | String 17 | CABYS de vehículos/aeronaves/embarcaciones → si falta, advertencia | 28 |
| `>>RegistroMedicamento` | 2 | 2 | 2 | String 100 | CABYS de medicamentos con registro sanitario | 28 |
| `>>FormaFarmaceutica` | 2 | 2 | 2 | String 3 | Nota 19 (catálogo externo) | 29 |
| `>>DetalleSurtido` (+ `LineaDetalleSurtido {1,20}` y sus ~25 campos `…Surtido`) | 2 | 2 | 2 | ComplexType | Solo fabricantes/importadores con surtidos de distintas tarifas (pie 8). **Fuera de V1** | 29–35 |
| `>>PrecioUnitario` | 1 | 1 | 1 | Decimal 18,5, > 0 | (salvo surtidos con regalías) | 35 |
| `>>MontoTotal` | 1 | 1 | 1 | Decimal 18,5 | = Cantidad × PrecioUnitario | 35 |
| `>>Descuento {0,5}` | 3 | 2 | 2 | ComplexType | Cada descuento sobre la base menos el anterior. No requerido en NC/ND **financieras** (Nota 9 códigos 09/10) | 35 |
| `>>>MontoDescuento` | 1 | 1 | 1 | Decimal 18,5, ≤ MontoTotal | | 35–36 |
| `>>>CodigoDescuento` | 1 | 1 | 1 | String 2 | **Nota 20** | 36 |
| `>>>CodigoDescuentoOTRO` | 2 | 2 | 2 | String 100 (mín. 5) | Si código 99 | 36 |
| `>>>NaturalezaDescuento` | 2 | 2 | 2 | String 80 (mín. 3) | Si código 99 | 36 |
| `>>SubTotal` | 1 | 1 | 1 | Decimal 18,5 | = MontoTotal − descuentos | 36 |
| `>>IVACobradoFabrica` | 2 | 2 | 2 | String 2 | Nota 21. Con 02 → impuesto con tarifa 10 (exenta) | 36–37 |
| `>>BaseImponible` | 1 | 1 | 1 | Decimal 18,5 | Obligatorio si está gravado: SubTotal + impuestos 02, 04, 05 y 12 cuando correspondan. Editable con fábrica 01 o impuesto 07 | 37 |
| `>>Impuesto {1,1000}` | 1 | 1 | 1 | ComplexType | Obligatorio en todos los comprobantes | 37 |
| `>>>Codigo` | 1 | 1 | 1 | String 2 | Nota 8. Regalías/bonificaciones (desc. 01/03) → IVA sobre MontoTotal; 07 → base imponible editada; 08 → factor × subtotal | 37 |
| `>>>CodigoImpuestoOTRO` | 2 | 2 | 2 | String 100 | Si código 99 | 38 |
| `>>>CodigoTarifaIVA` | 2 | 2 | 2 | String 2 | **Nota 8.1**; obligatorio con impuesto 01 y 07 | 38 |
| `>>>Tarifa` | 2 | 2 | 2 | Decimal 4,2 (13 = 13 %, 0.5 = 0,5 %) | Con 01 debe coincidir con el código de tarifa; cemento 5 % | 38–39 |
| `>>>FactorCalculoIVA` | 2 | 2 | 2 | Decimal 5,4 | Obligatorio con impuesto 08 (bienes usados) | 39 |
| `>>>DatosImpuestoEspecifico` (+ `CantidadUnidadMedida` 7,2 · `Porcentaje` 4,2 · `Proporcion` 10,5 · `VolumenUnidadConsumo` 7,2 · `ImpuestoUnidad` 18,5) | 2 | 2 | 2 | ComplexType | Impuestos 03, 04, 05, 06. **Fuera de V1** | 39–42 |
| `>>>Monto` | 1 | 1 | 1 | Decimal 18,5 | IVA = Tarifa × BaseImponible; tarifa 10 → «0»; fórmulas por impuesto en págs. 42–44 | 42 |
| `>>>MontoExportacion` | 4 | 2 | 2 | Decimal 18,5 | Solo exportación | 44 |
| `>>>Exoneracion` | 2 | 2 | 2 | ComplexType | Obligatorio si la venta tiene exoneración | 44 |
| `>>>>TipoDocumentoEX1` | 1 | 1 | 1 | String 2 | Nota 10.1 | 44 |
| `>>>>TipoDocumentoOTRO` | 2 | 2 | 2 | String 100 | Si 99 | 44 |
| `>>>>NumeroDocumento` | 1 | 1 | 1 | String 40 (mín. 3) | Tal como el documento, con guiones; con 02, 03, 08 → número de ley; con 04 y 11 se verifica vigencia | 44–45 |
| `>>>>Articulo` | 2 | 2 | 2 | Integer 6 | Obligatorio con tipo 02, 03, 06, 07, 08 | 45 |
| `>>>>Inciso` | 2 | 2 | 2 | Integer 6 | Si el artículo tiene inciso | 45 |
| `>>>>NombreInstitucion` | 1 | 1 | 1 | **String 2 fijo** | **Nota 23** | 45 |
| `>>>>NombreInstitucionOtros` | 2 | 2 | 2 | String 160 (mín. 5) | Si 99 | 45 |
| `>>>>FechaEmisionEX` | 1 | 1 | 1 | DateTime RFC 3339 | Con 04 y 11 se verifica contra el documento | 45 |
| `>>>>TarifaExonerada` | 1 | 1 | 1 | Decimal 4,2, **en puntos de tarifa** | Con 04 y 11, mayor a la autorizada → rechazo | 45–46 |
| `>>>>MontoExoneracion` | 1 | 1 | 1 | Decimal 18,5 | = TarifaExonerada × SubTotal (o BaseImponible); ≤ Monto del impuesto | 46 |
| `>>ImpuestoAsumidoEmisorFabrica` | 1 | 2 | 2 | Decimal 18,5, ≥ 0 | Suma de impuestos de regalías/bonificaciones, IVA a fábrica e impuestos específicos a fábrica; «0» si no aplica | 46–47 |
| `>>ImpuestoNeto` | 1 | 1 | 1 | Decimal 18,5 | = Monto − MontoExoneracion (o − ImpuestoAsumidoEmisorFabrica) | 47 |
| `>>MontoTotalLinea` | 1 | 1 | 1 | Decimal 18,5 | = **SubTotal + ImpuestoNeto** | 47 |

## 3. Otros cargos (págs. 47–49)

| Ruta | FE | NC | ND | Tipo / tamaño | Catálogo / regla | Pág. |
|---|---|---|---|---|---|---|
| `OtrosCargos {0,15}` | 2 | 2 | 2 | ComplexType | | 47 |
| `>TipoDocumentoOC` | 1 | 1 | 1 | String 2 | Nota 16 | 47 |
| `>TipoDocumentoOTROS` | 2 | 2 | 2 | String 100 | Si 99 | 48 |
| `>IdentificacionTercero` (`>>Tipo` String 2 Nota 4, `>>Numero` String 20) | 2 | 2 | 2 | ComplexType | Obligatorio con tipo 04 «Cobro de un tercero» | 48 |
| `>NombreTercero` | 2 | 2 | 2 | String 100 | Con tipo 04 | 49 |
| `>Detalle` | 1 | 1 | 1 | String 160 | | 49 |
| `>PorcentajeOC` | 2 | 2 | 2 | Decimal 9,5 | | 49 |
| `>MontoCargo` | 1 | 1 | 1 | Decimal 18,5, > 0 | | 49 |

## 4. Resumen (`ResumenFactura`, págs. 50–56)

| Ruta | FE | NC | ND | Tipo / tamaño | Regla | Pág. |
|---|---|---|---|---|---|---|
| `ResumenFactura` | 1 | 1 | 1 | ComplexType | | 50 |
| `>CodigoTipoMoneda` | 1 | 1 | 1 | ComplexType | Obligatorio en todos | 50 |
| `>>CodigoMoneda` | 1 | 1 | 1 | String 3 | Notas 13 y 13.1 («Codigodemoneda_V4.4», externo) | 50 |
| `>>TipoCambio` | 1 | 1 | 1 | Decimal 18,5 | CRC = 1; USD = tipo de cambio de **venta** de referencia BCCR; otras monedas respecto a colones, > 0 | 50 |
| `>TotalServGravados` | 2 | 2 | 2 | Decimal 18,5 | Servicios (CABYS inicia 5–9) gravados; con exoneración × (1 − %exon), %exon = Σ exonerado / Σ impuesto | 50 |
| `>TotalServExentos` | 2 | 2 | 2 | Decimal 18,5 | Servicios con tarifa **10** | 51 |
| `>TotalServExonerado` | 2 | 2 | 2 | Decimal 18,5 | %exon × monto total del servicio | 51 |
| `>TotalServNoSujeto` | 2 | 2 | 2 | Decimal 18,5 | Servicios con tarifa **01 u 11** | 51 |
| `>TotalMercanciasGravadas` | 2 | 2 | 2 | Decimal 18,5 | Mercancías (CABYS 0–4) gravadas, sin las de IVA a fábrica 02 | 51–52 |
| `>TotalMercanciasExentas` | 2 | 2 | 2 | Decimal 18,5 | Tarifa 10 | 52 |
| `>TotalMercExonerada` | 2 | 2 | 2 | Decimal 18,5 | | 52 |
| `>TotalMercNoSujeta` | 2 | 2 | 2 | Decimal 18,5 | Tarifas 01 y 11 | 52 |
| `>TotalGravado` | 2 | 2 | 2 | Decimal 18,5 | ServGravados + MercanciasGravadas | 53 |
| `>TotalExento` | 2 | 2 | 2 | Decimal 18,5 | ServExentos + MercanciasExentas | 53 |
| `>TotalExonerado` | 2 | 2 | 2 | Decimal 18,5 | ServExonerado + MercExonerada | 53 |
| `>TotalNoSujeto` | 2 | 2 | 2 | Decimal 18,5 | ServNoSujeto + MercNoSujeta | 53 |
| `>TotalVenta` | 1 | 1 | 1 | Decimal 18,5 | Gravado + Exento + Exonerado + NoSujeto | 53 |
| `>TotalDescuentos` | 2 | 2 | 2 | Decimal 18,5 | Σ MontoDescuento | 53 |
| `>TotalVentaNeta` | 1 | 1 | 1 | Decimal 18,5 | TotalVenta − TotalDescuentos | 53 |
| `>TotalDesgloseImpuesto {1,1000}` | 2 | 2 | 2 | ComplexType | Una entrada por código de impuesto de las líneas; impuesto asumido/a fábrica → tarifa y monto 0 | 53–54 |
| `>>Codigo` | 1 | 1 | 1 | String 2 | Nota 8 | 54 |
| `>>CodigoTarifaIVA` | 2 | 2 | 2 | String 2 | Nota 8.1; con 01 y 07; exoneración en surtidos → 08 | 54 |
| `>>TotalMontoImpuesto` | 1 | 1 | 1 | Decimal 18,5 | Σ por código | 54 |
| `>TotalImpuesto` | 2 | 2 | 2 | Decimal 18,5 | Σ Monto (líneas sin exoneración) + Σ ImpuestoNeto (con exoneración o asumido) | 54 |
| `>TotalImpAsumEmisorFabrica` | 2 | 2 | 2 | Decimal 18,5 | Σ ImpuestoAsumidoEmisorFabrica | 54–55 |
| `>TotalIVADevuelto` | 2 | 2 | 2 | Decimal 18,5 | Solo servicios de salud pagados con tarjeta (fórmula y pie 16) | 55 |
| `>TotalOtrosCargos` | 2 | 2 | 2 | Decimal 18,5 | Σ MontoCargo | 55 |
| `>MedioPago {1,4}` | 2 | 2 | 2 | ComplexType | Obligatorio salvo condición de venta **02, 08 y 10** (créditos) | 55 |
| `>>TipoMedioPago` | 2 | 2 | 2 | String 2 | Notas 6 y 7 | 56 |
| `>>MedioPagoOtros` | 2 | 2 | 2 | String 100 (mín. 3) | Si 99 | 56 |
| `>>TotalMedioPago` | 2 | 2 | 2 | Decimal 18,5, > 0 | Obligatorio con más de un medio | 56 |
| `>TotalComprobante` | 1 | 1 | 1 | Decimal 18,5 | TotalVentaNeta + TotalImpuesto + TotalOtrosCargos − TotalIVADevuelto; con ≥ 2 medios debe igualar Σ TotalMedioPago | 56 |

## 5. Información de referencia (págs. 57–58)

Máximo 10 repeticiones. Usos (pág. 57): NC que elimina un documento; ND que elimina una NC; NC/ND que corrigen
montos; FE por contingencia que sustituye un comprobante provisional; cualquier documento relacionado.

| Ruta | FE | NC | ND | Tipo / tamaño | Regla | Pág. |
|---|---|---|---|---|---|---|
| `InformacionReferencia {0,10}` | 2 | **1** | **1** | ComplexType | En FE obligatorio solo con exoneración tipo 11 (Nota 10.1); obligatorio en FEC, NC, ND y REP | 57 |
| `>TipoDocIR` | 1 | 1 | 1 | String 2 | Notas 10 y 7. 16 solo en FEC | 57 |
| `>TipoDocRefOTRO` | 2 | 2 | 2 | String 100 (mín. 5) | Si 99 | 57 |
| `>Numero` | 2 | 2 | 2 | String 50 | Obligatorio con TipoDocIR (salvo 13). Con TipoDocIR 1, 2, 3, 4, 9, 11, 12, 15, 17, 18, 19, 20 debe ser **clave válida** y los datos deben corresponder al comprobante referido (encabezado, tipo de cambio, CABYS). **En NC: Σ ajustes ≤ monto del comprobante + Σ ND sobre él** | 58 |
| `>FechaEmisionIR` | 1 | 1 | 1 | DateTime | ≤ 10 años de antigüedad | 58 |
| `>Codigo` | 2 | 2 | 2 | String 2 | Notas 9 y 7; obligatorio con TipoDocIR (salvo 13) | 58 |
| `>CodigoReferenciaOTRO` | 2 | 2 | 2 | String 100 (mín. 5) | Si 99 | 58 |
| `>Razon` | 2 | 2 | 2 | String 180 (mín. 3) | Obligatorio con TipoDocIR (salvo 13) | 58 |

## 6. Otros y firma (págs. 59–60)

| Ruta | FE | NC | ND | Tipo / tamaño | Regla | Pág. |
|---|---|---|---|---|---|---|
| `Otros` | 3 | 3 | 3 | ComplexType | Uso comercial, no tributario | 59 |
| `>OtroTexto {0,}` / `>OtroContenido {0,}` | 3 | 3 | 3 | String 500 (atributo `codigo` permitido) | | 59 |
| `ds:Signature` | 1 | 1 | 1 | `ds:SignatureType` | XAdES-EPES v1.3.2+, enveloped (Anexo 2) | 60, 84 |

## 7. Catálogos citados (Anexo 1, Notas)

| Nota | Nombre | Códigos (oficial) | Pág. |
|---|---|---|---|
| 1 | Representación gráfica | Tipo de documento, clave y consecutivo juntos | 65 |
| 2 | Etiqueta y XSD por tipo | URLs del CDN (FE, ND, NC, TE, FEC, FEE, REP) — **archivos no incluidos** | 65 |
| 3 | Consecutivo (20) y clave (50) | Consecutivo: establecimiento 3 (001 casa matriz) + terminal 5 («00001» si hay una sola o servidor centralizado) + tipo 2 + número 10, «iniciando en 1 para cada sucursal o terminal según sea el caso»; al tope se reinicia en 1. Tipos: 01 FE, 02 ND, 03 NC, 04 TE, 05/06/07 confirmaciones (aceptación/parcial/rechazo), 08 FEC, 09 FEE, 10 REP. Clave: 506 + día(2) + mes(2) + año(2) + cédula 12 caracteres (pos. 10–21) + consecutivo 20 (pos. 22–41) + situación 1 (pos. 42: 1 Normal, 2 Contingencia, 3 Sin internet) + seguridad 8 (pos. 43–50) generado por el sistema del obligado. Cambio de plataforma: **mantener** la numeración. Archivos `clave.xml`, `clave_respuesta.xml`, `clave.pdf` | 65–69 |
| 4 | Tipo de identificación | 01 Física, 02 Jurídica, 03 DIMEX, 04 NITE, 05 Extranjero No Domiciliado, 06 No Contribuyente | 69 |
| 4.1 | Relleno a 12 para la clave | Física: +3 ceros; Jurídica: +2 ceros (12 **caracteres**); DIMEX de 11: +1 cero; NITE: +2 ceros. (Sin regla para 05/06) | 69 |
| 5 | Condición de venta | 01 Contado, 02 Crédito, 03 Consignación, 04 Apartado, 05 Arrend. con opción de compra, 06 Arrend. en función financiera, 07 Cobro a favor de un tercero, 08 Servicios prestados al Estado, 09 Pago de servicios prestados al Estado (solo REP), 10 Venta a crédito en IVA hasta 90 días (art. 27 LIVA), 11 Pago de venta a crédito IVA 90 días (solo REP), 12 Venta mercancía no nacionalizada (solo FE), 13 Venta bienes usados no contribuyente (solo FEC), 14 Arrend. operativo, 15 Arrend. financiero, 99 Otros | 70 |
| 6 | Medio de pago | 01 Efectivo, 02 Tarjeta, 03 Cheque, 04 Transferencia/depósito, 05 Recaudado por terceros (el pie dice «06»: errata), 06 SINPE Móvil, 07 Plataforma digital, 99 Otros | 71 |
| 7 | Visualización | Se imprime la descripción, no el código | 71 |
| 8 | Código de impuesto | 01 IVA, 02 Selectivo de Consumo, 03 Único a los Combustibles, 04 Bebidas Alcohólicas, 05 Bebidas envasadas sin alcohol y jabones, 06 Productos de Tabaco, 07 IVA (cálculo especial), 08 IVA Régimen de Bienes Usados (Factor), 12 Cemento, 99 Otros | 71 |
| 8.1 | Tarifa de IVA | 01 Tarifa 0 % (art. 32 num. 1 RLIVA), 02 reducida 1 %, 03 reducida 2 %, 04 reducida 4 %, 05 Transitorio 0 % (solo NC/ND), 06 Transitorio 4 % (solo NC/ND), 07 transitoria 8 % (solo NC/ND), 08 general 13 %, 09 reducida 0,5 %, 10 Exenta (Ley 9635 art. 8), 11 0 % sin derecho a crédito | 72 |
| 9 | Código de referencia | 01 Anula documento de referencia, 02 Corrige monto, (no hay 03), 04 Referencia a otro documento, 05 Sustituye comprobante provisional por contingencia, 06 Devolución de mercancía, 07 Sustituye comprobante electrónico, 08 Factura endosada, 09 NC financiera, 10 ND financiera, 11 Proveedor no domiciliado, 12 NC financiera por exoneración posterior, 13 Anula por error material, 14 Corrige monto por error material, 15 Sustituye por error material, 16 **Sustituye comprobante electrónico rechazado**, 17 Pago a comprobante (solo REP), 99 Otros. Tratamiento contable: 01/02/06 en el período de la nota; 13/14 en el período del comprobante modificado; tras un 01 el nuevo comprobante lleva 07, tras un 13 lleva 15 | 73 |
| 10 | Tipo de documento de referencia | 01 FE, 02 ND, 03 NC, 04 TE, 05 Nota de despacho, 06 Contrato, 07 Procedimiento, 08 Comprobante emitido en contingencia, 09 Devolución mercadería (solo NC/ND), 10 **Comprobante electrónico rechazado por el MH**, 11 Sustituye factura rechazada por el receptor, 12 Sustituye factura de exportación, 13 Facturación mes vencido, 14 Comprobante de régimen especial, 15 Sustituye una FEC, 16 Proveedor no domiciliado (solo FEC), 17 NC a FEC, 18 ND a FEC, 19 FEE, 20 REP, 99 Otros | 74 |
| 10.1 | Tipo de documento de exoneración | 01 Compras autorizadas DGT (solo NC/ND), 02 Ventas exentas a diplomáticos, 03 Autorizado por ley especial, 04 DGH Autorización local genérica, 05 Transitorio V (solo NC/ND), 06 ICT (solo NC/ND), 07 Transitorio XVII (solo NC/ND), 08 Zona Franca, 09 Servicios complementarios exportación art. 11 RLIVA, 10 Órgano de corporaciones municipales, 11 DGH Autorización de impuesto local concreta, 99 Otros | 75 |
| 11 | Mensaje | 1 Aceptado, 2 Aceptación parcial, 3 Rechazado | 76 |
| 12 | Tipo de código comercial | 01 Vendedor, 02 Comprador, 03 Fabricante/industrial/importador, 04 Uso interno, 99 Otros | 76 |
| 13 / 13.1 | Moneda | **Remite al documento externo «Codigodemoneda_V4.4»** (sin enlace) | 76 |
| 14 | Provincia, cantón, distrito | **Remite a «Codificacionubicacion_V4.4»** (sin enlace) | 76 |
| 15 | Unidad de medida | ~129 símbolos en la tabla (págs. 77–80), p. ej. `Unid` Unidad, `Sp` Servicios profesionales, `Spe` Servicios personales, `St` Servicios técnicos, `Os` Otro tipo de servicio, `Al`/`Alc` alquiler, `Cm` Comisiones, `I` Intereses, `Kg`, `L`, `m²`, `h` hora, `Otros`. Sensible a mayúsculas | 77–80 |
| 16 | Tipo de otros cargos | 01 Contribución parafiscal, 02 Timbre Cruz Roja, 03 Timbre Bomberos, 04 Cobro de un tercero, 05 Costos de exportación, 06 Impuesto de servicio 10 %, 07 Timbre de colegios profesionales, 08 Depósitos de garantía, 09 Multas o penalizaciones, 10 Intereses moratorios, 99 Otros cargos | 80 |
| 17 | CABYS | **Remite al catálogo del BCCR** | 80 |
| 18 | Condición del IVA (mensaje receptor) | 01–05 | 81 |
| 19 | Forma farmacéutica | **Remite a «Código de forma Farmacéutica»** | 81 |
| 20 | Código de descuento | 01 Regalía, 02 Regalía o bonificación IVA cobrado al cliente, 03 Bonificación, 04 Volumen, 05 Temporada, 06 Promocional, 07 Comercial, 08 Frecuencia, 09 Sostenido, 99 Otros | 81 |
| 21 | IVA cobrado a fábrica | 01 Venta con IVA a nivel de fábrica, 02 Venta exenta (ya soportó el impuesto) | 82 |
| 22 | Tipo de transacción | 01 Venta normal, 02 Mercancía autoconsumo exento, 03 … gravado, 04 Servicio autoconsumo exento, 05 … gravado, 06 Cuota de afiliación, 07 … exenta, 08 Bienes de capital emisor, 09 … receptor, 10 … ambos, 11 Bienes de capital autoconsumo exento emisor, 12 … sin contraprestación exento emisor, 13 Sin contraprestación a terceros | 82 |
| 23 | Institución de la exoneración | 01 Ministerio de Hacienda, 02 RREE y Culto, 03 MAG, 04 MEIC, 05 Cruz Roja, 06 Bomberos, 07 Obras del Espíritu Santo, 08 Fecrunapa, 09 EARTH, 10 INCAE, 11 JPS, 12 Aresep, 99 Otros | 83 |

**Redondeo y formato numérico (pág. 14):** decimales con punto, sin separador de miles. Redondeo a 5 decimales
mirando el sexto: `20.203512 → 20.20351`, `20.203518 → 20.20352` (mitad hacia arriba). Cantidades 16,3; montos 18,5.
Histórico (bitácora 4.2, pág. 12): se admiten márgenes para sistemas que redondean a 2 decimales. Caracteres `&` y `<`
se escapan (pág. 14).

**Anulación, sustitución y límites de fecha:**
- Anular un comprobante válido: NC con `Codigo` **01** «Anula documento de referencia» (o **13** por error material) y
  `TipoDocIR` del original (01 FE). Si se emite uno nuevo, lleva `Codigo` **07** (o **15**). (Nota 9, pág. 73; Anexo 1 pág. 14)
- Sustituir un **rechazado por Hacienda**: comprobante nuevo con `TipoDocIR` **10** y `Codigo` **16** «Sustituye
  comprobante electrónico rechazado» (Notas 9 y 10, págs. 73–74). Sin NC (art. 9, borrador).
- **Límite de fecha**: el pie 28 del borrador (Nota 10, pág. 79 física) limitaba el código 10 a «mismo mes o no más de
  5 días de emitido al mes siguiente». **El oficial no trae ese pie.** El único límite oficial es `FechaEmisionIR` ≤ 10
  años (pág. 58). La «nota 28» del prompt P5 es ese pie del borrador.
- NC/ND **financieras** (Nota 9 códigos 09 y 10): sin CABYS ni nodo de descuento obligatorios (págs. 26, 35).

## 8. Diferencias FE vs NC vs ND (oficial)

| Elemento | FE | NC | ND |
|---|---|---|---|
| `InformacionReferencia` | 2 (solo con exoneración tipo 11) | **1** | **1** |
| `Receptor` y `Receptor>Identificacion` | **1** | 2 (si el original lo tiene, debe coincidir) | 2 |
| `CodigoActividadEmisor` | 1 | 2 | 2 |
| `Emisor>Ubicacion`, `Emisor>CorreoElectronico` | 1 | 2 | 2 |
| `Barrio` (emisor y receptor) | 3 | 2 | 2 |
| `CodigoCABYS` | 1 | 2 (obligatorio si el original tenía; no en financieras) | 2 |
| `PartidaArancelaria`, `MontoExportacion` | 4 | 2 (si modifica una FEE) | 2 |
| `Descuento` | 3 | 2 (no en financieras) | 2 |
| `ImpuestoAsumidoEmisorFabrica` | 1 | 2 | 2 |
| `CodigoMoneda` y `TipoCambio` | 1 | 1 (bitácora 108–109: obligatorios en NC/ND) | 1 |
| Tarifas 05, 06, 07 (Nota 8.1) y exoneraciones 01, 05, 06, 07 (Nota 10.1), devolución de mercadería (Nota 10 código 09) | no | solo NC/ND | solo NC/ND |
| Tope de monto | — | Σ NC ≤ comprobante + Σ ND (pág. 58) | — |
| Tipo en el consecutivo | 01 | 03 | 02 |
| XSD | `facturaElectronica` | `notaCreditoElectronica` | `notaDebitoElectronica` |
