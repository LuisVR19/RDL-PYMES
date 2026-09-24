# Prompt · P8b Web Portal de la PYME · Construcción (React + TypeScript)

> **Cómo usarlo**
> 1. Antes, termina el diseño con `P8a-web-portal-design.md` en Claude Design y exporta el paquete de entrega (handoff).
> 2. Crea el repo `RDL.Web.Portal` vacío (junto a `RDL.Contracts`, `RDL.Portal.Gateway` y las APIs), guarda el paquete de diseño en `design/` y copia en `docs/contexto/` el documento de arquitectura y el planning.
> 3. Completa los valores de la sección **Datos del entorno** (lo que está entre `<< >>`).
> 4. Abre Claude Code en el repo, entra en **modo plan** (Shift+Tab) y pega todo lo que está debajo de la línea.
> 5. Revisa y corrige el plan antes de dejarlo implementar. Revisa a mano todo lo que toque la sesión, el cambio de organización, la caché de datos y el formato de montos.

---

## Rol y objetivo

Eres un ingeniero frontend senior en React y TypeScript. Vas a construir el **portal web de la PYME** de un SaaS B2B multiempresa de facturación electrónica para Costa Rica, a partir del diseño hecho en Claude Design. El portal habla **solo con el Portal Gateway** (el BFF), que a su vez habla con Platform, Billing, E-Invoice (`fiscal`) y Receivables.

El trabajo va **poco a poco**: primero el sistema de diseño, después las pantallas por fase del planning, y cada pantalla pasa por tres momentos: **maqueta fiel al diseño → cableada a la API (o a mocks del contrato si la API aún no existe) → estados y pruebas completos**.

El portal muestra dinero, estados fiscales y datos de varias empresas a la misma persona. Un monto mal formateado, un dato de la empresa anterior que queda en caché después de cambiar de organización o un botón que aparece a quien no debe dañan la confianza en el producto. Por eso la **exactitud y el aislamiento** pesan más que la velocidad.

**Entregable:** el repositorio `RDL.Web.Portal` con las pantallas de las fases F1, F3, F4 y F5 del planning P8, fiel al diseño, cableado al Portal Gateway, con pruebas unitarias, de componentes, end to end (Playwright) y de accesibilidad, Dockerfile y documentación.

## Contexto que debes leer primero

- `design/`: el paquete de Claude Design (tokens, componentes, pantallas numeradas, rutas sugeridas, textos, prototipos y decisiones abiertas). **Es la fuente de verdad visual.** Si el diseño contradice un contrato de datos, manda el contrato: anótalo y pregunta.
- `docs/contexto/planning-v1.md`: sección **P8 Web** (pantallas por fase, Playwright y revisión de estados vacío, error y carga) y "Cómo trabajamos con Claude Code". `docs/contexto/arquitectura-v1.md`: secciones **2.2** (vista transversal), **5.2** (organización activa) y **12**.
- **Repo de contratos** en `<<ruta a RDL.Contracts>>`: `docs/convenciones.md` (formatos de dinero y fechas, errores, idempotencia, correlación, paginación por cursor), `docs/glosario.md`, `state-machines/*.yaml` (estados y transiciones que la interfaz muestra y habilita), `problems/*.yaml` (tipos de error y sus mensajes), `schemas/**` y `examples/**` (para mocks realistas).
- **Portal Gateway** en `<<ruta a RDL.Portal.Gateway>>`: su `api/openapi.yaml` (rutas `/portal/v1/...`), la degradación de la vista transversal, la propagación de headers y las notificaciones en tiempo real. Si todavía no existe, trabaja contra los OpenAPI del repo de contratos con mocks.
- **Platform API** en `<<ruta a RDL.Platform.API>>`: cómo funcionan el login con Supabase, la organización activa (`PUT /v1/me/active-organization` y luego **refrescar el token** para que el hook emita el nuevo `org_id`) y las invitaciones. Lee su `README.md` y su ADR 0003.

