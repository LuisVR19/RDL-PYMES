# Plan · E-Invoice API (prompt P5, servicio `fiscal`)

- **Fecha:** 2026-10-12 · **Estado:** **aprobado** el 2026-10-12: informes 0001 y 0003 y las 7 decisiones con sus
  propuestas (en la 2, la opción **a**). XSD 4.4 en `docs/hacienda/xsd/`. El 0002 espera aprobación.
- Informes: [`decisiones/0001-fuentes-de-hacienda.md`](decisiones/0001-fuentes-de-hacienda.md) ·
  [`decisiones/0002-estado-inicial-bd.md`](decisiones/0002-estado-inicial-bd.md) (2026-10-12, vía MCP; **espera aprobación**) ·
  [`decisiones/0003-mapeo-eventos-a-comprobantes.md`](decisiones/0003-mapeo-eventos-a-comprobantes.md).

## 1. Datos del entorno

| Dato | Valor |
|---|---|
| Proyecto Supabase dev | `dzlsnsstuqpxvwegeqcy` (nunca producción) |
| Pooler | `aws-0-us-east-2.pooler.supabase.com` (6543 transacción, 5432 sesión), igual que Platform |
| Go | 1.27.1 · contratos `bitbucket.org/rdl/contracts` v0.2.0 con `replace => ../RDL.Contracts` (sin tags) |
| Puertos | API **8082** (el de `openapi/fiscal.yaml`) · consumidor **8085** · worker **8086** (health y métricas) |
| Transporte de eventos (P2) | **pendiente** → puerto `EventSource`, `NoSource` + `cmd/replay`, `TODO(P2)` |
| Gestor de secretos (P2) | **pendiente** → decisión 6 |
| Almacenamiento de archivos | **pendiente** → decisión 6 |
| Credenciales y `.p12` del sandbox de Hacienda | **No los tengo, y no deben pasar por este chat.** Hacen falta a partir del incremento 8 |

## 2. Decisiones que necesito que usted tome

Cada una con una propuesta. Las que cambian el repo de contratos salen como propuesta a contratos (CHANGELOG
«Sin publicar» y 2 aprobaciones), no como cambio local.

### Decisión 1 · Comprobantes de V1
**Propuesta:** Factura (01), Nota de débito (02) y Nota de crédito (03). **Tiquete (04) fuera de V1**: Billing
exige cliente identificado en toda factura (snapshot obligatorio), así que no hay ventas a consumidor final que
mapear. Fuera también: FEC (08), FEE (09), REP (10) y mensaje receptor (05, 06, 07).

### Decisión 2 · Anulación (art. 9, `FUENTE: borrador`; Nota 9 código 01, oficial)
Un comprobante **aceptado** no se anula: se corrige con una NC código **01** «Anula documento de referencia». Uno
**rechazado** no lleva nota. Opciones:

- **(a) Billing emite la nota de crédito.** «Anular» en Billing, sobre una factura con documento fiscal aceptado,
  crea en la misma transacción una NC por el total con `referenceCode = 01` y pasa la factura a `cancelled`. Emite
  `CreditNoteIssued` (con el código) e `InvoiceCancelled`. Fiscal procesa la NC como cualquier otra.
  - *A favor:* cada comprobante fiscal tiene su documento comercial, con número visible, PDF y lugar en los listados.
  - *En contra:* toca Billing (necesita saber el estado fiscal, que le llega por `ElectronicDocumentAccepted/Rejected`)
    y Receivables (tiene que tomar uno solo de los dos eventos para no restar dos veces).
- **(b) Fiscal genera la NC desde `InvoiceCancelled`**, con las líneas del snapshot que ya guardó.
  - *A favor:* no toca Billing ni Receivables.
  - *En contra:* queda una NC fiscal **sin documento comercial**, sin número de Billing y fuera del listado de notas
    del portal; es una fuente de confusión para el contador.

**Recomiendo (a)**, con estas reglas:
- anular una factura cuyo documento está en `processing`, `signed`, `sent` o `contingency` → **409** hasta que haya
  respuesta (máx. 3 h, art. 11);
- documento `rejected` → anulación comercial sin NC;
- documento `error` → anulación sin NC, y fiscal marca el documento como descartado. **Esto exige un estado o
  marca nueva**, y como el consecutivo ya pudo haberse asignado es un **hueco de secuencia** a revisar con el
  contador.

### Decisión 3 · Rechazados (Nota 9 código 16, Nota 10 código 10, oficial)
El oficial ya **no** tiene el límite de fecha del borrador (el «pie 28»).

