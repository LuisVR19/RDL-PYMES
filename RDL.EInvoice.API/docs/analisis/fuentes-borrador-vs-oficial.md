# Fuentes de Hacienda: qué hay, qué falta y qué cambió

Análisis de los tres PDF de `RDL PYMES/docs/Hacienda/` (copiados a `RDL.EInvoice.API/docs/hacienda/`).
Texto extraído con `pdftotext -layout` y `-raw` (el modo `-raw` es el confiable para tablas y para las columnas de
condición; en `-layout` las columnas de códigos de varias notas salen corridas). Páginas = página impresa del PDF
(«Página N de 99» en el oficial; en el borrador se da la página física del PDF, que es la impresa + 8 para los anexos).

Convención de condición (Anexo 1, pág. 16): 1 obligatorio · 2 condicional · 3 opcional · 4 inexistente; columnas
FE FEE FEC TE NC ND REP.

---

## 1. Inventario

| Archivo | Qué es | Versión / fecha | Resolución que cita | Págs. | sha256 |
|---|---|---|---|---|---|
| `ANEXOS_Y_ESTRUCTURAS_V4.4.pdf` | **Oficial.** «Anexos y Estructuras para la Emisión de Comprobantes Electrónicos, Versión 4.4», DGT. Contiene **Anexo 1** (estructuras, mensajes, notas 1–23), **Anexo 2** (firma XAdES-EPES) y **Anexo 3** (API) | Portada: «Noviembre 2024». Bitácora: «Bitácora de Ajustes al **22/04/2026**» (pág. 3) → es una **revisión de 2026** del anexo, no la publicación original | Ninguna resolución por número. Cita el **Decreto N° 44739-H** (Reglamento de comprobantes electrónicos) art. 21 (Anexo 2, pág. 85) y resoluciones históricas (DGT-R-48-2016, DGT-R-13-2017, DGT-02-09) | 99 | `6e093226…2769` |
| `DGT-R-000-2024DisposicionesTecnicasDeComprobantesElectronicosCP.pdf` | **Borrador en consulta pública.** Resolución «MH-DGT-RES-000-2024» (13 artículos + 3 transitorios, págs. 1–8) + «Documento Borrador Anexos y Estructuras v4.4» (págs. 9–105, numeración interna 1–96) con Anexos 1, 2 y 3 | «Setiembre 2024»; fechas y número de resolución en blanco («00 horas del 000 de 000») | Decreto «N° xxxxxx-H» (sin número); el Anexo 2 cita el **Decreto 41820-H** (el reglamento anterior) | 105 | `fc3b288f…4cf2` |
| `ComprobantesElectronicos-GeneralidadesyVersion4.4.marzo2025.pdf` | Presentación de la DGT «Comprobantes Electrónicos y Versión 4.4» (resumen, sin reglas técnicas) | Marzo 2025 | Decreto **44739-H** (08/11/2024), **MH-DGT-RES-0027-2024** (19/11/2024), **MH-DGT-RES-0001-2025** (13/02/2025, modifica el transitorio I). Uso voluntario 4.4 desde 01/04/2025, obligatorio **01/09/2025**, convivencia 4.3/4.4 del 1 de abril al 1 de setiembre (pág. 5) | 21 | `632e42db…f1de` |

**Respuestas directas:**
- El oficial trae **solo los anexos**: **no** incluye la resolución MH-DGT-RES-0027-2024 (articulado) ni el Decreto 44739-H.
- **Sí** incluye el Anexo 2 (págs. 84–96) y el Anexo 3 (págs. 97–99).
- Por lo tanto, todo lo que venga del **articulado** (arts. 4, 5, 7, 9, 10, 11 y transitorios) sigue siendo `FUENTE: borrador sept-2024`.

Inconsistencias internas del oficial que conviene anotar:
- Anexo 1 rige desde el **01/09/2025** (pág. 3); el Anexo 2 dice «Rige a partir del **01 de junio del 2025**» (pág. 84).
- Canonicalización: Anexo 1 pág. 14 dice «C14n-20010315»; el ejemplo del Anexo 2 (págs. 86–87) usa
  `http://www.w3.org/2001/10/xml-exc-c14n#` (canonicalización **exclusiva**). Tomar el ejemplo del Anexo 2 y validarlo en el sandbox.
