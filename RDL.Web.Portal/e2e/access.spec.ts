import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Incremento 2 · pantallas de acceso, perfil y sesión vencida, con axe (WCAG 2.1 AA) en 1440 px y 390 px.
const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']

async function expectNoAxeViolations(page: Page) {
  const results = await new AxeBuilder({ page })
    .withTags(WCAG)
    .exclude('[aria-label^="Abrir barra de revisión"]')
    .analyze()
  expect(
    results.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`),
  ).toEqual([])
}

async function noHorizontalScroll(page: Page) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
  expect(overflow).toBeLessThanOrEqual(0)
}

test('pantalla 1 · recuperar contraseña', async ({ page }) => {
  await page.goto('/recuperar')
  await expect(page.getByRole('heading', { level: 1, name: 'Recuperar contraseña' })).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByLabel('Correo').fill('nadie@empresa.cr')
  await page.getByRole('button', { name: 'Enviar enlace' }).click()
  await expect(page.getByRole('heading', { name: 'Revise su correo' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 2 · selector de organización', async ({ page }) => {
  await page.goto('/organizaciones')
  await expect(page.getByRole('heading', { level: 1, name: 'Elija una organización' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Café Monteazul/ })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
  await page.getByRole('button', { name: /Café Monteazul/ }).click()
  await expect(page).toHaveURL(/\/$/)
  await expect(page.getByText('Ahora trabaja en Café Monteazul S.A.')).toBeVisible()
})

test('pantalla 3 · crear organización, también con errores', async ({ page }) => {
  await page.goto('/organizaciones/nueva')
  await expect(page.getByRole('heading', { level: 1, name: 'Crear organización' })).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: 'Crear organización' }).click()
  await expect(page.getByText('Escriba la razón social.')).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 4 · aceptar invitación y su estado vencida', async ({ page }) => {
  await page.goto('/invitacion/vencida')
  await expect(page.getByRole('heading', { level: 1, name: 'Lo invitaron a una organización' })).toBeVisible()
  await expectNoAxeViolations(page)
  await page.getByRole('button', { name: 'Aceptar e ingresar' }).click()
  await expect(page.getByRole('heading', { name: 'Esta invitación venció' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 33 · mi perfil', async ({ page }) => {
  await page.goto('/perfil')
  await expect(page.getByRole('heading', { level: 1, name: 'Mi perfil' })).toBeVisible()
  await expectNoAxeViolations(page)
  await noHorizontalScroll(page)
})

test('pantalla 35 · sesión vencida en diálogo, sin perder la página', async ({ page }) => {
  await page.goto('/perfil')
  await expect(page.getByRole('heading', { level: 1, name: 'Mi perfil' })).toBeVisible()
  await page.getByRole('button', { name: /Abrir barra de revisión/ }).click()
  await page.getByRole('button', { name: 'Vencer la sesión (pantalla 35)' }).click()
  const dialog = page.getByRole('dialog', { name: 'Su sesión venció' })
  await expect(dialog).toBeVisible()
  await expectNoAxeViolations(page)
  await page.keyboard.press('Escape')
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('Contraseña').fill('cualquiera')
  await dialog.getByRole('button', { name: 'Continuar' }).click()
  await expect(dialog).toBeHidden()
  await expect(page).toHaveURL(/\/perfil$/)
})
