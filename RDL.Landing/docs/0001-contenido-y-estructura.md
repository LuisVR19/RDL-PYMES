# 0001 · Contenido y estructura de la landing

**Estado:** propuesta para aprobación (Paso 1 del prompt P8c). **No se programa nada hasta aprobarla.**
**Supuesto de trabajo (decidido 2026-09-27):** la landing presenta el producto **completo**, tal como lo describen los
prompts P3–P8 (facturación, Hacienda, cobranza, multiempresa, roles). Llamado principal: **«Crear cuenta»**.
**Marca:** RDL (provisional). **Precios:** «Precios próximamente».

Reglas que guían todo el texto: español de Costa Rica, trato de usted, frases concretas, nada inventado (ni
testimonios, ni cifras, ni clientes, ni certificaciones). Hacienda no certifica proveedores: la palabra «certificado»
solo aparece para hablar del **certificado de firma** del contribuyente.

---

## 1. Público y mensajes

| Público                          | Su problema                                                                                                                                                     | Mensaje principal                                                                                                                                         |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Dueño de la PYME**             | Factura en un sistema, revisa Hacienda en otro y lleva el cobro en una hoja de cálculo. No sabe de un vistazo cuánto le deben ni qué facturas tienen problemas. | «Facture, siga a Hacienda y cobre en un solo lugar.» Ve en una pantalla el total, el estado ante Hacienda y lo que falta cobrar.                          |
| **Quien factura** (facturador)   | Arma la factura, espera la respuesta de Hacienda y, si la rechazan, tiene que averiguar por qué y corregirla.                                                   | «Emita en pocos pasos y sepa al momento si Hacienda la aceptó.» Buscador CABYS, borradores con totales calculados por el sistema y bandeja de rechazados. |
| **Contador con varias empresas** | Entra y sale de un sistema por cliente, con usuarios distintos, y pierde tiempo cambiando de sesión.                                                            | «Todas sus empresas, un solo acceso.» Cambia de empresa sin cerrar sesión, con el rol que le dio cada una.                                                |

---

## 2. Mapa del sitio

| Ruta          | Página                                      | Objetivo                                                     | Se indexa                      |
| ------------- | ------------------------------------------- | ------------------------------------------------------------ | ------------------------------ |
| `/`           | Principal                                   | Explicar, convencer y llevar a «Crear cuenta» o «Pedir demo» | Sí                             |
| `/contadores` | Para contadores                             | Página dedicada al tercer público (palabra clave propia)     | Sí                             |
| `/terminos`   | Términos y condiciones (borrador)           | Legal                                                        | **No** hasta la revisión legal |
| `/privacidad` | Política de privacidad (borrador, Ley 8968) | Legal y enlace del formulario                                | **No** hasta la revisión legal |
| `/cookies`    | Política de cookies                         | Solo si la analítica usa cookies                             | Según decisión D4              |
| `/gracias`    | Gracias por escribirnos                     | Confirmación del formulario                                  | No                             |
| `/404`        | No encontrada                               | Volver al inicio                                             | No                             |

**Por qué `/contadores` aparte:** el prompt pide la sección en la principal, y la mantengo; además propongo una página
corta propia porque «software contable para varias empresas» es una búsqueda distinta. Si no la aprueba, queda solo
la sección.

### Orden de secciones de la página principal

| #   | Sección                      | Ancla            | Objetivo                                                               |
| --- | ---------------------------- | ---------------- | ---------------------------------------------------------------------- |
| 1   | Encabezado fijo              | —                | Marca, navegación, «Iniciar sesión» y «Crear cuenta» siempre a mano    |
| 2   | Hero                         | `#inicio`        | Qué es y para quién en una línea; los dos llamados; maqueta del portal |
| 3   | El problema                  | `#problema`      | Reconocerse en el dolor de tres herramientas separadas                 |
| 4   | Funciones                    | `#funciones`     | Las seis capacidades, cada una con su beneficio                        |
| 5   | Una factura, tres respuestas | `#vista-factura` | Diferencial: total, Hacienda y saldo juntos (con maqueta)              |
| 6   | Cómo funciona                | `#como-funciona` | Tres pasos: empresa y Hacienda → facturar → cobrar                     |
| 7   | Para contadores              | `#contadores`    | Multiempresa y roles                                                   |
| 8   | Confianza y seguridad        | `#seguridad`     | Solo afirmaciones verificables                                         |
| 9   | Precios                      | `#precios`       | «Próximamente»; lleva a la demo                                        |
| 10  | Preguntas frecuentes         | `#preguntas`     | Resolver objeciones; JSON-LD `FAQPage`                                 |
| 11  | Llamado final + formulario   | `#contacto`      | Convertir: demo o contacto                                             |
| 12  | Pie de página                | —                | Contacto, legales, «Iniciar sesión», año                               |

