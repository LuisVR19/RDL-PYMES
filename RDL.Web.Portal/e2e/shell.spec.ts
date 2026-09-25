import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']

async function expectNoAxeViolations(page: import('@playwright/test').Page) {
  // La barra de revisión de diseño no es producto: se excluye del análisis.
  const results = await new AxeBuilder({ page })
    .withTags(WCAG)
    .exclude('[aria-label^="Abrir barra de revisión"]')
    .analyze()
  expect(
    results.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([])
}

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
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)
  expect(overflow).toBe(false)
})
