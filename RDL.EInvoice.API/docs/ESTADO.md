# Estado del proyecto · E-Invoice API (servicio `fiscal`)

**Última actualización:** 2026-10-12

## Sesión del 2026-10-12 · Paso 1 e incremento 1

**Paso 1 (aprobado):** informes 0001 (fuentes), 0002 (base) y 0003 (mapeo) y las 7 decisiones de `docs/PLAN.md`
(anulación: opción **a**, Billing emite la NC código 01). XSD 4.4 oficiales en `docs/hacienda/xsd/`.

**Incremento 1 (esqueleto): ✅ terminado y verificado en dev el 2026-10-12.**
- `go.mod` (`rdl/einvoice-api`, contratos v0.2.0 con `replace`), config validada (puertos 8082/8085/8086,
  `HACIENDA_ENVIRONMENT` solo `sandbox`, `PROVIDER_IDENTIFICATION` opcional, `RETENTION_YEARS` = 5), logger,
  OpenTelemetry, `/healthz` y `/readyz` (base y JWKS), `cmd/api`, `cmd/migrate` (con `mark-baseline`).
- Copiado de Receivables (que copió de Platform): config, health, logger, telemetry, pool, `pkg/correlation`,
  `cmd/migrate`; `internal/archtest` de Billing (sin `float` en Go ni SQL) + prueba del manifiesto de `docs/hacienda/`.
- Migraciones: `00001_baseline.sql` (reproduce el schema de dev, idempotente) y `00002_hacienda_4_4.sql` (B1–B13 y
  W1–W3 del informe 0002, con su `Down`).
- `scripts/dev/fiscal_login_roles.sql`: logins, lectura de `core` y `references` sobre `core.branches` (propuesta a
  `database-platform`).
- Makefile, Dockerfile, `bitbucket-pipelines.yml`, `.env.example`, `.gitattributes` (las fuentes de Hacienda sin
  conversión de fin de línea), `CLAUDE.md`, `README.md`.
- Verificado: `go build`, `go vet` (con y sin `integration`), `go test ./...` (5 paquetes con pruebas, en verde) y `golangci-lint`
  (0 issues). **`-race` no se corrió:** `gcc` no está en el `PATH` de esta máquina (el tablero dice que se instaló el
  2026-09-27).

### Verificación en dev (2026-10-12)

- Luis corrió `scripts/dev/fiscal_login_roles.sql` y creó el `.env`: `fiscal_api` (miembro de `fiscal_app`) y
  `fiscal_migrate` (`set role = fiscal_migrator`); `fiscal_app` ya lee `core.organizations`, `organization_users`,
  `organization_user_roles`, `users` y `branches`.
- `migrate mark-baseline` (00001 registrada sin ejecutar) y `migrate up-by-one` (00002 aplicada en 2,5 s). Verificado
  por MCP: clave alfanumérica, UK por documento de origen, `request_type` ampliado, guardián con `next_attempt_at` y
  `last_error`, 7 tablas nuevas con RLS y dueño `fiscal_migrator`.
- `cmd/api` con `fiscal_api`: `/healthz` 200 y `/readyz` 200 (`database` ok, `jwks` ok).

## Incremento 2 (JWT, TenantContext y primer endpoint): ✅ 2026-10-12

- Copiado de Receivables: `internal/adapters/auth` (JWKS de Supabase, ES256), `pkg/tenancy` (middleware, caché de
  membresías de 30 s), `pkg/requestinfo`, Problem Details (`urn:rdl:fiscal:problem:*`), `postgres/tx.go`
  (`set_config(..., true)`) y la revalidación de membresía (`queries/membership.sql`).
- `internal/domain/permission`: la matriz completa de fiscal, tomada de los roles de `openapi/fiscal.yaml`.
- `GET /v1/fiscal-profile` (owner, admin, accountant): perfil + actividad principal + vencimiento del certificado
  activo; nunca secretos (`haciendaCredentialsSet` en vez de las referencias). 404 si no hay perfil.
- `api/openapi.yaml` (0.2.0) con un test que lo compara con el router. La forma de `FiscalProfile` amplía el
  esqueleto de contratos: **propuesta pendiente a `RDL.Contracts`**.
- `tests/isolation` (tag `integration`) contra dev: sin token, token desconocido, sin `org_id`, membresía
  suspendida, `org_id` ajeno en el token (403), tenant que no cambia por query ni header, privilegios de `fiscal_app`
  (sin escritura en `core`, `billing`, `receivables`, `subscriptions`; no ve el outbox ajeno) y RLS por organización
  en las 13 tablas de `fiscal` con `organization_id`. **4/4 en verde.**
