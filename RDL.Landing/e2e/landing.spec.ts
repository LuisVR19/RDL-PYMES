import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Valores de .env.example (los temporales de prueba).
const PORTAL = 'http://localhost:5173'
const WHATSAPP = 'https://wa.me/50600000000'

async function expectNoAxeViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze()
  expect(
    results.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([])
}

/** Contra el ancho configurado: en móvil el viewport de diseño crece con el desborde (lección del portal). */
async function noHorizontalScroll(page: Page) {
  const width = page.viewportSize()?.width ?? 0
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
}

test('principal: secciones, accesibilidad y ancho', async ({ page }) => {
  await page.goto('/')
  await expect(
    page.getByRole('heading', { level: 1, name: 'Facture, siga a Hacienda y cobre, en un solo lugar' }),
  ).toBeVisible()
  for (const name of [
    'Tres tareas que no deberían vivir en tres lugares',
    'Todo lo que necesita para facturar y cobrar',
    'Hecho para el día a día de una PYME',
    'Empiece en tres pasos',
    'Para contadores: todas sus empresas en un solo acceso',
    'Sus datos, protegidos',
    'Precios',
    'Preguntas frecuentes',
    'Hablemos de su empresa',
  ]) {
    await expect(page.getByRole('heading', { level: 2, name })).toBeAttached()
  }
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('«Iniciar sesión» va al portal y conserva los UTM de la visita', async ({ page }) => {
  await page.goto('/?utm_source=facebook&utm_medium=social&utm_campaign=lanzamiento&otro=x')
  const login = page.getByRole('link', { name: 'Iniciar sesión' }).first()
  await expect(login).toHaveAttribute(
    'href',
    `${PORTAL}/ingresar?utm_source=facebook&utm_medium=social&utm_campaign=lanzamiento`,
  )
  // Los UTM siguen al pasar a otra página del sitio.
  await page.goto('/contadores')
  await expect(page.getByRole('link', { name: 'Iniciar sesión' }).first()).toHaveAttribute(
    'href',
    /utm_campaign=lanzamiento/,
  )
})

test('«Crear cuenta» dice «Próximamente» y no es un enlace', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('link', { name: /Crear cuenta/ })).toHaveCount(0)
  await expect(page.getByRole('note', { name: 'Crear cuenta: próximamente' }).first()).toBeVisible()
})

test('WhatsApp con el mensaje prellenado, en pestaña nueva', async ({ page }) => {
  await page.goto('/')
  const wa = page.getByRole('link', { name: /Contactarme por WhatsApp/ }).first()
  await expect(wa).toHaveAttribute('href', new RegExp(`^${WHATSAPP}\\?text=Hola%2C%20me%20interesa%20RDL`))
  await expect(wa).toHaveAttribute('target', '_blank')
  await expect(wa).toHaveAttribute('rel', /noopener/)
  await expect(page.getByRole('link', { name: /Escribirnos por WhatsApp/ })).toBeVisible()
})

test('la navegación lleva a cada sección', async ({ page }, info) => {
  await page.goto('/')
  if (info.project.name === 'movil') {
    const button = page.getByRole('button', { name: 'Abrir menú' })
    await button.click()
    await expect(page.getByRole('button', { name: 'Cerrar menú' })).toHaveAttribute('aria-expanded', 'true')
  }
  await page.getByRole('navigation', { name: 'Principal' }).getByRole('link', { name: 'Seguridad' }).click()
  await expect(page).toHaveURL(/#seguridad$/)
  await expect(page.getByRole('heading', { level: 2, name: 'Sus datos, protegidos' })).toBeInViewport()
  if (info.project.name === 'movil') {
    // Elegir un enlace cierra el menú.
    await expect(page.getByRole('button', { name: 'Abrir menú' })).toHaveAttribute('aria-expanded', 'false')
  }
})

test('menú móvil: Esc lo cierra y devuelve el foco', async ({ page }, info) => {
  test.skip(info.project.name !== 'movil', 'solo en móvil')
  await page.goto('/')
  await page.getByRole('button', { name: 'Abrir menú' }).click()
  await expect(page.getByRole('navigation', { name: 'Principal' })).toBeVisible()
  await expectNoAxeViolations(page)
  await page.keyboard.press('Escape')
  await expect(page.getByRole('navigation', { name: 'Principal' })).toBeHidden()
  await expect(page.getByRole('button', { name: 'Abrir menú' })).toBeFocused()
})

test('preguntas frecuentes: se abren con teclado y hay JSON-LD FAQPage', async ({ page }) => {
  await page.goto('/')
  const q = page.getByText('¿Qué pasa si Hacienda no responde?')
  await q.focus()
  await page.keyboard.press('Enter')
  await expect(page.getByText(/queda emitido y en contingencia/)).toBeVisible()
  const types = await page
    .locator('script[type="application/ld+json"]')
    .evaluateAll((els) => els.map((e) => JSON.parse(e.textContent ?? '{}')['@type']))
  expect(types).toEqual(['Organization', 'SoftwareApplication', 'FAQPage'])
})

test('SEO base: idioma, título, descripción, canónica y Open Graph', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('html')).toHaveAttribute('lang', 'es-CR')
  await expect(page).toHaveTitle('Factura electrónica y cobranza para PYMES en Costa Rica · RDL')
  await expect(page.locator('meta[name="description"]')).toHaveAttribute('content', /versión 4\.4/)
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute('href', 'https://www.rdl.example/')
  await expect(page.locator('meta[property="og:image"]')).toHaveAttribute('content', /og\.png$/)
  await expect(page.locator('meta[name="robots"]')).toHaveCount(0)
})

