# Handoff · RDL · Portal web de la PYME (P8a → P8b)

Guardar esta carpeta como `RDL.Web.Portal/design/`. La usa el prompt `P8b-web-portal-build.md`.

## Qué es
Diseño completo del portal web de facturación electrónica para PYMEs de Costa Rica: 36 pantallas (módulos A–G), 7 flujos navegables, sistema de diseño con tema claro y oscuro. Español de Costa Rica, trato de usted.

## Sobre los archivos de diseño
Los archivos `.dc.html` de `referencias/` son **referencias de diseño hechas en HTML**: prototipos que muestran el aspecto y el comportamiento esperados, **no código para copiar**. La tarea es **recrearlos en React con TypeScript** con un sistema de componentes propio y pequeño, usando los tokens de `tokens.css`. Los datos del prototipo son ficticios y viven en memoria.

## Fidelidad
**Alta fidelidad.** Colores, tipografía, espaciados, estados y textos son finales (salvo lo marcado en `decisiones.md`). Recrear con precisión.

## Contenido
| Archivo | Para qué |
|---|---|
| `tokens.css` | Variables CSS con nombres estables, valores claro (`:root`) y oscuro (`[data-theme="dark"]`), escala tipográfica, espaciado base 4, radios, sombras, capas, movimiento, puntos de corte. |
| `componentes.md` | Inventario de componentes (`StatusBadge`, `MoneyInput`, `DataTable`, `OrgSwitcher`…) con props, variantes y estados. |
| `pantallas.md` | Las 36 pantallas numeradas con ruta sugerida, roles, módulos de datos, responsive y estados; mapeo único de estados. |
| `textos.csv` | Textos de la interfaz listos para un archivo de traducciones (`clave,texto_es_CR`, con marcadores `{variable}`). |
| `decisiones.md` | Decisiones abiertas y supuestos. |
| `referencias/RDL Portal · Prototipo completo.dc.html` | Prototipo navegable A–G. Barra oscura superior = herramienta del prototipo (no es parte del producto): elige módulo, pantalla, estado, escritorio/móvil, simula notificaciones. |
| `referencias/RDL Portal · Paso 3 Sistema de diseño.dc.html` | Tokens y componentes con todos sus estados. |
| `referencias/RDL Portal · Tema oscuro.dc.html` | Pantallas 12 y 15 en oscuro. |
| `referencias/RDL Icono.dc.html` | Icono de la app (opción elegida: **1a Sigla**). |
| `referencias/RDL Portal · Paso 1 Direcciones.dc.html` | Direcciones visuales (elegida: **1a Institucional**). |

Para abrir los `.dc.html` en un navegador hace falta `support.js` en la misma carpeta (incluido).

## Estructura de la aplicación
- `AppShell`: barra lateral fija 232 px (fondo `--color-bg-nav`), contraíble a 72 px con solo iconos; barra superior 56 px con `OrgSwitcher`, `EnvBadge`, buscador (`/`), `NotificationBell`, `ThemeToggle`, menú de usuario. Contenido con padding 24/28 px; móvil 16 px.
- Móvil (<768): barra superior con menú hamburguesa (drawer a la izquierda), fila de organización debajo (abre hoja inferior), paneles a pantalla completa, áreas táctiles ≥ 44 px, controles de 44 px.
- El menú y los botones se filtran por rol: lo que un rol no puede hacer no se muestra; por enlace directo, pantalla 403.

## Comportamiento clave
- **Números del servidor.** Los totales de factura y nota se recalculan en el servidor (debounce ~650 ms en el prototipo): mostrar «Recalculando…» con valores atenuados; si falla, aviso de «no definitivos» y bloquear «Emitir».
- **Confirmación proporcional.** Guardar borrador: sin confirmación. Emitir: diálogo con resumen. Anular, revertir aplicación, anular pago: motivo ≥ 10 caracteres. Producción: escribir «PRODUCCIÓN».
- **Nada se pierde.** Salir o cambiar de organización con cambios sin guardar pide confirmación. Envíos fallidos conservan lo escrito y ofrecen reintentar con código de referencia copiable.
- **Emisión.** Borrador sin número → emitida con número → Hacienda: Firmando → Enviado → Aceptada/Rechazada/Contingencia/Error. Estados de factura y Hacienda son independientes; cuenta por cobrar se crea al emitir.
- **Registrar pago.** Autollenado de la cuenta más antigua a la más nueva; contador fijo Pago / Aplicado / Sin aplicar; excedente y sobre-saldo bloquean; puede quedar sin aplicar.
- **Formato.** `₡113 000,00`, `US$1 250,00` (espacio fino no separable de miles, coma decimal, negativo con «−»); fechas `dd/mm/aaaa` y `dd/mm/aaaa hh:mm` en la zona de la organización; cifras tabulares, montos a la derecha. `MoneyInput` acepta «113 000,00».
- **Accesibilidad.** WCAG 2.2 AA; foco visible; el color nunca es la única señal; toasts con `aria-live`; diálogos con `aria-modal`; respeta `prefers-reduced-motion`.
- **Movimiento.** 120 ms hover/foco, 180 ms menús y cambio de organización (atenuado + esqueletos), 260 ms drawer/diálogos, curva `cubic-bezier(.2,0,0,1)`.

## Estado y datos
- Estado global: usuario, organización activa (+ rol), ambiente fiscal, tema, notificaciones (tiempo real + estado de conexión).
- Cada bloque del Inicio y cada lista consulta por separado y maneja cargando / vacío / error / parcial.
- Paginación por cursor (sin total de páginas).

## Activos
- Fuentes: IBM Plex Sans y IBM Plex Mono (SIL OFL, Google Fonts).
- Icono de la app: SVG en `referencias/RDL Icono.dc.html` (convertir texto a trazos al exportar).
- Iconos del menú: trazos estilo Lucide (ISC) en el prototipo; se recomienda adoptar el paquete Lucide.
- Sin imágenes ni ilustraciones.
