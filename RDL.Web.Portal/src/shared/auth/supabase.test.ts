import type { SupabaseClient } from '@supabase/supabase-js'
import { describe, expect, it } from 'vitest'
import { fromClient } from './supabase'

type Auth = SupabaseClient['auth']

function client(auth: Partial<Record<keyof Auth, unknown>>) {
  return { auth: auth as unknown as Auth }
}

describe('autenticación con Supabase', () => {
  it('credenciales malas → mensaje genérico, sin distinguir la causa', async () => {
    const port = fromClient(
      client({ signInWithPassword: async () => ({ error: { status: 400, message: 'Invalid login' } }) }),
    )
    await expect(port.signIn('a@b.cr', 'x')).resolves.toEqual({ ok: false, reason: 'invalid-credentials' })
  })

  it('Supabase caído → «no disponible», no «credenciales malas»', async () => {
    const port = fromClient(
      client({ signInWithPassword: async () => ({ error: { status: 503, message: 'down' } }) }),
    )
    await expect(port.signIn('a@b.cr', 'x')).resolves.toEqual({ ok: false, reason: 'unavailable' })
  })

  it('el token sale de la sesión vigente', async () => {
    const port = fromClient(
      client({
        getSession: async () => ({ data: { session: { access_token: 'tok', user: { email: 'a@b.cr' } } } }),
      }),
    )
    await expect(port.accessToken()).resolves.toBe('tok')
    await expect(port.current()).resolves.toEqual({ email: 'a@b.cr' })
  })

  it('sin sesión no hay token', async () => {
    const port = fromClient(client({ getSession: async () => ({ data: { session: null } }) }))
    await expect(port.accessToken()).resolves.toBeNull()
  })

  it('un refresco que falla se reporta: el cambio de organización no puede seguir con el token viejo', async () => {
    const port = fromClient(client({ refreshSession: async () => ({ error: new Error('refresh vencido') }) }))
    await expect(port.refresh()).rejects.toThrow('refresh vencido')
  })
})
