import { appMessages } from './messages.app'
import { designMessages } from './messages.design'

const messages = { ...designMessages, ...appMessages }

export type MessageKey = keyof typeof messages

/**
 * Devuelve el texto de la interfaz (es-CR) y reemplaza los marcadores `{variable}`.
 * Una clave desconocida no compila; una variable faltante se deja visible (`{org}`) para detectarla en revisión.
 */
export function t(key: MessageKey, vars?: Record<string, string | number>): string {
  const text: string = messages[key]
  if (!vars) return text
  return text.replace(/\{(\w+)\}/g, (match, name: string) => (name in vars ? String(vars[name]) : match))
}
