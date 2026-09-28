import { expect, test } from '@playwright/test'
import { expectNoAxeViolations, noHorizontalScroll } from './helpers'

// Incremento 5 · cobranza (pantallas 22–27), con axe (WCAG 2.1 AA) en 1440 px y 390 px.

test('pantalla 22 · cuentas por cobrar', async ({ page }) => {
  await page.goto('/cobranza/cuentas')
  await expect(page.getByRole('table', { name: 'Cuentas por cobrar' })).toBeVisible()
  await expect(page.getByText('FAC-0000029')).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 23 · aging con gráfico y tabla', async ({ page }) => {
  await page.goto('/cobranza/aging')
  await expect(page.getByRole('table', { name: 'Saldo por tramo y moneda' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 24 · detalle de la cuenta con alta rápida de seguimiento', async ({ page }) => {
  await page.goto('/cobranza/cuentas/r29')
  await expect(page.getByRole('heading', { level: 1, name: 'FAC-0000029' })).toBeVisible()
  await page.getByRole('button', { name: '+ Nuevo seguimiento' }).click()
  await expect(page.getByRole('textbox', { name: /Nota/ })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 26 · registrar un pago en tres pasos, y pantalla 27 · su detalle', async ({ page }) => {
  await page.goto('/cobranza/pagos/nuevo')
  await expect(page.getByRole('heading', { level: 1, name: 'Registrar pago' })).toBeVisible()
  await page.getByRole('combobox', { name: /Cliente/ }).selectOption('c9')
  await page.getByRole('textbox', { name: /^Monto/ }).fill('150000')
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('button', { name: 'Siguiente: aplicar ›' }).click()
  await expect(page.getByRole('textbox', { name: 'Monto a aplicar a FAC-0000029' })).toHaveValue('85000')
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('button', { name: 'Siguiente: confirmar ›' }).click()
  await expect(page.getByText('Se aplicará a')).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: 'Registrar pago' }).click()

  await expect(page).toHaveURL(/\/cobranza\/pagos\/pg\d+$/)
  await expect(page.getByRole('heading', { level: 1, name: /^Pago del/ })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 25 · pagos, y pantalla 27 · anular con motivo', async ({ page }) => {
  await page.goto('/cobranza/pagos')
  await expect(page.getByRole('table', { name: 'Pagos' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.goto('/cobranza/pagos/pg18')
  await page.getByRole('button', { name: 'Anular pago…' }).click()
  const dialog = page.getByRole('dialog', { name: /^Anular el pago del/ })
  await expect(dialog).toBeVisible()
  await expectNoAxeViolations(page)
})
