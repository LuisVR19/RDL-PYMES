<!-- Generado desde Planning_Desarrollo_V1_SaaS_Facturacion.docx. La fuente de verdad es el .docx original. -->

Planning de desarrollo V1

SaaS B2B de facturación electrónica

Plan de ejecución por proyectos para construir la V1 con tres desarrolladores trabajando con Claude Code: qué repositorio se construye, en qué orden, quién es dueño de qué, y cómo se usa Claude Code en cada uno sin romper las fronteras que define la arquitectura.

Basado en “Arquitectura V1: Plataforma SaaS B2B de facturación electrónica”, versión 1.0 del 21 de septiembre de 2026.

| Venta | Factura comercial | Documento electrónico | Cuenta por cobrar | Pago |
|---|---|---|---|---|
| Billing | Billing | E-Invoice | Receivables | Receivables |

Contenido

## Resumen del plan

La V1 se divide en nueve proyectos, cada uno con su repositorio: contratos, base de datos, infraestructura con building blocks, Platform API (identidad y tenancy), las tres APIs de dominio (Billing, E-Invoice y Receivables), el BFF y el portal web. La app móvil y la consola interna quedan fuera de la V1.

La estimación es de 24 semanas (12 sprints de dos semanas) hasta un piloto con clientes reales, siguiendo las fases 0 a 5 del documento de arquitectura más un bloque final de endurecimiento. La ruta crítica pasa por E-Invoice: la integración real con Hacienda es la parte con más incertidumbre externa, así que arranca en paralelo al flujo vertical en lugar de esperar a que termine.

Sobre la estimación. Asume tres desarrolladores a tiempo completo, con experiencia en el stack y usando Claude Code a diario. Claude Code acelera mucho el código repetitivo (endpoints, migraciones, tests, DTOs), pero no acelera las decisiones pendientes, la validación normativa ni la espera de respuestas de Hacienda. Recalibren después del sprint 2 con la velocidad real.

## Equipo y ownership

La arquitectura asigna una API por desarrollador. Los proyectos transversales se reparten para equilibrar la carga; es una propuesta que pueden ajustar según los perfiles del equipo.

| Dev A | Dev B | Dev C |
|---|---|---|
| Billing API (dueño) | E-Invoice API (dueño) | Receivables API (dueño) |
| Platform API y schema core | Infra, CI/CD y building blocks | BFF y base del portal web |
| Portal web: módulos de facturación | Portal web: configuración fiscal | Portal web: cobranza |

Contratos y base de datos son propiedad compartida: cualquier cambio requiere PR con aprobación de los otros dos. Cada desarrollador es dueño end to end de su API, incluido su schema, sus migraciones, su pipeline y sus pantallas.

## Calendario

Semanas 1 a 24, agrupadas por fase. Cada celda indica el foco del proyecto en ese periodo; las celdas en blanco significan que el proyecto no tiene trabajo planificado en esa fase.

| Proyecto | F0sem 1–2 | F1–F2sem 3–6 | F3sem 7–10 | F4sem 11–16 | F5sem 17–20 | Cierresem 21–24 |
|---|---|---|---|---|---|---|
| ■ Contratos | Contratos v1 | Mantenimiento | Mantenimiento | Mantenimiento | Mantenimiento | Mantenimiento |
| ■ Base de datos | Convenciones, Docker local | Schemas, roles, RLS, pruebas | Soporte | Soporte | Soporte | Restore drill, retención |
| ■ Infra y blocks | Arranque (sem 2) | IaC, CI/CD, mensajería, OTel | Soporte | Soporte | Soporte | Operación |
| ■ Platform API |  | Tenancy y auth (sem 3–5) | Usuarios y sucursales |  | Suscripciones |  |
| ■ Billing API |  | Base (sem 5–6) | Clientes, productos, emisión | Ajustes fiscales; notas desde sem 15 | Notas y anulación |  |
| ■ E-Invoice API |  | Base + sandbox (sem 5–6) | Consumidor; XML desde sem 9 | XML, firma, envío, contingencia | Notas y rechazos |  |
| ■ Receivables API |  | Base (sem 5–6) | CxC desde evento | Pagos, aplicaciones | Aging, seguimientos |  |
| ■ BFF |  | Arranque (sem 6) | Vista transversal | Notificaciones | Read model |  |
| ■ Web |  | Base (sem 4–6) | Facturación | Configuración fiscal | Cobranza y admin | Cobranza; pulido |
| ■ Piloto |  |  |  |  |  | Criterios, normativa, piloto |