## Datos del entorno

- URL del Portal Gateway en dev: `<<http://localhost:8090>>`.
- Supabase (dev): `SUPABASE_URL=<<https://project-ref.supabase.co>>` y la **publishable key** `<<sb_publishable_...>>` (pública). **Nunca** la `service_role` key.
- Usuarios de prueba de dev: `<<usuario1 / usuario2>>` (credenciales fuera del repo, en `.e2e.local` ignorado por git).
- Versión de Node: `<<22 LTS>>`; gestor de paquetes: `<<pnpm>>`.
- Hosting previsto para el estático: `<<pendiente de P2>>`.

## Paso 1 · Análisis del diseño y de las APIs (obligatorio)

Antes de escribir código, entrega en `docs/decisiones/0001-analisis-diseno-y-apis.md`:
1. **Inventario del sistema de diseño:** tokens (color, tipografía, espaciado, radios, sombras, movimiento, breakpoints, tema oscuro) y componentes con sus variantes y estados, tal como vienen del diseño.
2. **Mapa de pantallas:** para cada pantalla numerada del diseño, ruta, roles, datos que muestra, **endpoint del Portal Gateway** que la alimenta y acciones que dispara (con su endpoint).
3. **Huecos:** pantallas o acciones sin endpoint (por ejemplo, la exportación de auditoría), datos que el diseño muestra y ningún contrato entrega, estados que el diseño no dibujó. Para cada uno, una propuesta: pedirlo al Portal Gateway o al repo de contratos, o dejarlo con mock y un TODO.
4. **Decisiones abiertas del diseño** que afectan al código (tramos del aging, códigos fiscales ilustrativos, columnas de exportación).
5. La propuesta de stack del Paso 2 con lo que haga falta confirmar.

**Detente y espera mi aprobación antes de implementar.**

## Paso 2 · Stack

Propuesta del planning: React con TypeScript, un sistema de componentes propio y pequeño, y Playwright. Concretamente:

| Tema | Propuesta | Nota |
|---|---|---|
| Base | Vite + React + TypeScript en modo `strict` | |
| Rutas | TanStack Router o React Router (ADR) | Rutas del diseño; guardas por sesión, organización y rol |
| Datos del servidor | TanStack Query | Una caché por organización activa (Paso 4) |
| Cliente HTTP | `openapi-fetch` con tipos generados por `openapi-typescript` desde el OpenAPI del Portal Gateway | Ningún `fetch` suelto en componentes |
| Formularios | `react-hook-form` + `zod` | Los esquemas replican los formatos de los contratos (patrones de `Money`, `Quantity`...) |
| Componentes | Propios, sobre primitivas accesibles sin estilo (por ejemplo Radix UI), con un ADR | Tokens del diseño como variables CSS |
| Estilos | Variables CSS de los tokens + CSS Modules (o la alternativa que justifiques en un ADR) | Tema claro y oscuro por variables |
| Dinero | Una librería decimal (`big.js` o `decimal.js`) | **Nunca `number` para montos** |
| Fechas | `Intl.DateTimeFormat` con `timeZone` de la organización (o `@date-fns/tz`) | Fechas de negocio `YYYY-MM-DD` sin convertir a UTC |
| Textos | Archivo de mensajes `es-CR` (por ejemplo `i18next` o un módulo propio) | Los textos del diseño, sin cadenas sueltas en componentes |
| Autenticación | `@supabase/supabase-js` | Almacenamiento de la sesión en un ADR (ver Paso 4) |
| Tiempo real | `EventSource` (SSE) contra `/portal/v1/notifications`, o lo que haya definido el Portal Gateway | Reconexión con token nuevo |
| Mocks | MSW, con respuestas tomadas de `examples/` del repo de contratos | Mismo mock en tests y en desarrollo |
| Catálogo de componentes | Storybook o Ladle (ADR) | Cada estado del diseño como historia |
| Pruebas | Vitest + Testing Library, Playwright (con su MCP) y `@axe-core/playwright` | |
| Calidad | ESLint (incluido `jsx-a11y`), Prettier, `tsc --noEmit` | |

