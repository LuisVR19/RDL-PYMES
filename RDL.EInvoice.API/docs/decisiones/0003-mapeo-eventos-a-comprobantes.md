# 0003 · Mapeo de los eventos de Billing al XML 4.4 (FE, NC y ND)

- **Fecha:** 2026-10-12
- **Estado:** Aprobado el 2026-10-12.
- **Fuente fiscal:** Anexo 1 **oficial** v4.4 (`docs/hacienda/ANEXOS_Y_ESTRUCTURAS_V4.4.pdf`, bitácora al 22/04/2026).
  Páginas = «Página N de 99». Inventario completo de campos: [`../analisis/campos-xml-4.4-fe-nc-nd.md`](../analisis/campos-xml-4.4-fe-nc-nd.md).
- **Fuente del contrato:** `RDL.Contracts` v0.2.0: `invoice-issued.v1.json`, `parts/issued-note.v1.json`,
  `parts/document-line.v1.json`, `parts/line-tax.v1.json`, `parts/exoneration.v1.json`, `common/customer-snapshot.json`.
- **XSD:** `docs/hacienda/xsd/` (descargados el 2026-10-12). Raíces `FacturaElectronica`, `NotaCreditoElectronica` y
  `NotaDebitoElectronica` con namespace `https://cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/<tipo>`. Ante
  cualquier diferencia de nombre, orden o cardinalidad entre este informe y el XSD, **manda el XSD**.
- **Columnas de la base:** dependen del informe 0002. Este informe dice **de dónde sale** cada dato; el 0002 dice
  **dónde se guarda** en el snapshot del documento.

Leyenda del origen: **E** = evento de Billing · **P** = perfil fiscal de la organización (`fiscal.taxpayer_profiles` y
relacionadas) · **R** = configuración de plataforma (RDL como proveedor) · **F** = lo genera fiscal · **C** = cálculo
de fiscal a partir del evento · **—** = no existe en el modelo (brecha). Condición del anexo: 1 obligatorio,
2 condicional, 3 opcional.

## 1. Encabezado