- Organizaciones de prueba en el `.env` local: A = `91c6f756…` (E2E, con miembro suspendido), B = `6c78af3f…`
  (Aislamiento GW B). El testkit aborta si el owner de una es miembro activo de la otra: con el primer B que elegí
  (`464717c0…`) pasaba y el caso cruzado daba un falso 404.
- En vivo: `/readyz` 200 con `fiscal_api`; `/v1/fiscal-profile` sin token o con un token falso → 401 con Problem
  Details. **Sin probar con un JWT real de Supabase** (la contraseña de los usuarios de prueba no está en esta máquina).
- Verificado: build, vet (con y sin `integration`), `golangci-lint` 0 issues, `go test ./...` (9 paquetes) y la suite
  de aislamiento. Sin `-race` (no hay `gcc`).

## Incremento 3 (consumidor, documento en processing, stub de Hacienda; F3): ✅ 2026-10-12

- **Catálogos:** `catalogs/*.csv` transcriben las Notas 3 (tipos de comprobante, pág. 66), 4 (tipos de
  identificación, pág. 69) y 6 (medios de pago, pág. 71) del anexo oficial, cada uno con su cita y el sha256 del PDF
  (una prueba lo compara con `docs/hacienda/manifest.sha256`). `cmd/import-catalogs` los carga con `fiscal_migrate`
  y registra versión + hash de la transcripción en `fiscal.catalog_imports`. **Cargados en dev** (10, 6 y 8 filas);
  idempotente. Ojo: `fiscal.payment_methods` estaba vacío y ahora tiene datos (Receivables lo usa).
- **Consumidor** (copiado de Receivables, ADR 0005 de su repo): `cmd/consumer` (:8085, `NoSource` hasta P2),
  `cmd/replay`, inbox `consumer_service = 'fiscal'`, 6 reintentos, dead letter en transacción aparte, tenant = el
  `organizationId` del evento (`WithinServiceTx`, sin usuario).
- **`InvoiceIssued` → documento en `processing`** (`internal/domain/document` + `app.CreateDocumentFromInvoice`): snapshot
  del receptor (evento), del emisor (perfil fiscal al recibir), líneas, impuestos con exoneración en puntos de tarifa,
  medios de pago (máx. 4, Anexo 1 pág. 55), historial y audit `electronic_document.created`. Sin terminal,
  consecutivo, clave ni XML todavía (incrementos 6 a 8). `next_attempt_at = now()` para el worker.
- Permanentes (dead letter al primer intento): payload inválido, tipo sin consumidor (notas y anulación hasta el
  incremento 11), organización **sin perfil fiscal** (sin emisor), otro evento del mismo documento comercial, más de
  4 medios de pago o de 1000 líneas.
- **Rutas internas del BFF:** `GET /internal/v1/electronic-documents/by-source/{id}` y el lote `POST …/by-source`
  (propuesta de contratos sin publicar), todos los roles, 404 entre organizaciones.
- **Stub de Hacienda** (`internal/adapters/hacienda`, puerto `app.HaciendaClient`): aceptado, rechazado o timeout
  según el último carácter de la clave; primera consulta «pendiente». **Sin usar todavía:** lo conecta el worker
  (incremento 9). Los valores de `indEstado` del API real (salvo `RECIBIDO`) no están en `docs/hacienda/`.
- **Máquina de estados** de contratos en `document.CanTransition` (propuesta C3, sin confirmar).
- Verificado: build, vet, `golangci-lint` 0, 14 paquetes en verde; **integración contra dev 2/2** (un solo documento
  por factura al reprocesar, otro evento del mismo documento → dead letter, by-source y lote con 404 cruzado, evento
  de B sin perfil → dead letter sin efectos); **aislamiento 4/4**; `cmd/replay` con los ejemplos de contratos (van a
  dead letter: su organización no tiene perfil). Verificado por MCP: inbox, dead letters y audit con `fiscal`.
- Datos de prueba que quedan en dev: perfil fiscal ficticio en ISOLATION_ORG_A (`3101000001`) y los documentos de
  las corridas de la integración.

## Incremento 4 (configuración fiscal, credenciales y certificado): ✅ 2026-10-12

