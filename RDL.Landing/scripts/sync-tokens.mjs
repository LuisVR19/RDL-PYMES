// Genera src/styles/tokens.generated.css desde design/tokens.css (copia exacta de los tokens del portal).
// Tema oscuro: igual que el portal, con [data-theme="dark"] (lo pone public/theme.js según el botón o la
// preferencia del sistema). Sin JavaScript, la preferencia del sistema se aplica con prefers-color-scheme.
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))
const source = readFileSync(`${root}design/tokens.css`, 'utf8')

const dark = /\[data-theme="dark"\]\s*\{([\s\S]*?)\n\}/.exec(source)
if (!dark) throw new Error('design/tokens.css no tiene el bloque [data-theme="dark"]')

const out = `/* GENERADO por scripts/sync-tokens.mjs desde design/tokens.css. No editar a mano. */
${source.trim()}

@media (prefers-color-scheme: dark) {
  :root:not([data-theme='light']) {${dark[1]}
  }
}
`
writeFileSync(`${root}src/styles/tokens.generated.css`, out)
console.log('tokens: src/styles/tokens.generated.css')
