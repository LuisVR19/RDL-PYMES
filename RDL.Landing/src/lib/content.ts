import sitio from '../content/sitio.json'

/**
 * Los textos del sitio viven en `src/content/` (nunca en los componentes). `fill` reemplaza los `{marcadores}` con
 * datos de configuración (correo, horario…); un marcador sin valor se queda visible para detectarlo en revisión, y
 * `check:content` hace fallar el build si llega a una página.
 */
export const content = sitio
export const { contact } = sitio

export function fill(text: string, vars: Record<string, string | number>): string {
  return text.replace(/\{(\w+)\}/g, (match, name: string) => {
    const v = vars[name]
    return v === undefined || v === '' ? match : String(v)
  })
}