- **`PUT /v1/fiscal-profile`** (owner, admin): `internal/domain/taxpayer` valida el nodo Emisor del Anexo 1 —número
  por tipo de identificación (pág. 20: física 9 dígitos sin cero inicial, jurídica 10 caracteres, DIMEX 11–12,
  NITE 10), nombre 5–100 y comercial 3–80 (págs. 19 y 21), correo con la regex de la pág. 22, teléfono 8–20
  dígitos, ubicación con otras señas 5–250— y el caso de uso verifica tipo de identificación, distrito y actividad
  contra sus catálogos. **`environment = production` → 422** (prohibido en V1, además de `HACIENDA_ENVIRONMENT`).
  La actividad indicada queda como la única principal. Audit `fiscal.profile.created/updated`.
- **`PUT /v1/fiscal-profile/hacienda-credentials`** (owner): usuario y contraseña del API (Anexo 3, pág. 98) al
  SecretStore; la base guarda solo las referencias y las anteriores se borran después del commit. Exige perfil (409).
  **Ruta nueva, a proponer en contratos.** Permiso `HaciendaCredentialsWrite`.
- **`PUT /v1/fiscal-profile/certificate`** (owner, multipart `certificate` + `pin`): se lee en memoria, sin
  `ParseMultipartForm` (no toca disco), máx. 64 KiB. `go-pkcs12` (SSLMate v0.7.4, admite p12 modernos con AES) abre
  el archivo; `internal/domain/certificate` exige **RSA 2048 o 4096** (Anexo 2, págs. 84–85) y vigencia. PIN
  incorrecto y archivo inválido no se distinguen (`certificate-invalid`); vencido → `certificate-expired`. Archivo y PIN
  al SecretStore; `fiscal.certificates` con sujeto, emisor, serie y vigencia, `storage_path = secret://<id>`, un solo
  activo. Audit `fiscal.certificate.uploaded` sin secretos.
- **SecretStore de desarrollo** (decisión 6): `internal/adapters/secrets.FileStore`, AES-256-GCM, un archivo por
  secreto en `SECRETS_DIR/<organización>/` y **la organización como dato asociado**: un secreto de A no se descifra
  como de B ni copiando el archivo. Llave `SECRETS_MASTER_KEY` (32 bytes base64) en el `.env` local (generada en esta
  máquina, no se comparte). Sin llave la API arranca y esas rutas responden 503.
- Certificados de prueba generados al vuelo (`internal/testcert`, autofirmados «PRUEBA RDL»): ninguna llave en el repo.
- `api/openapi.yaml` 0.4.0 con las tres rutas.
- Verificado: lint 0, `go test ./...` en verde (17 paquetes); **integración 4/4** contra dev (perfil, 422 por
  formato y catálogos, credenciales, certificado con PIN incorrecto, vencido, válido dos veces → un solo activo,
  ningún secreto en base, auditoría ni disco, nada en la carpeta de B); **aislamiento 5/5** (caso nuevo: con la
  sesión de B no se leen ni modifican perfil, certificado ni documentos de A).
- Datos de prueba en dev: el perfil de ISOLATION_ORG_A tiene credenciales y certificados de prueba cuyos secretos
  vivían en un directorio temporal ya borrado (las filas apuntan a referencias que no existen; no sirven para firmar).
- **Pendiente:** el catálogo de ubicación (Nota 14) y el padrón de actividades: hoy un perfil con ubicación o
  actividad responde 422 «todavía sin cargar». Verificar que el certificado sea del mismo contribuyente (TODO en
  `domain/certificate`). Qué hacer con los secretos del certificado reemplazado (TODO, decisión 7).

## Incremento 5 (catálogos y CABYS): ✅ 2026-10-12

- **Fuentes nuevas** en `docs/hacienda/catalogos/` (con su README y su sha256 en el manifiesto): ubicación
  (`Codificacionubicacion_V4.4.xlsx`) y moneda (`Codigodemoneda_V4.4.pdf`) de `hacienda.go.cr/docs/`, y el **CABYS
  2025** del BCCR (Excel + comunicado CP-BCCR-015-2025).
- **Transcritos** a `catalogs/*.csv` desde el anexo oficial: Notas 5 (condiciones de venta, con `requires_credit_term`
  para 02 y 10, pág. 25), 8 (impuestos), 8.1 (tarifas de IVA, con su `rate`), 9 (códigos de referencia), 10 (tipos de
  documento de referencia), 10.1 (exoneración), 15 (101 unidades, sensibles a mayúsculas), 20 (descuentos), 23
  (instituciones) y la moneda (168). **Cada uno coincide con la enumeración del XSD oficial** (prueba
  `catalogs/xsd_test.go`); la única diferencia es del recibo electrónico de pago (código de referencia 17).