- Nota 6 (pág. 71): la tabla da **05** a «Recaudado por terceros», pero su pie 27 dice «El código **06**, Recaudado por terceros». Igual en el borrador. Usar la tabla (05) y anotarlo.
- Nota 15 (págs. 77–80): la bitácora (ítem 43) dice que clasifica unidades por servicio/mercancía, pero la tabla oficial **no** trae la columna «Tipo de Unidad» (el borrador sí la traía).
- Nota 9 (pág. 73): no existe el código **03** (salta de 02 a 04). Es así en ambos documentos.

---

## 2. Diferencias borrador vs oficial que afectan código

| Tema | Borrador (pág. física) | Oficial (pág.) | Impacto |
|---|---|---|---|
| Clave: caracteres | «Solo permite ingresar números» (pág. 26) | «Permite ingresar **números y letras** para personas jurídicas» (pág. 18). Nota 3 e): «del décimo al vigésimo primero **caracter**» (pág. 67; borrador: «dígito») | La clave **no** es solo numérica: validar `[0-9A-Za-z]` en las posiciones 10–21. Tipo de dato `text`, nunca numérico |
| Cédula jurídica | «10 **dígitos**» (emisor, receptor, tercero, mensajes) | «10 **caracteres**» (págs. 20, 23, 48, 61–63). Bitácora 22/04/2026 ítem 1: alfanuméricos para personas jurídicas (pág. 3) | Validación de identificación del emisor/receptor y relleno de la clave: alfanumérico para tipo 02 |
| Nota 4.1 relleno | Jurídica: «completar 12 **dígitos**» (pág. 74) | Jurídica: «completar 12 **caracteres**» (pág. 69). Física +3 ceros, DIMEX 11→+1 cero, NITE +2 ceros: iguales | Igual algoritmo; el relleno con `0` a la izquierda aplica a una cadena alfanumérica. Ninguno de los dos define relleno para 05/06 (solo FEC) |
| `ProveedorSistemas` | «número de cédula del proveedor» (pág. 26) | «identificación del proveedor… **En caso de ser un desarrollo propio o comprado a la medida, se deberá indicar la identificación del mismo obligado tributario**» (pág. 18). String 20, obligatorio en los 7 tipos | Para RDL (proveedor SaaS) va la identificación de **RDL**, registrada en el RUT; dato de configuración de plataforma, no de la organización |
| `CodigoActividadEmisor` NC/ND | 1 (obligatorio) | **2** (condicional) (pág. 18) | En notas puede omitirse; el mapeo debe permitirlo |
| `CodigoActividadReceptor` | FE=1, NC/ND=1 (pág. 26) | FE=**2**, NC/ND=**2**; pie 1: Hacienda «comunicará oportunamente» cuándo pasa a obligatorio en FE (pág. 18) | Opcional hoy; dejar el campo en el modelo, no exigirlo |
| `FechaEmision` | — | Agrega: «debe coincidir con la fecha señalada en la clave numérica» (pág. 19) | Clave y `FechaEmision` del mismo instante y la **misma zona** (ddmmaa de la clave = fecha local de `FechaEmision`) |
| Ubicación y correo del emisor en NC/ND | 1 | **2** (págs. 21–22) | Condicionales en notas |
| Ubicación del receptor en TE | 2 | **3** (opcional) (pág. 23) | Solo si se hace tiquete |
| Validación de ubicación del emisor | Verificaba contra la sucursal de la clave (dígitos 22–24) (pág. 29) | Solo «códigos diferentes al inscrito en el RUT» → advertencia (pág. 21) | Menos acoplamiento sucursal ↔ ubicación |
| `RegistroMedicamento` | String 30 | String **100** (pág. 28) | Tamaño de columna |
| Surtidos | `LineaDetalleSurtido {1,10}`, obligatorio siempre que el CABYS sea de surtido | `{1,20}`; obligatorio solo si el surtido combina **distintas tarifas** de IVA o impuestos específicos, y solo para fabricantes/importadores (págs. 29–30, pie 8) | Fuera de V1, pero cambia el XSD |
| IVA cobrado a fábrica código 02 | «no deberá llenar el subnodo de impuestos» | «deberá llenar… el código **10** de tarifa exenta» (págs. 31, 36–37) | Regla de validación |
| `BaseImponible` | Subtotal + ISC (02) + cemento (12) | Subtotal + 02 + **04** + **05** + 12 (pág. 37) | Fórmula de cálculo |
| Monto del impuesto: bienes usados | «factor **por 100** por el subtotal» | «factor por el subtotal» (pág. 42) | Fórmula (fuera de V1) |
| Monto del impuesto con tarifa exenta (10) | «Calculo cuando se usa código 10» (sin regla) | «se deberá de indicar **"0"**» (pág. 42) | Regla explícita |
| ISC (02) | tarifa × subtotal | «conforme a la norma que regule este impuesto» (pág. 43) | Sin fórmula en el anexo |
| `ImpuestoAsumidoEmisorFabrica` | «mayor a cero» | «igual o mayor a cero»; «0» si no aplica; con exoneración = Monto − Exonerado (págs. 46–47) | Validación |
| `MontoTotalLinea` | Bitácora: incorpora `ImpuestoAsumidoEmisorFabrica` | Bitácora ítem 95 y pág. 47: **Subtotal + ImpuestoNeto únicamente** | Fórmula del total por línea |
| `Exoneracion/NombreInstitucion` | String 160, nota 22 | **String 2** codificado, **nota 23** (pág. 45) | Catálogo nuevo de instituciones; no texto libre |
| `Exoneracion/Inciso` | «De no existir un inciso, se indicará un cero» | Frase eliminada (pág. 45) | No enviar 0 por defecto |
| `NombreInstitucionOtros` | máx. 100 | máx. **160** (pág. 45) | Tamaño |
| `MontoExoneracion` | Tarifa exonerada × Subtotal | × Subtotal **o base imponible** si existe (pág. 46) | Fórmula |
| Porcentaje de exoneración en totales | Tarifa Exonerada / Tarifa IVA | **Suma de montos exonerados / Suma de montos de impuesto** (págs. 50–52) | Fórmula de `TotalServGravados`, `TotalMercanciasGravadas`, exonerados |
| Totales exentos / no sujetos | Sin criterio de tarifa | Exento = tarifa **10**; No sujeto = tarifas **01 y 11** (nota 8.1) (págs. 51–52) | Clasificación de cada línea en el resumen |
| Clasificación servicio/mercancía | «Categoría 1 del CAByS» | Primer dígito del CABYS: **5–9 servicio, 0–4 mercancía** (págs. 50–52; también pág. 26) | Regla para separar totales |
| Medio de pago | Nodo `TotalMedioPago {1,4}` con hijo `MedioPago` | Nodo **`MedioPago {1,4}`** con hijos **`TipoMedioPago`**, `MedioPagoOtros`, `TotalMedioPago` (págs. 55–56). Obligatorio salvo condición de venta 02, 08 y 10 | **Etiquetas distintas**: lo que se haya escrito con el borrador no valida |
| `TotalIVADevuelto` | Sin fórmula mixta | Fórmula `(monto pagado tarjeta / venta neta) × impuesto` y pie 16 (pág. 55) | Solo servicios de salud |
| `TipoCambio` | — | Distinto de CRC: «mayor a 0» y tipo de cambio de **venta** de referencia del BCCR (Ley 9635) (pág. 50) | Validación y fuente del tipo de cambio |
| `TotalDesgloseImpuesto` | — | Impuesto asumido o a fábrica: «tarifa y monto igual a 0»; exoneración en surtidos → `CodigoTarifaIVA` 08 (págs. 53–54) | Reglas de desglose |
| `InformacionReferencia` en REP | TipoDocIR 01, 02, 03, 04, 08 | 01, 04, 07, 08, 10 y **20** (pág. 57) | Fuera de V1 |
| `InformacionReferencia/Numero` | Clave validada para TipoDocIR 1,2,3,4,9,11,12,15 | + **17, 18, 19 y 20**; y en NC: **la suma de ajustes no puede superar el monto del comprobante más sus notas de débito** (pág. 58) | Validación de tope de NC antes de enviar |
| Nota 2 (XSD) | URLs «xxxxx» (pág. 70) | `https://cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/{facturaElectronica, notaDebitoElectronica, notaCreditoElectronica, tiqueteElectronico, facturaElectronicaCompra, facturaElectronicaExportacion, reciboElectronicoPago}` (pág. 65) | Namespaces/ubicación de los XSD; los archivos **no** están en el repo |
| Nota 5 código 08 | «Servicios prestados al Estado **a crédito**» | «Servicios prestados al Estado» (pág. 70); pies de 14 y 15 reescritos | Texto del catálogo |
| Nota 8.1 | Códigos 01–10 (10 = Tarifa Exenta) | 01–**11**: **11 «Tarifa 0% sin derecho a crédito»** (pág. 72). 05, 06, 07 solo para NC/ND | Catálogo de tarifas |
| Nota 9 (códigos de referencia) | 01, 02, 04–12, 99. 12 = «Crédito por exoneración posterior» | Agrega **13** Anula por error material, **14** Corrige monto por error material, **15** Sustituye comprobante por error material, **16 Sustituye comprobante electrónico rechazado**, **17** Pago a comprobante (solo REP); 12 renombrado «Nota de crédito financiera por exoneración posterior…»; nota de tratamiento contable (01/02/06 vs 13/14) (pág. 73) | Catálogo y reglas de anulación/sustitución |
| Nota 10 (tipo de documento de referencia) | 01–16, 99. **Pie 28: el código 10 solo si el rechazado es del mismo mes o tiene ≤ 5 días del mes siguiente** (pág. 79) | Agrega **17** NC a FEC, **18** ND a FEC, **19** FEE, **20** REP; **el pie 28 ya no existe** (pág. 74) | El límite de fecha para sustituir un rechazado **desaparece** en el oficial |
| Nota 12 código 03 | «asignado por la industria» | «asignado por el **fabricante – industriales o importadores**» (pág. 76) | Texto |
| Nota 13.1 / 14 / forma farmacéutica | Enlace «xxxxx (se indicará en la versión final)» | Solo «Ver documento denominado "Codigodemoneda_V4.4"», «"Codificacionubicacion_V4.4"», «"Código de forma Farmacéutica"», **sin enlace** (págs. 76, 81) | Catálogos externos sin ubicación |
| Nota 15 | Con columna «Tipo de Unidad» | Sin esa columna; **mayúsculas distintas**: oficial `cm` centímetro / `Cm` Comisiones, `h` hora / `H` henry, `S` segundo / `s` siemens, `t` tesla / `T` tonelada (págs. 77–80) | Catálogo **sensible a mayúsculas**; cargar desde el oficial |
| **Renumeración de notas** | 19 descuentos · 20 IVA a fábrica · 21 tipo de transacción · 22 instituciones · 23 forma farmacéutica | **19 forma farmacéutica · 20 descuentos · 21 IVA a fábrica · 22 tipo de transacción · 23 instituciones** | Toda cita «Nota N» tomada del borrador está corrida |
| **Códigos de descuento** | 01 Regalía, 02 Regalía IVA cobrado, 03 Bonificación, **04 Pronto pago**, 05 Volumen, 06 Temporada, 07 Promocional, **08 Menudeo**, 09 Comercial, 10 Frecuencia, 11 Sostenido, 99 (pág. 86) | 01 Regalía, 02 Regalía **o Bonificación** IVA cobrado, 03 Bonificación, **04 Volumen**, 05 Temporada, 06 Promocional, 07 Comercial, 08 Frecuencia, 09 Sostenido, 99 (pág. 81) | **Mismo código, distinto significado** a partir del 04. Desaparecen «Pronto pago» y «Menudeo» |
| Mensajes receptor/Hacienda | Cédulas `Integer 12`; clave «solo números» | `String 12`; clave «números y letras» (págs. 61–63) | Fuera de V1 (mensaje receptor) pero afecta el parseo de `MensajeHacienda` |
| Anexo 2: política de firma | URL «URLXXXXV4.4» (pág. 90) | `https://cdn.comprobanteselectronicos.go.cr/xml-schemas/Resoluci%C3%B3n_General_sobre_disposiciones_t%C3%A9cnicas_comprobantes_electr%C3%B3nicos_para_efectos_tributarios.pdf` con `SigPolicyHash` SHA-256 `DWxin1xWOeI8OuWQXazh4VjLWAaCLAA954em7DMh0h8=` en el ejemplo (págs. 85, 88) | Valor del `SigPolicyId`; el hash del ejemplo **no** está garantizado como vigente (hay que descargar el PDF y recalcular) |
| Anexo 2: reglamento citado | Decreto 41820-H | Decreto **44739-H** art. 21 (pág. 85) | Solo referencia |
| Anexo 3 (API) | — | **Idéntico** al borrador | Sin cambios |
| Redondeo y formato numérico | Pág. 22 física | Pág. 14: **idéntico** | Sin cambios |
| Nota 3 (consecutivo y clave) | Págs. 70–73 | Págs. 65–68: igual salvo «dígito»→«caracter» en la cédula | Sin cambios de posiciones |
| Nota 6 (medios de pago) | 01–07, 99 | Igual | Sin cambios |
| Nota 8 (impuestos) | 01–08, 12, 99 | Igual | Sin cambios |
| Nota 10.1 y 23 (exoneración) | Mismos códigos | Iguales (pie del 11 amplía uso en NC/ND) | Sin cambios de códigos |
| Nota 22 tipo de transacción (borrador 21) | 01–13 | Igual (pág. 82) | Solo cambia el número de nota |

