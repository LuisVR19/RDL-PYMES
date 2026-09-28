import { utmFrom, withUtm } from './links'

const KEY = 'rdl.utm'

/**
 * Conserva los UTM de la visita (Paso 4 §1): los toma de la URL de llegada, los recuerda durante la sesión del
 * navegador (por si la persona pasa por otra página antes) y los agrega a cada enlace marcado con `data-portal`.
 * `sessionStorage` no es una cookie y no sale del navegador.
 */
export function initUtm(): void {
  let utm = utmFrom(window.location.search)
  try {
    if (utm.length > 0) sessionStorage.setItem(KEY, JSON.stringify(utm))
    else utm = JSON.parse(sessionStorage.getItem(KEY) ?? '[]') as [string, string][]
  } catch {
    // Sin almacenamiento (modo privado estricto): se usan solo los de esta URL.
  }
  if (utm.length === 0) return
  for (const a of document.querySelectorAll<HTMLAnchorElement>('a[data-portal]')) {
    a.href = withUtm(a.href, utm)
  }
}