for (const [path, title] of [
  ['/terminos', 'Términos y condiciones'],
  ['/privacidad', 'Política de privacidad'],
] as const) {
  test(`${title}: borrador visible, noindex y datos del titular`, async ({ page }) => {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible()
    await expect(page.getByText(/Borrador sin revisión legal/)).toBeVisible()
    await expect(page.locator('meta[name="robots"]')).toHaveAttribute('content', /noindex/)
    await expect(page.getByText('RDL Soluciones S.A. (dato de prueba)').first()).toBeVisible()
    await expect(page.getByText(/\{legalName\}|\{email\}/)).toHaveCount(0)
    await expectNoAxeViolations(page)
    await noHorizontalScroll(page)
  })
}

test('para contadores: página propia con su mensaje de WhatsApp', async ({ page }) => {
  await page.goto('/contadores')
  await expect(
    page.getByRole('heading', {
      level: 1,
      name: 'Facturación electrónica para contadores con varias empresas',
    }),
  ).toBeVisible()
  await expect(page.getByRole('link', { name: /Contactarme por WhatsApp/ }).first()).toHaveAttribute(
    'href',
    /soy%20contador/,
  )
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('404 con enlace al inicio', async ({ page }) => {
  const res = await page.goto('/no-existe')
  expect(res?.status()).toBe(404)
  await expect(page.getByRole('heading', { level: 1, name: 'No encontramos esta página' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Ir al inicio' })).toHaveAttribute('href', '/')
  await expectNoAxeViolations(page)
})

test('sitemap y robots: solo las páginas indexables', async ({ request }) => {
  const sitemap = await (await request.get('/sitemap-0.xml')).text()
  expect(sitemap).toContain('https://www.rdl.example/contadores/')
  expect(sitemap).not.toContain('terminos')
  expect(sitemap).not.toContain('privacidad')
  const robots = await (await request.get('/robots.txt')).text()
  expect(robots).toContain('Sitemap: https://www.rdl.example/sitemap-index.xml')
})

test('sin enlaces internos rotos', async ({ page, request }) => {
  const seen = new Set<string>()
  for (const start of ['/', '/contadores', '/terminos', '/privacidad']) {
    await page.goto(start)
    const hrefs = await page
      .locator('a[href^="/"]')
      .evaluateAll((as) => as.map((a) => (a as HTMLAnchorElement).getAttribute('href') ?? ''))
    for (const h of hrefs) seen.add(h.split('#')[0] || '/')
  }
  for (const path of seen) {
    expect((await request.get(path)).status(), path).toBe(200)
  }
})

test('compatible con la CSP estricta: ni scripts ni estilos en línea', async ({ page }) => {
  for (const path of ['/', '/contadores', '/terminos', '/privacidad']) {
    await page.goto(path)
    const inline = await page.evaluate(() => ({
      scripts: [...document.querySelectorAll('script:not([src])')].filter(
        (s) => s.getAttribute('type') !== 'application/ld+json',
      ).length,
      styles: document.querySelectorAll('style').length,
      styleAttrs: document.querySelectorAll('[style]').length,
    }))
    expect(inline, path).toEqual({ scripts: 0, styles: 0, styleAttrs: 0 })
  }
})

test('botón de tema: cambia, se recuerda al recargar y no deja contraste roto', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' })
  await page.goto('/')
  const html = page.locator('html')
  await expect(html).toHaveAttribute('data-theme', 'light')
  await page.getByRole('button', { name: 'Cambiar a tema oscuro' }).click()
  await expect(html).toHaveAttribute('data-theme', 'dark')
  await expect(page.getByRole('button', { name: 'Cambiar a tema claro' })).toBeVisible()
  await expectNoAxeViolations(page)
  // Se recuerda: al recargar ya viene oscuro (theme.js lo pone antes de pintar).
  await page.reload()
  await expect(html).toHaveAttribute('data-theme', 'dark')
})

test('sin elección guardada, sigue el tema del sistema', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.goto('/contadores')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expectNoAxeViolations(page)
})

test('capturas del portal: AVIF y WebP, con texto alternativo y carga diferida debajo del hero', async ({
  page,
}) => {
  await page.goto('/')
  const hero = page.locator('#inicio picture img').first()
  await expect(hero).toHaveAttribute('alt', /Datos de ejemplo/)
  await expect(hero).toHaveAttribute('loading', 'eager')
  await expect(page.locator('#inicio picture source[type="image/avif"]').first()).toBeAttached()
  const below = page.locator('#vista-factura picture img')
  await expect(below).toHaveCount(3)
  for (const img of await below.all()) await expect(img).toHaveAttribute('loading', 'lazy')
})
