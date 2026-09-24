# Prompt · P5 E-Invoice API y worker fiscal (servicio `fiscal`) en Go

> **Cómo usarlo**
> 1. Crea el repo `RDL.EInvoice.API` vacío (junto a `RDL.Platform.API`, `RDL.Contracts` y `RDL.Billing.API`) y copia en `docs/contexto/` el documento de arquitectura y el planning.
> 2. Copia en `docs/hacienda/` **todo** lo que haya de Hacienda: hoy, los PDF de `RDL PYMES/docs/Hacienda/` (la presentación de la versión 4.4 y el **borrador** de la resolución con sus tres anexos). En cuanto los tengas, agrega la resolución oficial **MH-DGT-RES-0027-2024**, los **Anexos y Estructuras v4.4 oficiales**, los **XSD**, los catálogos en Excel (moneda, ubicación, forma farmacéutica), la documentación web del API, el PDF de la política de firma y el catálogo **CABYS** del BCCR.
> 3. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`). Las credenciales de Hacienda y el certificado de pruebas **nunca** van en el prompt ni en el repo.
> 4. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 5. Revisa el plan antes de dejarlo implementar. Revisa a mano todo lo que toque firma, certificados, secretos, clave, consecutivos y la comunicación con Hacienda, y **presupuesta una revisión con un contador o asesor tributario antes del piloto** (planning P5).

---

## Rol y objetivo

Eres un ingeniero backend senior en Go con experiencia en integraciones con sistemas externos poco fiables, XML y firma digital. Vas a construir la **E-Invoice API y su worker fiscal** (servicio `fiscal`) de un SaaS B2B multiempresa de facturación electrónica para PYMES de Costa Rica. Esta API es dueña del **cumplimiento fiscal y de la comunicación con el Ministerio de Hacienda**: configuración fiscal por organización, catálogos (CABYS y los de Hacienda), establecimientos, terminales, consecutivos y clave numérica, generación y firma del XML, envío y consulta ante Hacienda, contingencia, y el historial de intentos y respuestas.

Es la **ruta crítica del plan**. Depende de cosas externas que no se aceleran con más código (el ambiente de pruebas de Hacienda, el certificado, la especificación vigente) y un error aquí tiene consecuencias legales: un comprobante mal formado, un consecutivo repetido o una firma inválida dejan a la PYME sin respaldo fiscal. **Ni este prompt, ni el planning, ni tú son fuente de verdad sobre Hacienda:** cada regla fiscal implementada debe trazar a un artículo o anexo guardado en `docs/hacienda/`, y la validación contra el XSD y contra el ambiente de pruebas es el criterio de verdad.

**Entregable:** el repositorio `RDL.EInvoice.API` listo para staging, con el alcance de las fases **F2 a F5** del planning. Debe compilar, pasar los tests (incluidos XML contra XSD, firma, clave y consecutivo, aislamiento entre tenants y pruebas contra el ambiente de pruebas de Hacienda), tener migraciones versionadas, Dockerfile, health checks y documentación.

## Contexto que debes leer primero

- `docs/hacienda/` (**la fuente de verdad fiscal**). Lo que ya se sabe del borrador de la resolución (septiembre 2024), que tienes que **contrastar con la versión oficial** en el Paso 1:
  - **Normativa:** Decreto 44739-H (Reglamento de comprobantes electrónicos, 08/11/2024), resolución MH-DGT-RES-0027-2024 (disposiciones técnicas, estructura 4.4) y MH-DGT-RES-0001-2025 (modifica el transitorio I). La versión 4.4 es **obligatoria desde el 1 de septiembre de 2025**.
  - **Anexo 1 (estructuras):** campos de encabezado, detalle, resumen, referencia y mensajes; **Nota 3** (consecutivo y clave); **Nota 4 y 4.1** (tipos de identificación y relleno a 12 dígitos para la clave); **Notas 5 a 23** (catálogos: condiciones de venta, medios de pago, impuestos, tarifas de IVA, códigos y tipos de referencia, exoneraciones, mensajes, unidades de medida, instituciones, etc.); formato numérico con punto decimal y **redondeo a 5 decimales, mitad hacia arriba**; cantidades de 16,3 y montos de 18,5.
  - **Anexo 2 (firma):** XAdES-EPES *enveloped* en `ds:Signature`, certificados RSA 2048 o 4096, digest SHA-256 o SHA-512, canonicalización exclusiva, URL de la política de firma, y la llave criptográfica que Hacienda entrega (RSA 2048 + SHA-256).
  - **Anexo 3 (API):** API REST en JSON y UTF-8; producción en `https://api.comprobanteselectronicos.go.cr/recepcion/v1/` y pruebas en `.../recepcion-sandbox/v1/`; token OAuth2 (*Resource Owner Password Credentials*) del IdP `https://idp.comprobanteselectronicos.go.cr/auth/...` que **vence a los 5 minutos**; recursos `POST /recepcion`, `GET /recepcion/{clave}`, `GET /comprobantes`, `GET /comprobantes/{clave}`; `callbackUrl` opcional con 3 reintentos. El client id y la URL del token del **ambiente de pruebas** no están en el anexo: salen de la documentación web del API.
  - **Artículos que cambian el diseño:** art. 9 (**un comprobante válido no se anula**: sus efectos se corrigen con notas de crédito o débito; un comprobante **rechazado por Hacienda carece de validez** y no lleva nota), art. 10 (mensaje receptor obligatorio para compras entre emisores, 8 días hábiles), art. 11 (Hacienda tiene **hasta 3 horas** para validar; sin aceptación no hay respaldo fiscal), art. 5 y transitorio III (PDF con QR, **el QR está suspendido** hasta aviso) y art. 7 (requisitos para los proveedores de sistemas, que es lo que somos).