### Hitos

| Hito | Semana | Criterio para darlo por cumplido |
|---|---|---|
| H1 Contratos v1 | 2 | Glosario, máquinas de estado, InvoiceIssued v1 y convenciones aprobados por los tres y publicados en el repo de contratos. |
| H2 Tenancy probada | 5 | Login, selección de organización y TenantContext funcionando; la prueba “A no ve datos de B” corre en CI y pasa. |
| H3 Esqueletos desplegables | 6 | Las tres APIs, el BFF y la web se despliegan de forma independiente en staging con health checks, trazas y roles de BD propios. |
| H4 Flujo vertical | 10 | Demo: crear cliente y producto, emitir factura, ver documento fiscal en Processing y CxC creada, todo en una vista del portal. |
| H5 Hacienda sandbox | 15 | Primer comprobante firmado, enviado y aceptado en el ambiente de pruebas de Hacienda, con reintentos y rechazos manejados. |
| H6 V1 funcional completa | 20 | Pagos, aplicaciones, aging, notas de crédito y débito end to end; exportación de auditoría. |
| H7 Listo para piloto | 24 | Los diez criterios de aceptación arquitectónica de la sección 12 demostrados, restore drill ejecutado y revisión normativa cerrada. |

## Cómo trabajamos con Claude Code

Con tres APIs que comparten base de datos, el riesgo principal de desarrollar con un agente es que el código cruce fronteras: una consulta sin organization_id, un JOIN a un schema ajeno, un float para montos. La configuración de Claude Code debe hacer que esas reglas estén siempre en contexto y que se verifiquen automáticamente, no solo en la revisión humana.

### Estructura estándar de cada repositorio

billing-api/

├── CLAUDE.md                  # reglas del dominio y del repo (se lee en cada sesión)

├── .claude/

│   ├── settings.json          # permisos, hooks y servidores MCP del proyecto

│   ├── commands/              # comandos del equipo: /nueva-migracion, /nuevo-endpoint…

│   └── agents/                # subagentes: revisor de tenancy, guardián de contratos…

├── docs/

│   ├── decisiones/            # ADRs cortos, uno por decisión

│   └── contexto/              # extractos del documento de arquitectura que aplican

├── src/ …

└── tests/ …

### CLAUDE.md: las reglas no negociables

Cada repo tiene un CLAUDE.md corto con las reglas comunes (idénticas en todos) y las del dominio. Las comunes viven en el repo de contratos y se copian o importan; conviene que un check de CI verifique que no se desincronizan. Ejemplo del bloque común:

# Reglas de plataforma (no modificar sin PR en contracts)

- Este servicio SOLO escribe en el schema `billing`. Nunca generes INSERT/UPDATE/DELETE

  ni migraciones sobre core, fiscal, receivables, audit o subscriptions.

- Toda tabla de negocio tiene `organization_id uuid NOT NULL`, unicidades y FK compuestas

  (organization_id, id). Toda consulta filtra por el TenantContext, nunca por un valor

  recibido en body, query o header.

- Montos: decimal / numeric. Prohibido float o double.

- Fechas en UTC (timestamptz). Presentación en la zona horaria de la organización.

- Documentos emitidos: inmutables. Correcciones con estados, notas o reversos.

- Cambios que emiten eventos: escribir entidad + outbox en la MISMA transacción.

- Consumidores de eventos: idempotentes vía inbox (eventId único).