| Campo XML | FE / NC / ND | Origen | Regla y cita |
|---|---|---|---|
| `Clave` | 1/1/1 | **F** | Nota 3 (págs. 66–68): `506` + ddmmaa + identificación del **emisor** rellenada a 12 (Nota 4.1, pág. 69) + consecutivo + situación + seguridad 8 (`crypto/rand`). Alfanumérica para cédula jurídica (pág. 18) |
| `ProveedorSistemas` | 1/1/1 | **R** | Identificación de RDL como proveedor inscrito en el RUT (pág. 18). **Brecha:** la identificación de RDL no está en ningún lado: variable de configuración `PROVIDER_IDENTIFICATION`, sin valor por defecto |
| `CodigoActividadEmisor` | 1/2/2 | **P** | Padrón del RUT (pág. 18). **Brecha:** si la organización tiene varias actividades, el evento no dice cuál aplica. Propuesta: actividad **principal** del perfil y, en una versión posterior del evento, `economicActivityCode` opcional |
| `CodigoActividadReceptor` | 2/2/2 | **—** | Condicional hoy, obligatorio en FE cuando Hacienda lo comunique (pág. 18, pie 1). Se omite en V1; propuesta de campo opcional en `customerSnapshot` |
| `NumeroConsecutivo` | 1/1/1 | **F** | Nota 3: establecimiento (3) + terminal (5) + tipo (01 FE, 02 ND, 03 NC) + número (10). Se verifica la **secuencia** (pág. 19) |
| `FechaEmision` | 1/1/1 | **F** | **No es `issuedAt`.** «No podrán señalarse fechas posteriores ni anteriores a la fecha de generación del comprobante» y «debe coincidir con la fecha señalada en la clave» (pág. 19). Es el instante en que fiscal genera el XML, en RFC 3339 con el desfase de Costa Rica (`-06:00`), y el ddmmaa de la clave sale de ese mismo instante. Ver §6.2 |
| `Emisor>Nombre`, `Identificacion>Tipo/Numero`, `NombreComercial` | 1/1/1 | **P** | Nota 4 (pág. 69); formato del número por tipo (pág. 20) |
| `Emisor>Registrofiscal8707` | 2/2/2 | **P** | Solo con CABYS de bebidas alcohólicas (pág. 20). Fuera de V1 salvo que el perfil lo tenga |
| `Emisor>Ubicacion` (Provincia, Canton, Distrito, Barrio, OtrasSenas) | 1/2/2 | **P** | Nota 14: catálogo externo **que falta** (informe 0001) |
| `Emisor>Telefono`, `CorreoElectronico {1,4}` | 3 / 1-2-2 | **P** | Correo con la regex de la pág. 22 |
| `Receptor>Nombre` | 1/1/1 | **E** `customerSnapshot.legalName` | Mín. 3, máx. 100 (pág. 22). El snapshot permite 200: **validación** antes de firmar |
| `Receptor>Identificacion>Tipo` / `Numero` | 1/2/2 | **E** `customerSnapshot.identification.typeCode` / `.number` | Nota 4. En NC/ND deben coincidir con el original (pág. 23): se compara con el documento referenciado |
| `Receptor>Ubicacion` | 2/2/2 | **—** | «Si tiene domicilio en el país» (pág. 23). El snapshot solo trae `address` en texto. **Brecha:** propuesta de `provinceCode`, `cantonCode`, `districtCode` opcionales en `customerSnapshot`. Mientras tanto, `address` no puede ir en `OtrasSenas` sin los tres códigos y el nodo se omite |
| `Receptor>Telefono` | 3/3/3 | **E** `customerSnapshot.phone` | Hacienda pide `CodigoPais` + `NumTelefono` numéricos; el snapshot es texto libre. Se envía solo si el texto es numérico (8+ dígitos, código país 506); si no, se omite (es opcional) |
| `Receptor>CorreoElectronico` | 2/2/2 | **E** `customerSnapshot.email` | Pág. 24 |
| `CondicionVenta` | 1/1/1 | **E** `saleConditionCode` | Nota 5 (pág. 70). Se valida contra el catálogo; 12 solo en FE; 09 y 11 no se admiten (solo REP) |
| `CondicionVentaOtros` | 2/2/2 | **—** | Obligatorio con 99 (pág. 25). **Brecha** si Billing admite 99: propuesta `saleConditionOther` |
| `PlazoCredito` | 2/2/2 | **E** `creditTermDays` | Obligatorio con condición 02 o 10, entero > 0 en días (pág. 25) |

## 2. Detalle (`DetalleServicio/LineaDetalle`, una por `lines[]`)

