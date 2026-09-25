# Estado del proyecto

Última actualización: 2026-09-25

## Incremento 2 · Acceso y transversales — terminado (2026-09-25)
Pantallas 1–5, 33 y 35, construidas fieles al prototipo y **cableadas** (probadas en vivo contra Supabase dev +
Portal Gateway + Platform).

| # | Pantalla | Qué hace | Cableado |
|---|---|---|---|
| 1 | Iniciar sesión / recuperar | login (cableado 1) + recuperar con mensaje genérico | Supabase Auth |
| 2 | Selector de organización | cargando, vacío, error con código y reintento, búsqueda; elegir = cambio seguro | `me/memberships` + `PUT active-organization` |
| 3 | Crear organización | validación en línea, error del servidor conserva lo escrito, 409 y 422 en su campo, `Idempotency-Key` reutilizada al reintentar lo mismo; queda activa como propietario | `POST /organizations` |
| 4 | Aceptar invitación | exige sesión (vuelve tras ingresar), aceptar entra con el rol; vencida (410), ya no disponible (409), para otra cuenta (404) | `POST /invitations/{token}/accept` |
| 5 | Cambio de organización | ya estaba (cableado 1); el selector de la pantalla 2 y el perfil lo reutilizan | — |
| 33 | Mi perfil | datos, mis organizaciones (cambiar desde ahí), cerrar sesión | `me` + membresías |
| 35 | Sesión vencida | diálogo encima de la página, no se cierra con Esc; misma cuenta → sigue donde estaba y reintenta; «otra cuenta» → cierra y recuerda la ruta. Lo disparan un 401 del gateway o Supabase al perder la sesión | Supabase + 401 del gateway |

- Nuevo puerto `access` (crear organización, aceptar invitación), `ApiError.errors` (422 por campo) y
  `ApiError.is(código)`, `useIdempotencyKey`, `RequireSession` para las pantallas 2–4.
- **Arreglo:** la guardia raíz reemplazaba TODO el portal por «servicio no disponible» si fallaba la lista de
  organizaciones, así que la pantalla 2 nunca mostraba su propio error. Ahora esa guardia vive solo en el armazón.
- Barra de revisión: botón «Vencer la sesión» y enlaces a los estados de la invitación (tokens `valida`,
  `vencida`, `no-pendiente`, `ajena`).
- Verificación: `typecheck` y `lint` limpios · **127 pruebas** unitarias/componentes · **27 e2e** (axe WCAG 2.1 AA y
  sin scroll horizontal en 1440 y 390 px para cada pantalla nueva y el diálogo) · `build`. **En vivo:** perfil,
  elegir organización, crear una (token nuevo con su `org_id`), identificación repetida → error en el campo,
  sesión vencida por token inválido → diálogo → reingreso sin salir de `/perfil`, cerrar sesión, **el usuario B
  acepta una invitación real** creada por A y entra como Cobrador, y el mismo enlace otra vez → «ya no está
  disponible». Único 4xx: ese 409 esperado.

### Huecos del contrato encontrados (propuestas, no se inventaron datos)
- **Invitación por token:** el diseño muestra quién invitó, la organización, el rol y el vencimiento ANTES de
  aceptar. Platform solo tiene `POST .../accept`. Propuesta: `GET /v1/invitations/{token}` (sin revelar nada si es
  para otro correo). Hoy la pantalla 4 muestra un texto genérico y «Entró como …».
- **Revocada vs. ya usada:** Platform responde 409 `invitation-not-pending` para las dos; el diseño las distingue.
  Un solo mensaje hasta que el problem type las separe.
- **Nombre del perfil:** el diseño lo deja editar, pero `/v1/me` es solo GET. Se muestra de solo lectura con su
  explicación. Propuesta: `PATCH /v1/me` (`fullName`).
- **Tipos de identificación:** el contrato los deja `TODO(fiscal)`. El portal usa 01–04 de la Nota 4 del
  **borrador** de Hacienda (`src/shared/identification.ts`, `FUENTE: borrador`). Propuesta: catálogo en contratos.