---

## 3. Lo que no está en ninguno de los tres documentos

| Falta | Referencia en el oficial | Bloquea |
|---|---|---|
| **XSD 4.4** de FE, NC, ND (y TE, FEC, FEE, REP) | Nota 2 da solo las URL del CDN (pág. 65) | Generación y validación del XML (incremento 7) |
| `MensajeHacienda.xsd`, `MensajeReceptor.xsd` y los Word `MensajeReceptor.doc`, `MensajeHacienda.doc` | Sección III Mensajes (pág. 61) | Parseo de la respuesta de Hacienda (incremento 9) |
| Catálogo de **moneda** «Codigodemoneda_V4.4» | Nota 13.1 (pág. 76), sin enlace | `CodigoMoneda` (todos los comprobantes) |
| Catálogo de **ubicación** «Codificacionubicacion_V4.4» (provincia/cantón/distrito) | Nota 14 (pág. 76), sin enlace | Ubicación de emisor y receptor |
| Catálogo de **forma farmacéutica** «Código de forma Farmacéutica» | Nota 19 (pág. 81) | Solo medicamentos (fuera de V1) |
| **CABYS** del BCCR | Nota 17 y pág. 26: «catálogo vigente publicado en el sitio web del BCCR» | `CodigoCABYS` (obligatorio en FE) y la separación servicio/mercancía de los totales |
| **Documentación web del API** (JSON Schema de `POST /recepcion`, respuestas, códigos de error, `indEstado`) | Anexo 3: «Ver más información en la documentación Web» / «Portal de Hacienda» (págs. 97–98) | Cliente de Hacienda (incremento 9) |
| **Client id y token URL del sandbox** | El Anexo 3 solo da `Client Id = api-prod` y el realm `rut` de **producción** (pág. 98) | Pruebas contra el sandbox |
| **PDF de la política de firma** (para calcular `SigPolicyHash`) | URL en Anexo 2 (pág. 85) | Firma XAdES-EPES (incremento 8) |
| **Resolución MH-DGT-RES-0027-2024** oficial y **MH-DGT-RES-0001-2025** | Solo citadas en la presentación (pág. 3) | Revalidar arts. 4–11 y transitorios (hoy solo del borrador) |
| **Decreto 44739-H** (Reglamento) | Citado en Anexo 2 (pág. 85) y presentación | Contingencia, art. 13 (proveedores de sistemas), plazos de conservación |
| Padrón del RUT (actividades económicas) | `CodigoActividadEmisor/Receptor`: «código existente en el padrón del RUT» (pág. 18) | Catálogo de actividades económicas |
| Procedimiento de **contingencia** | Nota 3 solo define situación 2 y 3 (pág. 68); la presentación (pág. 20) solo enumera «Comprobantes preimpresos por contingencia» y «Validador fuera de servicio» | Incremento 9 (contingencia) |
| Tipo de cambio de referencia BCCR (fuente) | `TipoCambio` (pág. 50) | Comprobantes en USD |

