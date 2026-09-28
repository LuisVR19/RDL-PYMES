import { AxeBuilder } from '@axe-core/playwright'
import { expect, type Page } from '@playwright/test'

const WCAG = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']

/** axe (WCAG 2.1 AA) sobre la página, sin la barra de revisión de diseño, que no es producto. */
export async function expectNoAxeViolations(page: Page) {
  // Con una animación de entrada en curso (diálogos), axe mide colores a media opacidad y reporta contraste falso.
  // Las infinitas (el brillo de los esqueletos) no terminan nunca y no afectan el contraste: no se esperan.
  await page.waitForFunction(() =>
    document
      .getAnimations()
      .every((a) => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity),
  )
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
export async function noHorizontalScroll(page: Page) {
  const width = page.viewportSize()?.width ?? 0
  const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth)
  expect(scrollWidth).toBeLessThanOrEqual(width)
}