- Cada operación sensible genera un audit event con correlationId.

- Antes de terminar una tarea: `dotnet build`, `dotnet test` y los tests de aislamiento.

### Comandos del equipo

Los comandos en .claude/commands/ convierten los patrones repetidos en un flujo guiado y uniforme entre los tres repos. Los mínimos para la V1:

| Comando | Qué hace |
|---|---|
| /nueva-migracion | Crea una migración solo en el schema del repo, con organization_id, índices compuestos y patrón expand, migrate, contract si modifica algo existente. |
| /nuevo-endpoint | Endpoint con autorización por rol, TenantContext, Problem Details, idempotency key si es un comando, audit event y tests (feliz, validación y cross-tenant). |
| /nuevo-consumidor | Consumidor de un evento del catálogo de contratos, con inbox, reintentos, dead letter y test de reproceso sin duplicados. |
| /publicar-evento | Agrega un evento al outbox validando el payload contra el JSON Schema del repo de contratos. |
| /revisar-fronteras | Revisa el diff actual buscando escrituras a schemas ajenos, consultas sin tenant, floats y ediciones de documentos emitidos. |

### Subagentes especializados

- tenant-security-reviewer: revisa cada PR solo desde la óptica de aislamiento multiempresa y permisos; se invoca antes de abrir PR.

- contract-guardian: compara cambios contra el repo de contratos y detecta cambios incompatibles en eventos o APIs.

- test-writer: escribe tests a partir del contrato y los criterios de aceptación, no a partir de la implementación, para evitar tests que solo confirman lo que el código ya hace.

### Hooks y permisos

Los hooks convierten reglas en verificaciones automáticas. Recomendados: formatear después de cada edición (dotnet format o el formatter del frontend); bloquear ediciones en carpetas de migraciones de otros schemas y en archivos .env, certificados o secretos; ejecutar build y tests rápidos al terminar una tarea. En settings.json se niega el acceso a comandos de despliegue a producción y a cualquier credencial real. Claude Code nunca debe tener acceso a la base de producción ni a certificados de firma reales de Hacienda.

### Servidores MCP útiles

- PostgreSQL en modo solo lectura contra la base local o de desarrollo, para que Claude inspeccione el esquema real en vez de suponerlo.

- GitHub para issues, PRs y revisión.

- Jira o Linear para que cada sesión arranque desde el ticket con sus criterios de aceptación.

- Playwright en el portal web para que Claude verifique la UI que construye.

### Ritmo de trabajo por tarea

Cada historia sigue el mismo ciclo: el desarrollador abre la sesión desde el ticket, usa el modo plan para que Claude explore el código y proponga un plan, revisa y corrige ese plan, deja que implemente con tests, ejecuta /revisar-fronteras y el subagente de tenancy, y abre el PR. Para trabajar en dos historias a la vez sin mezclar cambios, usen git worktrees con una sesión de Claude Code en cada uno. En CI, Claude Code en modo headless o su integración con GitHub Actions puede hacer una primera revisión automática de cada PR, que nunca sustituye la aprobación humana.

Revisión humana obligatoria en: políticas RLS y roles de BD, cálculo de impuestos, generación y firma de XML, consecutivos y claves, aplicación de pagos a saldos, y cualquier cambio en contratos. Son las zonas donde un error es caro y difícil de revertir.

Detalles de configuración de CLAUDE.md, comandos, subagentes, hooks y MCP: documentación oficial de Claude Code en docs.claude.com/en/docs/claude-code/overview. Verifiquen ahí la sintaxis exacta, ya que puede cambiar entre versiones.

## P0  Contratos y acuerdos

| Repositorio | Dueño | Periodo | Depende de |
|---|---|---|---|
| contracts | Los tres (2 aprobaciones) | Sem 1–2, luego continuo | Nada; desbloquea todo |

Fuente única de verdad de lo que las APIs se prometen entre sí. Es la Fase 0 de la arquitectura convertida en artefactos versionados y verificables, no en un documento que se desactualiza.

