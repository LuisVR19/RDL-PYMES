// check:content (Paso 6): el build falla si en el contenido o en las páginas publicables queda un marcador
// «<<…>>» sin reemplazar, o una afirmación prohibida (Hacienda no certifica proveedores; nada de cifras ni
// testimonios inventados). Revisa las fuentes y, si existe, el sitio ya construido (dist/).
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('..', import.meta.url))

export const FORBIDDEN = [
  { re: /<<[^>]*>>/, why: 'marcador sin reemplazar' },
  {
    re: /certificad[oa]s?\s+(por|ante)\s+(el\s+)?(ministerio\s+de\s+)?hacienda/i,
    why: 'Hacienda no certifica proveedores',
  },
  {
    re: /(avalad|aprobad|autorizad|homologad)[oa]s?\s+(por|ante)\s+(el\s+)?(ministerio\s+de\s+)?hacienda/i,
    why: 'Hacienda no avala proveedores',
  },
  { re: /proveedor\s+(oficial|autorizado)\s+de\s+hacienda/i, why: 'Hacienda no tiene proveedores oficiales' },
  { re: /\+\s?\d[\d\s.,]*\s+(empresas|clientes|pymes|usuarios)/i, why: 'cifra de clientes inventada' },
  { re: /testimonio\s+pendiente/i, why: 'testimonio pendiente' },
]

/** Hallazgos en un texto: [{ line, why, match }]. La negación explícita («no está certificado ni avalado por Hacienda») se permite. */
export function findProblems(text) {
  const out = []
  text.split(/\r?\n/).forEach((line, i) => {
    for (const { re, why } of FORBIDDEN) {
      const m = re.exec(line)
      if (!m) continue
      const before = line.slice(Math.max(0, m.index - 40), m.index).toLowerCase()
      if (why.startsWith('Hacienda') && /\bno\s+(es|está|esta)\b|\bni\b/.test(before)) continue
      out.push({ line: i + 1, why, match: m[0] })
    }
  })
  return out
}

function walk(dir, exts) {
  if (!existsSync(dir)) return []
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) return walk(p, exts)
    return exts.some((e) => p.endsWith(e)) ? [p] : []
  })
}

function main() {
  const files = [
    ...walk(join(root, 'src'), ['.json', '.md', '.astro', '.ts']).filter((f) => !f.endsWith('.test.ts')),
    ...walk(join(root, 'dist'), ['.html', '.xml', '.txt']),
  ]
  const problems = files.flatMap((f) =>
    findProblems(readFileSync(f, 'utf8')).map(
      (p) => `${relative(root, f)}:${p.line}  ${p.why}: «${p.match}»`,
    ),
  )
  if (problems.length > 0) {
    console.error(`check:content encontró ${problems.length} problema(s):\n  ${problems.join('\n  ')}`)
    process.exit(1)
  }
  console.log(`check:content: ${files.length} archivos revisados, sin marcadores ni afirmaciones prohibidas.`)
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) main()
