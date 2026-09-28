/**
 * Puerto de autenticación. Lo implementan Supabase Auth (`supabase.ts`, con el Portal Gateway) y un adaptador
 * simulado (siempre con sesión, para la etapa de diseño). Las pantallas no conocen a Supabase.
 */
/** Parámetro con la ruta de vuelta después de iniciar sesión (P8b: al vencer, se vuelve sin perder la ruta). */
export const RETURN_PARAM = 'volver'

export type AuthStatus = 'loading' | 'signedIn' | 'signedOut'

export interface AuthSession {
  email: string
}

export type SignInResult = { ok: true } | { ok: false; reason: 'invalid-credentials' | 'unavailable' }

/** Parámetro con el que vuelve el enlace de confirmación del correo a la pantalla 1. */
export const CONFIRMED_PARAM = 'cuenta'

/**
 * Resultado del registro. `confirmEmail`: Supabase pide confirmar el correo antes de dar sesión (la cuenta existe,
 * pero todavía no se puede entrar). Con un correo que ya tiene cuenta Supabase responde igual que con uno nuevo
 * cuando la confirmación está activa: el portal no revela quién está registrado.
 */
export type SignUpResult =
  | { ok: true; next: 'signedIn' | 'confirmEmail' }
  | {
      ok: false
      reason: 'exists' | 'weak-password' | 'invalid-email' | 'disabled' | 'rate-limited' | 'unavailable'
    }

export interface SignUpInput {
  fullName: string
  email: string
  password: string
}

export interface AuthPort {
  /** Sesión actual; null si no hay. Resuelve después de leer lo guardado. */
  current(): Promise<AuthSession | null>
  /** Access token vigente (refrescado si hace falta). null = sin sesión. */
  accessToken(): Promise<string | null>
  signIn(email: string, password: string): Promise<SignInResult>
  /**
   * Crea la cuenta en Supabase Auth con el nombre en `user_metadata.full_name`, que es de donde Platform lo toma al
   * dar de alta al usuario en su primera llamada (`GET /v1/me`). No crea organización: eso es la pantalla 3.
   */
  signUp(input: SignUpInput): Promise<SignUpResult>
  signOut(): Promise<void>
  /**
   * Pide el correo de recuperación. Resuelve igual exista o no la cuenta: la pantalla siempre dice lo mismo
   * («si el correo está registrado…»), para no revelar quién tiene cuenta.
   */
  requestPasswordReset(email: string): Promise<void>
  /**
   * Pide un token nuevo. Se usa después de cambiar de organización: el hook de Platform emite el `org_id` nuevo
   * solo en el token siguiente (ADR 0003 de Platform).
   */
  refresh(): Promise<void>
  /** Avisa cuando la sesión cambia (login, logout, vencimiento). Devuelve la función para dejar de escuchar. */
  onChange(listener: (session: AuthSession | null) => void): () => void
}

/** Sesión simulada: siempre adentro. La usa `VITE_DATA_SOURCE=mock` y todas las pruebas de pantallas. */
export const mockAuth: AuthPort = {
  current: async () => ({ email: 'maria.rojas@correo.example' }),
  accessToken: async () => 'token-simulado',
  signIn: async () => ({ ok: true }),
  signUp: async () => ({ ok: true, next: 'signedIn' }),
  signOut: async () => {},
  requestPasswordReset: async () => {},
  refresh: async () => {},
  onChange: () => () => {},
}
