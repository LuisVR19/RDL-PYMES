# Catálogos externos (fuentes)

Archivos que el Anexo 1 oficial no trae y a los que remite. Descargados el **2026-10-12**; sus sha256 están en
`../manifest.sha256` y una prueba falla si cambian. **No se editan:** una versión nueva se agrega con su hash.

| Archivo | Qué es | De dónde | Lo usa |
|---|---|---|---|
| `Codificacionubicacion_V4.4.xlsx` | Provincias, cantones, distritos y barrios (Nota 14, pág. 76) | `https://www.hacienda.go.cr/docs/Codificacionubicacion_V4.4.xlsx` | `cmd/import-catalogs` lo lee tal cual (columnas B–G; los barrios no se guardan). Cantón y distrito con cero a la izquierda: el XSD exige `\d\d` |
| `Codigodemoneda_V4.4.pdf` | Códigos de moneda (Notas 13 y 13.1, pág. 76) | `https://www.hacienda.go.cr/docs/Codigodemoneda_V4.4.pdf` (no hay versión en Excel) | Transcrito a `catalogs/nota-13-1-monedas.csv` con `pdftotext -raw`; los 168 códigos coinciden con la enumeración `CodigoMoneda` del XSD |
| `CABYS_2025-04-01_catalogo-de-bienes-servicios.xlsx` | CABYS 2025 del BCCR (Nota 17, pág. 80): 20 506 códigos de 13 dígitos con su impuesto | Noticia del BCCR «Actualización del Catálogo de Bienes y Servicios (Cabys) 2025» (`bccr.fi.cr/content/dam/bccr/noticias/2025/2025-04-01-catalogo-de-bienes-servicios-1.xlsx`) | `cmd/import-catalogs` lo lee tal cual (hoja «Catálogo», columnas Q–S). Impuesto en puntos; «Exento» (885) y «na» (3) se guardan como texto (`tax_label`, migración 00003) |
| `CABYS_2025-04-01_comunicado-CP-BCCR-015-2025.pdf` | Comunicado de prensa del BCCR que publica la versión 2025 | Misma noticia | Respaldo de la fecha de publicación (2025-04-01) |

Falta: el **padrón de actividades económicas** del RUT y el documento de **forma farmacéutica** (Nota 19, fuera de V1).