### Entregables

- Glosario de dominio (organización vs cliente final, factura vs documento electrónico, CxC, aplicación de pago).

- Máquinas de estado de Invoice, ElectronicDocument y Receivable, con transiciones permitidas y quién las dispara.

- Matriz de ownership de tablas y permisos por schema (la tabla 2.1 del documento, en formato verificable).

- Convenciones: IDs UUID, fechas ISO-8601 UTC, dinero en decimal con precisión acordada, errores con Problem Details, header de idempotencia, propagación de correlationId.

- Catálogo de los ocho eventos iniciales en AsyncAPI con JSON Schema por evento; InvoiceIssued v1 completo con líneas, descuentos, impuestos y exoneraciones.

- Esqueletos OpenAPI de Platform, Billing, E-Invoice, Receivables y las rutas internas que usará el BFF.

- CI que valida los schemas, publica un paquete de DTOs de eventos y falla ante cambios incompatibles sin nueva versión.

#### Prompt de arranque para Claude Code

Lee docs/contexto/arquitectura-v1.md (secciones 3, 6.2 y 6.3).

Genera en /asyncapi un documento AsyncAPI 3 con los 8 eventos del catálogo.

Para cada evento crea un JSON Schema en /schemas/events/<evento>.v1.json con

eventId, eventType, version, occurredAt, correlationId y organizationId obligatorios.

Montos como string decimal, nunca number. No inventes campos fiscales: donde falte

definición deja un TODO y lístalos al final para revisarlos en equipo.

## P1  Base de datos

| Repositorio | Dueño | Periodo | Depende de |
|---|---|---|---|
| database-platform | Compartido; coordina Dev B | Sem 1–6, cierre en sem 21–24 | P0 (ownership y convenciones) |

La instancia PostgreSQL y todo lo que no pertenece a una sola API: creación de schemas, roles por servicio, auditoría, tablas de integración, RLS, backups. Los schemas de dominio (billing, fiscal, receivables) los migra cada API desde su propio repo; core y subscriptions los migra Platform API.

| Fase | Trabajo |
|---|---|
| F0 · sem 1–2 | Convenciones de nombres y tipos. Herramienta de migraciones por repo (propuesta: EF Core Migrations por API, con historial de migraciones dentro de su schema). Docker Compose local con PostgreSQL y dos tenants semilla. |
| F1 · sem 3–5 | Script de bootstrap: schemas, rol de aplicación y rol migrador por servicio, GRANT de solo lectura donde la matriz lo permita. Schema audit append only (sin UPDATE ni DELETE para roles de aplicación). Schema integration con outbox, inbox y dead letters. Mecanismo de sesión para RLS (SET LOCAL de la organización activa por transacción) y políticas en tablas sensibles. |
| F2 · sem 4–6 | Suite de pruebas negativas con Testcontainers: un rol no escribe en schemas ajenos; tenant A no lee ni relaciona datos de B; FK compuestas rechazan referencias cruzadas. Ambientes dev y staging, pooler con límites de conexión por rol, backups cifrados y PITR. |
| F5 · sem 21–24 | Restore drill documentado en entorno aislado, revisión de índices con datos de volumen realista, política de retención aplicada a audit y XML. |

### Definición de terminado

Las pruebas de aislamiento corren en el CI de cada API (no solo en este repo) y cualquier tabla nueva sin organization_id o sin política hace fallar el pipeline.

#### Prompt de arranque para Claude Code

Usa el MCP de Postgres (solo lectura) para inspeccionar la base local.

Escribe tests xUnit con Testcontainers que se conecten con cada rol de aplicación

(billing_app, fiscal_app, receivables_app, platform_app) y verifiquen que:

1) no pueden INSERT/UPDATE/DELETE fuera de su schema,

2) con la sesión en la organización A, un SELECT no devuelve filas de B,

3) una FK hacia un registro de otra organización es rechazada.