| Campo XML | FE / NC / ND | Origen | Regla y cita |
|---|---|---|---|
| `NumeroLinea` | 1 | **E** `lineNumber` | 1–1000 (pág. 26) |
| `CodigoCABYS` | 1/2/2 | **E** `cabysCode` | Nota 17 (pág. 26). En NC/ND obligatorio si el original lo tenía (siempre, en V1) |
| `CodigoComercial {0,5}` (`Tipo`, `Codigo`) | 2 | **E** `productCode` | Opcional fuera de surtidos. Si se envía: `Tipo` = **01 «Código del producto del vendedor»** (Nota 12, pág. 76) y `Codigo` máx. 20 (el evento admite 50: se envía solo si cabe) |
| `Cantidad` | 1 | **E** `quantity` | 16,3 > 0 (pág. 27) |
| `UnidadMedida` | 1 | **E** `unitOfMeasureCode` | Nota 15, **sensible a mayúsculas** (págs. 77–80) |
| `TipoTransaccion` | 2 | **—** | Nota 22; solo para tratamientos específicos (pág. 28). Se omite en V1 (venta normal) |
| `Detalle` | 1 | **E** `description` | Mín. 3, máx. 200 (pág. 28). El evento admite 1 carácter: **validación** |
| `PrecioUnitario` | 1 | **E** `unitPrice` | 18,5 > 0 (pág. 35) |
| `MontoTotal` | 1 | **E** `grossAmount` | = Cantidad × PrecioUnitario (pág. 35). **Se recalcula y compara** |
| `Descuento {0,5}` > `MontoDescuento` | 3/2/2 | **E** `discount` | Uno por línea en V1 (Billing admite uno). Solo si `discount` ≠ 0 |
| `Descuento` > `CodigoDescuento` | 1 si hay descuento | **—** | **Nota 20** (pág. 81). **Brecha:** el evento solo trae `discountReason` (texto). Propuesta: `discountCode` en la línea. Sin él, la única salida válida es **99 «Otros»** con `CodigoDescuentoOTRO` y `NaturalezaDescuento` = `discountReason` (mín. 5 y 3 caracteres, págs. 36). Se propone eso como regla transitoria, a confirmar con el contador |
| `SubTotal` | 1 | **E** `subtotal` | = MontoTotal − descuentos (pág. 36). Se recalcula |
| `IVACobradoFabrica` | 2 | **—** | Nota 21; fuera de V1 |
| `BaseImponible` | 1 si está gravada | **E** `taxes[].taxableBase` | = SubTotal + impuestos 02, 04, 05, 12 (pág. 37). En V1 (solo IVA) = SubTotal: se valida |
| `Impuesto {1,…}` | **1** | **E** `taxes[]` | **Obligatorio en todos los comprobantes** (pág. 37). Una línea con `taxes: []` (permitido por el contrato) **no cabe**: lo correcto es IVA con tarifa 10 (exenta) o 01/11 (no sujeta). **Brecha de contrato**: propuesta `minItems: 1` y que Billing envíe siempre el impuesto |
| `Impuesto>Codigo` | 1 | **E** `taxTypeCode` | Nota 8 (pág. 71) |
| `Impuesto>CodigoTarifaIVA` | 2 (1 con 01 y 07) | **E** `taxRateCode` | Nota 8.1 (pág. 72). **Opcional en el contrato y obligatorio para IVA**: validación y propuesta `if taxTypeCode = 01 then required` |
| `Impuesto>Tarifa` | 2 | **E** `rate` | 4,2; con 01 debe coincidir con la tarifa del código (pág. 38) |
| `Impuesto>Monto` | 1 | **E** `amount` | Tarifa/100 × BaseImponible; tarifa 10 → `0` (pág. 42). Se recalcula |
| `Exoneracion>TipoDocumentoEX1` | 1 si hay exoneración | **E** `exoneration.documentTypeCode` | Nota 10.1 (pág. 75); 01, 05, 06, 07 solo en NC/ND |
| `Exoneracion>TipoDocumentoOTRO` | 2 | **—** | Si 99. **Brecha** |
| `Exoneracion>NumeroDocumento` | 1 | **E** `exoneration.documentNumber` | Mín. 3, máx. 40 (págs. 44–45) |
| `Exoneracion>Articulo`, `Inciso` | 2 | **—** | Obligatorio el artículo con tipos 02, 03, 06, 07, 08 (pág. 45). **Brecha**: propuesta `article`, `subsection` opcionales |
| `Exoneracion>NombreInstitucion` | 1 | **E** `exoneration.institution` (texto) | **Código de 2** de la **Nota 23** (págs. 45, 83). **Brecha**: el contrato trae texto de 160. Propuesta `institutionCode` + `institutionOther` |
| `Exoneracion>FechaEmisionEX` | 1 | **E** `exoneration.issuedAt` | RFC 3339 (pág. 45) |
| `Exoneracion>TarifaExonerada` | 1 | **E** `exoneration.exoneratedRate` | Puntos de tarifa, 4,2 (págs. 45–46). Coincide con ADR 0007 de contratos |
| `Exoneracion>MontoExoneracion` | 1 | **E** `exoneration.amount` | = TarifaExonerada/100 × SubTotal (o base imponible); ≤ Monto (pág. 46). Se recalcula |
| `ImpuestoAsumidoEmisorFabrica` | 1/2/2 | **C** | `0` en V1: sin regalías ni IVA a fábrica (págs. 46–47). En FE va siempre, en `0` |
| `ImpuestoNeto` | 1 | **E** línea `tax` | = Monto − MontoExoneracion (pág. 47). Se recalcula |
| `MontoTotalLinea` | 1 | **E** línea `total` | = SubTotal + ImpuestoNeto (pág. 47). Se recalcula |

