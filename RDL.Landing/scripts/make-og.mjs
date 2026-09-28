// Genera public/og.png (1200 × 630), la imagen para Open Graph y Twitter Card, con la marca y los colores del
// sistema de diseño. Se corre a mano cuando cambie el mensaje: `node scripts/make-og.mjs` (usa el Chromium de
// Playwright). Sin capturas ni datos reales.
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const root = fileURLToPath(new URL('..', import.meta.url))
const sitio = JSON.parse(readFileSync(`${root}src/content/sitio.json`, 'utf8'))
const font = (w) =>
  readFileSync(
    `${root}node_modules/@fontsource/ibm-plex-sans/files/ibm-plex-sans-latin-${w}-normal.woff2`,
  ).toString('base64')

const html = `<!doctype html><html><head><style>
@font-face{font-family:Plex;font-weight:400;src:url(data:font/woff2;base64,${font(400)}) format('woff2')}
@font-face{font-family:Plex;font-weight:700;src:url(data:font/woff2;base64,${font(700)}) format('woff2')}
body{margin:0;width:1200px;height:630px;background:#0F1B33;color:#fff;font-family:Plex;display:flex;flex-direction:column;justify-content:space-between;padding:72px 80px;box-sizing:border-box}
.brand{display:flex;align-items:center;gap:20px;font-size:40px;font-weight:700}
h1{margin:0;font-size:64px;line-height:76px;font-weight:700;max-width:980px}
p{margin:0;font-size:28px;line-height:40px;color:#C9D1E0}
.bar{width:120px;height:8px;border-radius:4px;background:#7AA2F7}
</style></head><body>
<div class="brand"><svg width="88" height="88" viewBox="0 0 64 64"><rect width="64" height="64" rx="13" fill="#23345A"/><text x="32" y="39.5" text-anchor="middle" font-family="Plex" font-weight="700" font-size="20" letter-spacing="1.2" fill="#fff">RDL</text><rect x="14" y="45" width="36" height="3" rx="1.5" fill="#7AA2F7"/></svg>RDL</div>
<div><h1>${sitio.home.hero.title}</h1></div>
<div><div class="bar"></div><p style="margin-top:24px">${sitio.footer.tagline}</p></div>
</body></html>`

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1200, height: 630 } })
await page.setContent(html)
await page.evaluate(() => document.fonts.ready)
await page.screenshot({ path: `${root}public/og.png` })
await browser.close()
console.log('public/og.png')