Navegación del encabezado: Funciones · Cómo funciona · Contadores · Seguridad · Precios · Preguntas.

---

## 3. Textos

Convención: `ID` = clave en el archivo de contenido. `{portal}` y `{registro}` son las URLs configuradas por
ambiente. Los marcadores `<<…>>` son decisiones pendientes: **el build falla si alguno llega a una página
publicable**.

### 3.1 Encabezado y pie

| ID               | Texto                                                                                              |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| `nav.login`      | Iniciar sesión                                                                                     |
| `nav.signup`     | Crear cuenta                                                                                       |
| `nav.menu`       | Menú (botón en móvil; nombre accesible «Abrir menú» / «Cerrar menú»)                               |
| `nav.skip`       | Saltar al contenido                                                                                |
| `footer.tagline` | Facturación electrónica y cobranza para PYMES de Costa Rica.                                       |
| `footer.product` | Producto: Funciones · Cómo funciona · Para contadores · Seguridad · Precios · Preguntas frecuentes |
| `footer.contact` | Contacto: `<<ventas@…>>` · WhatsApp `<<+506 …>>` · `<<horario>>`                                   |
| `footer.legal`   | Términos y condiciones · Política de privacidad · Política de cookies (si aplica)                  |
| `footer.login`   | ¿Ya tiene cuenta? Iniciar sesión                                                                   |
| `footer.copy`    | © {año} RDL. Todos los derechos reservados.                                                        |

### 3.2 Hero