Estos tests deben fallar hoy donde falten políticas; no cambies las políticas,

reporta qué falla.

## P2  Infraestructura, DevOps y building blocks

| Repositorio | Dueño | Periodo | Depende de |
|---|---|---|---|
| infra, building-blocks | Dev B | Sem 2–6, luego soporte | Decisión de nube y mensajería |

Todo lo necesario para que cada API se construya, pruebe y despliegue sola. Los building blocks son una librería interna de infraestructura técnica; la regla es que nunca contengan lógica de negocio ni entidades de dominio.

### Entregables

- Infraestructura como código para dev, staging y producción (Terraform o la herramienta nativa de la nube elegida).

- Plantillas de pipeline reutilizables: build, tests, tests de aislamiento, migraciones del schema propio, imagen Docker y despliegue independiente.

- Mensajería según la decisión pendiente; propuesta para no bloquear: outbox en PostgreSQL con worker publicador hacia RabbitMQ o la cola gestionada de la nube.

- Observabilidad con OpenTelemetry: logs estructurados con organizationId, correlationId y service; métricas de colas, reintentos y documentos pendientes; trazas BFF → API → worker.

- Gestión de secretos para cadenas de conexión y, sobre todo, certificados de firma de Hacienda por organización.

- Paquete building-blocks: middleware de TenantContext, correlation ID, publicador de outbox, consumidor con inbox idempotente, Problem Details, health checks, cliente de auditoría.

- Revisión automática de PRs con Claude Code en GitHub Actions y plantillas de .claude/ para los repos nuevos.

## P3  Platform API (identidad y tenancy)

| Repositorio | Dueño | Schemas | Depende de |
|---|---|---|---|
| platform-api | Dev A | core, subscriptions | P0, P1, proveedor de identidad |

La Fase 1 completa. Sin esto ninguna otra API puede probar su aislamiento, por eso es el primer trabajo de backend.

| Fase | Trabajo |
|---|---|
| F1 · sem 3–5 | Migraciones de organizations, users, organization_users y branches. Integración con el proveedor de identidad. Endpoint para listar memberships y seleccionar organización activa. Claims de organización y rol; validación de membership en cada request. Roles iniciales (propuesta: propietario, administrador, facturador, cobrador, contador, solo lectura). Alta de organización y usuarios con audit events. |
| F3 · sem 7–10 | Invitación de usuarios, gestión de sucursales, usuarios con varias organizaciones (caso contador). |
| F5 · sem 17–20 | Schema subscriptions mínimo: planes, suscripción activa y conteo de uso para los límites que defina producto. El cobro automático del SaaS puede quedar después de V1. |

### Definición de terminado

Un token válido de la organización A usado contra cualquier endpoint con un organization_id de B devuelve 403 o 404, verificado por test en cada API.

## P4  Billing API

| Repositorio | Dueño | Schema | Depende de |
|---|---|---|---|
| billing-api | Dev A | billing | P0, P1, P2, P3 |

Operación comercial de cada organización con sus clientes. Es el productor de los eventos que mueven al resto del sistema, así que su calidad de datos (snapshots, montos, eventos) condiciona a E-Invoice y Receivables.

| Fase | Trabajo |
|---|---|
| F2 · sem 5–6 | Esqueleto: proyecto, Dockerfile, pipeline, health checks, rol de BD, building blocks integrados, primer endpoint protegido por tenant. |
| F3 · sem 7–10 | Clientes (identificación única por organización). Productos y servicios con referencia a código CABYS e impuesto. Facturas en borrador con líneas. Cálculo de subtotales, descuentos e impuestos según la decisión de dónde vive el cálculo. Emisión DRAFT → ISSUED con snapshots de cliente y producto y InvoiceIssued v1 en el outbox en la misma transacción. Numeración visible al negocio. |
| F4 · sem 11–16 | Ajustes que exija la integración fiscal real: condiciones de venta, medios de pago, exoneraciones, campos que pida el XML. Consumo de ElectronicDocumentRejected para marcar facturas que requieren corrección. |
| F5 · sem 15–20 | Notas de crédito y débito, anulación con motivo obligatorio, eventos CreditNoteIssued, DebitNoteIssued e InvoiceCancelled. |

