import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Incremento 3 · borrador, emitir y notas, con axe (WCAG 2.1 AA) en 1440 px y 390 px.
const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']

async function expectNoAxeViolations(page: Page) {
  // Con una animación de entrada en curso (diálogos), axe mide colores a media opacidad y reporta contraste falso.
  await page.waitForFunction(() => document.getAnimations().every((a) => a.playState !== 'running'))
  const results = await new AxeBuilder({ page })
    .withTags(WCAG)
    .exclude('[aria-label^="Abrir barra de revisión"]')
    .analyze()
  expect(
    results.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([])
}

/**
 * Contra el ancho configurado y no contra `innerWidth`: en un navegador móvil el viewport de diseño crece con el
 * contenido que desborda, así que `scrollWidth - innerWidth` da 0 aunque la página se salga de los 390 px.
 */
async function noHorizontalScroll(page: Page) {
  const width = page.viewportSize()?.width ?? 0
  const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(scrollWidth).toBeLessThanOrEqual(width)
}

/** Navega dentro del portal sin recargar: los datos simulados viven en memoria. */
async function navigateInApp(page: Page, path: string) {
  await page.evaluate((p) => {
    window.history.pushState({}, '', p)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }, path)
}

async function draftInvoice(page: Page) {
  await page.goto('/facturas/nueva')
  await expect(page.getByRole('heading', { level: 1, name: 'Nueva factura' })).toBeVisible()
  await page.getByRole('combobox', { name: /Cliente/ }).fill('Roble')
  await page.getByRole('option', { name: /Ferretería El Roble/ }).click()
  await page.getByRole('button', { name: '+ Agregar del catálogo' }).click()
  await page.getByRole('button', { name: /Pintura acrílica blanca/ }).click()
}

test('pantalla 13 · borrador con totales del servidor, y pantalla 14 · emitir', async ({ page }) => {
  await draftInvoice(page)
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: /Guardar borrador/ }).click()
  await expect(page).toHaveURL(/\/facturas\/b\d+\/editar$/)
  await expect(page.getByText(/Calculado por el sistema/)).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('button', { name: 'Emitir factura…' }).click()
  const dialog = page.getByRole('dialog', { name: '¿Emitir la factura?' })
  await expect(dialog).toBeVisible()
  await expectNoAxeViolations(page)
  await dialog.getByRole('button', { name: 'Emitir factura' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'FAC-0000041' })).toBeVisible()
})

test('pantalla 13 · lo que falta antes de emitir y el alta rápida de cliente', async ({ page }) => {
  await page.goto('/facturas/nueva')
  await page.getByRole('button', { name: 'Emitir factura…' }).click()
  await expect(page.getByText('Revise estos datos antes de emitir:')).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByRole('combobox', { name: /Cliente/ }).fill('Taller Nuevo')
  await page.getByRole('option', { name: '+ Crear cliente «Taller Nuevo»' }).click()
  await expect(page.getByRole('dialog', { name: 'Nuevo cliente' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 16 · nota de crédito sobre una factura recién emitida', async ({ page }) => {
  await draftInvoice(page)
  await page.getByRole('button', { name: 'Emitir factura…' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Emitir factura' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'FAC-0000041' })).toBeVisible()
  const invoicePath = new URL(page.url()).pathname

  await navigateInApp(page, `${invoicePath}/nota-credito`)
  await expect(page.getByRole('heading', { level: 1, name: 'Nota de crédito' })).toBeVisible()
  await page.getByRole('textbox', { name: /Motivo/ }).fill('La identificación del receptor era incorrecta')
  await page.getByRole('button', { name: 'Guardar borrador' }).click()
  await expect(page).toHaveURL(/\?borrador=b\d+$/)
  await expect(page.getByText(/Calculado por el sistema/)).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})