- **Leídos tal cual** (sin transcribir) con `excelize`, verificando el sha256 antes: 7 provincias, 84 cantones y 492
  distritos; 20 506 códigos CABYS como versión vigente. El impuesto del CABYS se lee como texto y se lleva a puntos con
  decimal exacto.
- **Migración 00003** (expand, aplicada en dev): `cabys_items.tax_label` para «Exento» (885) y «na» (3), que no son
  0 % (Exento ≈ tarifa 10 de la Nota 8.1). La tarifa queda NULL; nunca se inventa.
- `cmd/import-catalogs`: upserts por lote con `unnest` y `COPY` para el CABYS (8 s en total contra dev); idempotente;
  registra cada carga en `fiscal.catalog_imports`. **Cargado en dev.** Nota: el hash de transcripción de las Notas 3, 4
  y 6 cambió de formato y quedaron registradas dos veces (mismo contenido).
- **Endpoints** (todos los roles): `GET /v1/catalogs/cabys?q=&limit=` (prefijo del código o texto en la descripción;
  `%` y `_` escapados) y `GET /v1/catalogs/{catalog}` (los 8 del contrato + monedas, códigos y tipos de referencia,
  descuentos, instituciones, provincias, cantones y distritos con `provinceCode`/`cantonCode`). `api/openapi.yaml` 0.5.0.
- El perfil fiscal ya acepta la ubicación del emisor (se verifica contra el catálogo).
- Verificado: lint 0, 17 paquetes en verde; **integración 6/6** contra dev (CABYS por texto, por prefijo, exento,
  comodines, límites; catálogos con tarifas, plazos, unidades, monedas, ubicación; perfil con ubicación); aislamiento 5/5.
- **Falta:** padrón de actividades económicas (el perfil sigue respondiendo 422 si trae `economicActivityCode`);
  otros cargos (Nota 16) y tipos de transacción (Nota 22) no tienen tabla (fuera de V1).

## Próximo: incremento 6

Establecimientos, terminales, consecutivo y clave (decisión 4): `POST/GET /v1/establishments`, terminal 00001 por
establecimiento, secuencia por (organización, terminal, tipo) bajo bloqueo, clave de 50 con `crypto/rand` y la
cédula alfanumérica (Notas 3 y 4.1). Necesita que Platform cree la casa matriz en `core.branches` (informe 0002 §6).

## Reglas `FUENTE: borrador sept-2024` pendientes de revalidar

Con la resolución oficial MH-DGT-RES-0027-2024 y el Decreto 44739-H (no están en `docs/hacienda/`):
- Art. 9: un comprobante válido no se anula; un rechazado no lleva nota.
- Art. 10: mensaje receptor (fuera de V1).
- Art. 11: Hacienda valida en hasta 3 horas.
- Arts. 4 y 5 y transitorio III: QR suspendido.
- Art. 7: obligaciones del proveedor de sistemas.
- Conservación de 5 años (art. 109 del Código Tributario, considerando del borrador).

## TODOs y decisiones abiertas

- **Criterio 10:** una factura de una organización sin perfil fiscal va a dead letter, no a la bandeja. ¿Documento
  en `error` visible en vez de dead letter? (TODO en `app.CreateDocumentFromInvoice`).
- **Hacienda (falta en `docs/hacienda/`):** PDF de la política de firma, padrón de actividades económicas,
  documentación del API del sandbox (client id y URL del token), procedimiento de contingencia.
- **Identificación de RDL como proveedor de sistemas:** vacía; el worker no enviará sin ella.
- **Contratos (informe 0003 §7):** propuestas C1–C11, sin redactar todavía en `RDL.Contracts`.
- **Billing y Receivables:** cambios por las decisiones 2 y 3 (NC de anulación, factura sustituta de un rechazado).
- **Platform:** que cada organización tenga su casa matriz en `core.branches` (informe 0002 §6).
- **Contador:** `FechaEmision` vs. `issueDate` cuando el evento llega otro día; código de referencia por defecto de
  las notas; descuento con código 99; contingencia con situación 1.
- **Pooler:** después de restaurar el proyecto dev, `platform_api` y `platform_migrate` dan «password authentication
  failed».
