<!-- Generado desde Arquitectura_SaaS_B2B_Facturacion_Electronica.docx. La fuente de verdad es el .docx original. -->

ARQUITECTURA V1

# Plataforma SaaS B2B de facturación electrónica

Propuesta técnica para Billing E Invoice y Receivables

```
Documento de decisiones arquitectónicas y base para implementación
Versión 1.0  •  21 de septiembre de 2026
```

Estado  Propuesta para validación del equipo

## Resumen ejecutivo

La plataforma se plantea como un SaaS B2B multiempresa para que distintas organizaciones administren clientes, productos, facturas, documentos electrónicos, cuentas por cobrar y pagos. La decisión recomendada para la primera versión es operar tres APIs independientes, alineadas con tres dominios y tres desarrolladores, sobre una única instancia PostgreSQL organizada por schemas y con controles estrictos de propiedad y acceso.

La separación física de las APIs permite trabajo paralelo y responsabilidades end to end. La base compartida evita duplicación prematura y sincronización eventual innecesaria entre datos naturalmente relacionados. Los schemas, permisos, contratos y eventos conservan las fronteras necesarias para extraer servicios o bases independientes en el futuro.

| Decisión | Propuesta V1 | Motivo principal |
|---|---|---|
| Arquitectura de servicios | Tres APIs independientes | Responsabilidad clara y desarrollo paralelo |
| Persistencia | Una base PostgreSQL | Relaciones fuertes dentro del mismo proceso empresarial |
| Separación lógica | Schemas por dominio | Ownership de datos y evolución independiente |
| Tenancy | Multi tenant desde el día uno | Aislamiento de múltiples empresas |
| Integración | Eventos y contratos versionados | Evitar acoplamiento secuencial entre APIs |
| Auditoría | Registro de negocio append only | Trazabilidad demostrable ante clientes y auditorías |

### Alcance del documento

Este documento consolida las decisiones tomadas hasta ahora y define una base implementable. No sustituye una validación legal, tributaria, contable ni de protección de datos. Los requisitos vigentes de Hacienda, conservación documental y privacidad deben verificarse antes de cerrar el diseño de producción.

## 1 Contexto y principios

El sistema cubre un flujo empresarial continuo: una organización vende, factura, reporta fiscalmente, genera una cuenta por cobrar y registra pagos. Aunque estas capacidades están relacionadas, cada una posee reglas, estados y ritmos de evolución distintos.

```
Venta  →  Factura comercial  →  Documento electrónico  →  Cuenta por cobrar  →  Pago
```

### 1.1 Principios rectores

- Separar responsabilidades de dominio aunque la infraestructura se comparta.

- Cada API escribe únicamente en el schema que posee.

- Una falla o lentitud de Hacienda no debe bloquear la emisión comercial.

- El tenant se obtiene de la identidad autorizada y nunca se acepta a ciegas desde el frontend.

- La base de datos debe reforzar el aislamiento entre organizaciones, no depender solo del código.

- Los documentos emitidos conservan snapshots históricos de datos relevantes.

- Las operaciones financieras y fiscales sensibles se corrigen mediante estados, notas o reversos; no mediante borrado destructivo.

- Los contratos de integración se versionan y admiten idempotencia, reintentos y trazabilidad.

### 1.2 Decisiones explícitas

| Tema | Decisión | Condición |
|---|---|---|
| Servicios | Billing API, E Invoice API y Receivables API | Autonomía de despliegue y ownership |
| Repositorio | Preferiblemente uno por API y uno pequeño de contratos | No compartir entidades, repositorios ni DbContext |
| Base de datos | Una instancia compartida con schemas | Permisos separados y migraciones por owner |
| Procesamiento fiscal | Asíncrono | Billing responde sin esperar a Hacienda |
| Cliente final | billing.customers | No confundir con core.organizations |
| Suscripción SaaS | subscriptions o platform | Separada de la facturación de la PYME |

## 2 Arquitectura propuesta

