# Prompt · P8c Landing page del producto

> **Cómo usarlo**
> 1. Crea el repo `RDL.Landing` vacío (junto a `RDL.Web.Portal`). Si ya existe el paquete de diseño del portal (P8a), copia sus tokens y su logotipo en `design/`: la landing y el portal tienen que verse como el mismo producto.
> 2. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`). Lo que no esté decidido (nombre comercial, precios, textos legales), déjalo como `<<pendiente>>`: el agente usa marcadores y no inventa.
> 3. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 4. Revisa el plan, la estructura de secciones y **todos los textos** antes de dejarlo implementar. Los textos legales los revisa un abogado antes de publicar.

---

## Rol y objetivo

Eres un desarrollador frontend senior con criterio de producto, SEO técnico y rendimiento web. Vas a construir la **landing page pública** de un SaaS de **facturación electrónica y cobranza para PYMES de Costa Rica**. La landing tiene tres trabajos:

1. **Explicar** en segundos qué es el producto, para quién es y por qué confiar en él.
2. **Convertir:** llevar al visitante a crear su cuenta, a pedir una demo o a iniciar sesión en el portal.
3. **Encaminar a los clientes actuales** al portal web (`<<URL del portal>>`) con un botón "Iniciar sesión" siempre visible.

**Entregable:** el repositorio `RDL.Landing` con un sitio **estático, rápido, accesible y bien posicionado en buscadores**, en español de Costa Rica, con un formulario de contacto y demo que funciona de verdad, páginas legales, pruebas y despliegue listo.

## El producto (fuente para los textos; no agregues funciones que no estén aquí)

- **Facturación electrónica** según la versión 4.4 de los comprobantes de Hacienda: factura, notas de crédito y débito. Clientes, productos con **buscador CABYS**, borradores y emisión.
- **Estado ante Hacienda en tiempo real:** cada factura muestra si está en proceso, aceptada, rechazada o en contingencia, con una **bandeja de rechazados** para corregirlos y avisos en pantalla cuando cambia el estado.
- **Cobranza:** cuentas por cobrar que se crean solas al emitir, **pagos aplicados a una o varias facturas**, vencimientos, **antigüedad de saldos (aging)**, seguimientos de cobro y promesas de pago.
- **Una sola pantalla por factura:** total, estado de Hacienda y saldo pendiente juntos.
- **Varias empresas por usuario:** pensado para **contadores** que llevan varias PYMES y cambian de empresa sin cerrar sesión.
- **Usuarios y roles:** propietario, administrador, facturador, cobrador, contador y solo lectura; invitaciones por enlace.
- **Seguridad:** cada empresa aislada de las demás a nivel de base de datos, **bitácora de auditoría** de las operaciones sensibles y certificado de firma guardado en un gestor de secretos.

Estado real del producto: `<<en desarrollo / beta privada / disponible>>`. Si no está disponible todavía, los llamados a la acción son **lista de espera o demo**, no "empiece hoy".

## Datos del entorno

- Nombre comercial y dominio: `<<RDL / pendiente>>`, `<<https://www.dominio.cr>>`.
- URL del portal: `<<https://app.dominio.cr>>` (iniciar sesión: `<<https://app.dominio.cr/login>>`; crear cuenta: `<<ruta de registro o "no existe todavía">>`).
- Contacto: correo `<<ventas@...>>`, WhatsApp `<<+506 ...>>`, horario `<<...>>`.
- Precios: `<<planes definidos o "pendiente">>` (el planning propone límites por documentos emitidos al mes).
- Destino del formulario: `<<servicio de correo / función serverless / CRM / "pendiente">>`.
- Analítica: `<<ninguna / Plausible / Umami / GA4>>`.
- Hosting del estático: `<<pendiente de P2>>`.
- Redes sociales: `<<...>>`.

## Paso 1 · Contenido y estructura (obligatorio, antes de programar)