### Definición de terminado

- Emite facturas con E-Invoice apagada (criterio de aceptación 2).

- Cambiar un cliente o producto no altera ninguna factura emitida (criterio 5).

- Tests basados en propiedades para el cálculo: la suma de líneas cuadra siempre con los totales y ningún redondeo pierde céntimos.

#### Prompt de arranque para Claude Code

/nuevo-endpoint POST /invoices/{id}/issue

Contexto: estados en contracts/states/invoice.md, evento en

contracts/schemas/events/InvoiceIssued.v1.json.

Requisitos: solo desde DRAFT; copia snapshot de cliente y de cada producto a la

factura; asigna número visible; guarda factura + OutboxMessage en una transacción;

responde 201 sin llamar a ningún otro servicio.

Primero muéstrame el plan. Tests: emisión feliz, doble emisión con la misma

idempotency key, factura de otra organización, factura ya emitida.

## P5  E-Invoice API y worker fiscal

| Repositorio | Dueño | Schema | Depende de |
|---|---|---|---|
| einvoice-api | Dev B | fiscal | P0, P1, P2; eventos de P4 |

Ruta crítica del plan. Tiene dependencias externas (ambiente de pruebas de Hacienda, certificados, especificación vigente) que no se aceleran con más código, por eso el trabajo de investigación y configuración empieza en la semana 5, antes de que el flujo vertical esté terminado.

| Fase | Trabajo |
|---|---|
| F2 · sem 5–6 | Esqueleto. En paralelo: obtener acceso al ambiente de pruebas de Hacienda, certificado de pruebas, y guardar en docs/hacienda/ la especificación, los XSD y los catálogos de la versión vigente para que Claude Code trabaje contra la fuente oficial. |
| F3 · sem 7–10 | Consumidor de InvoiceIssued con inbox: crea electronic_document con snapshot de líneas en estado Processing. Integración con Hacienda simulada (stub) para cerrar el flujo vertical. |
| F4 · sem 9–16 | Configuración fiscal por organización, sucursal y terminal. Actividades económicas. Catálogo CABYS: importación, búsqueda y versionado. Consecutivos y clave numérica según el alcance que se decida. Generación del XML validado contra XSD. Firma con el certificado de la organización. Autenticación con Hacienda, envío, consulta de estado, reintentos con backoff y contingencia. Persistencia de XML firmado, respuestas e historial de intentos. Eventos ElectronicDocumentAccepted y Rejected. |
| F5 · sem 15–20 | Documentos fiscales para notas de crédito y débito, bandeja de rechazados con flujo de corrección, métricas y alertas de tasa de rechazo y cola estancada. |

Normativa. Ni este plan ni Claude Code son fuente de verdad sobre los requisitos de Hacienda. Cada regla fiscal implementada debe trazar a un artículo de la especificación guardada en el repo, y la validación contra XSD y contra el ambiente de pruebas es el criterio de verdad. Presupuesten una revisión con un contador o asesor tributario antes del piloto.

### Definición de terminado

Reprocesar un InvoiceIssued no crea un segundo documento (criterio 3), y un documento rechazado aparece en una bandeja visible con su motivo y una acción de corrección (criterio 10).

#### Prompt de arranque para Claude Code

Lee docs/hacienda/ (especificación y XSD de la versión vigente).

Genera el mapeo de electronic_document + document_lines al XML de Factura

Electrónica. Cada campo del XML debe tener un comentario con la sección de la

especificación de la que sale. Valida el resultado contra el XSD en un test.

No inventes reglas: si un dato no existe en nuestro modelo, lístalo como faltante.

No uses certificados reales; usa el de pruebas en tests/fixtures.

## P6  Receivables API

