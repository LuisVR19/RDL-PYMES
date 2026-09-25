import { useCallback, useRef } from 'react'

/**
 * `Idempotency-Key` de un formulario (P8b 4.3): una clave por intento de envío del usuario. Si reintenta el MISMO
 * envío (tras un error de red), se reutiliza la clave y la API devuelve lo mismo en vez de crear dos veces; si
 * cambió el contenido, es otro envío y lleva una clave nueva.
 */
export function useIdempotencyKey(newId: () => string = () => crypto.randomUUID()) {
  const last = useRef<{ content: string; key: string } | null>(null)
  return useCallback(
    (content: unknown): string => {
      const serialized = JSON.stringify(content)
      if (last.current?.content !== serialized) last.current = { content: serialized, key: newId() }
      return last.current.key
    },
    [newId],
  )
}