**Propuesta:** la corrección la origina **Billing**. «Corregir» sobre una factura rechazada crea un borrador nuevo
copiado de la original, con `replacesInvoiceId`. Al emitirlo, el `InvoiceIssued` lleva ese id (campo opcional nuevo) y
fiscal agrega `InformacionReferencia` con `TipoDocIR` 10, la **clave del rechazado** y código 16. La factura
rechazada pasa en Billing a `cancelled` sin NC (carece de validez), y Receivables cancela su cuenta con el
`InvoiceCancelled` de siempre.

**Bandeja (criterio 10):** `GET /v1/electronic-documents?status=rejected` con el motivo de Hacienda y una acción
«Corregir en facturación» que el portal enlaza a Billing. El reintento (`POST …/retry`) es solo para `error`, porque
un rechazado nunca se reenvía.

### Decisión 4 · Consecutivo (D11; Nota 3, oficial)
La Nota 3 dice textualmente que la terminal es «00001» si hay una sola **o si es un servidor centralizado» (pág. 66),
que es nuestro caso.

**Propuesta:**
- **establecimiento ↔ sucursal 1:1**: `fiscal.establishments.branch_id` → `core.branches`. La factura sin sucursal
  usa el establecimiento **001** (casa matriz), que se crea con el perfil fiscal;
- **una terminal 00001 por establecimiento** en V1. Nadie la elige al emitir: la asigna fiscal;
- **secuencia por (organización, establecimiento, terminal, tipo de comprobante)**, asignada bajo bloqueo
  (`SELECT … FOR UPDATE` sobre `fiscal.document_sequences`) en la transacción que guarda el XML sin firmar, y solo
  después de pasar las validaciones (0003 §6.3). Al llegar a 9 999 999 999 se reinicia en 1 (Nota 3);
- si la organización ya emitía con otro sistema, la numeración **continúa** (Nota 3, cambio de plataforma): se
  permite fijar el siguiente número al configurar el establecimiento, con auditoría.

### Decisión 5 · Respuesta de Hacienda y contingencia
**Propuesta:**
- **consultar el estado** con `GET /recepcion/{clave}` y backoff (30 s, 1, 2, 5, 10 y 30 min, y después cada hora
  hasta 3 h; pasado ese plazo, alerta);
- **sin `callbackUrl` en V1**, porque exige un endpoint público autenticado (P2). Se puede sumar después sin cambiar
  la máquina de estados.

**Contingencia, a confirmar.** La situación 2 de la clave es para comprobantes **físicos** emitidos en contingencia,
y la 3 para «sin internet». «Hacienda no responde» no es ninguna de las dos, y no hay procedimiento en `docs/hacienda/`.
- **Propuesta:** el estado `contingency` significa «firmado, Hacienda no disponible»: se reintenta **el mismo XML**
  con situación 1, sin regenerar nada.
- **Riesgo:** la pág. 19 dice que una `FechaEmision` anterior a la validación solo se admite con situación «sin
  internet», y no sabemos cuánto atraso tolera Hacienda con situación 1.
- `TODO(fiscal)` para el contador y para el sandbox; no se resuelve con código.

### Decisión 6 · Secretos y archivos mientras P2 no exista
**Propuesta:**
- **`SecretStore`** con un adaptador de desarrollo: archivos en `.secrets/` (ignorado por git), cifrados con
  **AES-256-GCM** con una llave maestra que viene de una variable de entorno (`SECRETS_MASTER_KEY`). Ahí van el `.p12`
  de cada organización, su PIN y su usuario y contraseña de Hacienda. La base solo guarda las referencias
  (`*_secret_id`). El `.p12` y el PIN se descifran en memoria al firmar y se descartan. Nunca en claro en disco, logs,
  auditoría ni respuestas;
- **`BlobStore`** con un adaptador local en `.blobs/` (ignorado por git). La base guarda la ruta y el `sha256`, y la
  descarga verifica el hash. En P2 se reemplaza por Supabase Storage o un bucket, sin tocar el dominio.

### Decisión 7 · Retención
**Propuesta:** `RETENTION_YEARS` configurable, con valor por defecto **5** (art. 109 del Código Tributario, citado en
el considerando del borrador). **V1 no borra nada automáticamente**: no hay tarea de purga. Queda `TODO(legal)` para
confirmar el plazo con asesoría legal.

## 3. Decisiones técnicas (un ADR cada una, al implementar)