Ninguna otra dependencia sin justificarla en un ADR.

## Paso 3 · Arquitectura del frontend

```
RDL.Web.Portal/
├── design/                      # paquete de Claude Design (solo lectura)
├── src/
│   ├── app/                     # arranque, providers, router, layout, guardas
│   ├── design-system/
│   │   ├── tokens/              # variables CSS generadas desde el diseño (claro y oscuro)
│   │   └── components/          # Button, MoneyInput, DataTable, StatusBadge, OrgSwitcher... (con historias)
│   ├── features/
│   │   ├── auth/                # login, recuperar contraseña, selector y cambio de organización, invitaciones
│   │   ├── home/
│   │   ├── billing/             # clientes, productos, CABYS, documentos, crear/emitir, detalle transversal, notas
│   │   ├── fiscal/              # configuración, certificado, establecimientos, bandeja, documento electrónico
│   │   ├── receivables/         # cuentas por cobrar, aging, pagos, aplicaciones, seguimientos, promesas
│   │   └── admin/               # organización, sucursales, usuarios y roles, invitaciones, auditoría
│   │       └── (cada feature: pages/, components/, api/ con hooks de TanStack Query, schemas/)
│   ├── shared/
│   │   ├── api/                 # cliente generado, middleware de headers, mapeo de Problem Details
│   │   ├── money/               # parseo y formato de montos (string decimal ↔ pantalla), sin number
│   │   ├── dates/               # fechas en la zona de la organización
│   │   ├── session/             # sesión, organización activa, refresco del token
│   │   ├── permissions/         # matriz de permisos de UI por rol (una sola)
│   │   ├── status/              # mapeo único estado → etiqueta, tono e icono (el del diseño)
│   │   └── i18n/
│   └── mocks/                   # handlers de MSW desde los ejemplos del repo de contratos
├── e2e/                         # Playwright
├── docs/{contexto,decisiones}/  docs/ESTADO.md
├── CLAUDE.md, README.md, Dockerfile, package.json, vite.config.ts, playwright.config.ts
```

**Reglas que se tienen que notar en el código:**
- **Sin reglas de negocio:** el servidor decide totales, estados y permisos. La interfaz valida formatos para ayudar al usuario, oculta lo que el rol no puede hacer y muestra lo que devuelve la API. Un total de factura se muestra como "recalculando" hasta que llega del servidor.
- **Las transiciones se habilitan desde las máquinas de estado** del repo de contratos: "Emitir" solo en `draft`, "Anular" solo en `issued`, "Reintentar" solo en `error`. El mapeo vive en `shared/status`, no repartido en `if` por las pantallas.
- **Permisos de UI en una sola matriz** (`shared/permissions`) que refleja la de las APIs. Es solo experiencia de usuario: un 403 de la API se muestra siempre como "Sin permiso", aunque la matriz local diga otra cosa.
- **Componentes de pantalla sin `fetch`:** todo acceso a datos pasa por hooks de `features/*/api` que usan el cliente de `shared/api`.
- **Dinero como string decimal de punta a punta:** llega como string, se formatea con la librería decimal y `Intl.NumberFormat('es-CR')` (`₡113 000,00`, `US$1 250,00`) y se envía como string. Un test falla si aparece `parseFloat`, `Number(` o aritmética con `number` sobre un monto.
- **Fechas:** instantes (`...Z`) se muestran en la zona de la organización; fechas de negocio (`YYYY-MM-DD`) se muestran tal cual, sin pasar por `new Date()` (eso las corre un día).
- Componentes accesibles (roles, etiquetas, foco, teclado), textos desde el archivo de mensajes y estilos solo con tokens (sin colores ni tamaños sueltos).

## Paso 4 · Sesión, organización activa y aislamiento

