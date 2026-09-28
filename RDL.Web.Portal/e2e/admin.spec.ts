import { expect, test } from '@playwright/test'
import { expectNoAxeViolations, noHorizontalScroll } from './helpers'

// Incremento 6 · inicio y administración, con axe (WCAG 2.1 AA) en 1440 px y 390 px.

test('pantalla 6 · inicio con cifras y paneles', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Inicio' })).toBeVisible()
  await expect(
    page.getByRole('region', { name: 'Últimas facturas' }).getByRole('button').first(),
  ).toBeVisible()
  await expect(page.getByRole('region', { name: 'Últimos pagos' }).getByRole('button').first()).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantallas 28 y 29 · organización y sucursales', async ({ page }) => {
  await page.goto('/admin/organizacion')
  await expect(page.getByLabel('Zona horaria')).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('tab', { name: 'Sucursales' }).click()
  await expect(page).toHaveURL(/\/admin\/organizacion\/sucursales$/)
  await page.getByRole('button', { name: 'Nueva sucursal' }).click()
  await page.getByLabel(/Código/).fill('003')
  await page.getByLabel(/Nombre/).fill('Heredia')
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: 'Crear sucursal' }).click()
  await expect(page.getByText('Heredia', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Desactivar la sucursal 002' }).click()
  await expect(page.getByRole('button', { name: 'Activar la sucursal 002' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantallas 30 y 31 · usuarios, invitación y revocar', async ({ page }) => {
  await page.goto('/admin/usuarios')
  await expect(page.getByRole('combobox', { name: 'Rol de Sofía Chaves León' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('button', { name: 'Invitar usuario' }).click()
  await expect(page).toHaveURL(/\/admin\/usuarios\/invitaciones$/)
  await page.getByLabel(/Correo de la persona/).fill('jorge.mora@correo.example')
  await page.getByRole('button', { name: 'Crear invitación' }).click()
  await expect(page.getByText(/\/invitacion\/demo/)).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)

  await page.getByRole('button', { name: 'Revocar la invitación de laura.vindas@almendros.example' }).click()
  const dialog = page.getByRole('dialog', { name: '¿Revocar la invitación?' })
  await expect(dialog).toBeVisible()
  await expectNoAxeViolations(page)
  await dialog.getByRole('button', { name: 'Revocar' }).click()
  await expect(dialog).toBeHidden()
})

test('pantalla 32 · exportar auditoría (sin API)', async ({ page }) => {
  await page.goto('/admin/auditoria')
  await expect(page.getByRole('button', { name: 'Descargar CSV' })).toBeDisabled()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})
