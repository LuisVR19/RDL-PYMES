# RDL.Web.Portal

Portal web de la PYME: facturación electrónica, Hacienda, cobranza y administración. Es la interfaz que usan los
clientes del negocio; consume **solo** el Portal Gateway (`/portal/v1`).

> **Etapa actual: diseño y arquitectura, sin funcionalidad.** Todas las pantallas del diseño tienen ruta, permiso y
> lugar en el armazón; los datos son simulados (`src/mocks`). No hay llamadas a ninguna API todavía. Ver
> [`docs/ESTADO.md`](docs/ESTADO.md).

## Requisitos

- Node.js 22 o superior (probado con 22.22) y npm 10.
- Para las pruebas de extremo a extremo: `npx playwright install chromium` (una vez).

## Cómo levantarlo

```bash
npm install
cp .env.example .env.local   # opcional: VITE_DATA_SOURCE=mock es el valor por defecto
npm run dev                  # http://localhost:5173
```

Rutas útiles en desarrollo:

| Ruta | Qué es |
|---|---|
| `/` | Inicio dentro del armazón (organización simulada «Comercial Los Almendros», rol Administrador) |
| `/_catalogo` | Catálogo del sistema de diseño: componentes, variantes y estados. Solo con datos simulados |
| `/ingresar`, `/organizaciones`… | Pantallas de acceso (fuera del armazón) |

### Contra el Portal Gateway real (`VITE_DATA_SOURCE=gateway`)

Necesita Platform (`:8080`) y el Portal Gateway (`:8090`) corriendo, y un usuario de prueba de Supabase dev (los
del E2E de Platform). En `.env.local`:

```bash
VITE_DATA_SOURCE=gateway
VITE_GATEWAY_URL=http://localhost:8090
VITE_SUPABASE_URL=https://dzlsnsstuqpxvwegeqcy.supabase.co
VITE_SUPABASE_PUBLISHABLE_KEY=sb_publishable_...   # pública; NUNCA la service_role key
```

`npm run dev` y abra `http://localhost:5173` (con `localhost`, no `127.0.0.1`: es el origen que admite el CORS del
gateway). Sin sesión lo manda a la pantalla 1. Hoy se ven datos reales en el armazón (usuario, organizaciones, rol,
cambio de organización); las pantallas siguen siendo marcadores provisionales hasta sus incrementos.

Con datos simulados aparece un botón morado punteado abajo al centro: la **barra de revisión**. Permite cambiar de
organización, ver como otro rol, forzar estados de los datos (cargando, vacío, error, datos parciales), la latencia y
la conexión en tiempo real. **No es parte del producto** y no existe con `VITE_DATA_SOURCE` distinto de `mock`.

## Comandos

| Comando | Qué hace |
|---|---|
| `npm run dev` | Servidor de desarrollo con recarga en caliente |
| `npm run build` | Verifica tipos y genera `dist/` |
| `npm run preview` | Sirve `dist/` localmente |
| `npm run typecheck` | TypeScript estricto sin emitir |
| `npm run lint` | oxlint (React, a11y, import, unicorn) |
| `npm run format` | Prettier |
| `npm test` | Pruebas unitarias y de componentes (Vitest + Testing Library) |
| `npm run test:e2e` | Playwright: humo del armazón y accesibilidad (axe, WCAG 2.1 AA) en escritorio y 390 px |
| `npm run texts:import` | Regenera `src/shared/i18n/messages.design.ts` desde `design/textos.csv` |

## Stack

Vite 8 · React 19 · TypeScript 6 estricto · React Router 8 · TanStack Query · Radix UI (primitivas accesibles) ·
CSS Modules sobre los tokens del diseño · lucide-react · big.js (montos) · react-hook-form + zod (formularios, en
las pantallas) · Vitest · Playwright + axe · oxlint · Prettier. Fuentes IBM Plex servidas desde el propio sitio.
Razones en [`docs/decisiones`](docs/decisiones).

## Estructura

```
design/                  Paquete de diseño (fuente de verdad visual): README, pantallas, componentes, textos, tokens, prototipo
src/
  app/                   Armazón y composición: router, registro de pantallas, menú, guardias, proveedores, barra de revisión
    layout/              AppShell, Sidebar, OrgSwitcher, NotificationBell, UserMenu, MobileNav, ShortcutSheet, AuthLayout
  design-system/
    tokens/              tokens.css (copia exacta del diseño) + tokens-app.css (valores del prototipo sin nombre)
    components/          Button, DataTable, Dialog/ConfirmDialog/Drawer, Feedback, Field, PageHeader, StatusBadge,
                         Surface, SystemScreen, Tabs, Toast, Toolbar
  features/<módulo>/     auth, home, billing, fiscal, receivables, admin, system — pantallas de cada módulo
  shared/
    api/                 Puertos de datos (ports.ts), tipos, escenario de revisión y adaptador simulado (mock/)
    money/ dates/        Montos como string decimal (nunca number) y fechas de negocio sin new Date()
    status/ permissions/ Tabla única de estados y matriz única de permisos
    i18n/ session/ theme/ hotkeys/ realtime/
  mocks/                 Datos ficticios del prototipo (ninguna empresa ni persona real)
e2e/                     Pruebas Playwright
docs/                    ESTADO.md y decisiones (ADR)
```

## Reglas que no se rompen

- **Montos**: siempre `string` decimal de la API; se formatean con `formatMoney` (`₡113 000,00`, separador de miles
  espacio fino, signo menos `−`, 2 decimales con redondeo half-up). Nunca `Number`/`parseFloat`.
- **Fechas**: un instante se muestra en la zona de la organización; una fecha de negocio (`2026-10-24`) no pasa por
  `new Date()`.
- **Estados**: solo con `StatusBadge` y la tabla de `shared/status` (icono + texto, nunca solo color).
- **Permisos**: solo con `can(role, capacidad)` de `shared/permissions`; es experiencia de usuario, quien decide es
  la API.
- **Textos**: desde `t()`; los de `design/textos.csv` no se editan a mano.
- **Datos**: las pantallas usan puertos (`useDataSource()`), nunca `fetch` directo. Al cambiar de organización se
  vacía toda la caché que no sea de sesión.
- Sin secretos en el repositorio, sin datos reales, sin llamadas a producción.
