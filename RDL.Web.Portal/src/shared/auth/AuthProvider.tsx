import { useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { mockAuth, type AuthPort, type AuthSession, type AuthStatus, type SignInResult } from './auth'
import { onSessionExpired } from './expiry'

/**
 * `expired` (pantalla 35): la sesión venció con el usuario adentro. La página y la caché se quedan —«seguirá en la
 * misma página y no perderá lo que estaba viendo»— y un diálogo pide la contraseña del MISMO correo. Salir con otra
 * cuenta sí vacía todo.
 */
export type SessionState = AuthStatus | 'expired'

interface AuthValue {
  status: SessionState
  session: AuthSession | null
  port: AuthPort
  signIn: (email: string, password: string) => Promise<SignInResult>
  /** Vuelve a entrar con el correo de la sesión vencida; si sale bien, reintenta lo que había fallado. */
  reauthenticate: (password: string) => Promise<SignInResult>
  /** Cierra la sesión y vacía TODA la caché: nada del usuario queda en memoria. */
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthValue | null>(null)

export function AuthProvider({ children, port = mockAuth }: { children: ReactNode; port?: AuthPort }) {
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<SessionState>('loading')
  const [session, setSession] = useState<AuthSession | null>(null)
  // Estado vigente para los listeners (que no se vuelven a suscribir en cada render).
  const current = useRef<{ status: SessionState; signingOut: boolean }>({
    status: 'loading',
    signingOut: false,
  })
  const apply = useCallback((next: SessionState, s: AuthSession | null) => {
    current.current.status = next
    setStatus(next)
    setSession(s)
  }, [])
  const expire = useCallback(() => {
    current.current.status = 'expired'
    setStatus('expired')
  }, [])

  useEffect(() => {
    let alive = true
    void port.current().then(
      (s) => alive && apply(s ? 'signedIn' : 'signedOut', s),
      () => alive && apply('signedOut', null),
    )
    const stopChange = port.onChange((s) => {
      if (!alive) return
      if (s) return apply('signedIn', s)
      // Se perdió la sesión sin que el usuario la cerrara aquí (refresh vencido): diálogo, no expulsión.
      if (current.current.status === 'signedIn' && !current.current.signingOut) return expire()
      if (current.current.status !== 'expired') {
        queryClient.clear()
        apply('signedOut', null)
      }
    })
    const stopExpiry = onSessionExpired(() => {
      if (alive && current.current.status === 'signedIn') expire()
    })
    return () => {
      alive = false
      stopChange()
      stopExpiry()
    }
  }, [port, queryClient, apply, expire])

  const signIn = useCallback(
    async (email: string, password: string) => {
      const r = await port.signIn(email, password)
      if (r.ok) {
        // Otro usuario puede haber usado este navegador: se empieza sin caché.
        queryClient.clear()
        apply('signedIn', await port.current())
      }
      return r
    },
    [port, queryClient, apply],
  )

  const reauthenticate = useCallback(
    async (password: string) => {
      const r = await port.signIn(session?.email ?? '', password)
      if (r.ok) {
        apply('signedIn', await port.current())
        // Lo que falló por el token vencido se vuelve a pedir; lo que estaba bien se queda.
        void queryClient.invalidateQueries()
      }
      return r
    },
    [port, queryClient, session, apply],
  )

  const signOut = useCallback(async () => {
    current.current.signingOut = true
    // Aunque Supabase no responda, localmente la sesión se cierra igual: nada queda en caché.
    await port.signOut().catch(() => {})
    current.current.signingOut = false
    queryClient.clear()
    apply('signedOut', null)
  }, [port, queryClient, apply])

  const value = useMemo(
    () => ({ status, session, port, signIn, reauthenticate, signOut }),
    [status, session, port, signIn, reauthenticate, signOut],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth fuera de AuthProvider')
  return ctx
}