| Tema | Propuesta | Por qué |
|---|---|---|
| Validación XSD | Puerto `XSDValidator`. Adaptador principal: **libxml2 vía cgo** (`lestrrat-go/libxml2`) dentro de la imagen Docker (`debian:bookworm-slim` + `libxml2`, no distroless estática). En Windows no hay `libxml2` ni `xmllint` (verificado): las pruebas XSD corren con el tag `xsd` en Docker o en el CI, y localmente se saltan con aviso, como las suites de base | Go no tiene un validador XSD completo; el XSD oficial es la fuente de verdad |
| Firma XAdES-EPES | `beevik/etree` + `russellhaering/goxmldsig` para el `ds:Signature` enveloped, y las `xades:SignedProperties` (SigningTime, SigningCertificate, SignaturePolicyIdentifier con el hash del PDF de la política) armadas a mano según el Anexo 2. Canonicalización **exclusiva**, la del ejemplo del Anexo 2 (0001 §2). La prueba de verdad es el sandbox | No hay una librería Go de XAdES mantenida |
| Decimales | `shopspring/decimal`, igual que Billing y Receivables. `rounding.Round5` (mitad hacia arriba) con los ejemplos de la pág. 14 como pruebas, y la misma fórmula que `money.Round` del contrato | Coherencia con D2 |
| Cola del worker | Sobre la base: `next_retry_at` y `FOR UPDATE SKIP LOCKED` en `fiscal.electronic_documents` o `submission_attempts` (según el 0002). Varios workers sin pisarse; apagado ordenado que termina el envío en curso | No depende de P2 |
| Reutilización de Platform | Copiar `pkg/tenancy`, `correlation`, `requestinfo`, auth, `TxManager`, Problem Details e idempotencia, **como hizo Receivables (su ADR 0003)** | El módulo de Platform no es importable |
| Consumidor | El de Receivables (su ADR 0005): inbox `consumer_service = 'fiscal'`, 6 reintentos con backoff, dead letter, `WithinServiceTx(org del evento)` | Ya está probado contra dev |

## 4. Incrementos

Los 13 del prompt, en orden. Cada uno compila y queda en verde, actualiza `docs/ESTADO.md` y termina con una pausa.

| # | Incremento | Bloqueado por |
|---|---|---|
| 1 | Esqueleto, config, health, logins, baseline, inventario de `docs/hacienda/` | Informe 0002 (base) |
| 2 | JWT, `TenantContext`, primer endpoint (`GET /v1/fiscal-profile`) | — |
| 3 | Consumidor de `InvoiceIssued` → documento en `processing` con snapshot; stub de Hacienda; `GET /internal/v1/electronic-documents/by-source/{id}` (**F3, flujo vertical**) | — |
| 4 | Perfil fiscal, ambiente, certificado con `SecretStore` | Catálogo de ubicación (para la ubicación del emisor) |
| 5 | Catálogos (Notas del anexo oficial) y CABYS, importación versionada y búsqueda | Excel de CABYS, moneda y ubicación |
| 6 | Establecimientos, terminales, consecutivo y clave | Decisión 4 |
| 7 | XML 4.4 validado contra el XSD | — (XSD en el repo desde el 2026-10-12) |
| 8 | Firma XAdES-EPES | **PDF de la política de firma**; `.p12` de pruebas generado para los tests |
| 9 | Cliente real de Hacienda, worker, reintentos y contingencia, contra el sandbox | **Doc del API del sandbox, credenciales y `.p12` de pruebas**; decisión 5 |
| 10 | Eventos Accepted y Rejected, bandeja y reintento | — |
| 11 | Notas de crédito y débito, y anulación | Decisiones 2 y 3; propuestas C6 y C11 a contratos |
| 12 | PDF (QR apagado), métricas y alertas | — |
| 13 | Endurecimiento: aislamiento, autorrevisión, README, TODOs | — |

Con lo que hay hoy se puede avanzar sin bloqueos en **1–3, 6, 7 y 10**, y en parte de **4–5**.

## 5. Lo que necesito de usted

1. Aprobar o corregir los informes 0001 y 0003 y las **7 decisiones**. El 0002 lo entrego en cuanto la base acepte
   la conexión.
2. Traer a `docs/hacienda/` los **XSD** 4.4 de FE, NC, ND y `MensajeHacienda`, el **PDF de la política de firma**,
   los Excel de **moneda** y **ubicación**, el **CABYS** del BCCR y la **documentación del API del sandbox**. También
   me puede autorizar a descargar del CDN de Hacienda lo que tiene URL pública (XSD y política).
3. La **identificación de RDL como proveedor de sistemas** (`ProveedorSistemas`), o confirmar que todavía no existe.
4. Confirmar que la **revisión con un contador** antes del piloto queda agendada. Hay varios puntos marcados para
   esa revisión.
