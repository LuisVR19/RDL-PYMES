# CLAUDE.md — RDL.Landing

Guía para agentes. Leer también `README.md`, `docs/0001-contenido-y-estructura.md` y `docs/ESTADO.md`.

## Qué es

Landing estática (Astro) del producto RDL. No tiene backend ni base de datos: explica, lleva al portal y abre
WhatsApp. Español de Costa Rica, trato de usted.

## Reglas

- **Textos solo en `src/content/`** (`sitio.json` y `legal/*.md`). Ningún texto visible escrito en un componente.
- **Nada inventado:** ni testimonios, ni clientes, ni cifras, ni premios, ni certificaciones, ni funciones que el
  producto no tenga (la lista está en el prompt P8c). Hacienda **no certifica** proveedores. Un dato que falta va
  como marcador `<<…>>`; `check:content` hace fallar el build si llega a una página.
- **Tokens del sistema de diseño:** `design/tokens.css` es la copia exacta del portal; `npm run tokens` genera
  `src/styles/tokens.generated.css` (no editarlo a mano). Colores y medidas solo con variables.
- **CSP estricta:** nada de scripts ni estilos en línea (`assetsInlineLimit: 0`, `inlineStylesheets: 'never'`, sin
  atributos `style`). Un origen externo nuevo (analítica, formulario) se agrega a la CSP del `Caddyfile`.
- **Presupuesto:** < 150 KB de JavaScript comprimido en la principal (hoy < 1 KB). JavaScript solo si hace falta
  (menú móvil, UTM). Lighthouse ≥ 95 en móvil; WCAG 2.2 AA; sin scroll horizontal a 390 px.
- **Enlaces al portal** con `data-portal` (conservan los UTM); URLs y contacto por variables `PUBLIC_*`.
- **Imágenes del producto:** capturas del portal en **modo simulado** (`npm run capturas`), con los datos ficticios del
  prototipo; nunca de un ambiente con datos reales. Siempre con `alt` que diga «Datos de ejemplo» y por `Shot.astro`
  (AVIF/WebP, carga diferida salvo la del hero).
- **Tema:** claro y oscuro (botón + sistema); todo color nuevo con tokens o válido en las bandas oscuras
  (`band-dark`, iguales en los dos temas). axe se corre en los dos.
- Textos legales: borradores, `noindex`, revisión legal obligatoria antes de publicar.
- Sin cookies. La analítica es Umami (sin cookies); si se cambia por una con cookies, primero el consentimiento.
- No ejecutar comandos de git.

## Cierre

`npm run lint && npm run check:content && npm test && npm run test:e2e` (y `npm run lighthouse:local` si se tocó el
diseño o se agregó contenido pesado). Revisión visual en escritorio y móvil antes de dar una sección por lista.
