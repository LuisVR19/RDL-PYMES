import { createClient, type Session, type SupabaseClient } from '@supabase/supabase-js'
import type { AuthPort, AuthSession } from './auth'

/**
 * Autenticación con Supabase Auth (correo y contraseña).
 *
 * Almacenamiento (ADR 0005): `localStorage`, el valor por defecto de supabase-js, con una CSP estricta que limita
 * de dónde se ejecuta código y a dónde se conecta. La sesión sobrevive recargas y el refresco es automático.
 */
export function createSupabaseAuth(url: string, publishableKey: string): AuthPort {
  const client: SupabaseClient = createClient(url, publishableKey, {
    auth: { persistSession: true, autoRefreshToken: true, detectSessionInUrl: false },
  })
  return fromClient(client)
}

const toSession = (s: Session | null): AuthSession | null => (s ? { email: s.user.email ?? '' } : null)

/** Separado para poder probarlo con un cliente falso. */
export function fromClient(client: Pick<SupabaseClient, 'auth'>): AuthPort {
  return {
    async current() {
      const { data } = await client.auth.getSession()
      return toSession(data.session)
    },
    async accessToken() {
      // getSession refresca el token si está por vencer.
      const { data } = await client.auth.getSession()
      return data.session?.access_token ?? null
    },
    async signIn(email, password) {
      const { error } = await client.auth.signInWithPassword({ email, password })
      if (!error) return { ok: true }
      // Mensaje genérico (P8b): no se distingue «no existe» de «contraseña incorrecta».
      const status = (error as { status?: number }).status ?? 0
      return { ok: false, reason: status >= 400 && status < 500 ? 'invalid-credentials' : 'unavailable' }
    },
    async signOut() {
      await client.auth.signOut()
    },
    async requestPasswordReset(email) {
      // TODO(diseño): el enlace del correo vuelve al portal con una sesión de recuperación, pero el diseño no
      // tiene la pantalla «crear contraseña nueva». Hasta entonces el enlace deja al usuario en la pantalla 1.
      // Un error (cuenta inexistente, límite de envíos) no se muestra: el mensaje es siempre el mismo.
      await client.auth.resetPasswordForEmail(email, { redirectTo: `${window.location.origin}/ingresar` })
    },
    async refresh() {
      const { error } = await client.auth.refreshSession()
      if (error) throw error
    },
    onChange(listener) {
      const { data } = client.auth.onAuthStateChange((_event, session) => listener(toSession(session)))
      return () => data.subscription.unsubscribe()
    },
  }
}