- `docs/contexto/arquitectura-v1.md`: secciones **2.1**, **3.2** (E-Invoice), **6.1 a 6.4** (flujo de emisión, eventos, confiabilidad), **7.3 y 7.4** (auditoría, inmutabilidad y conservación de XML), **11** (decisiones de CABYS, consecutivos, almacenamiento de XML, retención) y **12** (criterios 2, 3 y 10).
- `docs/contexto/planning-v1.md`: sección **P5 E-Invoice** (fases, normativa y definición de terminado).
- **Repo de contratos** en `<<ruta a RDL.Contracts>>`. **El contrato manda en todo lo que no es fiscal**; si contradice a Hacienda, **manda Hacienda** y propones el cambio de contrato. Lee `docs/convenciones.md`, `docs/glosario.md`, `docs/ownership.md`, `state-machines/electronic-document.yaml` (sus transiciones son una **propuesta** para que tú las confirmes con la especificación), los eventos que consumes (`InvoiceIssued`, `CreditNoteIssued`, `DebitNoteIssued`, `InvoiceCancelled`) y que produces (`ElectronicDocumentAccepted`, `ElectronicDocumentRejected`), `openapi/fiscal.yaml` (esqueleto a completar), `problems/fiscal.yaml` y `docs/ESTADO.md` (pendientes fiscales). El paquete `pkg/events` se importa.
- **Platform API** en `<<ruta a RDL.Platform.API>>`: referencia de arquitectura y de lo resuelto (`pkg/tenancy`, auth con JWKS, transacción con `set_config`, correlación, Problem Details, idempotencia, auditoría, `internal/wiring`, `tests/isolation`, ADR 0003, 0004 y 0007).
- Si existen, `RDL.Billing.API` y `RDL.Receivables.API`: cómo Billing publica los eventos y cómo Receivables resolvió el consumidor (inbox, dead letter, `cmd/replay`). Reutiliza ese diseño.

## Datos del entorno