```
Web Mobile BFF
       │
       ├───────────────┬──────────────────┐
       ▼               ▼                  ▼
  Billing API     E Invoice API     Receivables API
       │               │                  │
       └───────────────┴──────────────────┘
                       ▼
                   PostgreSQL
      core  billing  fiscal  receivables
         subscriptions  audit  integration
```

Las tres APIs son deployables independientes. Pueden usar el mismo stack inicial, por ejemplo ASP.NET Core, PostgreSQL, Docker y una plataforma común de observabilidad, sin compartir lógica de negocio.

### 2.1 Fronteras de escritura y lectura

| Componente | Puede escribir | Puede leer directamente |
|---|---|---|
| Billing API | billing.* | core.* y catálogos autorizados |
| E Invoice API | fiscal.* | core.* y snapshots o vistas autorizadas de billing.* |
| Receivables API | receivables.* | core.* y datos mínimos de billing.* |
| Servicios de plataforma | core.* y subscriptions.* | Datos de identidad, plan y organización |
| Audit pipeline | audit.* | Eventos publicados; no altera dominios |

El acceso directo de lectura entre schemas debe ser excepcional y explícito. Para operaciones de negocio, la preferencia es usar eventos, endpoints internos o proyecciones. Una API nunca actualiza las tablas propiedad de otra.

### 2.2 Vista transversal para la interfaz

Una pantalla puede necesitar total de factura, estado en Hacienda y saldo pendiente. Esa composición corresponde al BFF, a una Query API o a un read model, no al dominio interno de Billing.

```
Factura FE00100034
Total: ₡113 000  |  Hacienda: Aceptada  |  Saldo: ₡63 000
```

## 3 Responsabilidad de cada API

### 3.1 Billing API

Es dueña de la operación comercial realizada por cada organización a sus propios clientes.

- Clientes de la empresa

- Productos y servicios

- Facturas y líneas

- Cálculo comercial de subtotales, descuentos e impuestos

- Notas de crédito y débito

- Estados comerciales y numeración visible al negocio

No es responsable de comunicarse con Hacienda ni de aplicar pagos a saldos.

### 3.2 E Invoice API

Es dueña del cumplimiento fiscal y de la comunicación con Hacienda. Debe funcionar de forma desacoplada porque maneja timeouts, certificados, firma, XML, cambios de versión, respuestas asíncronas, reintentos y contingencias.

- Configuración fiscal por organización

- Actividades económicas y catálogo CABYS

- Establecimientos, terminales, consecutivos y claves

- Generación y firma de XML

- Envíos y consultas a Hacienda

- Estados Processing Accepted Rejected y contingencia

- Historial de intentos y respuestas

### 3.3 Receivables API

Es dueña de cuentas por cobrar, pagos, aplicación de pagos, vencimientos, morosidad y seguimiento. El nombre Receivables refleja mejor su alcance que Payments.

- Creación de cuentas por cobrar desde facturas emitidas

- Pagos parciales o totales

- Un pago aplicado a varias facturas

- Varios pagos aplicados a una factura

- Saldos, vencimientos, aging y morosidad

- Seguimientos y compromisos de pago

## 4 Diseño de base de datos

### 4.1 Organización por schemas

| Schema | Propósito | Tablas iniciales |
|---|---|---|
| core | Tenancy, identidad y estructura organizativa | organizations, users, organization_users, branches |
| billing | Operación comercial de la PYME | customers, products, invoices, invoice_lines, credit_notes, debit_notes |
| fiscal | Cumplimiento y documentos electrónicos | electronic_documents, document_lines, configurations, consecutives, hacienda_submissions, cabys |
| receivables | Cobranza y pagos | accounts_receivable, payments, payment_applications, follow_ups |
| subscriptions | Relación comercial SaaS con la empresa | plans, subscriptions, usage, subscription_payments |
| audit | Trazabilidad de negocio | audit_events, access_events |
| integration | Entrega confiable de eventos | outbox_messages, inbox_messages, dead_letters |

