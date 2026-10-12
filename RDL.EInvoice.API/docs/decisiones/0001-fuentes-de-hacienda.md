# 0001 · Fuentes de Hacienda: qué hay, qué falta y qué cambió

> **Hay anexos oficiales v4.4 (revisión con bitácora al 22/04/2026), pero NO la resolución oficial MH-DGT-RES-0027-2024.**
> Las reglas de estructura, catálogos, firma y API se toman del anexo oficial. Las que salen del articulado
> (arts. 4, 5, 7, 9, 10, 11 y transitorios) siguen marcadas `FUENTE: borrador sept-2024`.

- **Fecha:** 2026-10-12
- **Estado:** Aprobado el 2026-10-12. XSD 4.4 descargados el mismo día (`docs/hacienda/xsd/`, ver su README).
- **Detalle completo:** [`../analisis/fuentes-borrador-vs-oficial.md`](../analisis/fuentes-borrador-vs-oficial.md) (diferencias
  campo por campo, con página) y [`../analisis/campos-xml-4.4-fe-nc-nd.md`](../analisis/campos-xml-4.4-fe-nc-nd.md)
  (inventario de campos de FE, NC y ND, y catálogos).
- **Cómo se leyó:** `pdftotext -layout` y `-raw` (el modo `-raw` es el confiable para las tablas de condición). Las
  páginas citadas son las impresas («Página N de 99» en el oficial).

## 1. Inventario de `docs/hacienda/`

| Archivo | Qué es | Versión y fecha | Págs. | sha256 |
|---|---|---|---|---|
| `ANEXOS_Y_ESTRUCTURAS_V4.4.pdf` | **Oficial.** Anexo 1 (estructuras, mensajes, Notas 1–23), Anexo 2 (firma XAdES-EPES) y Anexo 3 (API) | Portada «Noviembre 2024»; **bitácora de ajustes al 22/04/2026** (pág. 3). Anexo 1 rige desde el 01/09/2025; Anexo 2 dice «rige a partir del 01/06/2025» (pág. 84) | 99 | `6e093226b29b38c5c8de825f70c1b1cb8ed81e2f4a6eb0b3ff52708fc1eb2769` |
| `DGT-R-000-2024DisposicionesTecnicasDeComprobantesElectronicosCP.pdf` | **Borrador en consulta pública**: resolución MH-DGT-RES-000-2024 (13 artículos y 3 transitorios) y borrador de los tres anexos | Setiembre 2024, sin número ni fecha definitivos | 105 | `fc3b288f1bf23638bcde75143d4a931eeb4ca5f22847665d182712caa0524cf2` |
| `ComprobantesElectronicos-GeneralidadesyVersion4.4.marzo2025.pdf` | Presentación de la DGT (resumen, sin reglas técnicas) | Marzo 2025. Cita el Decreto 44739-H (08/11/2024), MH-DGT-RES-0027-2024 (19/11/2024) y MH-DGT-RES-0001-2025 (13/02/2025); 4.4 obligatoria desde el **01/09/2025** | 21 | `632e42dbc1a1d1da74873a0505b51042d1e90214ade3c7895b819f1c90a6f1de` |

`cmd/import-catalogs` guardará este sha256 junto con la versión de cada catálogo que cargue desde estos archivos.

## 2. Qué cambió del borrador al oficial (lo que afecta el código)

Las reglas de contratos v0.2.0 (ADR 0007 de contratos) se escribieron con el borrador. Contra el oficial:

