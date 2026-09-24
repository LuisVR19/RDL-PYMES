# Glosario

Cada término tiene un único significado en todo el sistema. Columna "Dueño": la API que lo crea y lo modifica.

| Término | Definición | Dueño | No es |
|---|---|---|---|
| **Organización** | Empresa (PYME) que contrata el SaaS. Es el tenant: todo dato de negocio pertenece a una (`organization_id`). | Platform (`core.organizations`) | No es el cliente final al que la PYME le factura. |
| **Tenant** | Sinónimo técnico de organización en el contexto de aislamiento de datos. | Platform | No es un usuario ni una sucursal. |
| **Organización activa** | La organización con la que el usuario está operando ahora. Sale del claim `org_id` del JWT. | Platform | No es un parámetro de la petición. |
| **Usuario** | Persona que inicia sesión (Supabase Auth). Puede pertenecer a varias organizaciones. | Platform (`core.users`) | No es un cliente final. |
| **Miembro / membresía** | Relación usuario ↔ organización, con un rol y un estado (`active`, `suspended`). | Platform (`core.organization_users`) | No es una invitación. |
| **Rol** | Conjunto de permisos dentro de una organización: `owner`, `admin`, `biller`, `collector`, `accountant`, `read_only`. | Platform (`core.roles`) | No es un rol de base de datos (`billing_app`). |
| **Invitación** | Propuesta de membresía a un email, con token de un solo uso y vencimiento. | Platform | No crea al usuario. |
| **Sucursal** | Punto de operación de la organización (`core.branches`), con código propio. Se usa en facturación y numeración. | Platform | No es, necesariamente, el establecimiento fiscal. Ver "Establecimiento". |
| **Establecimiento / terminal** | Unidades que exige Hacienda para numerar los comprobantes (`fiscal.establishments`, código de 3 dígitos; `fiscal.terminals`, 5 dígitos). | fiscal | No es la sucursal. La relación sucursal ↔ establecimiento/terminal está **abierta** (D11). |
| **Cliente (final)** | Persona o empresa a la que la organización le vende y factura. Identificación única por organización. | Billing (`billing.customers`) | No es una organización del SaaS ni un usuario. |
| **Producto / servicio** | Lo que la organización vende, con código CABYS, unidad, precio e impuestos. | Billing (`billing.products`) | No es un plan del SaaS. |
| **CABYS** | Catálogo de bienes y servicios de Hacienda. Código de 13 dígitos. | fiscal (catálogo); Billing lo referencia | Billing no mantiene el catálogo. |
| **Factura** (comercial) | Documento de venta de la organización a un cliente. Estados `draft`, `issued`, `cancelled`. | Billing (`billing.invoices`, `document_type = invoice`) | No es el documento electrónico ni el comprobante que va a Hacienda. |
| **Nota de crédito / débito** | Documento comercial que corrige una factura emitida (disminuye o aumenta lo cobrado). Referencia la factura y lleva motivo. | Billing (`billing.invoices` con `document_type` `credit_note` / `debit_note`) | No es un ajuste de CxC: el ajuste es su efecto en Receivables. |
| **Emisión** | Transición `draft` → `issued` de una factura o nota: fija número, snapshots y montos, y publica el evento. | Billing | No es el envío a Hacienda. |
| **Anulación** | Transición a `cancelled` de un documento emitido, con motivo obligatorio. | Billing | No es borrar: lo emitido nunca se borra. |
| **Número visible** | Numeración comercial por organización, tipo de documento y sucursal (`billing.document_sequences`). | Billing | No es el consecutivo fiscal. |
| **Documento electrónico** | Representación fiscal de una factura o nota ante Hacienda: XML firmado, clave y estado fiscal. | fiscal (`fiscal.electronic_documents`) | No es la factura comercial. Una factura puede estar `issued` y su documento `processing`, `accepted` o `rejected`. |
| **Consecutivo fiscal** | Número de 20 dígitos que exige Hacienda por documento. | fiscal (`fiscal.document_sequences`) | No es el número visible. |
| **Clave numérica** | Identificador de 50 dígitos del comprobante ante Hacienda. | fiscal | No es un UUID del sistema. |
| **Contingencia** | Estado fiscal cuando no se puede comunicar con Hacienda y el documento se procesa según el procedimiento de contingencia. | fiscal | No es un error del documento. |
| **Cuenta por cobrar (CxC)** | Saldo que un cliente le debe a la organización por un documento emitido. Estados `open`, `partially_paid`, `paid`, `cancelled`. | Receivables (`receivables.receivables`) | No es la factura: nace de ella por evento. |
| **Pago** | Dinero recibido de un cliente, en una fecha, con un medio de pago. Estados `posted`, `voided`. | Receivables (`receivables.payments`) | No es la aplicación. Un pago puede quedar sin aplicar. |
| **Aplicación de pago** | Asignación de una parte de un pago a una CxC. Un pago se aplica a varias CxC y una CxC recibe varios pagos. Se revierte con motivo. | Receivables (`receivables.payment_applications`) | No es el pago. |
| **Ajuste de CxC** | Cambio del saldo por nota de crédito, nota de débito, anulación o castigo (`write_off`). | Receivables (`receivables.receivable_adjustments`) | No es un pago. |
| **Aging / morosidad** | Antigüedad de los saldos vencidos agrupada por rangos. | Receivables | No es un estado de la CxC. |
| **Promesa de pago / seguimiento** | Compromiso del cliente y registro de gestiones de cobro. | Receivables | No modifica el saldo. |
| **Snapshot** | Copia de los datos usados al emitir (cliente, producto, precios, impuestos). Los cambios posteriores en los maestros no lo alteran. | quien emite (Billing) | No es una referencia al maestro. |
| **Evento** | Hecho de negocio ya ocurrido, publicado por su dueño vía outbox (`InvoiceIssued`). | el productor | No es un comando ni una petición. |
| **Outbox / inbox** | Tablas de `integration` para publicar eventos de forma atómica y deduplicar al consumirlos. | database-platform | No son colas de mensajería. |
| **Suscripción** | Relación comercial del SaaS con la organización (plan, período, uso). | Platform (`subscriptions`) | No tiene nada que ver con la facturación de la PYME a sus clientes. |

## TODOs y preguntas abiertas

- D11: relación entre sucursal y establecimiento/terminal fiscal, y alcance exacto de los consecutivos. Sin valor por
  defecto: fiscal + contabilidad.
- `TODO(fiscal)`: nombres oficiales de los tipos de comprobante según la especificación de Hacienda.
