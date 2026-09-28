// Lighthouse local contra el sitio construido (Paso 5: ≥ 95 en las cuatro categorías, en móvil).
// En Windows, `lhci autorun` falla al cerrar Chrome (chrome-launcher: EPERM al borrar su carpeta temporal). Este
// script levanta Chrome (el de Playwright) con un puerto de depuración y le conecta Lighthouse; en CI se usa lhci.
// Uso: npm run build && npx astro preview --port 4323 & node scripts/lighthouse-local.mjs
import { spawn } from 'node:child_process'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { chromium } from '@playwright/test'
import lighthouse from 'lighthouse'

const BASE = process.env.LH_BASE ?? 'http://127.0.0.1:4323'
const PAGES = ['/', '/contadores/', '/terminos/', '/privacidad/']
const MIN = 0.95
const PORT = 9333
// Los borradores legales llevan noindex a propósito: ahí «is-crawlable» en 0 es lo esperado, no una falla.
const EXPECTED = { '/terminos/': ['is-crawlable'], '/privacidad/': ['is-crawlable'] }

const chrome = spawn(
  chromium.executablePath(),
  [
    '--headless=new',
    `--remote-debugging-port=${PORT}`,
    `--user-data-dir=${mkdtempSync(join(tmpdir(), 'rdl-lh-'))}`,
    '--no-first-run',
    'about:blank',
  ],
  { stdio: 'ignore' },
)
await new Promise((r) => setTimeout(r, 1500))

let failed = false
try {
  for (const path of PAGES) {
    const result = await lighthouse(`${BASE}${path}`, { port: PORT, output: 'json', logLevel: 'error' })
    const cats = result?.lhr.categories ?? {}
    const scores = Object.fromEntries(Object.entries(cats).map(([k, c]) => [k, c.score ?? 0]))
    const allowed = EXPECTED[path] ?? []
    const failing = (k) =>
      (cats[k]?.auditRefs ?? []).some((ref) => {
        const a = result?.lhr.audits[ref.id]
        return ref.weight > 0 && a && a.score !== null && a.score < 1 && !allowed.includes(a.id)
      })
    const low = Object.entries(scores).filter(([k, s]) => s < MIN && (allowed.length === 0 || failing(k)))
    if (low.length) failed = true
    const lcp = result?.lhr.audits['largest-contentful-paint']?.displayValue
    const cls = result?.lhr.audits['cumulative-layout-shift']?.displayValue
    console.log(
      `${path.padEnd(14)} ${Object.entries(scores)
        .map(([k, s]) => `${k} ${Math.round(s * 100)}`)
        .join(
          ' · ',
        )} · LCP ${lcp} · CLS ${cls}${low.length ? '  ← bajo el umbral' : allowed.length ? '  (noindex esperado)' : ''}`,
    )
    for (const [k] of low) {
      const refs = cats[k]?.auditRefs ?? []
      for (const ref of refs) {
        const a = result?.lhr.audits[ref.id]
        if (a && a.score !== null && a.score < 1 && ref.weight > 0)
          console.log(`    ${k}: ${a.id} (${a.score})`)
      }
    }
  }
} finally {
  chrome.kill()
}
process.exit(failed ? 1 : 0)