| Repositorio | Dueño | Schema | Depende de |
|---|---|---|---|
| receivables-api | Dev C | receivables | P0, P1, P2; eventos de P4 |

Cuentas por cobrar, pagos y seguimiento. La lógica de aplicación de pagos (uno a muchos y muchos a uno) es la parte que más se beneficia de tests exhaustivos generados desde las invariantes.

| Fase | Trabajo |
|---|---|
| F2 · sem 5–6 | Esqueleto con pipeline y rol de BD propio. |
| F3 · sem 7–10 | Consumidor de InvoiceIssued con inbox: crea la cuenta por cobrar con saldo, fecha de vencimiento y condición de venta. Endpoint interno de saldo por factura para el BFF. |
| F5 · sem 11–20 | Registro de pagos; aplicación de un pago a varias facturas y de varios pagos a una factura; saldos y cancelación; vencimientos, aging por tramos y morosidad; seguimientos y compromisos de pago. Consumidores de notas de crédito, débito y anulaciones para ajustar saldos. Eventos PaymentReceived y ReceivableSettled. |

### Invariantes que deben tener test

- La suma de aplicaciones de un pago nunca supera el monto del pago.

- El saldo de una cuenta nunca es negativo y siempre es igual al original más débitos menos créditos y aplicaciones.

- Revertir una aplicación deja los saldos exactamente como estaban.

## P7  BFF

| Repositorio | Dueño | Periodo | Depende de |
|---|---|---|---|
| bff | Dev C | Sem 6–20 | Endpoints de P3 a P6 |

Compone lo que la interfaz necesita sin meter dependencias entre dominios. No tiene reglas de negocio ni escribe en la base; si una pantalla necesita una regla, esa regla pertenece a una API.

- Sem 6: esqueleto, propagación de identidad y correlationId hacia las APIs.

- Sem 7–10: vista transversal de factura (total de Billing, estado de Hacienda de E-Invoice, saldo de Receivables) por composición de endpoints internos; listados paginados.

- Sem 11–16: notificaciones en tiempo real del estado fiscal (SSE o WebSocket) a partir de los eventos Accepted y Rejected.

- Sem 15–20: read model propio alimentado por eventos para listados y reportes que no deben pegarle a las tres APIs en cada carga.

## P8  Web: portal de la PYME

| Repositorio | Dueño | Periodo | Depende de |
|---|---|---|---|
| web-app | Base Dev C; módulos por dominio | Sem 4–24 | P7 (BFF) |

El stack no está definido en la arquitectura; la propuesta es React con TypeScript, un sistema de componentes propio pequeño y pruebas end to end con Playwright. Cada desarrollador construye las pantallas de su dominio para mantener la responsabilidad end to end.

| Fase | Pantallas |
|---|---|
| F1 · sem 4–6 | Login, selector de organización activa, layout, componentes base, formatos de colones y fechas en zona horaria de la organización. |
| F3 · sem 7–10 | Clientes, productos con buscador CABYS, crear y emitir factura, detalle de factura con estado fiscal y saldo. |
| F4 · sem 11–16 | Configuración fiscal: certificado, sucursales, terminales, actividades. Bandeja de documentos rechazados y en contingencia. |
| F5 · sem 15–22 | Pagos y aplicación de pagos, cuentas por cobrar con aging, seguimientos, notas de crédito y débito, usuarios y roles, exportación de auditoría. |

Con el MCP de Playwright, pidan a Claude Code que ejecute el flujo que acaba de construir y revise la pantalla resultante antes de dar la tarea por terminada, incluyendo los estados vacío, de error y de carga.

## P9  Después de V1

Quedan fuera del alcance de las 24 semanas, pero la arquitectura ya los contempla: app móvil sobre el mismo BFF; consola interna de la plataforma para gestionar organizaciones, planes y soporte; cobro automatizado de suscripciones; modalidad enterprise con infraestructura dedicada y SSO; y la extracción de fiscal a su propia base para reutilizarlo en otros productos.