### 4.2 Modelo conceptual mínimo

```
core.organizations
  ├── core.organization_users ── core.users
  ├── core.branches
  ├── billing.customers
  ├── billing.products
  └── billing.invoices ── billing.invoice_lines
            ├── fiscal.electronic_documents ── fiscal.hacienda_submissions
            └── receivables.accounts_receivable
                       └── receivables.payment_applications ── receivables.payments
```

### 4.3 Relaciones entre schemas

En V1 pueden existir claves foráneas entre schemas cuando representan una relación estable del dominio, por ejemplo un documento electrónico o una cuenta por cobrar originada por una factura. Toda FK tenant owned debe incluir el identificador de organización para impedir referencias cruzadas por error.

```
UNIQUE (organization_id, id)
FOREIGN KEY (organization_id, customer_id)
  REFERENCES billing.customers (organization_id, id)

UNIQUE (organization_id, invoice_number)
UNIQUE (organization_id, customer_identification)
```

Si posteriormente un dominio se mueve a otra base física, estas FK deberán sustituirse por identificadores externos, contratos y controles de consistencia eventual. Esta evolución es aceptable y no debe simularse prematuramente.

### 4.4 Snapshots históricos

Aunque exista una relación al cliente o producto, la factura emitida y el documento fiscal deben conservar los datos históricos usados en ese momento: nombre, identificación, dirección, descripción, CABYS, precio e impuestos. Cambios futuros en los maestros no alteran documentos emitidos.

### 4.5 Migraciones y permisos

- Cada API administra exclusivamente las migraciones de su schema.

- Cada API usa un rol de base de datos distinto y sin permisos de escritura sobre otros schemas.

- Los cambios cross schema se revisan conjuntamente y se despliegan con compatibilidad hacia atrás.

- Las migraciones no deben depender de un orden implícito entre repositorios; el pipeline declara dependencias.

## 5 Modelo multiempresa

Cada empresa que contrata el SaaS se representa como una organization. Todas las entidades de negocio pertenecientes al tenant incluyen organization_id UUID NOT NULL. El sistema debe admitir múltiples empresas activas simultáneamente sin posibilidad de lectura, relación o modificación cruzada.

### 5.1 Empresa usuario y establecimiento

```
Usuario ──< organization_users >── Organización ──< branches
                                      │
                                      ├── clientes
                                      ├── facturas
                                      ├── configuración fiscal
                                      └── cuentas por cobrar
```

Un usuario puede pertenecer a varias organizaciones con roles diferentes. Esto cubre propietarios, grupos empresariales y contadores que administran varias compañías. Una organización puede tener varias sucursales, establecimientos o terminales.

### 5.2 Identidad del tenant

- El usuario se autentica.

- Selecciona o conserva una organización activa de sus memberships válidos.

- El backend valida esa pertenencia y construye TenantContext.

- La operación usa ese contexto en autorización, consultas, comandos, eventos y auditoría.

- La base de datos aplica restricciones y, donde corresponda, Row Level Security como barrera adicional.

El organization_id enviado en un body, query string o header no es suficiente para autorizar una operación. Debe coincidir con la identidad y membership efectivas.

### 5.3 Aislamiento reforzado en PostgreSQL

- Columnas organization_id obligatorias y no anulables.

- Índices y unicidades compuestas por tenant.

- FK compuestas que impidan relaciones cross tenant.

- RLS evaluado para tablas sensibles, con políticas probadas.

- Roles de conexión con mínimo privilegio.

- Pruebas automatizadas negativas de aislamiento.

- Backups cifrados y restauraciones ensayadas.

### 5.4 Suscripción de la plataforma

La facturación que la PYME realiza a sus clientes no debe mezclarse con lo que la plataforma cobra a la PYME. Planes, suscripciones, límites y pagos del SaaS pertenecen a subscriptions.* o platform.*.

## 6 Integración y eventos

