import { useEffect, useRef, useState } from 'react'

/**
 * Paginación por cursor (convenciones §9: solo «Anterior / Siguiente»). Guarda la pila de cursores ya visitados
 * para poder volver, y vuelve a la primera página cuando cambian los filtros.
 */
export function usePager(resetOn: unknown[]) {
  const [stack, setStack] = useState<(string | undefined)[]>([undefined])
  const key = JSON.stringify(resetOn)
  const last = useRef(key)
  useEffect(() => {
    if (last.current !== key) {
      last.current = key
      setStack([undefined])
    }
  }, [key])

  const cursor = stack[stack.length - 1]
  return {
    cursor,
    controls: (nextCursor: string | null) => ({
      hasPrev: stack.length > 1,
      hasNext: nextCursor !== null,
      onPrev: () => setStack((s) => (s.length > 1 ? s.slice(0, -1) : s)),
      onNext: () => nextCursor && setStack((s) => [...s, nextCursor]),
    }),
  }
}