| # | Cambio | Impacto en el código | Pág. oficial |
|---|---|---|---|
| 1 | **La cédula jurídica y la clave admiten letras** («números y letras para personas jurídicas»; cédula jurídica de 10 **caracteres**; relleno de la Nota 4.1 a 12 **caracteres**) | La clave **no** es `^[0-9]{50}$`: el `CHECK electronic_documents_numeric_key_ck`, `numericKey` en `parts/electronic-document.v1.json` y `openapi/fiscal.yaml` quedan **más estrictos que Hacienda**. Propuesta a contratos y a la base (informe 0002). El consecutivo sigue siendo solo números | 3, 18, 20, 67, 69 |
| 2 | **Notas 19 a 23 renumeradas**: 19 forma farmacéutica, 20 descuentos, 21 IVA a fábrica, 22 tipo de transacción, 23 instituciones | Toda cita «Nota N» tomada del borrador está corrida (p. ej., la descripción de `exoneration.v1.json` cita «Nota 22» para instituciones: es la **23**) | 81–83 |
| 3 | **Códigos de descuento**: desde el 04 cambian de significado (04 pasa de «Pronto pago» a «Volumen»; desaparecen «Pronto pago» y «Menudeo») | El catálogo se carga solo desde el oficial | 81 |
| 4 | **`NombreInstitucion` de la exoneración** pasa de texto (160) a **código de 2** (Nota 23), con `NombreInstitucionOtros` si es 99 | `exoneration.institution` (texto, 160) del contrato no alcanza: propuesta en el informe 0003 | 45 |
| 5 | **Medio de pago** con otras etiquetas: `MedioPago {1,4}` con `TipoMedioPago`, `MedioPagoOtros`, `TotalMedioPago`, en el **resumen**; obligatorio salvo condición de venta 02, 08 y 10 | Mapeo directo de `paymentMethods`; validación previa | 55–56 |
| 6 | **Nota 8.1** agrega la tarifa **11** «0 % sin derecho a crédito». Exento = tarifa 10; no sujeto = tarifas 01 y 11 | Clasificación de cada línea en los totales del resumen | 72, 51–52 |
| 7 | **Servicio o mercancía** por el primer dígito del CABYS (5–9 servicio, 0–4 mercancía), no por la categoría 1 | El `isService` del evento debe coincidir con el CABYS; si no, validación (informe 0003) | 26, 50–52 |
| 8 | **Porcentaje de exoneración en totales** = Σ exonerado / Σ impuesto (antes, tarifa exonerada / tarifa) | Fórmula de `TotalServGravados`, `TotalServExonerado` y equivalentes de mercancías | 50–52 |
| 9 | `MontoTotalLinea` = **SubTotal + ImpuestoNeto** (sin `ImpuestoAsumidoEmisorFabrica`) | Coincide con la fórmula de contratos v0.2.0 | 47 |
| 10 | `BaseImponible` = SubTotal + impuestos 02, 04, 05 y 12 cuando correspondan; `MontoExoneracion` = TarifaExonerada × SubTotal **o base imponible** | Para IVA puro (V1) la base es el subtotal: sin cambio práctico | 37, 46 |
| 11 | **Nota 9** agrega 13 a 17; **16 = «Sustituye comprobante electrónico rechazado»**. **Nota 10** agrega 17 a 20 | Decisiones 2 y 3 | 73–74 |
| 12 | **Desaparece el pie 28 del borrador**: sustituir un rechazado (TipoDocIR 10) ya no está limitado al mismo mes + 5 días. Único límite oficial: `FechaEmisionIR` ≤ 10 años | El prompt citaba esa «nota 28»: ya no aplica | 58, 74 |
| 13 | **Tope de las notas de crédito**: Σ ajustes ≤ monto del comprobante + Σ notas de débito sobre él | Validación antes de firmar; Billing no lo impone hoy (decisión abierta de F5) | 58 |
| 14 | **`ProveedorSistemas`**: identificación del proveedor inscrito en el RUT; si es desarrollo propio, la del mismo obligado | Para nosotros es la identificación de **RDL**: configuración de **plataforma**, no de la organización | 18 |
| 15 | Campos que pasan a **condicionales**: `CodigoActividadReceptor` (FE, NC, ND; pasará a obligatorio en FE cuando Hacienda lo avise); en NC/ND también `CodigoActividadEmisor`, ubicación y correo del emisor | El mapeo los permite omitir en notas | 18–22 |
| 16 | **`FechaEmision` debe coincidir con la fecha de la clave** | La clave y `FechaEmision` se arman desde el mismo instante y la misma zona horaria (informe 0003) | 19 |
| 17 | **Nota 15 (unidades) distingue mayúsculas** (`cm` centímetro / `Cm` comisiones, `h` hora / `H` henry, `t` tesla / `T` tonelada) | La carga y la comparación no normalizan mayúsculas; `units_of_measure.code` se compara exacto | 77–80 |

