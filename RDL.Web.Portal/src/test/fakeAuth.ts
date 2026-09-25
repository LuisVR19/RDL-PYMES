import type { AuthPort, AuthSession } from '@/shared/auth/auth'

/** Sesión falsa y controlable: arranca con o sin sesión y registra el orden de las llamadas. */
export function fakeAuth({ signedIn, password = 'secreta' }: { signedIn: boolean; password?: string }) {
  const log: string[] = []
  let session: AuthSession | null = signedIn ? { email: 'a@b.cr' } : null
  let listener: ((s: AuthSession | null) => void) | null = null
  const port: AuthPort = {
    current: async () => session,
    accessToken: async () => (session ? 'tok' : null),
    signIn: async (email, pw) => {
      log.push(`signIn:${email}`)
      if (pw !== password) return { ok: false, reason: 'invalid-credentials' }
      session = { email }
      return { ok: true }
    },
    signOut: async () => {
      log.push('signOut')
      session = null
    },
    requestPasswordReset: async (email) => {
      log.push(`reset:${email}`)
    },
    refresh: async () => {
      log.push('refresh')
    },
    onChange: (l) => {
      listener = l
      return () => {
        listener = null
      }
    },
  }
  /** Simula que Supabase perdió la sesión por su cuenta (refresh vencido). */
  const lose = () => {
    session = null
    listener?.(null)
  }
  return { port, log, lose }
}
