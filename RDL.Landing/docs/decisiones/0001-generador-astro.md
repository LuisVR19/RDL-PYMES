# ADR 0001 · Generador: Astro

**Estado:** aceptada (2026-09-27, aprobada por Luis junto con el documento de contenido).

## Contexto

La landing es contenido casi estático que tiene que cargar rápido en celular (buena parte del tráfico llega por
WhatsApp), posicionar en buscadores y verse como el portal. El prompt P8c pide Lighthouse ≥ 95, menos de 150 KB de
JavaScript y textos separados de los componentes.

## Decisión

Astro 7, salida estática. Componentes `.astro` sin framework de cliente; JavaScript solo en dos módulos chicos
(menú móvil y UTM, < 1 KB comprimido). Textos en `src/content/`, tokens generados desde `design/tokens.css`.

## Alternativas

- **Vite + React** (como el portal): reutiliza componentes, pero manda un runtime de React innecesario para una página
  de lectura y complica el SEO.
- **HTML a mano:** sin build, pero repite encabezado y pie en cada página y no separa textos de estructura.

## Consecuencias

- Rendimiento 99–100 y LCP de 1,7 s en móvil (Lighthouse local, 2026-09-27).
- Los componentes del portal (React) no se reutilizan: las maquetas se rehacen en HTML con los mismos tokens.