**Sin cambios** entre borrador y oficial: el Anexo 3 (API), el redondeo y el formato numérico (pág. 14: punto
decimal, sin separador de miles, 5 decimales mitad hacia arriba con los ejemplos `20.203512 → 20.20351` y
`20.203518 → 20.20352`), las posiciones de la Nota 3, las Notas 6, 8, 10.1 y la de tipo de transacción.
**D2 (redondeo) queda confirmado con el oficial**: ya no es `FUENTE: borrador`.

### Inconsistencias dentro del propio anexo oficial (se anotan, no se resuelven solos)

- **Canonicalización:** la pág. 14 dice C14N 20010315; el ejemplo del Anexo 2 (págs. 86–87) usa la **exclusiva**
  (`xml-exc-c14n#`). Se toma la del ejemplo y la valida el sandbox (incremento 8).
- **Nota 6:** la tabla da **05** a «Recaudado por terceros» y su pie dice 06. Se usa la tabla.
- **Nota 9:** no existe el código 03 (igual en el borrador).
- **Nota 15:** la bitácora (ítem 43) dice que clasifica servicio y mercancía, pero la tabla no trae esa columna.
- **Vigencia:** Anexo 1 desde el 01/09/2025; Anexo 2 desde el 01/06/2025.

## 3. Reglas que siguen siendo `FUENTE: borrador sept-2024`

El anexo oficial no trae el articulado. Hasta tener la resolución 0027-2024 y el Decreto 44739-H:

| Regla | Contenido | Respaldo oficial parcial |
|---|---|---|
| Art. 9 · anulación | Un comprobante válido no se anula: se corrige con NC/ND. Un **rechazado** carece de validez y no lleva nota | Anexo 1 pág. 14 («toda corrección… vía nota de crédito o débito») y Nota 9 código 01; la presentación (pág. 4) lo atribuye al Decreto 44739-H |
| Art. 10 · mensaje receptor | Obligatorio entre emisores; 8 días hábiles | — (fuera de V1) |
| Art. 11 · plazo | Hacienda tiene hasta **3 horas** para validar | — (alerta del incremento 12) |
| Arts. 4 y 5, transitorio III · QR | PDF con QR, **suspendido** | El oficial solo dice que la clave sirve para el QR (pág. 18) |
| Art. 7 · proveedores de sistemas | Art. 13 del Reglamento; administrar al menos 2 meses de información transaccional; entregar al emisor | `ProveedorSistemas` obligatorio (pág. 18) |
| Conservación | Art. 109 del Código Tributario: 5 años (considerando IV) | — (decisión 7) |

## 4. Lo que falta y a qué bloquea