- **Crear contraseña nueva:** el enlace de recuperación vuelve al portal, pero el diseño no tiene esa pantalla.
- La pantalla 2 no muestra la identificación de las organizaciones no activas: la lista de membresías no la trae.

## Cableado 1 · Sesión real contra el Portal Gateway — terminado
Primer tramo de «Cableado (otra etapa)»: lo que el armazón ya usaba ahora sale de las APIs reales.

- `VITE_DATA_SOURCE=gateway` (`src/app/sources.ts`): Supabase Auth (`shared/auth/`) + adaptador `gateway/` de los
  puertos. `mock` sigue siendo el valor por defecto y lo que usan todas las pruebas de pantallas.
- **Pantalla 1 · Iniciar sesión** construida, fiel al prototipo: validación, mensaje genérico ante credenciales
  malas, aviso «su sesión venció» con la ruta de vuelta (`?volver=`, solo rutas internas).
- `RequireAuth`: sin sesión → pantalla 1; sin organizaciones → pantalla 3; sin organización activa → pantalla 2.
- **Cambio de organización seguro** (P8b 4.5): `PUT /me/active-organization` → refresco del token → caché vaciada
  antes y después → lista releída. Durante la transición el contenido son esqueletos (no se ve ningún dato). El
  toast confirma cuando terminó; si falla, avisa y se queda en la anterior.
- Cerrar sesión cierra Supabase y vacía toda la caché.
- **ADR 0005** (decidido): sesión en `localStorage` + **CSP estricta** en el build, verificada por una e2e.
- **ADR 0006**: cliente HTTP a mano (desviación de `openapi-fetch`: el OpenAPI del gateway no declara los cuerpos
  del paso directo).
- `Organization` tiene opcionales lo que la lista de membresías no da (`timezone`, `defaultCurrency`,
  `identification`) y lo que da E-Invoice (`environment`): sin dato, la insignia de ambiente no se muestra.
- Verificación: `typecheck` y `lint` limpios · **106 pruebas** unitarias/componentes (30 nuevas: cliente HTTP,
  adaptador, Supabase, acceso y cambio de organización) · **15 e2e** (login con axe en 1440 y 390 px, CSP) ·
  `build` correcto. **Probado en vivo** con Chromium contra Supabase dev + gateway + Platform reales: redirección
  al login, credenciales malas, login, datos reales en el armazón, cambio de organización con token nuevo
  (`org_id` y `organizations/current` coinciden), 6 llamadas al gateway todas con token y `X-Correlation-Id`,
  ninguna con organización en la URL, logout que borra la sesión, cero errores.

### Pendiente del cableado
- `TODO(api)` notificaciones: `/portal/v1/notifications` es el incremento 6 del gateway (bloqueado por P2): la
  campana queda vacía con `gateway`.
- `TODO(api)` contador de la Bandeja: depende de E-Invoice (P5). El menú no muestra número.
- Ambiente de Hacienda (`EnvBadge`): lo da E-Invoice. El diseño lo quiere «siempre visible»; hoy no se muestra.
- Configuración en tiempo de ejecución, Dockerfile y cabeceras (`frame-ancestors`, etc.): P8b paso 7.

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
| 2 | Acceso + transversales | ✅ 1–5, 33, 35 (arriba) |
| 3 | Facturación | 7–17 (incluye borrador con totales del servidor, emitir, notas, anular) |
| 4 | Hacienda | 18–21 |
| 5 | Cobranza | 22–27 (aging con gráfico) |
| 6 | Inicio + administración | 6, 28–32 |
| 7 | Pulido | revisión visual contra el prototipo, rendimiento, `lazy` por módulo, pruebas visuales |
| — | Cableado (otra etapa) | ✅ sesión (arriba) · cada pantalla se cablea en su incremento · Realtime espera P2 |

## Decisiones abiertas
- Separador de miles U+202F (README del diseño) vs U+00A0 (prototipo). Ver ADR 0001.
- Pantalla 32 depende de una API de auditoría no definida.