| ID                   | Texto                                                                                                                                                                                   |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hero.eyebrow`       | Facturación electrónica · versión 4.4 de Hacienda                                                                                                                                       |
| `hero.title`         | Facture, siga a Hacienda y cobre, en un solo lugar                                                                                                                                      |
| `hero.subtitle`      | RDL es el sistema de facturación electrónica y cobranza para PYMES de Costa Rica. Emita facturas y notas, sepa al momento si Hacienda las aceptó y lleve el control de lo que le deben. |
| `hero.cta.primary`   | Crear cuenta                                                                                                                                                                            |
| `hero.cta.secondary` | Pedir una demo                                                                                                                                                                          |
| `hero.note`          | ¿Ya usa RDL? [Iniciar sesión]                                                                                                                                                           |
| `hero.image.alt`     | Vista de una factura en RDL: total de ₡113 000,00, aceptada por Hacienda y con un saldo pendiente de ₡63 000,00. Datos de ejemplo.                                                      |

La maqueta es la pantalla 15 del diseño del portal con los **datos ficticios del prototipo** («Comercial Los
Almendros S.A.», FAC-0000034). Nunca una captura con datos reales.

### 3.3 El problema

| ID                | Texto                                                                                                                |
| ----------------- | -------------------------------------------------------------------------------------------------------------------- |
| `problem.title`   | Tres tareas que no deberían vivir en tres lugares                                                                    |
| `problem.1.title` | Facturar                                                                                                             |
| `problem.1.body`  | Un sistema para emitir, con clientes y productos que hay que mantener al día.                                        |
| `problem.2.title` | Estar pendiente de Hacienda                                                                                          |
| `problem.2.body`  | Revisar si cada comprobante fue aceptado, rechazado o quedó en espera, y averiguar qué corregir.                     |
| `problem.3.title` | Cobrar                                                                                                               |
| `problem.3.body`  | Una hoja de cálculo para saber quién debe, cuánto y desde cuándo.                                                    |
| `problem.close`   | En RDL las tres están conectadas: al emitir una factura, el sistema la envía a Hacienda y crea la cuenta por cobrar. |

### 3.4 Funciones

| ID                     | Título                                      | Cuerpo                                                                                                                                                                                |
| ---------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `features.title`       | Todo lo que necesita para facturar y cobrar | —                                                                                                                                                                                     |
| `features.invoicing`   | Facturación electrónica 4.4                 | Facturas, notas de crédito y notas de débito según la versión 4.4 de los comprobantes de Hacienda. Clientes, productos con buscador CABYS, borradores y emisión.                      |
| `features.hacienda`    | El estado de Hacienda, al momento           | Cada documento muestra si está en proceso, aceptado, rechazado o en contingencia. Los rechazados llegan a una bandeja para corregirlos, y el portal le avisa cuando cambia un estado. |
| `features.collections` | Cobranza sin hojas de cálculo               | La cuenta por cobrar se crea sola al emitir. Registre pagos y aplíquelos a una o varias facturas, vea los vencimientos y anote seguimientos y promesas de pago.                       |
| `features.aging`       | Antigüedad de saldos                        | Vea cuánto le deben por tramos de atraso, por moneda y a la fecha que elija.                                                                                                          |
| `features.multiorg`    | Varias empresas, un solo acceso             | Pensado para contadores que llevan varias PYMES: cambie de empresa sin cerrar sesión.                                                                                                 |
| `features.roles`       | Cada quien con su rol                       | Propietario, administrador, facturador, cobrador, contador y solo lectura. Invite a su equipo con un enlace.                                                                          |

### 3.5 Una factura, tres respuestas

| ID               | Texto                                                                                                                                                           |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `view.title`     | Una factura, tres respuestas en la misma pantalla                                                                                                               |
| `view.body`      | Abra cualquier factura y vea juntos cuánto se facturó, qué dijo Hacienda y cuánto falta por cobrar. Si una parte no está disponible, el resto sigue a la vista. |
| `view.point.1`   | **Total** — lo que se facturó, con sus líneas e impuestos.                                                                                                      |
| `view.point.2`   | **Hacienda** — en proceso, aceptada, rechazada con su motivo o en contingencia.                                                                                 |
| `view.point.3`   | **Saldo** — lo pendiente, los pagos aplicados y los días de atraso.                                                                                             |
| `view.image.alt` | Detalle de factura con tres cifras: total ₡113 000,00, estado Aceptada y saldo ₡63 000,00. Datos de ejemplo.                                                    |

### 3.6 Cómo funciona

| ID            | Texto                                                                                                                               |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `how.title`   | Empiece en tres pasos                                                                                                               |
| `how.1.title` | Cree su empresa y configure Hacienda                                                                                                |
| `how.1.body`  | Registre su empresa, suba su certificado de firma y conecte su usuario de Hacienda. Primero puede probar en el ambiente de pruebas. |
| `how.2.title` | Facture                                                                                                                             |
| `how.2.body`  | Elija el cliente, agregue productos del catálogo y emita. El sistema calcula los totales y envía el comprobante a Hacienda.         |
| `how.3.title` | Cobre y dé seguimiento                                                                                                              |
| `how.3.body`  | Cada factura crea su cuenta por cobrar. Registre los pagos y vea qué está vencido.                                                  |
| `how.cta`     | Crear cuenta                                                                                                                        |

### 3.7 Para contadores (sección y página `/contadores`)

| ID                     | Texto                                                                                                                                                                                 |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `acc.title`            | Para contadores: todas sus empresas en un solo acceso                                                                                                                                 |
| `acc.body`             | Si lleva la facturación de varias PYMES, entre una sola vez y cambie de empresa desde la barra superior, sin cerrar sesión. En cada una trabaja con el rol que esa empresa le asignó. |
| `acc.point.1`          | Cambio de empresa en un clic, sin mezclar datos: cada empresa está aislada de las demás.                                                                                              |
| `acc.point.2`          | El rol de contador ve facturación, Hacienda y cobranza sin poder emitir ni modificar.                                                                                                 |
| `acc.point.3`          | Las empresas lo invitan con un enlace; usted acepta con su mismo usuario.                                                                                                             |
| `acc.cta`              | Pedir una demo para contadores                                                                                                                                                        |
| `acc.page.title`       | Facturación electrónica para contadores con varias empresas                                                                                                                           |
| `acc.page.description` | Lleve la facturación y la cobranza de varias PYMES de Costa Rica con un solo usuario. Cambie de empresa sin cerrar sesión.                                                            |

Afirmación a confirmar: `acc.point.2` sale de la matriz de permisos del portal (el contador lee facturación, Hacienda
y cobranza; no emite). Si el equipo cambia ese rol, se ajusta el texto.

### 3.8 Confianza y seguridad

| ID                | Texto                                                                                                                                        |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| `trust.title`     | Sus datos, protegidos                                                                                                                        |
| `trust.isolation` | **Cada empresa, aislada.** La separación entre empresas se aplica en la propia base de datos, no solo en la pantalla.                        |
| `trust.audit`     | **Bitácora de auditoría.** Las operaciones sensibles (emitir, anular, cambiar roles, registrar pagos) quedan registradas con quién y cuándo. |
| `trust.cert`      | **Certificado de firma protegido.** Se guarda cifrado en un gestor de secretos y el PIN nunca se vuelve a mostrar.                           |
| `trust.version`   | **Versión vigente de Hacienda.** Comprobantes según la versión 4.4 de los documentos electrónicos.                                           |
| `trust.note`      | RDL no es un sistema de Hacienda: es un proveedor que emite sus comprobantes a través de los servicios oficiales.                            |

### 3.9 Precios

| ID              | Texto                                                                                       |
| --------------- | ------------------------------------------------------------------------------------------- |
| `pricing.title` | Precios                                                                                     |
| `pricing.body`  | Estamos definiendo los planes. Pida una demo y le contamos las condiciones para su empresa. |
| `pricing.cta`   | Pedir una demo                                                                              |

### 3.10 Preguntas frecuentes (también en JSON-LD `FAQPage`)

| ID                | Pregunta                                          | Respuesta                                                                                                                                                                                                                        |
| ----------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `faq.v44`         | ¿Qué es la versión 4.4 de la factura electrónica? | Es la versión vigente del formato de los comprobantes electrónicos que define el Ministerio de Hacienda. RDL emite facturas y notas de crédito y débito en esa versión.                                                          |
| `faq.start`       | ¿Qué necesito para empezar?                       | Su certificado de firma (la llave criptográfica y su PIN) y el usuario y la contraseña de los servicios de comprobantes electrónicos de Hacienda. Con eso configura su empresa y puede probar primero en el ambiente de pruebas. |
| `faq.switch`      | ¿Puedo cambiarme desde otro sistema?              | Sí. Registre sus clientes y productos y configure la numeración para seguir con el consecutivo que traía.                                                                                                                        |
| `faq.down`        | ¿Qué pasa si Hacienda no responde?                | El documento queda emitido y en contingencia, y el sistema lo reenvía solo. Usted puede seguir facturando y ve el estado de cada comprobante en la bandeja.                                                                      |
| `faq.accountants` | Soy contador, ¿puedo llevar varias empresas?      | Sí. Con un solo usuario trabaja en todas las empresas que lo inviten y cambia entre ellas sin cerrar sesión.                                                                                                                     |
| `faq.team`        | ¿Puedo dar acceso a mi equipo?                    | Sí. Invite a cada persona con un enlace y asígnele un rol: administrador, facturador, cobrador, contador o solo lectura.                                                                                                         |
| `faq.price`       | ¿Cuánto cuesta?                                   | Estamos definiendo los planes. Pida una demo y le contamos las condiciones.                                                                                                                                                      |
| `faq.support`     | ¿Cómo me dan soporte?                             | Por correo a `<<ventas@…>>` y por WhatsApp al `<<+506 …>>`, `<<horario>>`.                                                                                                                                                       |

Afirmaciones a confirmar con el equipo: `faq.switch` (la numeración configurable existe en Billing; **no** se promete
importar clientes ni productos, porque esa función no está en el producto) y `faq.down` (contingencia y reenvío
automático son del diseño de E-Invoice, P5).

### 3.11 Llamado final y formulario

| ID                         | Texto                                                                                                     |
| -------------------------- | --------------------------------------------------------------------------------------------------------- |
| `contact.title`            | Hablemos de su empresa                                                                                    |
| `contact.body`             | Pida una demo o escríbanos. Le respondemos en `<<plazo, p. ej. un día hábil>>`.                           |
| `contact.alt`              | ¿Prefiere empezar ya? [Crear cuenta]                                                                      |
| `form.name`                | Nombre *                                                                                                  |
| `form.company`             | Empresa *                                                                                                 |
| `form.email`               | Correo *                                                                                                  |
| `form.phone`               | Teléfono (opcional)                                                                                       |
| `form.type`                | Usted es * — «Una PYME» / «Contador o firma contable»                                                     |
| `form.volume`              | Facturas al mes, aproximadamente * — «Menos de 50» / «50 a 200» / «200 a 1000» / «Más de 1000»            |
| `form.message`             | Mensaje (opcional)                                                                                        |
| `form.message.placeholder` | Cuéntenos qué necesita                                                                                    |
| `form.consent`             | Acepto que RDL use estos datos para responder mi solicitud, según la [Política de privacidad]. *          |
| `form.submit`              | Enviar                                                                                                    |
| `form.submitting`          | Enviando…                                                                                                 |
| `form.err.required`        | Este dato es obligatorio.                                                                                 |
| `form.err.email`           | Escriba un correo válido, por ejemplo nombre@empresa.cr.                                                  |
| `form.err.phone`           | Escriba un teléfono válido, por ejemplo 8888 8888.                                                        |
| `form.err.consent`         | Para enviar, acepte el uso de sus datos.                                                                  |
| `form.err.summary`         | Revise los datos marcados.                                                                                |
| `form.err.send`            | No pudimos enviar su mensaje. Lo que escribió sigue aquí; intente de nuevo o escríbanos a `<<ventas@…>>`. |
| `form.err.rate`            | Recibimos varios envíos seguidos. Espere unos minutos e intente de nuevo.                                 |
| `form.err.challenge`       | No pudimos verificar el envío. Recargue la página e intente de nuevo.                                     |
| `thanks.title`             | Gracias por escribirnos                                                                                   |
| `thanks.body`              | Recibimos su mensaje. Le respondemos en `<<plazo>>` al correo que nos dejó.                               |
| `thanks.back`              | Volver al inicio                                                                                          |

### 3.12 Página 404

| ID          | Texto                                                                               |
| ----------- | ----------------------------------------------------------------------------------- |
| `404.title` | No encontramos esta página                                                          |
| `404.body`  | Puede que el enlace esté incompleto. Vuelva al inicio o inicie sesión en el portal. |
| `404.home`  | Ir al inicio                                                                        |

### 3.13 Metadatos (SEO)

| Página             | `<title>`                                                         | Descripción                                                                                                                                                  |
| ------------------ | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `/`                | Factura electrónica y cobranza para PYMES en Costa Rica · RDL     | Emita facturas electrónicas versión 4.4, siga su estado ante Hacienda y controle lo que le deben, en un solo sistema. Para PYMES y contadores de Costa Rica. |
| `/contadores`      | Facturación electrónica para contadores con varias empresas · RDL | (`acc.page.description`)                                                                                                                                     |
| `/terminos`        | Términos y condiciones · RDL                                      | Condiciones de uso del servicio RDL.                                                                                                                         |
| `/privacidad`      | Política de privacidad · RDL                                      | Cómo tratamos sus datos personales según la Ley 8968.                                                                                                        |
| `/gracias`, `/404` | Gracias · RDL / Página no encontrada · RDL                        | —                                                                                                                                                            |

---

## 4. Palabras clave (Costa Rica)

| Palabra clave                                 | Página que apunta | Dónde aparece                                        |
| --------------------------------------------- | ----------------- | ---------------------------------------------------- |
| factura electrónica Costa Rica                | `/`               | título, H1 (variante), descripción                   |
| sistema de facturación electrónica para pymes | `/`               | subtítulo, sección Funciones                         |
| facturación electrónica versión 4.4           | `/`               | eyebrow del hero, Funciones, FAQ                     |
| software de cobranza / cuentas por cobrar     | `/`               | Funciones (cobranza), sección de la vista de factura |
| antigüedad de saldos                          | `/`               | Funciones (aging)                                    |
| bandeja de rechazados Hacienda                | `/`               | Funciones (Hacienda)                                 |
| facturación electrónica para contadores       | `/contadores`     | título, H1, cuerpo                                   |
| programa de facturación varias empresas       | `/contadores`     | cuerpo                                               |

Ninguna página usa «certificado por Hacienda», «aprobado por Hacienda» ni similares (el build lo verifica).

---

## 5. Wireframes

### Escritorio (1440 px, contenido de 1200 px máximo)

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ [RDL] RDL   Funciones  Cómo funciona  Contadores  Seguridad  Precios  Preguntas│
│                                                  Iniciar sesión [Crear cuenta]│  ← fijo
├──────────────────────────────────────────────────────────────────────────────┤
│ FACTURACIÓN ELECTRÓNICA · VERSIÓN 4.4      ┌──────────────────────────────┐  │
│ Facture, siga a Hacienda y cobre,          │  maqueta pantalla 15         │  │
│ en un solo lugar                           │  ₡113 000,00 │ Aceptada │ ₡63 │  │
│ Subtítulo (2–3 líneas)                     │  líneas …                    │  │
│ [Crear cuenta]  [Pedir una demo]           └──────────────────────────────┘  │
│ ¿Ya usa RDL? Iniciar sesión                                                   │
├──────────────────────────────────────────────────────────────────────────────┤
│ Tres tareas que no deberían vivir en tres lugares                             │
│ [Facturar]        [Estar pendiente de Hacienda]        [Cobrar]    + cierre   │
├──────────────────────────────────────────────────────────────────────────────┤
│ Todo lo que necesita…   rejilla 3 × 2 de tarjetas con icono Lucide           │
├──────────────────────────────────────────────────────────────────────────────┤
│ Una factura, tres respuestas    │ texto + 3 puntos │ maqueta de las 3 cifras │
├──────────────────────────────────────────────────────────────────────────────┤
│ Empiece en tres pasos   ①───────②───────③          [Crear cuenta]            │
├──────────────────────────────────────────────────────────────────────────────┤
│ Para contadores (fondo --color-bg-nav, texto claro) │ maqueta del selector   │
├──────────────────────────────────────────────────────────────────────────────┤
│ Sus datos, protegidos   4 bloques en 2 × 2 + nota                             │
├──────────────────────────────────────────────────────────────────────────────┤
│ Precios · próximamente [Pedir una demo]                                        │
├──────────────────────────────────────────────────────────────────────────────┤
│ Preguntas frecuentes   acordeón (<details>), 2 columnas de ancho de lectura   │
├──────────────────────────────────────────────────────────────────────────────┤
│ Hablemos de su empresa  │ formulario (2 columnas de campos)                   │
├──────────────────────────────────────────────────────────────────────────────┤
│ pie: marca + lema │ Producto │ Contacto │ Legal │ Iniciar sesión │ © 2026     │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Móvil (390 px, 16 px de margen)

```
┌────────────────────────────┐
│ [RDL] RDL   [Iniciar] [☰]  │  ← fijo; el menú abre un panel con anclas y «Crear cuenta»
├────────────────────────────┤
│ FACTURACIÓN ELECTRÓNICA 4.4│
│ Facture, siga a Hacienda   │
│ y cobre, en un solo lugar  │
│ Subtítulo                  │
│ [ Crear cuenta          ]  │  ← botones a lo ancho, 44 px
│ [ Pedir una demo        ]  │
│ ¿Ya usa RDL? Iniciar sesión│
│ ┌────────────────────────┐ │
│ │ maqueta (versión móvil │ │
│ │ de la pantalla 15)     │ │
│ └────────────────────────┘ │
├────────────────────────────┤
│ secciones en una columna   │
│ tarjetas apiladas          │
│ pasos en vertical          │
│ FAQ en acordeón            │
│ formulario en una columna  │
├────────────────────────────┤
│ pie apilado                │
└────────────────────────────┘
  [WhatsApp] flotante abajo a la derecha si hay número configurado
