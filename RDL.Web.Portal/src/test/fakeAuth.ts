import type { AuthPort, AuthSession, SignUpResult } from '@/shared/auth/auth'

/** Sesión falsa y controlable: arranca con o sin sesión y registra el orden de las llamadas. */
export function fakeAuth({
  signedIn,
  password = 'secreta',
  signUp = { ok: true, next: 'confirmEmail' },
}: {
  signedIn: boolean
  password?: string
  /** Lo que responde el registro (por defecto, «confirme su correo»). */
  signUp?: SignUpResult
}) {
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
    signUp: async (input) => {
      log.push(`signUp:${input.email}:${input.fullName}`)
      if (signUp.ok && signUp.next === 'signedIn') session = { email: input.email }
      return signUp
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