URLs externas que menciona el oficial: `cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/*` (XSD),
`cdn.comprobanteselectronicos.go.cr/xml-schemas/Resoluci%C3%B3n_General…pdf` (política),
`api.comprobanteselectronicos.go.cr/recepcion/v1/` (producción), `…/recepcion-sandbox/v1/` (pruebas),
`idp.comprobanteselectronicos.go.cr/auth` y `…/auth/realms/rut/protocol/openid-connect/token` (IdP de producción),
`http://www.w3.org/Signature/`, `http://openid.net/connect/`, `http://uri.etsi.org/01903/v1.3.2#`.

---

## 4. Reglas clave con su cita y su fuente

| Regla | Contenido | Fuente | Cita |
|---|---|---|---|
| Art. 9 · anulación | «Los comprobantes electrónicos válidamente emitidos no se pueden anular directamente, sus efectos contables deben anularse o modificarse mediante… notas de crédito o débito… el documento original… no podrá ser reutilizado… cuando el mensaje de confirmación por parte del Ministerio de Hacienda es de **rechazo**, el comprobante emitido **carece de validez**… no debe realizarse la respectiva nota de crédito o débito» | **Borrador** (resolución, págs. 6–7). Respaldo oficial parcial: Anexo 1 pág. 14 («Toda corrección… vía nota de crédito o débito… no se permite la modificación ni la eliminación») y Nota 9 código 01 «Anula Documento de Referencia» (pág. 73). La presentación (pág. 4) atribuye «rechazados no requieren NC» al **Decreto 44739-H** (arts. 2, 7, 9, 10, 16) | `FUENTE: borrador sept-2024` + Anexo 1 pág. 14 |
| Sustituir tras anular | Nota 9: con código 01, si hace falta un nuevo comprobante se le pone código de referencia **07** «Sustituye comprobante electrónico»; con 13 (error material) el nuevo lleva **15** | Oficial | Nota 9, pág. 73 |
| Sustituir un rechazado | Nuevo comprobante con TipoDocIR **10** «Comprobante electrónico rechazado por el MH» y código de referencia **16** «Sustituye comprobante electrónico rechazado». Sin límite de fecha (el pie 28 del borrador —mismo mes o ≤ 5 días del mes siguiente— no está en el oficial) | Oficial (códigos); límite solo en borrador | Notas 9 y 10, págs. 73–74; borrador pág. 79 |
| Art. 10 · mensaje receptor | Obligatorio entre emisores-receptores para respaldar compras; plazo **8 días hábiles** desde el primer día del mes siguiente; ante aceptación parcial o rechazo del receptor, el emisor hace «de forma inmediata» la NC/ND | **Borrador** | Resolución pág. 7 |
| Art. 11 · plazo de validación | Hacienda tiene **máximo 3 horas** desde la recepción; sin mensaje de aceptación no sirve como respaldo de créditos fiscales ni gastos | **Borrador** | Resolución págs. 7–8 |
| Validación en tres niveles | Estructura → formato → datos en BD de Hacienda; cualquier falla = rechazo con detalle | Oficial | Anexo 1 pág. 17 |
| Contingencia | Situación **2 Contingencia** = sustituye el comprobante **físico** emitido por contingencia; **3 Sin internet** = generado sin acceso a internet; `FechaEmision` anterior a la validación solo se permite con situación sin internet; referencia a «comprobante provisional» con código 05 (Nota 9) y TipoDocIR 08 (Nota 10). **No hay procedimiento** (plazos de envío posterior) en ningún documento | Oficial (códigos); procedimiento: falta (Reglamento) | Nota 3 pág. 68; pág. 19; págs. 15, 57, 73–74 |
| QR | Art. 4 y 5 (QR abajo a la derecha, 2,5 × 2,5 cm, PDF); **Transitorio III: el QR queda suspendido** hasta comunicado de la DGT. El oficial solo dice que la clave «se tiene que utilizar para la consulta del código QR» | **Borrador** | Resolución págs. 4, 8; Anexo 1 pág. 18 |
| Art. 7 · proveedores de sistemas | Cumplir el art. 13 del Reglamento (incisos 1–11 íntegros; inciso 12: administrar al menos **2 meses** de información transaccional); entregar la información al emisor, responsable de su conservación; inscripción en RUT, informar su condición, permitir revisión, descarga y respaldo de todos los documentos, registro de clientes | **Borrador** | Resolución págs. 5–6 |
| Proveedor de sistemas en el XML | `ProveedorSistemas` obligatorio, identificación del proveedor inscrito en el RUT (si no, advertencia); desarrollo propio → identificación del propio obligado | Oficial | Pág. 18 |
| Conservación | Art. 109 Código Tributario: duplicados **5 años** (considerando IV) | **Borrador** (considerando) | Resolución pág. 1 |
| Nombres de archivo | `clave.xml`, `clave_respuesta.xml`, `clave.pdf` | Oficial | Nota 3, pág. 68 |
| Representación gráfica | Tipo de documento, clave y consecutivo juntos | Oficial | Nota 1, pág. 65 |
| Firmas adicionales | Con firma de receptor o endosos (`xades:ClaimedRole`) **no** se envía a Hacienda; vale el primer archivo del emisor | Oficial | Pág. 60; Anexo 2 pág. 85 |