1. **Login** con Supabase (correo y contraseña) y recuperación de contraseña. Mensajes genéricos. La sesión se refresca sola; al vencer, se vuelve a iniciar sin perder la ruta.
2. **Almacenamiento de la sesión:** `supabase-js` guarda la sesión en `localStorage` por defecto. Propón en un ADR si se mantiene (con una CSP estricta que reduzca el riesgo de XSS) o si se usa almacenamiento en memoria con refresh, y **espera mi decisión**.
3. **Headers en cada llamada al Portal Gateway:** `Authorization: Bearer <access token>`, `X-Correlation-Id` (un UUID por acción del usuario) y, en los comandos `POST`, `Idempotency-Key`: un UUID **por intento de envío del usuario**, que se **reutiliza** si el usuario reintenta ese mismo envío tras un error de red, y se renueva si cambia el contenido.
4. **La organización nunca viaja en la URL ni en un header**: sale del token. El portal no envía `organizationId` en ningún lado.
5. **Cambio de organización** (el punto más delicado): `PUT /v1/me/active-organization` vía Portal Gateway → **refrescar la sesión** para obtener el token con el nuevo `org_id` → **vaciar por completo la caché de TanStack Query** (y cualquier estado derivado) → reabrir la conexión de notificaciones → volver al inicio de la nueva organización. Mientras tanto, la interfaz muestra un estado de transición y no deja ver datos de la organización anterior.
6. **Cerrar sesión** vacía sesión, caché, notificaciones y cualquier dato guardado en el navegador.
7. **Errores:** los Problem Details se muestran con el mensaje de la interfaz para su `type` (tabla de textos del diseño y `problems/*.yaml`) y el `correlationId` copiable como código de referencia. 401 → iniciar sesión de nuevo; 403 → "Sin permiso"; 404 → "No encontrado" (también cubre recursos de otra organización); 409 → mensaje del caso ("La factura ya fue emitida"); 422 → errores por campo en el formulario; 5xx o sin red → reintentar.
8. **Degradación:** si el Portal Gateway devuelve una parte como no disponible (por ejemplo el estado de Hacienda), la pantalla muestra el estado "datos parciales" del diseño y el resto funciona.

## Paso 5 · Fidelidad al diseño

- **Tokens primero:** genera las variables CSS desde los tokens del paquete de diseño (claro y oscuro) y no escribas valores sueltos en los componentes.
- **Componentes antes que pantallas:** cada componente del inventario con todas sus variantes y estados como historias del catálogo, comparados con el diseño.
- **Pantallas contra el diseño:** con el MCP de Playwright, abre cada pantalla terminada en los breakpoints del diseño (escritorio, tableta y móvil cuando la pantalla lo tiene), toma capturas y compáralas con el diseño antes de dar la tarea por terminada. Anota las diferencias intencionales en `docs/ESTADO.md`.
- **Estados siempre:** cada pantalla implementa cargando (esqueleto), vacío, error con reintento, sin permiso y, donde aplique, datos parciales, y la prueba e2e los recorre (planning P8).

## Paso 6 · Pantallas por fase (incrementos)

Cada incremento sigue el mismo ciclo por pantalla: **maqueta fiel** con datos de MSW → **cableado** al Portal Gateway (o se queda en MSW con `TODO(api)` si el endpoint no existe todavía) → **estados, textos, permisos y atajos** → **pruebas** (unitarias, de componentes y e2e con Playwright, incluida la revisión visual y de accesibilidad).