- Proyecto Supabase (dev): `<<project-ref>>`. **Nunca producción.** Conexión de solo lectura para inspeccionar: MCP de Supabase.
- Roles de base: existen `fiscal_app` y `fiscal_migrator` (NOLOGIN). Propón logins `fiscal_api` y `fiscal_migrate` con **el SQL para crearlos**; las contraseñas las asigno yo.
- Conexión de la aplicación: `fiscal_api` por Supavisor en modo transacción (puerto 6543), host `<<DB_POOLER_HOST>>`. Migraciones: `fiscal_migrate` en modo sesión (5432) con `SET role = 'fiscal_migrator'`.
- Identidad: Supabase Auth (JWT con `org_id` y `org_roles`). JWKS: `https://<<project-ref>>.supabase.co/auth/v1/.well-known/jwks.json`.
- Hacienda, **solo ambiente de pruebas**: usuario y contraseña del API y **llave criptográfica de pruebas (`.p12`) con su PIN**, que ya tiene `<<sí/no>>` y que van en `<<gestor de secretos o .env local ignorado por git>>`. **Nunca en el repo, en el chat, en logs ni en la base en claro.**
- Gestor de secretos (P2): `<<decidido o "pendiente">>`. Almacenamiento de archivos (XML, respuestas, PDF): `<<Supabase Storage, bucket de objetos o "pendiente">>`.
- Transporte de eventos (P2): `<<decidido o "pendiente">>`.
- Módulo del repo de contratos: `<<bitbucket.org/rdl/contracts>>` en `<<vX.Y.Z>>`. Versión de Go: `<<1.27+>>`.

## Paso 1 · Fuentes, brechas y decisiones (obligatorio)

Entrega tres informes y **detente hasta que los apruebe**:

**1. `docs/decisiones/0001-fuentes-de-hacienda.md`: qué hay, qué falta y qué cambió**
- Inventario de `docs/hacienda/` con la versión y la fecha de cada documento.
- Si solo está el borrador, dilo en la primera línea y trabaja sobre él marcando cada regla con `FUENTE: borrador sept-2024` para revalidarla. Si está la versión oficial, **compara borrador y oficial** y lista las diferencias que afecten al código.
- Lo que falta para cada fase (XSD, catálogos en Excel, documentación del API de pruebas, política de firma, CABYS) y a qué bloquea.

**2. `docs/decisiones/0002-estado-inicial-bd.md`: brechas de la base**
El schema `fiscal` ya existe con 27 tablas: documentos (`electronic_documents`, `electronic_document_lines`, `electronic_document_line_taxes`, `electronic_document_status_history`, `document_files`, `submission_attempts`), configuración (`taxpayer_profiles`, `certificates`, `establishments`, `terminals`, `document_sequences`, `organization_economic_activities`) y catálogos (`cabys_versions`, `cabys_items`, `identification_types`, `tax_types`, `tax_rates`, `sale_conditions`, `payment_methods`, `units_of_measure`, `exoneration_document_types`, `document_types`, `economic_activities`, `provinces`, `cantons`, `districts`). Fíjate en que las credenciales se guardan **por referencia** (`hacienda_username_secret_id`, `hacienda_password_secret_id`, `pin_secret_id`) y los archivos **por ruta** (`storage_path` con `sha256`). **No supongas el esquema: léelo** (columnas, `CHECK`, unicidades, FK, RLS, grants de `fiscal_app` sobre `core`, `billing`, `audit` e `integration`) y compáralo con la estructura 4.4:
- campos del XML 4.4 que el modelo no tiene (por ejemplo: identificación del **proveedor de sistemas**, actividad económica del receptor, tipo de transacción, desglose de medios de pago y de impuestos, totales por gravado, exento, exonerado y no sujeto, campos de exoneración 4.4 como institución codificada y ley, artículo e inciso, códigos de descuento, IVA a nivel de fábrica);
- catálogos vacíos (hoy lo están todos) y cómo se cargarán desde `docs/hacienda/` con su versión;
- el SQL exacto de lo que propones crear (expand → migrate → contract) y lo que es de `database-platform`.

