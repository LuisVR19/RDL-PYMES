/**
 * Enlaces de la landing: al portal (iniciar sesión, crear cuenta) y a WhatsApp. Funciones puras, sin `import.meta`,
 * para probarlas con `node --test` y usarlas igual en el build y en el navegador.
 */

/** Los parámetros de campaña que se conservan al saltar al portal. */
export const UTM_KEYS = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_term', 'utm_content'] as const

/** `https://app.dominio.cr` + `/ingresar` → URL absoluta. Tolera barras de más o de menos. */
export function portalUrl(base: string, path: string): string {
  return new URL(path.replace(/^\/*/, '/'), base.replace(/\/*$/, '/')).toString()
}

/** Los UTM de una búsqueda (`?utm_source=…`), en orden estable. Lo demás se ignora. */
export function utmFrom(search: string): [string, string][] {
  const params = new URLSearchParams(search)
  return UTM_KEYS.flatMap((k) => {
    const v = params.get(k)?.trim()
    return v ? [[k, v.slice(0, 200)] as [string, string]] : []
  })
}

/**
 * Agrega los UTM de la visita a un enlace del portal. Los que el enlace ya trae no se pisan: una campaña puesta a
 * propósito en un botón manda sobre la de la visita.
 */
export function withUtm(href: string, utm: [string, string][]): string {
  if (utm.length === 0) return href
  const url = new URL(href)
  for (const [k, v] of utm) if (!url.searchParams.has(k)) url.searchParams.set(k, v)
  return url.toString()
}

/** `https://wa.me/50600000000?text=…`, o null si no hay número (el botón no se muestra). */
export function whatsappUrl(number: string | undefined, message: string): string | null {
  const digits = (number ?? '').replace(/\D/g, '')
  if (digits.length < 8) return null
  return `https://wa.me/${digits}?text=${encodeURIComponent(message)}`
}
