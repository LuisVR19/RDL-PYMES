# Estado del proyecto

Última actualización: 2026-09-24

## Incremento 1 · Fundaciones y armazón — terminado
Sin funcionalidad ni cableado: solo diseño y arquitectura.

- Proyecto Vite 8 + React 19 + TS 6 estricto, oxlint, Prettier, Vitest, Playwright + axe.
- Paquete de diseño en `design/`; tokens exactos + `tokens-app.css`; textos importados de `textos.csv`.
- Base compartida: montos (`big.js`, formato `₡113 000,00`), fechas de negocio e instantes, tabla de estados,
  matriz de permisos, i18n, sesión con cambio de organización y limpieza de caché, tema claro/oscuro, atajos.
- Capa de datos por puertos con adaptador simulado y escenarios (ADR 0003).
- Sistema de diseño: Button, DataTable, Dialog/ConfirmDialog/Drawer, Feedback (alertas, esqueletos, vacío, error,
  código de referencia), Field, PageHeader, StatusBadge, Surface, SystemScreen, Tabs, Toast, Toolbar.
- Armazón fiel al prototipo: menú lateral por rol (contraíble), barra superior con selector de organización,
  ambiente de Hacienda, búsqueda (`/`), notificaciones con estado sin conexión, tema, menú de usuario; móvil con
  hamburguesa y fila de organización; aviso de tiempo real; hoja de atajos (`?`); confirmación de cambios sin
  guardar al cambiar de organización.
- Las 36 pantallas con ruta, guardia de permiso (403 con el rol) y marcador provisional; 404 y error.
- Catálogo `/_catalogo` y barra de revisión (solo datos simulados).
- Verificación: `typecheck` y `lint` sin errores · 76 pruebas unitarias/componentes · 11 pruebas e2e (axe WCAG 2.1
  AA en claro, oscuro y catálogo; escritorio 1440 px y móvil 390 px) · `build` correcto.

## Siguientes incrementos (propuestos)
| # | Alcance | Pantallas |
|---|---|---|
| 2 | Acceso + transversales | 1–5, 33, 35 (sesión vencida en diálogo) |
| 3 | Facturación | 7–17 (incluye borrador con totales del servidor, emitir, notas, anular) |
| 4 | Hacienda | 18–21 |
| 5 | Cobranza | 22–27 (aging con gráfico) |
| 6 | Inicio + administración | 6, 28–32 |
| 7 | Pulido | revisión visual contra el prototipo, rendimiento, `lazy` por módulo, pruebas visuales |
| — | Cableado (otra etapa) | adaptador `gateway/` contra `/portal/v1`, Supabase Auth, Realtime |

## Decisiones abiertas
- Separador de miles U+202F (README del diseño) vs U+00A0 (prototipo). Ver ADR 0001.
- Pantalla 32 depende de una API de auditoría no definida.