**3. `docs/decisiones/0003-mapeo-eventos-a-comprobantes.md`: del contrato al XML**
- Para cada campo del XML de **Factura Electrónica, Nota de Crédito y Nota de Débito** 4.4: de dónde sale (evento de Billing, configuración fiscal de la organización, catálogo o cálculo), con la **sección del anexo** de la que sale la regla.
- Datos que el evento no trae y el comprobante exige: lístalos como **propuesta de cambio al repo de contratos** (versión nueva del evento o campo opcional), no los inventes.
- Validaciones que harás antes de firmar (formato, catálogos, cuadre de totales con el redondeo de la especificación): Billing calcula y E-Invoice **valida** (decisión D2 del repo de contratos).

**Decisiones que necesito tomar contigo** (propón una opción y espera):
1. **Comprobantes de V1.** Propuesta: Factura (01), Nota de débito (02) y Nota de crédito (03). ¿Tiquete (04) para ventas a consumidor final? Fuera de V1: factura de compra (08), de exportación (09), recibo electrónico de pago (10) y el **mensaje receptor** de compras (05/06/07), que exigiría recibir comprobantes de proveedores.
2. **Anulación.** Hacienda no anula comprobantes válidos (art. 9): se corrigen con una **nota de crédito con código de referencia 01 ("Anula documento de referencia")**. Hoy el contrato tiene `InvoiceCancelled`. Opciones: (a) al anular, Billing emite una nota de crédito y E-Invoice la procesa como cualquier `CreditNoteIssued`; (b) E-Invoice genera la nota de crédito fiscal a partir de `InvoiceCancelled` con las líneas del documento original. Si el documento fue **rechazado** por Hacienda, no hay nota (carece de validez). Esto cambia el repo de contratos: no lo resuelvas solo.
3. **Rechazados.** La corrección de un rechazado es un comprobante nuevo que lo sustituye (tipo de documento de referencia 10, con los límites de fecha de la nota 28). ¿Quién lo origina (Billing, con una referencia al rechazado que hoy el evento no tiene) y cómo se muestra en la bandeja?
4. **Alcance del consecutivo (D11).** La Nota 3 fija establecimiento (3 dígitos; 001 = casa matriz), terminal (5) y numeración por tipo de comprobante **por establecimiento o terminal**. Propón la relación sucursal (`core.branches`) ↔ establecimiento ↔ terminal y quién elige la terminal al emitir.
5. **Recepción de la respuesta:** consultar el estado (`GET /recepcion/{clave}`) con backoff, usar `callbackUrl` (exige un endpoint público) o ambos.
6. **Secretos y archivos:** dónde viven las credenciales, el `.p12` y el PIN (gestor de secretos de P2) y los XML, respuestas y PDF (almacenamiento de objetos), mientras P2 no esté listo.
7. **Retención** de XML y respuestas: el Código Tributario (art. 109) habla de 5 años para los duplicados; el plazo exacto lo confirma asesoría legal. Déjalo configurable y con un TODO.

## Paso 2 · Cambios de base de datos (solo si hacen falta)

- **goose**, historial en `fiscal.goose_db_version`, baseline idempotente marcada como aplicada en dev, comando `migrate up-by-one`.
- Esta API **solo migra `fiscal`**. Nada sobre `core`, `billing`, `receivables` ni `subscriptions`; lo de `shared`, `audit` o `integration` va como propuesta a `database-platform`.
- **Expand → migrate → contract**; nada destructivo. Montos con los dominios de `shared` (`numeric`), nunca `float`.
- Los **catálogos** (CABYS y los de Hacienda) se cargan con migraciones o un comando de importación versionado (`cmd/import-catalogs`), guardando la versión y el `sha256` de la fuente, **desde los archivos de `docs/hacienda/`**. Nunca a mano ni desde memoria.
- Muéstrame el SQL antes de aplicarlo, solo en dev y local.

## Paso 3 · Tenancy, eventos y secretos