Fuera de V1, sin origen y sin emitir: `PartidaArancelaria`, `NumeroVINoSerie`, `RegistroMedicamento`,
`FormaFarmaceutica`, `DetalleSurtido`, `FactorCalculoIVA`, `DatosImpuestoEspecifico`, `MontoExportacion`. Un CABYS
que los exija (vehículos, medicamentos, alcohol) sale con advertencia de Hacienda o se rechaza: queda como
**TODO(fiscal)** en la validación.

## 3. Otros cargos

`OtrosCargos` (pág. 47) no existe en el modelo de Billing. V1 no lo emite. Si se necesita (p. ej., impuesto de
servicio 10 % de restaurantes, Nota 16 código 06), es un cambio de contrato.

## 4. Resumen (`ResumenFactura`)

Todo lo calcula **fiscal desde las líneas** (C) y lo **cuadra** contra los totales del evento. Billing calcula y
fiscal valida (D2).

| Campo XML | Origen | Regla y cita |
|---|---|---|
| `CodigoTipoMoneda>CodigoMoneda` | **E** `currency` | Nota 13.1: catálogo externo que falta (pág. 76) |
| `CodigoTipoMoneda>TipoCambio` | **E** `exchangeRate` | CRC = 1; USD = tipo de cambio de **venta** de referencia del BCCR; > 0 (pág. 50). Fiscal no lo consulta: valida CRC ⇒ 1 y > 0 |
| `TotalServGravados` / `TotalMercanciasGravadas` | **C** | Líneas con IVA gravado, separadas por el **primer dígito del CABYS** (5–9 servicio, 0–4 mercancía), sobre `MontoTotal`; con exoneración × (1 − %exon), %exon = Σ exonerado / Σ impuesto (págs. 50–52) |
| `TotalServExentos` / `TotalMercanciasExentas` | **C** | Líneas con tarifa **10** (págs. 51–52) |
| `TotalServExonerado` / `TotalMercExonerada` | **C** | %exon × MontoTotal (págs. 51–52) |
| `TotalServNoSujeto` / `TotalMercNoSujeta` | **C** | Líneas con tarifa **01 u 11** (págs. 51–52) |
| `TotalGravado`, `TotalExento`, `TotalExonerado`, `TotalNoSujeto` | **C** | Sumas de servicios y mercancías (pág. 53) |
| `TotalVenta` | **C** | Gravado + Exento + Exonerado + NoSujeto (pág. 53) |
| `TotalDescuentos` | **C** = **E** `discount` | Σ MontoDescuento (pág. 53) |
| `TotalVentaNeta` | **C** = **E** `subtotal` | TotalVenta − TotalDescuentos (pág. 53) |
| `TotalDesgloseImpuesto {1,…}` | **C** | Una entrada por `Codigo` + `CodigoTarifaIVA` con Σ `Monto` (págs. 53–54) |
| `TotalImpuesto` | **C** = **E** `tax` − `exoneration` | Σ Monto de líneas sin exoneración + Σ ImpuestoNeto de las exoneradas (pág. 54) |
| `TotalImpAsumEmisorFabrica` | **C** | `0` en V1 |
| `TotalIVADevuelto` | — | Solo servicios de salud con tarjeta (pág. 55). Fuera de V1 |
| `TotalOtrosCargos` | — | V1 sin otros cargos |
| `MedioPago {1,4}` > `TipoMedioPago`, `TotalMedioPago` | **E** `paymentMethods[]` | Notas 6 y 7 (págs. 55–56). **Obligatorio salvo condición de venta 02, 08 y 10.** Con más de un medio, `TotalMedioPago` obligatorio y Σ = TotalComprobante. El contrato permite `paymentMethods` vacío y `amount` opcional: **validación** |
| `MedioPago>MedioPagoOtros` | **—** | Si 99. **Brecha** |
| `TotalComprobante` | **C** = **E** `total` | TotalVentaNeta + TotalImpuesto + TotalOtrosCargos − TotalIVADevuelto (pág. 56) |

