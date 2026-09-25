// Importa design/textos.csv (clave,texto_es_CR) a src/shared/i18n/messages.design.ts.
// Uso: node scripts/import-texts.mjs. El archivo generado no se edita a mano: los textos nuevos del código van en
// src/shared/i18n/messages.app.ts.
import { readFileSync, writeFileSync } from 'node:fs'

const csv = readFileSync(new URL('../design/textos.csv', import.meta.url), 'utf8')

function parse(text) {
  const rows = []
  let row = []
  let field = ''
  let quoted = false
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    if (quoted) {
      if (c === '"' && text[i + 1] === '"') {
        field += '"'
        i++
      } else if (c === '"') quoted = false
      else field += c
    } else if (c === '"') quoted = true
    else if (c === ',') {
      row.push(field)
      field = ''
    } else if (c === '\n' || c === '\r') {
      if (c === '\r' && text[i + 1] === '\n') i++
      row.push(field)
      if (row.some((f) => f !== '')) rows.push(row)
      row = []
      field = ''
    } else field += c
  }
  if (field !== '' || row.length) rows.push([...row, field])
  return rows
}

const [header, ...rows] = parse(csv)
if (header?.[0] !== 'clave') throw new Error('textos.csv: se esperaba el encabezado clave,texto_es_CR')

const entries = rows.map(([key, text]) => `  ${JSON.stringify(key)}: ${JSON.stringify(text)},`)
const out = `// Generado por scripts/import-texts.mjs desde design/textos.csv. No editar a mano.
export const designMessages = {
${entries.join('\n')}
} as const
`
writeFileSync(new URL('../src/shared/i18n/messages.design.ts', import.meta.url), out)
console.log(`${rows.length} textos importados`)
