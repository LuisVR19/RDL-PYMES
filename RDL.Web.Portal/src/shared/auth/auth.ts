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

export interface AuthPort {
  /** Sesión actual; null si no hay. Resuelve después de leer lo guardado. */
  current(): Promise<AuthSession | null>
  /** Access token vigente (refrescado si hace falta). null = sin sesión. */
  accessToken(): Promise<string | null>
  signIn(email: string, password: string): Promise<SignInResult>
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
  signOut: async () => {},
  requestPasswordReset: async () => {},
  refresh: async () => {},
  onChange: () => () => {},
}