**Ojo con la definición de `TotalVenta`:** en el anexo se arma con `MontoTotal` (antes del descuento). El
`subtotal` del evento ya tiene restado el descuento, y por eso cuadra con `TotalVentaNeta`, no con `TotalVenta`.

## 5. Información de referencia (NC y ND; obligatoria, pág. 57)

| Campo XML | Origen | Regla y cita |
|---|---|---|
| `TipoDocIR` | **C** | Tipo del documento referenciado según la Nota 10 (pág. 74): **01** si `referencedInvoiceId` es una factura. Fiscal lo sabe por su propio `electronic_document` del original |
| `Numero` | **F** | **Clave** del comprobante referenciado (pág. 58), sacada del `electronic_document` del original **en estado `accepted`**. Si el original no está aceptado, la nota espera (decisiones 2 y 3) |
| `FechaEmisionIR` | **F** | `FechaEmision` del original (≤ 10 años, pág. 58) |
| `Codigo` | **—** | **Nota 9** (pág. 73): 01 anula, 02 corrige monto, 06 devolución, 09/10 financieras, etc. **Brecha:** el evento solo trae `referenceReason` (texto). Propuesta: `referenceCode` obligatorio en `IssuedNote` (v2) o, hasta entonces, **02 «Corrige monto»** por defecto para NC/ND (a confirmar con el contador). Se suma a la decisión 2 |
| `CodigoReferenciaOTRO` | **—** | Si 99 |
| `Razon` | **E** `referenceReason` | Mín. 3, máx. **180** (pág. 58). El contrato admite **500**: **brecha** (validación que manda el documento a `error`, o propuesta de bajar el máximo en Billing) |

Además, para NC: **Σ notas de crédito ≤ monto del comprobante + Σ notas de débito** (pág. 58). Fiscal lo verifica
con sus propios documentos antes de firmar, y una NC que lo exceda va a `error` con un motivo claro.

## 6. Validaciones antes de firmar

Si alguna falla, el documento pasa a `error` con el detalle en `submission_attempts`. **No se asigna consecutivo ni
clave** (§6.3), y la corrección es un reintento manual.

### 6.1 Formato y catálogo
- Cada código contra su catálogo **versionado** (Notas 4, 5, 6, 8, 8.1, 10.1, 12, 15, 20, 23 y la moneda).
- Longitudes del anexo, que son más estrictas que las del contrato: `legalName` ≤ 100, `description` 3–200,
  `referenceReason` 3–180, `discountReason` ≥ 5 si va a `CodigoDescuentoOTRO`, y `productCode` ≤ 20 para enviarlo.
- Formato del número de identificación por tipo (pág. 20): física 9 dígitos sin cero inicial, jurídica 10
  alfanuméricos, DIMEX 11–12, NITE 10.
- `isService` del evento coherente con el primer dígito del CABYS (si no coincide, manda el CABYS para los totales y
  se registra un aviso).
- Reglas condicionales: `PlazoCredito` con 02/10, `MedioPago` salvo 02/08/10, `CodigoTarifaIVA` con impuesto 01,
  al menos un `Impuesto` por línea, `TipoCambio` = 1 con CRC.

### 6.2 Fechas
- `FechaEmision` = instante de generación, con el desfase de Costa Rica. La organización puede tener otra zona
  configurada en Platform, pero Hacienda evalúa la hora local de Costa Rica. **TODO(fiscal):** confirmar con el
  sandbox si se acepta `Z`.
- Si la fecha de negocio (`issueDate`) del evento **no coincide** con la fecha de generación (por ejemplo, el evento
  llegó después de medianoche porque fiscal estuvo caída), **igual se emite con la fecha de generación** (pág. 19) y
  se registra la diferencia. Es un efecto de que fiscal está desacoplado: la factura comercial puede quedar con fecha
  de un día y el comprobante con la del siguiente. **Para decidir con el contador.**