### 6.1 Flujo principal de emisión

```
POST /invoices
      │
      ▼
Billing crea Invoice y OutboxMessage en una transacción
      │
      └── publica InvoiceIssued v1
              ├── E Invoice crea documento y procesa Hacienda
              └── Receivables crea la cuenta por cobrar

Billing responde 201 sin esperar la respuesta de Hacienda.
```

La factura comercial y el documento electrónico tienen máquinas de estado diferentes. Una factura puede estar ISSUED mientras el documento fiscal está PROCESSING, ACCEPTED o REJECTED.

### 6.2 Catálogo inicial de eventos

| Evento | Productor | Consumidores | Uso |
|---|---|---|---|
| InvoiceIssued v1 | Billing | Fiscal y Receivables | Crear documento fiscal y CxC |
| InvoiceCancelled v1 | Billing | Fiscal y Receivables | Iniciar corrección o reverso permitido |
| CreditNoteIssued v1 | Billing | Fiscal y Receivables | Documento fiscal y ajuste de saldo |
| DebitNoteIssued v1 | Billing | Fiscal y Receivables | Documento fiscal y aumento de saldo |
| ElectronicDocumentAccepted v1 | Fiscal | BFF Billing notificaciones | Mostrar aceptación y habilitar acciones |
| ElectronicDocumentRejected v1 | Fiscal | BFF Billing notificaciones | Alertar y gestionar corrección |
| PaymentReceived v1 | Receivables | BFF reportes | Actualizar vistas y notificaciones |
| ReceivableSettled v1 | Receivables | BFF reportes | Marcar saldo cancelado en read models |

### 6.3 Contrato mínimo de InvoiceIssued

```
{
  "eventId": "uuid",
  "eventType": "InvoiceIssued",
  "version": 1,
  "occurredAt": "ISO-8601",
  "correlationId": "uuid",
  "organizationId": "uuid",
  "invoiceId": "uuid",
  "branchId": "uuid",
  "invoiceNumber": "string",
  "customerSnapshot": { "name": "...", "identification": "...", "email": "..." },
  "currency": "CRC",
  "subtotal": 10000,
  "tax": 1300,
  "total": 11300
}
```

Las líneas, descuentos, exoneraciones y demás datos fiscales requerirán un contrato completo antes de desarrollar. Los montos deben usar decimal o unidades menores acordadas; nunca float.

### 6.4 Confiabilidad

- Transactional outbox para guardar factura y evento atómicamente.

- Inbox o tabla de eventos procesados para idempotencia en consumidores.

- eventId único y claves idempotentes por operación.

- Reintentos con backoff y dead letter queue.

- Correlation ID propagado entre API, evento, worker y auditoría.

- Contratos compatibles hacia atrás y versionados explícitamente.

## 7 Seguridad aislamiento y auditoría

### 7.1 Respuesta ante auditorías de clientes

Una base compartida no es, por sí sola, una debilidad de auditoría. El riesgo real es no poder demostrar aislamiento, autorización, integridad, trazabilidad, retención y capacidad de recuperación. El sistema debe producir evidencia verificable de esos controles.

### 7.2 Capas de control

| Capa | Control esperado | Evidencia |
|---|---|---|
| Identidad | Autenticación robusta y sesiones seguras | Registros de autenticación y configuración |
| Autorización | Membership y rol por organización | Pruebas y matriz de permisos |
| Aplicación | TenantContext obligatorio | Tests de integración y revisión de código |
| Base de datos | FK compuestas, roles y RLS selectivo | DDL, políticas y pruebas negativas |
| Cifrado | TLS en tránsito y cifrado en reposo | Configuración del proveedor |
| Auditoría | Eventos de negocio inmutables | Exportación por tenant y correlación |
| Operación | Backups, restore drills y monitoreo | Reportes de ejecución y alertas |

### 7.3 Auditoría de negocio

