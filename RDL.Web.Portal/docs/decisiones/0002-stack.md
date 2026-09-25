# 0002 · Stack del portal

- Estado: aceptada · 2026-09-24

## Decisión
| Pieza | Elección | Por qué |
|---|---|---|
| Build | Vite 8 | Arranque instantáneo, sin configuración; SPA detrás de login (no necesita SSR ni SEO) |
| UI | React 19 + TypeScript 6 estricto (`noUncheckedIndexedAccess`) | Ecosistema, contratación, tipos que atrapan errores de montos y estados |
| Rutas | React Router 8 (modo datos: `createBrowserRouter`) | Estándar; `lazy` por ruta para dividir el paquete por módulo |
| Datos del servidor | TanStack Query | Caché por organización, estados de carga/error, reintentos, invalidación |
| Primitivas | Radix UI | Foco, teclado y ARIA resueltos (diálogos, menús, popovers, pestañas); estilo propio |
| Estilos | CSS Modules + variables de `tokens.css` | Fidelidad exacta al prototipo, sin capa de traducción de utilidades; tema oscuro por `data-theme` |
| Iconos | lucide-react | Propuesto por el diseño; ISC; se importan uno por uno |
| Montos | big.js | Decimal exacto con redondeo half-up; nunca `number` |
| Formularios | react-hook-form + zod | Validación en línea declarativa (se usa desde las pantallas de formularios) |
| Pruebas | Vitest + Testing Library; Playwright + axe | Unidad/componentes rápidos; humo real en navegador y WCAG 2.1 AA en escritorio y 390 px |
| Calidad | oxlint + Prettier | Ver 0004 |
| Fuentes | @fontsource IBM Plex Sans/Mono | Servidas desde el sitio: sin dependencia de terceros ni fuga de IP a Google |

React Router 8 mantiene la API de la v7 que usa este código (`createBrowserRouter`, `RouterProvider`, `NavLink`,
`Outlet`, `useNavigate`, `useBlocker`), así que la guía de la v7 aplica.

## Consecuencias
- Paquete inicial ~60 kB propios + bibliotecas en paquetes separados (`react`, `ui`, `data`) que el navegador
  conserva en caché entre versiones. Cada módulo se cargará con `lazy` al construirse.