### 6.3 Consecutivo y secuencia
Hacienda verifica la secuencia del consecutivo (pág. 19). Para no dejar huecos:
- el consecutivo y la clave se asignan **después** de que el documento pasa todas las validaciones y **antes** de
  firmar, bajo bloqueo y en la misma transacción que guarda el XML sin firmar;
- una vez asignados, **nunca se reutilizan ni se regeneran**: un fallo de envío reintenta el mismo XML firmado.

### 6.4 Cuadre de totales (D2)
Con `shopspring/decimal` y el redondeo de la pág. 14 (5 decimales, mitad hacia arriba, por campo):
- por línea: `grossAmount = quantity × unitPrice`, `subtotal = grossAmount − discount`,
  `amount = rate/100 × taxableBase`, `exoneration.amount = exoneratedRate/100 × subtotal`,
  `tax = Σ amount − Σ exoneration.amount`, `total = subtotal + tax`;
- en el documento: `subtotal`, `discount`, `tax`, `exoneration` y `total` del evento contra las sumas de las líneas,
  y `TotalComprobante` calculado igual a `total`.

**Tolerancia: cero.** El anexo admite márgenes históricos para sistemas que redondean a 2 decimales (bitácora 4.2,
pág. 12), pero Billing redondea a 5 igual que fiscal, así que cualquier diferencia es un error. Si no cuadra, el
documento va a `error` con el campo que no cuadra; **fiscal nunca corrige un monto de Billing**.

## 7. Propuestas al repo de contratos que salen de este mapeo

Ninguna se aplica sin aprobación. Todas son **campos opcionales nuevos** (compatibles) salvo las marcadas como v2.

| # | Evento / schema | Cambio | Por qué |
|---|---|---|---|
| C1 | `parts/document-line` | `discountCode` (Nota 20) | `CodigoDescuento` obligatorio con descuento (pág. 36) |
| C2 | `parts/line-tax` | `taxRateCode` obligatorio si `taxTypeCode` = 01 | `CodigoTarifaIVA` obligatorio con IVA (pág. 38). Más estricto: **v2** o regla de Billing sin cambiar el schema |
| C3 | `parts/document-line` | `taxes` con `minItems: 1` | `Impuesto` obligatorio en todo comprobante (pág. 37). Más estricto: **v2** o regla de Billing |
| C4 | `parts/exoneration` | `institutionCode` (Nota 23), `institutionOther`, `documentTypeOther`, `article`, `subsection` | Págs. 44–45 |
| C5 | `common/customer-snapshot` | `provinceCode`, `cantonCode`, `districtCode`, `neighborhood`, `economicActivityCode` | Ubicación del receptor (pág. 23) y actividad del receptor (pág. 18) |
| C6 | `parts/issued-note` | `referenceCode` (Nota 9) | `InformacionReferencia>Codigo` (pág. 58). Decisión 2 |
| C7 | `parts/issued-note` | `referenceReason` máx. 180 | `Razon` (pág. 58). Más estricto: **v2** o regla de Billing |
| C8 | `invoice-issued`, `parts/issued-note` | `saleConditionOther`; `paymentMethods[].otherDescription` | Códigos 99 (págs. 25, 56) |
| C9 | `invoice-issued`, `parts/issued-note` | `economicActivityCode` del emisor | Varias actividades por organización (pág. 18) |
| C10 | `parts/electronic-document` | `numericKey` alfanumérico | Clave con letras para personas jurídicas (informe 0001, cambio 1) |
| C11 | Notas que sustituyen a un rechazado | `replacesInvoiceId` o un evento nuevo | Decisión 3 |

Mientras C1–C9 no existan, fiscal aplica las reglas transitorias de las secciones 1 y 2: omitir lo opcional, usar
99 con el texto cuando el anexo lo permite y mandar a `error` lo que no puede emitir. **No inventa ningún dato.**