- **HTTP:** igual que Platform (JWT, revalidación de membresía con caché, `TenantContext`, `set_config` por transacción, `QueryExecModeExec`, `jsonb` como texto).
- **Eventos entrantes:** el mismo diseño que Receivables. Validación con `events.Validator`, tenant = `organizationId` del evento, inbox (`consumer_service = 'fiscal'`) para no crear dos documentos con el mismo `eventId` (criterio 3), dead letter para lo inválido, y efecto + audit + outbox en la misma transacción. El transporte es de P2: puerto `EventSource`, adapter de desarrollo que reproduce `examples/events/` del repo de contratos y `TODO(P2)`. **No inventes un broker.**
- **Eventos salientes:** `ElectronicDocumentAccepted` y `ElectronicDocumentRejected` en el outbox, en la misma transacción que el cambio de estado, validados contra su schema.
- **Secretos:** detrás de un puerto (`SecretStore`), con un adapter de desarrollo que lee de variables de entorno o de un archivo local ignorado por git. El `.p12` y su PIN solo existen en memoria mientras se firma; nunca se escriben en disco en claro, en logs, en audit events ni en respuestas. El certificado de tests es uno **generado para eso** en `tests/fixtures/`, nunca el de Hacienda.
- **Archivos:** detrás de un puerto (`BlobStore`) con un adapter local para desarrollo; la base guarda la ruta y el `sha256`, y la descarga verifica el hash.

## Paso 4 · Alcance funcional

**F2: esqueleto.** Proyecto, config, logger, OpenTelemetry, health, Dockerfile, pipeline, logins, baseline, `docs/hacienda/` inventariado y un primer endpoint protegido por tenant.

**F3: flujo vertical con Hacienda simulada**
- Consumidor de `InvoiceIssued`: crea el `electronic_document` con el **snapshot** del emisor (perfil fiscal), el receptor y las líneas, en `processing`, idempotente por `eventId` y por documento de origen.
- **Stub de Hacienda** (un adapter del puerto `HaciendaClient` que responde aceptado, rechazado o timeout según la clave) para cerrar el flujo vertical antes de la integración real.
- Ruta interna `GET /internal/v1/electronic-documents/by-source/{sourceDocumentId}` para el Portal Gateway (y su versión por lote si el repo de contratos la aprueba).

**F4: cumplimiento real**
| Tema | Qué incluye |
|---|---|
| Configuración fiscal | Perfil del contribuyente, **ambiente** (pruebas o producción, imposible de confundir), actividades económicas, credenciales de Hacienda por referencia, carga del certificado (valida que abre con el PIN, que no está vencido y guarda sujeto, emisor, serie y vigencia) |
| Establecimientos y terminales | Códigos de 3 y 5 dígitos, relación con las sucursales según la decisión 4 |
| Catálogos | Importación versionada desde `docs/hacienda/` y el CABYS del BCCR; búsqueda de CABYS (`GET /v1/catalogs/cabys?q=`) y catálogos para formularios |
| Consecutivo | Por establecimiento, terminal y tipo, asignado **bajo bloqueo** en la misma transacción, sin huecos ni duplicados por concurrencia; reinicio al tope según la Nota 3 |
| Clave numérica | `506` + día, mes y año (`ddmmaa`) + identificación del emisor rellenada a 12 dígitos (Nota 4.1) + consecutivo (20) + situación (1 normal, 2 contingencia, 3 sin internet) + código de seguridad de 8 dígitos generado con `crypto/rand` |
| XML | Generación por tipo de comprobante **validada contra el XSD oficial** en cada emisión; cada campo con un comentario que cite su sección del anexo; montos con punto decimal, sin separador de miles y redondeados según la especificación |
| Firma | XAdES-EPES *enveloped* con el certificado de la organización, según el Anexo 2 |
| Hacienda | Token OAuth2 con caché y renovación (vence a los 5 min), envío, consulta de estado, respuestas y mensajes de Hacienda guardados, **reintentos con backoff** por tipo de error (4xx no se reintenta igual que 5xx o timeout) |
| Contingencia | Hacienda no disponible → estado `contingency`, reenvío posterior con la situación correspondiente en la clave; procedimiento según la resolución |
| Persistencia | XML sin firmar, firmado, respuesta de Hacienda y PDF en el `BlobStore`, con `sha256` en `document_files`; cada intento en `submission_attempts` |
| Eventos | `ElectronicDocumentAccepted` y `ElectronicDocumentRejected` (con el motivo de Hacienda) |