## Decisiones que bloquean

Las decisiones pendientes de la sección 11 tienen fecha límite según el primer proyecto que bloquean. Donde tiene sentido se propone un valor por defecto para no detener el trabajo mientras se decide.

| Decisión | Bloquea | Límite | Propuesta por defecto |
|---|---|---|---|
| Proveedor de identidad | P3 Platform | Sem 2 | Un proveedor gestionado compatible con OIDC; evitar identidad propia en V1. |
| Nube y mensajería | P2 Infra | Sem 2 | Outbox en PostgreSQL con worker hacia RabbitMQ o cola gestionada. |
| Stack del portal web | P8 Web | Sem 3 | React con TypeScript. |
| Mecanismo y alcance de RLS | P1 Base de datos | Sem 3 | Variable de sesión por transacción; RLS en todas las tablas con datos de clientes. |
| Dónde vive el cálculo de impuestos | P4 Billing, P5 E-Invoice | Sem 6 | Billing calcula y E-Invoice valida antes de firmar. |
| Alcance de consecutivos | P5 E-Invoice | Sem 8 | Sin valor por defecto: requiere validar con la especificación y contabilidad. |
| CABYS: mantenimiento y versionado | P5 E-Invoice | Sem 8 | E-Invoice es dueño; importación periódica con versión. |
| Almacenamiento de XML | P5 E-Invoice | Sem 10 | Object storage con hash y metadatos en fiscal. |
| Límites de planes SaaS | P3 Platform | Sem 16 | Por documentos emitidos al mes. |
| Retención y datos personales | Piloto | Sem 20 | Sin valor por defecto: requiere asesoría legal. |

## Riesgos propios del desarrollo con IA

Los riesgos de la sección 9 siguen vigentes. Estos son los que aparecen específicamente por construir con un agente de código:

| Riesgo | Mitigación |
|---|---|
| Código que cruza fronteras de schema o de tenant y parece correcto | Roles de BD que lo impiden físicamente, hooks que bloquean rutas ajenas, subagente de tenancy y tests de aislamiento en cada pipeline. |
| Tests que confirman la implementación en lugar del requisito | Escribir primero los tests desde el contrato y los criterios de aceptación, en una sesión separada de la implementación. |
| Reglas fiscales inventadas o de una versión anterior | Especificación oficial en el repo, validación XSD, pruebas en sandbox y revisión humana de toda regla fiscal. |
| Deriva de convenciones entre los tres repos | Bloque común de CLAUDE.md verificado en CI y retro de cada sprint para actualizar CLAUDE.md con los errores repetidos. |
| Exposición de secretos o datos reales | Permisos que niegan acceso a secretos, solo datos sintéticos en desarrollo, nunca certificados reales en el entorno del agente. |
| PRs demasiado grandes para revisarlos bien | Historias pequeñas, un PR por historia y límite acordado de tamaño de diff. |

## Primera semana

Para arrancar el lunes sin esperar a que todo esté decidido:

- Crear la organización en GitHub y los repos vacíos de los nueve proyectos con la plantilla de .claude/ y el CLAUDE.md común.

- Guardar el documento de arquitectura en contracts/docs/contexto/ para que todas las sesiones de Claude Code lo usen como referencia.

- Sesión de medio día para glosario y máquinas de estado; Claude Code las transcribe a los formatos del repo de contratos.

- Decidir proveedor de identidad, nube y mensajería antes del viernes de la semana 2.

- Levantar el Docker Compose local con PostgreSQL y los dos tenants semilla.

- Crear el backlog en Jira o Linear con las historias de las fases 0 a 3, cada una con criterios de aceptación que Claude Code pueda leer.

- Pedir acceso al ambiente de pruebas de Hacienda: suele ser el trámite que más tarda.

Las semanas son estimaciones para recalibrar al cierre del sprint 2. Este planning no sustituye la validación legal, tributaria ni de protección de datos que exige el documento de arquitectura.
