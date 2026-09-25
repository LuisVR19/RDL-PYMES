import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'

export type Theme = 'light' | 'dark'

const KEY = 'rdl.theme'

function readStored(): Theme | undefined {
  try {
    const v = window.localStorage.getItem(KEY)
    return v === 'dark' || v === 'light' ? v : undefined
  } catch {
    return undefined
  }
}

const ThemeContext = createContext<{ theme: Theme; toggle: () => void } | null>(null)

/** Tema claro u oscuro: cambia `data-theme` en la raíz; los colores salen de los tokens (decisión 9 del diseño). */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(() => readStored() ?? 'light')

  useEffect(() => {
    document.documentElement.dataset.theme = theme
    try {
      window.localStorage.setItem(KEY, theme)
    } catch {
      // Preferencia de conveniencia: si el navegador no deja guardar, el tema igual funciona en la sesión.
    }
  }, [theme])

  const toggle = useCallback(() => setTheme((t) => (t === 'dark' ? 'light' : 'dark')), [])
  return <ThemeContext.Provider value={{ theme, toggle }}>{children}</ThemeContext.Provider>
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme fuera de ThemeProvider')
  return ctx
}