La auditoría registra quién hizo qué, sobre cuál entidad, para qué organización y cuándo. Debe separarse de los logs técnicos. Un error HTTP 500 ayuda a operar el sistema; no demuestra el cambio de estado de una factura.

```
audit.audit_events
id | organization_id | actor_user_id | action | entity_type | entity_id
occurred_at | correlation_id | source_service | ip_address | metadata

Ejemplo: Invoice 123  DRAFT → ISSUED  por user X  organization Y
```

- Append only y sin UPDATE o DELETE para usuarios de aplicación.

- Registro de valores anteriores y nuevos cuando sea apropiado.

- Motivo obligatorio para anulaciones, reversos y cambios sensibles.

- Timestamps en UTC y presentación en zona horaria de la organización.

- Acceso a exportación de auditoría limitado por rol.

- Protección de metadata para no almacenar secretos ni datos personales innecesarios.

### 7.4 Inmutabilidad financiera y fiscal

Una factura emitida no debe editarse como si fuera borrador. Las correcciones se realizan mediante flujos explícitos, notas de crédito o débito, anulaciones permitidas y nuevos documentos. XML firmados, respuestas y evidencias deben conservarse de acuerdo con las reglas vigentes que se validen.

### 7.5 Oferta enterprise futura

La arquitectura puede ofrecer dos niveles sin alterar el modelo funcional: modalidad estándar con infraestructura compartida y aislamiento lógico; modalidad enterprise con base o infraestructura dedicada, SSO, claves propias, mayor retención y controles contractuales específicos. No es necesario construir la modalidad dedicada en V1.

## 8 Operación y evolución

### 8.1 Independencia de despliegue

Cada API posee su pipeline, Dockerfile, health checks, migraciones, pruebas, métricas y despliegue. Compartir PostgreSQL no debe obligar a desplegar las tres simultáneamente.

### 8.2 Observabilidad

- Logs estructurados con organizationId, correlationId y service.

- Métricas de latencia, errores, colas, reintentos y documentos pendientes.

- Trazas distribuidas entre BFF, APIs y workers.

- Alertas por tasa de rechazo de Hacienda, cola estancada y fallos de publicación.

- Dashboards técnicos separados de reportes de auditoría de negocio.

### 8.3 Estrategia de extracción futura

Fiscal es el principal candidato a separarse en una base propia por su dependencia externa, carga asíncrona y potencial reutilización por Slatiko, Garage, Kitchen u otros productos. Mantener fiscal.* autocontenido facilita esa extracción. Receivables podría separarse después si gana suficiente complejidad o escala.

## 9 Riesgos y mitigaciones

| Riesgo | Impacto | Mitigación |
|---|---|---|
| JOINs indiscriminados entre schemas | Monolito distribuido y dependencia circular | Permisos, revisión, BFF y read models |
| Fuga cross tenant por consulta incompleta | Exposición crítica de datos | TenantContext, RLS selectivo, FK compuestas y tests negativos |
| Hacienda bloquea Billing | Facturación indisponible | Procesamiento asíncrono y outbox |
| Evento duplicado | CxC o documento duplicado | Inbox e idempotencia con índices únicos |
| Migraciones incompatibles | Caídas entre deploys | Cambios expand migrate contract |
| Auditoría mezclada con logs | Evidencia insuficiente | audit.* append only y política de retención |
| Snapshots incompletos | Historia fiscal inconsistente | Contrato y esquema de snapshot obligatorio |
| Base compartida como punto único | Mayor radio de impacto | Backups, HA, PITR, restore drills y límites de conexión |

## 10 Plan de implementación

### Fase 0 Acuerdos del equipo

- Glosario común y límites de cada dominio.

- Ownership de tablas y matriz de permisos.

- Estados de Invoice, ElectronicDocument y Receivable.

- Contrato InvoiceIssued v1 y catálogo inicial de eventos.

- Convenciones de IDs, fechas, dinero, errores e idempotencia.

### Fase 1 Núcleo multi tenant

- core.organizations, users, organization_users y branches.