Entrega en `docs/0001-contenido-y-estructura.md`:
1. **Público y mensajes:** los tres públicos (dueño de PYME, persona que factura y contador que lleva varias empresas), el problema de cada uno y el mensaje principal para cada uno.
2. **Mapa del sitio** y **orden de secciones** de la página principal (propuesta abajo) con el objetivo de cada una.
3. **Todos los textos** (titulares, subtítulos, cuerpo, llamados a la acción, preguntas frecuentes, mensajes del formulario y sus errores) en una tabla, listos para revisar.
4. **Palabras clave** para buscadores en Costa Rica (por ejemplo "factura electrónica Costa Rica", "sistema de facturación electrónica para pymes", "facturación electrónica versión 4.4", "software de cobranza") y la página que apunta a cada una.
5. **Wireframe** en texto o imágenes simples, en escritorio y en móvil.
6. **Decisiones pendientes** (registro de cuentas, destino del formulario, precios, analítica, textos legales).

**Detente y espera mi aprobación antes de programar.**

**Secciones propuestas para la página principal:**
1. **Encabezado fijo:** logotipo, navegación a secciones, "Iniciar sesión" (al portal) y el llamado principal ("Crear cuenta" o "Pedir demo").
2. **Hero:** propuesta de valor en una línea, subtítulo concreto, los dos llamados a la acción y una captura o maqueta del portal (del diseño de P8a o una ilustración clara; nunca una captura con datos reales).
3. **El problema:** facturar, perseguir a Hacienda y cobrar en herramientas separadas.
4. **Funciones:** facturación 4.4, estado de Hacienda y bandeja de rechazados, cobranza y aging, vista de una factura con total, Hacienda y saldo, multiempresa para contadores, roles.
5. **Cómo funciona:** tres pasos (cree su empresa y configure Hacienda → facture → cobre y dé seguimiento).
6. **Para contadores:** varias empresas, un solo acceso.
7. **Confianza y seguridad:** aislamiento entre empresas, auditoría, certificado protegido, cumplimiento con la versión vigente de Hacienda. Solo afirmaciones verdaderas.
8. **Precios:** planes y límites, o "Precios próximamente: pida una demo" si no están definidos.
9. **Preguntas frecuentes:** qué es la versión 4.4, qué necesito para empezar (certificado y usuario de Hacienda), si puedo cambiarme desde otro sistema, cómo se cobra, qué pasa si Hacienda no responde, soporte.
10. **Llamado final** con el formulario de contacto y demo.
11. **Pie de página:** contacto, redes, enlaces legales, "Iniciar sesión" y el año.

**Otras páginas:** términos y condiciones, política de privacidad, política de cookies (si hay cookies no esenciales), página 404 y página de "gracias" tras enviar el formulario.

## Paso 2 · Reglas de contenido

- **Español de Costa Rica, trato de usted**, claro, concreto y sin exageraciones ("emita facturas aceptadas por Hacienda" en lugar de "revolucione su negocio").
- **Nada inventado:** ni testimonios, ni logotipos de clientes, ni cifras ("+1000 empresas"), ni premios, ni certificaciones. Hacienda **no certifica** proveedores: no digas "certificado por Hacienda". Si hacen falta testimonios o cifras, deja un marcador visible `<<testimonio pendiente>>` que **no se publica** (el build falla si queda alguno).
- **Nada de funciones fuera de la lista** del producto. Lo que está en planes futuros (app móvil, cobro automático) no aparece, o aparece como "próximamente" solo si yo lo apruebo.
- Los montos de ejemplo en capturas o maquetas son ficticios y con formato `₡113 000,00`.
- **Textos legales:** borradores claros basados en la Ley 8968 de Protección de la Persona frente al Tratamiento de sus Datos Personales, marcados como borrador y **con revisión legal obligatoria antes de publicar**.

## Paso 3 · Stack y diseño

| Tema | Propuesta |
|---|---|
| Generador | **Astro** (HTML estático, JavaScript solo donde hace falta), o la alternativa que justifiques en un ADR |
| Estilos | Variables CSS con los **tokens del portal** (`design/`), para que landing y portal sean el mismo producto; CSS Modules o Tailwind configurado con esos tokens |
| Tipografía | La del sistema de diseño, alojada en el propio sitio (sin depender de un CDN externo) y con `font-display: swap` |
| Imágenes | Formatos modernos (AVIF/WebP) con respaldo, tamaños responsivos y carga diferida debajo del primer pantallazo |
| Formulario | Isla interactiva mínima (sin framework pesado) |
| Contenido | Textos en archivos de contenido (Markdown o JSON), no mezclados en los componentes |

