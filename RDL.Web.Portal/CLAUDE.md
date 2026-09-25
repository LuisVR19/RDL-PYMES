# CLAUDE.md — RDL.Web.Portal

Guía para agentes que trabajen en este repositorio. Leer también `README.md` y `docs/ESTADO.md`.

## Contexto
- Portal React de la PYME. Consume solo el Portal Gateway (`/portal/v1`). Dos modos por `VITE_DATA_SOURCE`:
  `mock` (datos simulados, lo que usan las pruebas) y `gateway` (Supabase Auth + Portal Gateway, ver README).
  El único `fetch` está en `src/shared/api/gateway/http.ts`; `src/app/sources.ts` elige los adaptadores.
- Fuente de verdad visual: `design/` (README, `pantallas.md`, `componentes.md`, `decisiones.md`, `textos.csv`,
  `tokens.css`, `referencias/*.dc.html`). Ante una duda de estilo, leer los estilos exactos del prototipo en
  `design/referencias/RDL Portal · Prototipo completo.dc.html`, no adivinar.
- Idioma de la interfaz, comentarios y documentación: español de Costa Rica, trato de «usted».

## Cómo agregar una pantalla
1. Su entrada ya existe en `src/app/screens.ts` (número, ruta, capacidad, referencia).
2. Crear `src/features/<módulo>/pages/<Nombre>Page.tsx` (+ `.module.css`).
3. Registrar el componente en `BUILT` de `src/app/router.tsx` con el `id` de la pantalla.
4. Si necesita datos: agregar el método al puerto en `src/shared/api/ports.ts`, implementarlo en `mock/` con
   `simulate()`/`simulateSecondary()` y datos de `src/mocks`, y consumirlo con TanStack Query. La primera parte de la
   `queryKey` nunca es `'session'` salvo datos de sesión (así el cambio de organización la limpia).
5. Cubrir sus estados (cargando, vacío, error con código, sin permiso, datos parciales si aplica) y revisar con la
   barra de revisión.
6. Prueba de componente para la lógica no trivial; `npm run typecheck && npm run lint && npm test && npm run test:e2e`.

## Reglas
- Montos: `string` + `formatMoney`/`parseMoneyInput`/`sumMoney`. Prohibido `parseFloat` (oxlint lo bloquea) y `Number` sobre montos.
- Fechas de negocio: `formatBusinessDate`, `daysOverdue`; nunca `new Date('YYYY-MM-DD')`.
- Estados: `StatusBadge` + `shared/status`. Permisos: `can()` + `shared/permissions`. Nada de `role === '...'` en pantallas.
- Colores y medidas solo con variables de `tokens.css`/`tokens-app.css`; si falta una, se agrega a `tokens-app.css`.
- Componentes interactivos sobre primitivas Radix (foco, teclado, ARIA). Objetivo WCAG 2.1 AA; axe corre en e2e.
- Móvil (📱 en `pantallas.md`) a 390 px sin scroll horizontal; objetivos táctiles de 44 px.
- La organización nunca viaja desde el portal (ni URL, ni header, ni query). La única excepción es el cuerpo de
  `PUT /portal/v1/me/active-organization`, que es lo que ese contrato define; después se refresca el token.
- Sesión en `localStorage` con CSP estricta (ADR 0005): nada de scripts ni estilos en línea, ni orígenes nuevos sin
  sumarlos a `contentSecurityPolicy` de `vite.config.ts`.
- No ejecutar comandos de git (el usuario publica los repositorios). No usar datos reales ni secretos.
