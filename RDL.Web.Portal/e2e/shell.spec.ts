import { expect, test } from '@playwright/test'
import { expectNoAxeViolations, noHorizontalScroll } from './helpers'

test('inicio carga el armazón sin violaciones de accesibilidad', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Inicio' })).toBeVisible()
  await expectNoAxeViolations(page)
})

test('tema oscuro sin violaciones de accesibilidad', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Cambiar a tema oscuro' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expectNoAxeViolations(page)
})

test('cambio de organización vuelve al inicio y confirma el rol', async ({ page }, info) => {
  await page.goto('/clientes')
  await page.getByRole('button', { name: /Organización activa/ }).click()
  await page.getByRole('button', { name: /Ferretería El Roble/ }).click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByText('Ahora trabaja en Ferretería El Roble S.A.')).toBeVisible()
  if (info.project.name === 'escritorio')
    await expect(page.getByRole('banner').getByText('Producción', { exact: true })).toBeVisible()
})

test('atajo ? abre la hoja de atajos', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('heading', { level: 1, name: 'Inicio' }).waitFor()
  await page.keyboard.press('?')
  await expect(page.getByRole('dialog', { name: 'Atajos de teclado' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeHidden()
})

test('catálogo de componentes sin violaciones de accesibilidad', async ({ page }) => {
  await page.goto('/_catalogo')
  await expect(page.getByRole('heading', { level: 1, name: 'Catálogo de componentes' })).toBeVisible()
  await expectNoAxeViolations(page)
})

test('móvil: menú de hamburguesa navega y se cierra', async ({ page }, info) => {
  test.skip(info.project.name !== 'movil', 'solo en 390 px')
  await page.goto('/')
  await page.getByRole('button', { name: 'Menú', exact: true }).click()
  await page.getByRole('link', { name: /Documentos/ }).click()
  await expect(page).toHaveURL(/\/documentos$/)
  await expect(page.getByRole('heading', { level: 1, name: 'Documentos' })).toBeVisible()
  await noHorizontalScroll(page)
})

test('pantalla 1 · iniciar sesión sin violaciones de accesibilidad, también con errores', async ({
  page,
}) => {
  await page.goto('/ingresar?volver=%2Fclientes')
  await expect(page.getByRole('heading', { level: 1, name: 'Iniciar sesión' })).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: 'Ingresar' }).click()
  await expect(page.getByText('Escriba un correo válido.')).toBeVisible()
  await expectNoAxeViolations(page)
})

test('la CSP del build está activa y no bloquea nada del portal', async ({ page }) => {
  const blocked: string[] = []
  page.on('console', (m) => {
    if (/Content Security Policy|Content-Security-Policy/i.test(m.text())) blocked.push(m.text())
  })
  for (const path of ['/', '/ingresar', '/_catalogo']) {
    await page.goto(path)
    await page.waitForLoadState('networkidle')
  }
  const csp = await page.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute('content')
  expect(csp).toContain("script-src 'self'")
  expect(blocked).toEqual([])
})

test('el login y los formularios quedan centrados', async ({ page }) => {
  const viewport = page.viewportSize()
  if (!viewport) throw new Error('sin viewport')

  // Login: centrado en ancho y en alto (prototipo «01»: justify-content: safe center).
  await page.goto('/ingresar')
  const card = await page.locator('main').boundingBox()
  if (!card) throw new Error('sin tarjeta de login')
  expect(Math.abs(card.x + card.width / 2 - viewport.width / 2)).toBeLessThanOrEqual(2)
  expect(card.y).toBeGreaterThan(viewport.height * 0.15)

  // Un formulario del armazón: centrado en el área de contenido (a la derecha del menú).
  await page.goto('/clientes/nuevo')
  await expect(page.getByRole('heading', { level: 1, name: 'Nuevo cliente' })).toBeVisible()
  const form = await page
    .locator('form', { has: page.getByRole('heading', { name: 'Nuevo cliente' }) })
    .boundingBox()
  const content = await page.getByRole('main').boundingBox()
  if (!form || !content) throw new Error('sin formulario')
  expect(Math.abs(form.x + form.width / 2 - (content.x + content.width / 2))).toBeLessThanOrEqual(2)
})