**Diseño:** sobrio y profesional, coherente con el portal. Escritorio y móvil al mismo nivel (una buena parte del tráfico llega desde el celular por WhatsApp). Tema claro; el oscuro solo si el sistema de diseño lo trae. Si no existe el paquete de P8a, propón una dirección visual breve en el Paso 1 usando los mismos principios (sobria, datos claros, color de estado siempre con texto).

## Paso 4 · Lo funcional

1. **Enlaces al portal:** "Iniciar sesión" y "Crear cuenta" apuntan a las URLs del portal configuradas por variable de entorno (sin recompilar por ambiente). **Conservan los parámetros UTM** de la visita para medir campañas. Si el registro de cuentas todavía no existe en el portal, "Crear cuenta" se reemplaza por "Pedir demo" o "Unirse a la lista de espera".
2. **Formulario de contacto y demo:** nombre, empresa, correo, teléfono (opcional), tipo (PYME o contador), número aproximado de facturas al mes y mensaje.
   - Validación accesible en el navegador y **otra vez en el servidor**.
   - **Protección contra spam:** campo trampa (honeypot), tiempo mínimo de llenado y un desafío invisible (Cloudflare Turnstile o hCaptcha), más límite de envíos por IP.
   - **Destino:** propón en un ADR (servicio de correo transaccional, función serverless o CRM) y espera la decisión. **No guardes los datos en los schemas de las APIs de dominio.** Mientras tanto, un adapter de desarrollo que escribe en consola.
   - Casilla de consentimiento para el tratamiento de datos (con enlace a la política de privacidad), sin marcar por defecto.
   - Estados de enviando, enviado (página de gracias) y error con reintento; nunca se pierde lo escrito.
3. **WhatsApp:** botón con mensaje prellenado, si hay número configurado.
4. **Analítica:** si se elige una que usa cookies no esenciales, aviso de consentimiento **antes** de cargarla; si se elige una sin cookies (Plausible, Umami), sin aviso. Eventos: clic en "Iniciar sesión", "Crear cuenta" o "Pedir demo" y envío del formulario.

## Paso 5 · SEO, rendimiento y accesibilidad

- **SEO técnico:** `lang="es-CR"`, título y descripción únicos por página, URL canónica, Open Graph y Twitter Card con imagen propia, `sitemap.xml`, `robots.txt`, datos estructurados JSON-LD (`Organization`, `SoftwareApplication` y `FAQPage` para las preguntas frecuentes), encabezados en orden y textos alternativos útiles.
- **Rendimiento:** Lighthouse **≥ 95** en rendimiento, accesibilidad, buenas prácticas y SEO en móvil. Core Web Vitals en verde (LCP < 2,5 s, CLS < 0,1, INP < 200 ms). Presupuesto: primera carga de la página principal por debajo de `<<150>>` KB de JavaScript comprimido.
- **Accesibilidad WCAG 2.2 AA:** contraste, foco visible, navegación completa con teclado, etiquetas en el formulario, errores anunciados a lectores de pantalla y respeto de "reducir movimiento".
- **Seguridad:** cabeceras CSP estricta (solo los orígenes necesarios: el del formulario, el del desafío antispam y el de la analítica), `X-Content-Type-Options`, `Referrer-Policy`, `Permissions-Policy` y `frame-ancestors 'none'`. Cero secretos en el cliente (las claves privadas del formulario viven en el servidor).

## Paso 6 · Pruebas

