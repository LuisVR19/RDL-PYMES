# XSD oficiales v4.4

Descargados el **2026-10-12** de `https://www.hacienda.go.cr/docs/<Nombre>_V4.4.xsd`. Las URL de la Nota 2 del
Anexo 1 (`https://cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/...`) son los **namespaces**, no los archivos:
el CDN responde 403. Todos declaran `version="4.4"`. **No se editan:** si Hacienda publica otra revisión, se reemplazan
con su nuevo sha256 y se anota aquí.

| Archivo | Elemento raíz | targetNamespace | sha256 |
|---|---|---|---|
| `FacturaElectronica_V4.4.xsd` | `FacturaElectronica` | `https://cdn.comprobanteselectronicos.go.cr/xml-schemas/v4.4/facturaElectronica` | `926ba81325bc21102755bd8e4e570a63caaca086e680a877c3a6d15d6ee8a62f` |
| `NotaCreditoElectronica_V4.4.xsd` | `NotaCreditoElectronica` | `…/v4.4/notaCreditoElectronica` | `b7ed3347c5599e78e5683d9ae83842695ab9497e173df9cd3be8d77c0851f556` |
| `NotaDebitoElectronica_V4.4.xsd` | `NotaDebitoElectronica` | `…/v4.4/notaDebitoElectronica` | `3709389ccc434fb1ef4571079b1a628654fa04f07c1f3f1044b2416ba12eb7ff` |
| `MensajeHacienda_V4.4.xsd` | `MensajeHacienda` | `…/v4.4/mensajeHacienda` | `b2606156042b7aa7b7f7f61945f0ff1323661b43b5a1489ad5eafad1c52fd25e` |
| `TiqueteElectronico_V4.4.xsd` | `TiqueteElectronico` | `…/v4.4/tiqueteElectronico` | `d7783baf818f316596d0b58b17481be689595982a248d98102d707f72967ae6e` |
| `FacturaElectronicaCompra_V4.4.xsd` | `FacturaElectronicaCompra` | `…/v4.4/facturaElectronicaCompra` | `57e4cedb12d5045884670ff2e4795d2027e69b96d45752f60cd1a1930e2768a3` |
| `FacturaElectronicaExportacion_V4.4.xsd` | `FacturaElectronicaExportacion` | `…/v4.4/facturaElectronicaExportacion` | `e571d8296bb74e86cb9ef7889601844c61ac50bedca90aa8d52badf15198242e` |
| `ReciboElectronicoPago_V4.4.xsd` | `ReciboElectronicoPago` | `…/v4.4/reciboElectronicoPago` | `86e3aa5f0615b757a3b2e9c300973d573ac321f76adba7b4b05e59d854da82b7` |
| `MensajeReceptor_V4.4.xsd` | `MensajeReceptor` | `…/v4.4/mensajeReceptor` | `b883e3a9321eadf2ccc1a9dc853d9e3bd483918dcd6e2152d902ddf2f244365f` |
| `xmldsig-core-schema.xsd` | — (XMLDSig, W3C) | `http://www.w3.org/2000/09/xmldsig#` | `d102ad3df7664c307e0c2c776ba4a90513b1969974d8a940bae1a77f9f21e15d` |

V1 usa los tres primeros y `MensajeHacienda` (decisión 1); los demás se guardan por completitud.

## Dependencia de la firma

Los nueve XSD de Hacienda importan `http://www.w3.org/2000/09/xmldsig#` con `schemaLocation="../../xmldsig-core-schema.xsd"`,
una ruta relativa que solo existe en el servidor de Hacienda. Por eso se guarda aquí la copia de la W3C
(`https://www.w3.org/TR/xmldsig-core/xmldsig-core-schema.xsd`, descargada el mismo día). El adaptador `xsd` resuelve ese
import hacia este archivo con un catálogo o una sustitución en memoria, **sin modificar los XSD de Hacienda**. El
schema de la W3C trae un `DOCTYPE` con DTD externa: el validador no debe descargarla (sin red al validar).

## Lo que confirman (contra los informes)

- `ClaveType`: `[a-zA-Z0-9]{50,50}`. La clave **es alfanumérica** (informe 0001, cambio 1).
- `NumeroConsecutivoType`: `\d{20,20}`.
- `DecimalDineroType`: `xs:decimal`, 18 dígitos, 5 decimales, entre 0 y 9999999999999.99999.
- `CodigoActividadEmisor`: string de exactamente 6, obligatorio en FE.