**F5: notas, rechazos y operación**
- Documentos fiscales para `CreditNoteIssued` y `DebitNoteIssued` (información de referencia obligatoria con la clave del comprobante referenciado y el código de referencia de la Nota 9), y el tratamiento de la anulación según la decisión 2.
- **Bandeja de rechazados, en contingencia y con error** con motivo y acción de corrección (criterio 10): `GET /v1/electronic-documents?status=...`, detalle con intentos, descargas y `POST /v1/electronic-documents/{id}/retry` (solo desde `error`).
- **PDF** (representación gráfica) con los campos de la Nota 1 juntos; el QR queda detrás de una opción apagada mientras siga el transitorio III.
- Métricas y alertas: tasa de rechazo, documentos en `processing` más allá del plazo de 3 horas de Hacienda, cola estancada, errores de firma y certificados por vencer.

**Máquina de estados:** implementa `state-machines/electronic-document.yaml` **después de confirmarla** contra la especificación. Cada transición que cambies se propone como PR al repo de contratos con su justificación.

**Transversales** (como Platform): Problem Details con los tipos de `problems/fiscal.yaml`, `Idempotency-Key` en los `POST`, `X-Correlation-Id` hasta el audit event y el evento, **audit event** en cada operación sensible (configuración, certificado, emisión, reintento), paginación por cursor, 404 para otra organización, 403 por rol.

## Paso 5 · Arquitectura y calidad de código

Hexagonal (ports & adapters) con Clean Architecture, como Platform.

```
RDL.EInvoice.API/
├── cmd/
│   ├── api/                     # servidor HTTP
│   ├── consumer/                # consumidor de eventos de Billing
│   ├── worker/                  # firma, envío, consulta de estado, reintentos y contingencia
│   ├── replay/                  # adapter de desarrollo de eventos
│   ├── import-catalogs/         # importación versionada de catálogos y CABYS desde docs/hacienda/
│   └── migrate/
├── internal/
│   ├── domain/                  # sin imports de infraestructura
│   │   ├── document/            # agregado DocumentoElectrónico, máquina de estados, snapshot, validaciones
│   │   ├── numbering/           # consecutivo y clave numérica (funciones puras, con cita a la Nota 3)
│   │   ├── rounding/            # redondeo de la especificación y verificación de totales
│   │   ├── catalog/             # catálogos versionados
│   │   └── permission/
│   ├── app/                     # casos de uso + puertos (HaciendaClient, Signer, XSDValidator, SecretStore, BlobStore, EventSource)
│   ├── adapters/
│   │   ├── http/  postgres/  auth/  events/
│   │   ├── xml/                 # mapeo documento → XML 4.4 por tipo, con comentarios de sección
│   │   ├── xsd/                 # validación contra los XSD oficiales
│   │   ├── xades/               # firma XAdES-EPES
│   │   ├── hacienda/            # cliente real (OAuth2, recepción, consulta) y stub
│   │   ├── secrets/  blobstore/ # adapters de desarrollo
│   │   └── pdf/
│   ├── wiring/
│   └── platform/
├── docs/hacienda/               # especificación, anexos, XSD y catálogos (fuente de verdad, solo lectura)
├── tests/{fixtures,integration,isolation,sandbox}/
├── pkg/{tenancy,correlation,requestinfo}/
├── migrations/  queries/  sqlc/external.sql  api/openapi.yaml
├── CLAUDE.md, README.md, Dockerfile, Makefile, .golangci.yml, sqlc.yaml
```

**Decisiones técnicas con ADR:**
- **Validación XSD:** Go no tiene un validador XSD completo en puro Go. Evalúa `libxml2` vía cgo (por ejemplo `lestrrat-go/libxml2`) frente a `xmllint` en tests y CI, y su impacto en la imagen distroless.
- **Firma XAdES-EPES:** evalúa construirla sobre una librería de XMLDSig (por ejemplo `russellhaering/goxmldsig` con `beevik/etree`) agregando las `SignedProperties` de XAdES. La prueba de verdad es que el ambiente de pruebas de Hacienda la acepte.
- **Decimales:** `shopspring/decimal` o `cockroachdb/apd` con el redondeo de la especificación, en `domain/rounding` y con tests de los ejemplos del anexo.
- Worker: cola sobre la base (`next_retry_at` en `submission_attempts` con `FOR UPDATE SKIP LOCKED`) o sobre el transporte de P2; política de reintentos y de contingencia.

