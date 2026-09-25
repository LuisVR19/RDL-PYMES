import { useEffect, useRef } from 'react'

/**
 * Atajos de teclado globales (pantalla 36). Las teclas de una sola letra no se disparan mientras se escribe en un
 * campo; las combinaciones con Ctrl/⌘ sí (Ctrl S guarda desde un campo).
 *
 * `combo`: '/', '?', 'n', 'Escape', 'mod+s' (mod = Ctrl en Windows y Linux, ⌘ en macOS).
 */
export function useHotkey(combo: string, handler: (e: KeyboardEvent) => void, enabled = true): void {
  const ref = useRef(handler)
  useEffect(() => {
    ref.current = handler
  })

  useEffect(() => {
    if (!enabled) return
    const wantsMod = combo.startsWith('mod+')
    const key = wantsMod ? combo.slice(4) : combo

    const onKey = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey
      if (wantsMod !== mod) return
      if (e.key.toLowerCase() !== key.toLowerCase()) return
      if (!wantsMod && isTyping(e.target)) return
      e.preventDefault()
      ref.current(e)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [combo, enabled])
}

export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable
}