- Autenticación, selección de organización y TenantContext.

- Roles iniciales y pruebas cross tenant.

- Base de auditoría y correlation IDs.

### Fase 2 Esqueleto de servicios

- Repositorios y pipelines independientes.

- Roles PostgreSQL y schemas.

- Health checks, telemetría y manejo de errores.

- Outbox, bus de mensajes e inbox idempotente.

### Fase 3 Flujo vertical mínimo

- Crear cliente y producto.

- Crear y emitir factura.

- Publicar InvoiceIssued.

- Crear documento fiscal en estado Processing.

- Crear cuenta por cobrar.

- Mostrar vista transversal desde el BFF.

### Fase 4 Integración fiscal real

- Configuración por organización, sucursal y terminal.

- Generación, firma, envío y consulta.

- Reintentos y contingencia.

- Persistencia de XML y respuestas.

- Validación contra normativa vigente.

### Fase 5 Cobranza y operación

- Pagos y payment applications.

- Vencimientos, aging y seguimientos.

- Notas de crédito y débito end to end.

- Exportación de auditoría, backups y pruebas de restauración.

## 11 Decisiones pendientes

| Decisión | Pregunta a resolver | Responsables sugeridos |
|---|---|---|
| Proveedor de identidad | ¿Propio, Supabase, Auth0, Azure AD u otro? | Equipo completo |
| Mensajería | ¿PostgreSQL outbox con worker, RabbitMQ, cloud queue? | Arquitectura e infraestructura |
| RLS | ¿En cuáles tablas y con qué mecanismo de sesión? | Backend y seguridad |
| Impuestos | ¿Billing calcula y Fiscal valida o existe motor común? | Billing y Fiscal |
| CABYS | ¿Quién mantiene catálogo, búsqueda y versionado? | Fiscal |
| Consecutivos | ¿Alcance exacto por organización, sucursal y terminal? | Fiscal y contabilidad |
| Almacenamiento XML | ¿BD, object storage o estrategia híbrida? | Fiscal e infraestructura |
| Retención | ¿Plazos para XML, respuestas, auditoría y logs? | Legal, fiscal y seguridad |
| PII | ¿Clasificación, exportación, rectificación y borrado aplicable? | Legal y seguridad |
| Planes SaaS | ¿Límites por documentos, usuarios o sucursales? | Producto |

Antes de producción se debe revisar la normativa vigente de Hacienda de Costa Rica, la regulación aplicable de protección de datos y los compromisos contractuales ofrecidos a clientes. Este documento no fija plazos legales de conservación ni controles certificados.

## 12 Criterios de aceptación arquitectónica

- Una prueba automatizada demuestra que un usuario de la organización A no puede leer ni relacionar datos de B.

- Billing puede emitir una factura aunque E Invoice esté temporalmente fuera de servicio.

- Reprocesar InvoiceIssued no crea documentos fiscales ni cuentas por cobrar duplicadas.

- Cada API carece de permisos de escritura sobre schemas ajenos.

- La factura emitida conserva snapshots aunque cambien cliente o producto.

- Toda operación sensible produce un audit event correlacionado.

- Una restauración de backup se ejecuta y verifica en un entorno aislado.

- Los tres servicios se despliegan de manera independiente sin romper contratos vigentes.

- El BFF presenta una factura con su estado fiscal y saldo sin introducir dependencia circular en Billing.

- Los documentos rechazados por Hacienda tienen un flujo operativo visible y recuperable.

## Conclusión

La propuesta equilibra autonomía de equipo, simplicidad operativa y seguridad multiempresa: tres APIs, una base PostgreSQL y schemas claramente gobernados. El éxito no depende de separar físicamente cada dato desde V1, sino de aplicar ownership, permisos, contratos, idempotencia, aislamiento tenant y auditoría verificable. Con estas reglas, la plataforma puede servir a PYMEs desde el inicio y conservar una ruta razonable hacia clientes enterprise y servicios independientes.
