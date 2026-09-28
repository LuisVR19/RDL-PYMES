// Capturas del portal para la landing, en src/assets/capturas/. Se toman del portal en modo SIMULADO (sus datos
// son los ficticios del prototipo de diseño): nunca de un ambiente con datos reales.
//
//   cd ../RDL.Web.Portal && npm run build && npx vite preview --port 4174
//   node scripts/capture-portal.mjs            (PORTAL_URL=http://127.0.0.1:4174 por defecto)
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { chromium } from '@playwright/test'

const PORTAL = process.env.PORTAL_URL ?? 'http://127.0.0.1:4174'
const out = fileURLToPath(new URL('../src/assets/capturas/', import.meta.url))
mkdirSync(out, { recursive: true })

// La barra de revisión (botón flotante del modo simulado) no es parte del producto.
const HIDE = '[aria-label^="Abrir barra de revisión"]{display:none!important}'

const browser = await chromium.launch()

async function page(viewport, scale, theme = 'light') {
  // bypassCSP: la CSP del portal (bien) no deja agregar el estilo que oculta la barra de revisión.
  const p = await browser.newPage({ viewport, deviceScaleFactor: scale, colorScheme: theme, bypassCSP: true })
  await p.addInitScript((t) => {
    try {
      localStorage.setItem('rdl.theme', t)
    } catch {}
  }, theme)
  return p
}

async function shot(p, name, clip) {
  await p.addStyleTag({ content: HIDE })
  await p.waitForTimeout(400)
  await p.screenshot({ path: `${out}${name}.png`, ...(clip ? { clip } : {}) })
  console.log(`capturas/${name}.png`)
}

const desk = { width: 1440, height: 900 }

// Detalle de factura (pantalla 15): las tres cifras.
let p = await page(desk, 2)
await p.goto(`${PORTAL}/facturas/i34`)
await p.getByText('Hacienda: Aceptada').waitFor()
await shot(p, 'detalle')
await p.close()

// Documentos (pantalla 12): la lista con Hacienda y saldo por fila.
p = await page(desk, 2)
await p.goto(`${PORTAL}/documentos`)
await p.getByRole('table', { name: 'Documentos' }).getByText('FAC-0000040').waitFor()
await shot(p, 'documentos')
await p.close()

// Borrador (pantalla 13) con totales calculados.
p = await page(desk, 2)
await p.goto(`${PORTAL}/facturas/nueva`)
await p.getByRole('combobox', { name: /Cliente/ }).fill('Roble')
await p.getByRole('option', { name: /Ferretería El Roble/ }).click()
for (const product of [/Pintura acrílica blanca/, /Servicio de instalación/]) {
  await p.getByRole('button', { name: '+ Agregar del catálogo' }).click()
  await p.getByRole('button', { name: product }).click()
}
await p.getByRole('button', { name: /Guardar borrador/ }).click()
await p.getByText(/Calculado por el sistema/).waitFor()
await p.mouse.move(0, 0)
await shot(p, 'borrador')
await p.close()

// Selector de organizaciones abierto (contadores).
p = await page(desk, 2)
await p.goto(`${PORTAL}/documentos`)
await p.getByRole('table', { name: 'Documentos' }).getByText('FAC-0000040').waitFor()
const trigger = p.getByRole('button', { name: /Comercial Los Almendros/ }).first()
await trigger.click()
const last = p.getByText('Ver todas mis organizaciones')
await last.waitFor()
await p.waitForTimeout(500)
// Solo el selector abierto: desde el botón hasta el final de la lista, con un margen.
const a = await trigger.boundingBox()
const b = await last.boundingBox()
const search = await p.getByPlaceholder('Buscar organización').boundingBox()
if (!a || !b || !search) throw new Error('no se encontró el selector de organizaciones')
const pad = 16
const right = Math.max(a.x + a.width, search.x + search.width + 12)
await shot(p, 'organizaciones', {
  x: a.x - pad,
  y: a.y - pad,
  width: right - a.x + 2 * pad,
  height: b.y + b.height - a.y + 2 * pad + 12,
})
await p.close()

// Detalle en el celular (pantalla 15 a 390 px).
p = await page({ width: 390, height: 844 }, 3)
await p.goto(`${PORTAL}/facturas/i34`)
await p.getByText('Hacienda: Aceptada').waitFor()
await shot(p, 'detalle-movil')
await p.close()

await browser.close()