1. **Fundaciones:** proyecto, calidad (lint, tipos, formato), tokens, tema claro y oscuro, componentes base del diseño con historias, `shared/money`, `shared/dates`, `shared/status`, `shared/permissions`, cliente HTTP con headers, mapeo de errores, MSW, Playwright con axe, Dockerfile y pipeline.
2. **F1 · Acceso y estructura:** login y recuperación, selector de organización, crear organización, aceptar invitación, estructura de la aplicación (barra lateral, barra superior con selector de organización, menú de usuario), **cambio de organización con vaciado de caché**, pantallas de sistema (403, 404, sesión vencida, error) y formatos de colones y fechas en la zona de la organización.
3. **F3 · Facturación:** clientes (lista, crear y editar, ficha), productos con **buscador CABYS**, lista de documentos con estado fiscal y saldo, **crear y emitir factura** (líneas, totales del servidor, confirmación, idempotencia) y **detalle transversal** con sus variantes de Hacienda.
4. **Tiempo real:** campana y panel de notificaciones, toasts y reconexión (con MSW o el adapter de desarrollo del Portal Gateway si el transporte real no existe).
5. **F4 · Hacienda:** configuración fiscal con certificado (carga de `.p12` y PIN que nunca se guarda en el navegador), establecimientos y terminales, **bandeja de rechazados y en contingencia** y detalle del documento electrónico con reintento.
6. **F5 · Cobranza:** cuentas por cobrar con días de atraso, **aging**, detalle con seguimientos y promesas, pagos, **registrar pago y repartirlo entre cuentas** (autollenado de la más antigua, contador de aplicado y sin aplicar, validación de que lo aplicado no supere el pago ni el saldo), revertir aplicación y anular pago.
7. **F5 · Documentos y administración:** notas de crédito y débito desde una factura, anulación con motivo, organización, sucursales, **usuarios y roles** (cambiar rol, suspender, la regla del último propietario), invitaciones (con el enlace que se muestra una sola vez) y **exportación de auditoría** (con mock y `TODO(api)` hasta que exista el endpoint).
8. **Inicio y endurecimiento:** pantalla de inicio con bloques que cargan y fallan por separado, versión móvil de las pantallas marcadas en el diseño, atajos de teclado, autorrevisión, README y TODOs.

## Paso 7 · Pruebas

- **Unitarias** (Vitest): `shared/money` (formato `es-CR`, parseo de lo que el usuario pega, redondeo solo de presentación, rechazo de `number`), `shared/dates` (un instante cerca de medianoche UTC cae en el día correcto de Costa Rica; una fecha de negocio no se corre), `shared/status` (cada estado de las máquinas de estado tiene etiqueta, tono e icono), `shared/permissions` e `Idempotency-Key` (se reutiliza en el reintento y cambia con otro contenido).
- **Componentes** (Testing Library): cada componente del sistema de diseño en sus estados, con teclado y lector de pantalla (roles y etiquetas).
- **Integración con MSW:** cada pantalla en sus estados (cargando, vacío, error, sin permiso, parcial) y sus acciones, con respuestas tomadas de los ejemplos del repo de contratos, incluidos los Problem Details.
- **End to end** (Playwright):
  - los flujos del diseño (iniciar sesión → organización → inicio; crear cliente, factura con CABYS, emitir y ver el detalle; documento rechazado → nota de crédito; registrar y repartir un pago; invitar a un usuario y entrar con su rol);
  - **aislamiento:** con dos organizaciones, después de cambiar de organización no queda ningún dato de la anterior en pantalla ni en la caché; un usuario de solo lectura no ve acciones; un enlace directo a un recurso de otra organización muestra "No encontrado";
  - accesibilidad con axe sin violaciones serias en cada pantalla;
  - una etiqueta (`@dev`) para correr los flujos principales contra el entorno de dev real con los usuarios de `.e2e.local`.
- **Visual:** capturas por pantalla y breakpoint comparadas con el diseño (Paso 5).

## Paso 8 · Entrega

