# Estado · RDL.Landing

**Última actualización:** 2026-09-27 (tarde: rediseño visual y botón de tema)

## Rediseño visual (2026-09-27, pedido de Luis: «muy apagado»)

- **Botón de tema** claro/oscuro en el encabezado, recordado en el navegador; sin elección, el del sistema.
  `public/theme.js` lo aplica antes de pintar.
- **Capturas reales del portal** en modo simulado (datos ficticios del prototipo), en AVIF/WebP: hero con el detalle
  de factura en navegador + versión móvil superpuesta + sellos flotantes (Aceptada, Pago parcial, Cuenta por cobrar
  creada); sección nueva **«Así se ve RDL»** con Documentos, Borrador y Detalle en zigzag; selector de empresas real
  en contadores. Reemplazan las maquetas HTML.
- **Hero y llamado final en bandas oscuras** con brillos del azul de marca; franja de 4 datos del producto bajo el
  hero; iconos de funciones con los colores de estado del sistema de diseño.
- Verificación: 35 e2e (axe también en tema oscuro), Lighthouse móvil sin cambios (99–100, LCP 1,7 s, CLS 0).

## Hecho (incrementos 1–6 del prompt P8c, adaptados a las decisiones del 2026-09-27)

- **Contenido aprobado** (`docs/0001-contenido-y-estructura.md`) con los cambios de Luis: «Crear cuenta» →
  «Próximamente», contacto desde `.env` con datos de prueba, **WhatsApp en lugar de formulario**, Umami, textos
  legales completos como borrador.
- **Páginas:** principal (12 secciones), `/contadores`, `/terminos`, `/privacidad` (borradores, `noindex`), 404.
- **Diseño:** tokens del portal generados desde `design/tokens.css` (tema oscuro por `prefers-color-scheme`), IBM
  Plex alojada en el sitio, icono «1a Sigla», iconos Lucide, maquetas del portal en HTML con datos ficticios.
- **Funcional:** «Iniciar sesión» al portal conservando los UTM (también al pasar de página), «Crear cuenta ·
  Próximamente» (no es enlace), WhatsApp con mensaje prellenado (botones y flotante), menú móvil con teclado.
- **SEO:** `lang="es-CR"`, título y descripción por página, canónica, Open Graph y Twitter con `og.png` propia,
  `sitemap` (sin los borradores legales), `robots.txt`, JSON-LD `Organization`, `SoftwareApplication` y `FAQPage`.
- **Seguridad:** `Caddyfile` con CSP estricta sin `unsafe-inline` (verificado: ni scripts ni estilos en línea),
  cabeceras y `frame-ancestors 'none'`. Cero secretos (todo es `PUBLIC_*`). Sin cookies.
- **Verificación:** `astro check` y Prettier limpios · 9 pruebas unitarias · **29 e2e** + 1 omitida a propósito (Playwright + axe WCAG 2.2
  AA, escritorio 1440 y móvil 390, sin scroll horizontal, enlaces internos sin romper) · `check:content` ·
  **Lighthouse móvil:** rendimiento 99–100, accesibilidad 100, buenas prácticas 100, SEO 100 (legales: `noindex`
  esperado) · LCP 1,5–1,7 s, CLS ≤ 0,006 · JavaScript < 1 KB comprimido.

## Pendiente

| #   | Qué                                                                                                                                                                                     | Quién     |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------- |
| 1   | **Datos reales** en el `.env` de cada ambiente: dominio, portal, correo, WhatsApp, horario, titular (razón social, cédula, domicilio). Hoy son de prueba (`rdl.example`, `50600000000`) | Luis      |
| 2   | **Revisión legal** de términos y privacidad; confirmar los plazos puestos como borrador (30 días para pedir copia, 15 días de aviso, tope de 12 meses) y quitar `noindex` y el aviso    | Abogado   |
| 3   | **Registro de cuentas en el portal** (`/registro`): hasta entonces «Crear cuenta» dice «Próximamente» (`PUBLIC_SIGNUP_ENABLED=false`)                                                   | Portal    |
| 4   | **Umami:** dónde se aloja; agregar su origen a la CSP (`UMAMI_ORIGIN`)                                                                                                                  | Luis      |
| 5   | **Hosting** (P2) y verificar la imagen Docker (Docker no está instalado en esta máquina)                                                                                                | P2 / Luis |
| 6   | **Nombre comercial definitivo** (hoy «RDL», provisional)                                                                                                                                | Luis      |
| 7   | **Formulario** de contacto (ADR 0002): cuando se decida su destino                                                                                                                      | Equipo    |
| 8   | Afirmaciones a confirmar con el equipo (0001 §3.10): «cambiarse desde otro sistema» (sin importación) y reenvío automático en contingencia (diseño de P5)                               | Equipo    |
| 9   | `lhci autorun` falla en Windows al cerrar Chrome; en CI (Linux) debería andar. Localmente: `npm run lighthouse:local`                                                                   | —         |