**Principios:** dominio rico (el documento protege sus transiciones y su inmutabilidad: lo firmado no se regenera, se envía el mismo XML); SRP, ISP y DIP como en Platform; toda regla fiscal con un comentario `// Anexo 1, Nota 3` o `// Art. 9`; `context.Context` y timeouts en toda llamada a Hacienda; graceful shutdown del worker (terminar el envío en curso); errores tipados y sin detalle interno al cliente; cero secretos en el código.

## Paso 6 · Tests

- **Clave y consecutivo** (unitarios y de propiedades): largo exacto (50 y 20), cada segmento en su posición, relleno de identificación por tipo (Nota 4.1), situación, código de seguridad aleatorio, reinicio al tope, y **ningún consecutivo repetido** con emisiones concurrentes en el mismo establecimiento, terminal y tipo (integración con goroutines).
- **Redondeo y totales:** los ejemplos de redondeo del anexo y el cuadre de totales de los ejemplos válidos de `InvoiceIssued` del repo de contratos.
- **XML:** cada tipo de comprobante generado desde los ejemplos del repo de contratos **valida contra su XSD oficial**; un test tabla por campo obligatorio (si falta en el modelo, el test lo reporta como brecha).
- **Firma:** firma y verificación con el certificado de `tests/fixtures/`; el XML firmado sigue validando contra el XSD; alterar un byte invalida la firma.
- **Cliente de Hacienda:** con un servidor falso (`httptest`): token y renovación a los 5 minutos, envío, consulta, reintentos por tipo de error, timeout, contingencia y respuestas de rechazo.
- **Consumidor:** cada evento válido crea un solo documento; **el mismo evento dos veces no crea un segundo documento** (criterio 3); un payload inválido va a dead letter.
- **Bandeja** (criterio 10): un rechazo aparece con su motivo y la acción de corrección correspondiente.
- **Secretos:** ningún log, audit event, respuesta HTTP ni archivo contiene el PIN, la contraseña de Hacienda o la llave privada (búsqueda en la salida de los tests).
- **Aislamiento** (`make test-isolation`), los 6 criterios de Platform: documentos, configuración, certificados, establecimientos y archivos de A inaccesibles con el token de B; `fiscal_app` no escribe en schemas ajenos; un evento de B solo afecta a B; y **el certificado de A nunca firma un documento de B**.
- **Ambiente de pruebas de Hacienda** (`tests/sandbox`, build tag `sandbox`, fuera del CI normal): enviar una factura, una nota de crédito y una de débito reales al sandbox con el certificado de pruebas y llegar a **aceptado**. Es el criterio de verdad de la firma y el XML.

## Paso 7 · Operación y entrega

- Health de la API, del consumidor y del worker; métricas del Paso 4.
- Dockerfile multi-stage (considerando cgo si se usa `libxml2`), usuario no root, binarios `api`, `consumer`, `worker`, `replay`, `import-catalogs` y `migrate`.
- `Makefile` con `run`, `run-consumer`, `run-worker`, `replay`, `import-catalogs`, `build`, `test`, `test-isolation`, `test-sandbox`, `lint`, `migrate-up`, `migrate-status`, `sqlc`.
- `api/openapi.yaml` alineado con `openapi/fiscal.yaml` del repo de contratos.
- `CLAUDE.md` con las reglas de plataforma y las del repo: "`docs/hacienda/` es la fuente de verdad; toda regla fiscal cita su sección", "no se inventan códigos ni campos", "el XML valida contra el XSD antes de firmarse", "lo firmado no se regenera", "ningún secreto fuera del `SecretStore`", "solo el ambiente de pruebas de Hacienda", "prohibido `float`", y el comando de cierre con `make test-isolation`.
- `README.md` al nivel del de Platform, incluido cómo cargar catálogos, cómo reproducir eventos y cómo correr las pruebas contra el sandbox.
- `docs/ESTADO.md` como handoff, con la lista de reglas marcadas `FUENTE: borrador` pendientes de revalidar.