- Build estático con configuración en tiempo de ejecución (URL del Portal Gateway y de Supabase por variables, sin recompilar por entorno).
- Dockerfile multi-stage que sirve el estático con un servidor ligero (por ejemplo nginx o Caddy), con **cabeceras de seguridad**: CSP estricta (orígenes del Portal Gateway y de Supabase), `X-Content-Type-Options`, `Referrer-Policy` y `frame-ancestors 'none'`.
- Scripts: `dev`, `dev:mock` (todo contra MSW), `build`, `test`, `test:e2e`, `test:e2e:dev`, `lint`, `typecheck`, `storybook`.
- `CLAUDE.md` con las reglas del repo: solo el Portal Gateway, sin reglas de negocio, dinero como string decimal, fechas en la zona de la organización, estilos solo con tokens, textos desde el archivo de mensajes, vaciar la caché al cambiar de organización, estados vacío, error y carga en cada pantalla, y el comando de cierre `pnpm lint && pnpm typecheck && pnpm test && pnpm test:e2e`.
- `README.md` con cómo levantarlo contra mocks y contra dev, variables, scripts, estructura y cómo agregar una pantalla.
- `docs/ESTADO.md` como handoff al cerrar cada sesión (incluidas las diferencias intencionales con el diseño).
- ADRs cortos: router, primitivas de componentes y estilos, almacenamiento de la sesión, idempotencia en la interfaz, cambio de organización y caché, catálogo de componentes, configuración en tiempo de ejecución.

## Forma de trabajo

1. **Plan primero:** análisis del diseño y de las APIs (Paso 1), stack, decisiones pendientes y lista de historias pequeñas en orden. Espera mi aprobación.
2. Implementa **por los incrementos del Paso 6**, cada uno funcionando, con sus pruebas en verde y revisado con Playwright en los estados vacío, error y carga.
3. Al terminar cada incremento: `pnpm lint && pnpm typecheck && pnpm test && pnpm test:e2e`, capturas comparadas con el diseño, actualiza `docs/ESTADO.md`, resume lo hecho y lo pendiente, y **pausa** para revisión.
4. Antes de dar el trabajo por terminado, revisa tu propio diff buscando: llamadas directas a las APIs de dominio (solo el Portal Gateway), un `organizationId` enviado desde el portal, montos como `number`, fechas de negocio pasadas por `new Date()`, reglas de negocio en la interfaz, caché que sobreviva a un cambio de organización, botones visibles para un rol que no puede usarlos, textos o colores sueltos fuera del archivo de mensajes y los tokens, pantallas sin sus estados, secretos y la `service_role` key.

## Prohibido

- Llamar directamente a Platform, Billing, E-Invoice o Receivables: todo pasa por el Portal Gateway.
- Usar la `service_role` key, datos reales o certificados reales (en pruebas, un `.p12` de muestra generado para eso).
- Enviar un `organizationId` desde el portal o conservar datos de una organización después de cambiar a otra.
- Calcular totales, impuestos o saldos en el navegador como si fueran definitivos, o usar `number` para montos.
- Guardar el PIN del certificado o cualquier secreto en el navegador.
- Inventar endpoints, códigos fiscales o reglas: usa MSW con un `TODO(api)` o un `TODO(fiscal)` y agrégalo a la lista de pendientes.

## Definición de terminado

- [ ] Análisis del diseño y de las APIs aprobado; huecos pedidos al Portal Gateway o al repo de contratos.
- [ ] Sistema de diseño en código (tokens claro y oscuro, componentes con historias) fiel al paquete de diseño.
- [ ] Pantallas de F1, F3, F4 y F5 implementadas, cableadas al Portal Gateway (o con MSW y `TODO(api)` donde falte el endpoint).
- [ ] Cambio de organización seguro: token refrescado, caché vaciada y sin datos de la organización anterior (probado en e2e).
- [ ] Montos siempre como string decimal con formato `es-CR`; fechas en la zona de la organización.
- [ ] Cada pantalla con sus estados cargando, vacío, error, sin permiso y parcial, recorridos con Playwright.
- [ ] Accesibilidad sin violaciones serias (axe) y operable con teclado.
- [ ] Pruebas unitarias, de componentes, de integración con MSW y e2e en verde.
- [ ] Imagen Docker con CSP y configuración en tiempo de ejecución.
- [ ] Lista final de TODOs (endpoints faltantes, decisiones del diseño, pendientes fiscales) para revisar en equipo.