- **Playwright:** cada llamado a la acción lleva a la URL correcta del portal y conserva los UTM; el formulario valida, rechaza spam (campo trampa) y muestra los estados de envío, gracias y error; la navegación por secciones funciona; todo se revisa en escritorio y en móvil.
- **Accesibilidad:** `@axe-core/playwright` sin violaciones serias en cada página.
- **Lighthouse CI** con los umbrales del Paso 5; el pipeline falla si bajan.
- **Enlaces:** ningún enlace roto (interno ni externo).
- **Contenido:** el build falla si queda algún marcador `<<...>>` sin reemplazar en una página publicable, o una frase prohibida como "certificado por Hacienda".
- Con el MCP de Playwright, revisa visualmente cada sección terminada en escritorio y en móvil antes de darla por lista.

## Paso 7 · Entrega

- Build estático con configuración en tiempo de ejecución o por ambiente (URLs del portal, formulario, analítica).
- Dockerfile que sirve el estático con un servidor ligero (nginx o Caddy) y las cabeceras de seguridad, o la configuración equivalente del hosting que decida P2.
- Scripts: `dev`, `build`, `preview`, `test`, `test:e2e`, `lint`, `check:content`, `lighthouse`.
- `README.md`: cómo levantarla, cómo cambiar textos, variables de entorno, cómo agregar una sección y cómo publicar.
- `CLAUDE.md` con las reglas del repo: textos solo desde los archivos de contenido, nada inventado, tokens del sistema de diseño, presupuesto de rendimiento, marcadores prohibidos en producción y el comando de cierre `pnpm lint && pnpm check:content && pnpm test && pnpm test:e2e`.
- ADRs cortos: generador, destino del formulario, protección antispam, analítica y consentimiento.

## Forma de trabajo

1. **Plan primero:** el documento de contenido y estructura del Paso 1 y las decisiones pendientes. Espera mi aprobación.
2. Incrementos, cada uno funcionando y con sus pruebas:
   1. proyecto, tokens, tipografía, encabezado y pie, SEO base y pipeline;
   2. hero y funciones, con enlaces al portal y UTM;
   3. cómo funciona, contadores, confianza y preguntas frecuentes (con JSON-LD);
   4. precios (o su marcador aprobado) y formulario con antispam y adapter de desarrollo;
   5. páginas legales, 404 y gracias; consentimiento si la analítica lo requiere;
   6. endurecimiento: Lighthouse, accesibilidad, enlaces, contenido, cabeceras de seguridad, README.
3. Al terminar cada incremento: lint, pruebas y revisión visual en escritorio y móvil; resume lo hecho y lo pendiente, y **pausa**.
4. Antes de terminar, revisa buscando: afirmaciones no verificables, funciones que el producto no tiene, testimonios o cifras inventadas, marcadores sin reemplazar, enlaces al portal rotos o sin UTM, secretos en el cliente, cookies antes del consentimiento y páginas por debajo de los umbrales de Lighthouse.

## Prohibido

- Inventar testimonios, clientes, cifras, premios, certificaciones o funciones.
- Decir que el producto está "certificado" por Hacienda.
- Publicar textos legales sin revisión legal.
- Guardar los datos del formulario en los schemas de las APIs de dominio o exponer claves privadas en el cliente.
- Cargar analítica o cookies no esenciales antes del consentimiento.
- Usar capturas con datos reales de personas o empresas.

## Definición de terminado

- [ ] Contenido, estructura y textos aprobados.
- [ ] Página principal con todas sus secciones, páginas legales (marcadas para revisión), 404 y gracias.
- [ ] "Iniciar sesión" y "Crear cuenta" (o su alternativa) llevan al portal y conservan los UTM.
- [ ] Formulario funcionando de punta a punta con el destino decidido, antispam y consentimiento.
- [ ] Lighthouse ≥ 95 en las cuatro categorías en móvil; axe sin violaciones serias; sin enlaces rotos.
- [ ] SEO técnico completo (metadatos, Open Graph, sitemap, robots, JSON-LD).
- [ ] Cabeceras de seguridad y cero secretos en el cliente.
- [ ] Ningún marcador `<<...>>` ni afirmación prohibida en páginas publicables.
- [ ] README, CLAUDE.md, ADRs y lista de pendientes (precios, textos legales, testimonios, registro de cuentas).