```

---

## 6. Dirección visual y stack (resumen; ADRs al programar)

- **Mismo producto que el portal:** `design/tokens.css` copiado tal cual (colores, tipografía, radios, sombras,
  movimiento), IBM Plex Sans y Mono alojadas en el propio sitio con `font-display: swap`, icono «1a Sigla», iconos
  Lucide. Tema claro; el portal trae oscuro, así que se ofrece según `prefers-color-scheme` sin botón.
- **Maquetas del portal hechas en HTML/CSS** con los tokens (no capturas): nítidas, livianas y con datos ficticios.
- **Astro** (HTML estático; JavaScript solo en el menú móvil y el formulario). Textos en archivos de contenido
  (`src/content/*.json`), nunca en los componentes.
- Presupuesto: < 150 KB de JavaScript comprimido en la principal (el objetivo real es < 20 KB).

---

## 7. Decisiones tomadas (2026-09-27, Luis)

- **Aprobado** el contenido, la estructura y el stack (Astro), con estos cambios:
- **«Crear cuenta» dice «Próximamente»** (botón deshabilitado con esa leyenda) hasta que el portal tenga registro (D1).
- **Sin formulario por ahora.** El llamado final es **«Contactarme por WhatsApp»** con mensaje prellenado; la página
  `/gracias` y los textos `form.*` / `thanks.*` quedan fuera. D3 y D5 quedan sin efecto mientras no haya formulario.
- **Contacto desde variables de entorno** (`PUBLIC_CONTACT_*`), con valores temporales de prueba en `.env.example`.
- **Analítica: Umami** (sin cookies): sin aviso de consentimiento ni página `/cookies`. Si no se configura, no se
  carga ningún script.
- **Textos legales completos** (términos y privacidad), como **borrador** con `noindex` y revisión legal obligatoria.

## 8. Decisiones pendientes (registro original)

| #   | Decisión                                                                                                                                               | Propuesta                                                                                                                                                                                                                    | Bloquea                           |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |
| D1  | **Registro de cuentas en el portal.** Hoy el portal no tiene «crear cuenta» (Supabase `signUp` + pantalla).                                            | La landing apunta a `{portal}/registro` por variable de entorno. Hay que construir esa pantalla en el portal.                                                                                                                | «Crear cuenta» funcional          |
| D2  | **Dominio y URLs:** sitio, portal, inicio de sesión, registro                                                                                          | `<<https://www.dominio.cr>>`, `<<https://app.dominio.cr>>`, `/ingresar`, `/registro`                                                                                                                                         | SEO (canónicas, sitemap), enlaces |
| D3  | **Destino del formulario**                                                                                                                             | Función serverless que valida, aplica antispam y manda un correo con un servicio transaccional (Resend o Postmark). Nunca a los schemas de las APIs de dominio. Mientras tanto, adapter de desarrollo a consola. ADR aparte. | Formulario en producción          |
| D4  | **Analítica**                                                                                                                                          | **Plausible o Umami** (sin cookies): sin aviso de consentimiento ni página de cookies.                                                                                                                                       | Aviso de cookies, `/cookies`      |
| D5  | **Antispam**                                                                                                                                           | Campo trampa + tiempo mínimo + **Cloudflare Turnstile** + límite por IP en la función.                                                                                                                                       | Formulario en producción          |
| D6  | **Contacto:** correo, WhatsApp, horario, plazo de respuesta                                                                                            | `<<pendiente>>`                                                                                                                                                                                                              | Pie, FAQ soporte, botón WhatsApp  |
| D7  | **Precios**                                                                                                                                            | «Próximamente» (decidido)                                                                                                                                                                                                    | —                                 |
| D8  | **Textos legales**                                                                                                                                     | Borradores basados en la Ley 8968, marcados «borrador», `noindex` y **revisión legal obligatoria** antes de publicar                                                                                                         | Publicar                          |
| D9  | **Nombre comercial**                                                                                                                                   | RDL (provisional, decidido)                                                                                                                                                                                                  | Marca definitiva, dominio         |
| D10 | **Hosting**                                                                                                                                            | Depende de P2; entrega con Dockerfile (Caddy + cabeceras de seguridad)                                                                                                                                                       | Despliegue                        |
| D11 | **Redes sociales**                                                                                                                                     | `<<pendiente>>`; si no hay, el pie no las muestra                                                                                                                                                                            | Pie, JSON-LD `sameAs`             |
| D12 | **Página `/contadores`** aparte                                                                                                                        | Sí (ver §2)                                                                                                                                                                                                                  | —                                 |
| D13 | **Afirmaciones a confirmar:** `acc.point.2` (alcance del rol contador), `faq.switch` (sin importación), `faq.down` (contingencia y reenvío automático) | Confirmar con el equipo                                                                                                                                                                                                      | Textos finales                    |
