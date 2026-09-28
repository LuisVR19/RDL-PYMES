# RDL.Landing

Landing pública de **RDL**, facturación electrónica y cobranza para PYMES de Costa Rica. Sitio **estático** hecho con
Astro: explica el producto, lleva al portal (`Iniciar sesión`) y abre el contacto por WhatsApp. Español de Costa
Rica, trato de usted. Estado y pendientes: [`docs/ESTADO.md`](docs/ESTADO.md).

## Levantarla

Requisitos: Node ≥ 22.

```sh
npm install
cp .env.example .env      # valores TEMPORALES de prueba; ver «Variables»
npm run dev               # http://localhost:4321
```

## Scripts

| Script             | Qué hace                                                                                                        |
| ------------------ | --------------------------------------------------------------------------------------------------------------- |
| `dev`              | Servidor de desarrollo (regenera los tokens antes)                                                              |
| `build`            | Tokens → `check:content` → sitio estático en `dist/`                                                            |
| `preview`          | Sirve `dist/` como en producción                                                                                |
| `lint`             | `astro check` (tipos) + Prettier                                                                                |
| `check:content`    | Falla si queda un marcador `<<…>>` o una afirmación prohibida («certificado por Hacienda», cifras de clientes…) |
| `test`             | Pruebas unitarias (`node --test`): enlaces, UTM, WhatsApp, verificador de contenido                             |
| `test:e2e`         | Playwright + axe en escritorio (1440) y móvil (390), contra el build                                            |
| `lighthouse`       | Lighthouse CI (≥ 95 en las cuatro categorías, móvil)                                                            |
| `lighthouse:local` | Lo mismo en Windows, donde `lhci` falla al cerrar Chrome. Antes: `npx astro preview --port 4323`                |
| `tokens`           | Regenera `src/styles/tokens.generated.css` desde `design/tokens.css`                                            |
| `og`               | Regenera `public/og.png` (imagen para compartir)                                                                |
| `capturas`         | Regenera las capturas del portal en `src/assets/capturas/` (ver abajo)                                          |

Cierre de cada cambio: `npm run lint && npm run check:content && npm test && npm run test:e2e`.

## Variables

Todas son `PUBLIC_*`: se leen **al construir** y terminan en el HTML público (ninguna es secreta). Cada ambiente
construye con las suyas. Ver [`.env.example`](.env.example).

| Variable                                             | Para qué                                                                                |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------- |
| `PUBLIC_SITE_URL`                                    | Dominio del sitio (canónicas, sitemap, Open Graph)                                      |
| `PUBLIC_PORTAL_URL`, `PUBLIC_PORTAL_LOGIN_PATH`      | Destino de «Iniciar sesión»                                                             |
| `PUBLIC_SIGNUP_ENABLED`, `PUBLIC_PORTAL_SIGNUP_PATH` | `false` → «Crear cuenta · Próximamente». `true` cuando el portal tenga registro         |
| `PUBLIC_CONTACT_*`                                   | Correo, WhatsApp (solo dígitos, con 506), teléfono visible, horario, plazo de respuesta |
| `PUBLIC_LEGAL_*`, `PUBLIC_PRIVACY_EMAIL`             | Titular del servicio en los textos legales                                              |
| `PUBLIC_UMAMI_SRC`, `PUBLIC_UMAMI_WEBSITE_ID`        | Analítica Umami (sin cookies). Vacías = no se carga nada                                |

Sin número de WhatsApp, los botones de WhatsApp no se muestran; sin correo, tampoco su enlace.

## Cambiar textos

- Todo el texto está en [`src/content/sitio.json`](src/content/sitio.json): los componentes no tienen texto propio.
  Los `{marcadores}` (`{email}`, `{hours}`…) se llenan desde las variables.
- Los textos legales están en [`src/content/legal/`](src/content/legal) (Markdown). Son **borradores**: se publican
  con `noindex` y un aviso hasta que los revise un abogado.
- Nada inventado: ni testimonios, ni cifras, ni clientes, ni «certificado por Hacienda». `check:content` lo verifica.

## Agregar una sección

1. Textos en `src/content/sitio.json`.
2. Componente en `src/components/sections/` con `<section id="…" aria-labelledby="…">` y su `<h2>`.
3. Agregarlo en `src/pages/index.astro` y, si va en el menú, en `nav.links`.
4. Prueba en `e2e/landing.spec.ts` y revisión visual en escritorio y móvil.

## Capturas del portal

Las imágenes del hero, de «Así se ve RDL» y de contadores son capturas **reales del portal en modo simulado**: sus
datos son los ficticios del prototipo de diseño, nunca de un ambiente con datos reales. Astro las sirve en AVIF y
WebP en varios anchos. Para regenerarlas cuando cambie el portal:

```sh
cd ../RDL.Web.Portal && npm run build && npx vite preview --port 4174   # modo simulado
cd ../RDL.Landing && npm run capturas
```

## Tema

Claro u oscuro con el botón del encabezado, como el portal. Sin elección guardada se sigue el del sistema.
`public/theme.js` lo aplica antes de pintar (sin parpadeo) y es un archivo, no código en línea (CSP).

## Publicar

```sh
docker build --build-arg PUBLIC_SITE_URL=https://www.dominio.cr --build-arg … -t rdl-landing .
docker run -p 8080:8080 -e UMAMI_ORIGIN=https://analytics.dominio.cr rdl-landing
```

`Caddyfile` sirve el sitio con CSP estricta (sin `unsafe-inline`), `X-Content-Type-Options`, `Referrer-Policy`,
`Permissions-Policy`, HSTS y `frame-ancestors 'none'`. El hosting definitivo depende de P2. Antes de publicar:
reemplazar los datos de prueba del `.env`, pasar la revisión legal y quitar el aviso de borrador.

## Estructura

```
design/                 tokens.css e icono, copiados del paquete de diseño del portal (fuente única)
docs/                   contenido y estructura (0001), ADRs, ESTADO
src/content/            textos (sitio.json) y legales (Markdown)
src/components/         encabezado, pie, maquetas del portal, secciones
src/lib/                configuración, enlaces y UTM, preguntas frecuentes
src/pages/              /, /contadores, /terminos, /privacidad, 404, robots.txt
scripts/                tokens, verificador de contenido, imagen OG, Lighthouse local
e2e/                    Playwright + axe
```