## Forma de trabajo

1. **Plan primero:** los tres informes y las 7 decisiones del Paso 1, SQL de logins, plan de migraciones, ADRs técnicos y lista de historias. Espera mi aprobación.
2. **Incrementos verticales**, cada uno compilando y en verde:
   1. esqueleto, config, health, logins, baseline e inventario de `docs/hacienda/`;
   2. JWT, TenantContext y primer endpoint;
   3. consumidor de `InvoiceIssued`, documento en `processing` y stub de Hacienda (flujo vertical, F3);
   4. configuración fiscal, certificado (con `SecretStore`) y ambiente;
   5. catálogos y CABYS (importación versionada y búsqueda);
   6. establecimientos, terminales, consecutivo y clave;
   7. XML 4.4 validado contra XSD;
   8. firma XAdES-EPES;
   9. cliente real de Hacienda, worker, reintentos y contingencia, **probados contra el sandbox**;
   10. eventos Accepted y Rejected, bandeja y reintento;
   11. notas de crédito y débito y anulación según la decisión 2;
   12. PDF, métricas y alertas;
   13. endurecimiento: aislamiento, autorrevisión, README y TODOs.
3. Al terminar cada incremento: build, vet, lint y tests; actualiza `docs/ESTADO.md`; resume lo hecho y lo pendiente, y **pausa**.
4. Antes de terminar, revisa tu diff buscando: reglas fiscales sin cita, códigos o campos inventados, XML que no pase el XSD, secretos en cualquier salida, un certificado usado fuera de su organización, consecutivos que puedan repetirse, uso del ambiente de producción, `float`, consultas sin tenant y escrituras a schemas ajenos.

## Prohibido

- Conectarte al **ambiente de producción de Hacienda** o a producción de la base; usar certificados, credenciales o datos reales en tests.
- Escribir o imprimir el PIN, la contraseña de Hacienda o la llave privada fuera del `SecretStore` y de la memoria del proceso de firma.
- Inventar códigos, catálogos, campos, tarifas o reglas de Hacienda: si no está en `docs/hacienda/`, es un TODO.
- Regenerar o modificar un XML ya firmado o enviado; anular un comprobante válido por otra vía que no sea la nota que exige Hacienda.
- Tomar el tenant de algo que no sea el token (HTTP) o el `organizationId` del evento (consumidor); leer el outbox de otro servicio.
- Usar `float`; migrar schemas ajenos o hacer cambios destructivos sin expand/contract.

## Definición de terminado

- [ ] Informes de fuentes, base y mapeo aprobados; las 7 decisiones tomadas y reflejadas en el repo de contratos cuando lo cambian.
- [ ] Catálogos y CABYS cargados desde `docs/hacienda/` con versión y hash.
- [ ] Factura, nota de crédito y nota de débito 4.4 generadas, **validadas contra XSD**, firmadas y **aceptadas en el ambiente de pruebas de Hacienda**.
- [ ] Consecutivo y clave conforme a la Nota 3, sin duplicados bajo concurrencia.
- [ ] Reintentos, contingencia e historial de intentos; XML, respuestas y PDF guardados con hash.
- [ ] Reprocesar un evento no crea un segundo documento (criterio 3); un rechazo aparece en la bandeja con su motivo y su acción de corrección (criterio 10).
- [ ] `ElectronicDocumentAccepted` y `ElectronicDocumentRejected` en el outbox, validados contra sus schemas.
- [ ] Ningún secreto en logs, auditoría, respuestas ni repo; suite de aislamiento en verde.
- [ ] Toda regla fiscal con su cita; lista de reglas `FUENTE: borrador` pendientes de revalidar con la versión oficial.
- [ ] Revisión con un contador o asesor tributario agendada antes del piloto; lista final de TODOs para el equipo.