| Falta | Dónde lo cita el oficial | Bloquea | Cómo conseguirlo |
|---|---|---|---|
| ~~**XSD 4.4**~~ ✅ 2026-10-12 | Las URL de la Nota 2 son namespaces (el CDN da 403); los archivos están en `https://www.hacienda.go.cr/docs/<Nombre>_V4.4.xsd` | — | En `docs/hacienda/xsd/` con su sha256, más `xmldsig-core-schema.xsd` de la W3C que todos importan |
| Catálogo de **moneda** «Codigodemoneda_V4.4» | Nota 13.1 (pág. 76), sin enlace | `CodigoMoneda` en todos los comprobantes (incr. 5 y 7). V1 puede arrancar con CRC y USD si usted lo aprueba como dato del Excel | Sitio de Hacienda |
| Catálogo de **ubicación** «Codificacionubicacion_V4.4» | Nota 14 (pág. 76), sin enlace | `Emisor>Ubicacion` (obligatoria en FE) y la del receptor (incr. 4, 5 y 7) | Sitio de Hacienda |
| Forma farmacéutica | Nota 19 (pág. 81) | Solo medicamentos: fuera de V1 | — |
| **CABYS** del BCCR | Nota 17 (pág. 80) | Búsqueda y validación de `CodigoCABYS` (incr. 5); separación servicio/mercancía | Excel del sitio del BCCR |
| **Padrón del RUT** (actividades económicas) | `CodigoActividadEmisor` (pág. 18) | Catálogo `economic_activities` (incr. 4–5). Alternativa: validar solo el formato y que el contribuyente la ingrese | Hacienda (ATV / consulta de situación tributaria) |
| **Documentación web del API**: client id y URL del token del **sandbox**, JSON de `POST /recepcion`, `ind-estado` y códigos de error | Anexo 3 (págs. 97–98) solo trae `client_id = api-prod` y el realm `rut` de producción | **Incremento 9** (cliente real). El stub de F3 no lo necesita | Portal de Hacienda |
| **PDF de la política de firma** | Anexo 2, URL en pág. 85 | `SigPolicyHash` del XAdES (incr. 8). El hash del ejemplo (pág. 88) no está garantizado como vigente | Descargar de la URL del anexo |
| **Resolución MH-DGT-RES-0027-2024**, **MH-DGT-RES-0001-2025** y **Decreto 44739-H** | Presentación (pág. 3) | Revalidar la sección 3 de este informe (anulación, plazos, contingencia, proveedores) | Sitio de Hacienda / SCIJ |
| **Procedimiento de contingencia** | Nota 3 solo define las situaciones 2 (sustituye un comprobante **físico** de contingencia) y 3 (sin internet) (pág. 68) | Estado `contingency` (incr. 9): con lo que hay, «Hacienda no responde» **no** cambia la situación de la clave; solo se reintenta | Reglamento (Decreto 44739-H) |
| Credenciales y **llave criptográfica de pruebas** (`.p12` y PIN) | — | Incrementos 8 y 9 contra el sandbox | Usted, en ATV; nunca en el repo |
| Tipo de cambio de referencia del BCCR | `TipoCambio` (pág. 50) | Comprobantes en USD: hoy Billing envía `exchangeRate` y fiscal solo lo valida (> 0) | Fuera del alcance de fiscal |

### Bloqueos por fase

- **F2 (esqueleto) y F3 (consumidor + stub):** **no** están bloqueados.
- **F4:** el incremento 5 (catálogos) se puede hacer con las Notas del anexo oficial; CABYS, moneda y ubicación
  esperan sus archivos. El 6 (consecutivo y clave) y el **7** (XSD ya en el repo) no están bloqueados. El **8** espera
  la política de firma y el `.p12` de pruebas, y el **9** la documentación del sandbox y las credenciales.
- **F5:** los códigos de referencia ya están en la Nota 9 oficial; depende de las decisiones 2 y 3.

## 5. Propuestas que salen de este informe (no se aplican sin aprobación)

1. **Contratos:** relajar el patrón de la clave a 50 caracteres alfanuméricos (`^[0-9A-Za-z]{50}$`, o más preciso:
   letras solo en las posiciones 10–21) en `parts/electronic-document.v1.json` y `openapi/fiscal.yaml`. El XSD oficial lo confirma: `ClaveType` = `[a-zA-Z0-9]{50,50}`. Es un cambio
   que **amplía** lo aceptado: compatible para los productores, pero el consumidor que valida con el patrón viejo
   rechazaría una clave con letras. Se propone como v2 del schema base o se publica antes de la primera emisión real.
2. **Contratos:** corregir las citas de Notas renumeradas (`exoneration.v1.json`: instituciones = Nota 23) y quitar
   `FUENTE: borrador` del redondeo (confirmado en el oficial, pág. 14).
3. **Base:** relajar `electronic_documents_numeric_key_ck` (informe 0002).
