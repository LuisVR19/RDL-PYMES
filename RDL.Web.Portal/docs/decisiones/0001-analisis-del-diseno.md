# 0001 · Análisis del paquete de diseño

- Estado: aceptada · 2026-09-24

## Contexto
El diseño llega como paquete de Claude Design en `design/`: README, `pantallas.md` (36 pantallas con rutas, roles y
estados), `componentes.md`, `decisiones.md`, `textos.csv`, `tokens.css` y un prototipo navegable
(`referencias/*.dc.html`) con estilos en línea.

## Decisión
- `design/` se copia tal cual al repositorio y es la fuente de verdad visual. `src/design-system/tokens/tokens.css`
  es su copia exacta; los valores que el prototipo usa sin nombre (hover del menú, gris de iconos, fila activa…)
  se nombran en `tokens-app.css`, sin tocar el archivo del diseño.
- Las medidas y colores se toman de los estilos en línea del prototipo, no de capturas de pantalla.
- `textos.csv` se importa con `npm run texts:import` a `messages.design.ts`; los textos de estructura que el CSV no
  trae (menú, roles, atajos) viven en `messages.app.ts`.
- `src/app/screens.ts` registra las 36 pantallas (número, ruta, módulo, capacidad, referencia). El router, las
  pantallas provisionales, el catálogo y las pruebas salen de ese registro.
- Los glifos provisionales del prototipo (○ ● ⊘ ▲ ◷ ✓ ✕) se reemplazan por Lucide (ISC), como propone el propio
  diseño; siempre icono + texto.

## Pendientes de confirmar con diseño
- Separador de miles: el README pide espacio fino (U+202F) y el prototipo usa espacio duro (U+00A0). Se implementa
  U+202F por ser la especificación escrita; es un cambio de una constante (`THIN_NBSP` en `shared/money`).
- La pantalla 32 (Exportar auditoría) depende de una API aún no definida.
