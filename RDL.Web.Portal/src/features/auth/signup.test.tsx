import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { fromClient } from '@/shared/auth/supabase'
import { fakeAuth } from '@/test/fakeAuth'
import { renderApp } from '@/test/renderApp'

async function fill(user: ReturnType<typeof userEvent.setup>, password = 'segura-2026') {
  await user.type(await screen.findByLabelText('Nombre completo'), '  María   Rojas ')
  await user.type(screen.getByLabelText('Correo'), 'maria@empresa.cr')
  await user.type(screen.getByLabelText('Contraseña'), password)
  await user.type(screen.getByLabelText('Confirme la contraseña'), password)
}

describe('crear cuenta (/registro)', () => {
  it('valida antes de llamar: nombre, correo, largo y que las contraseñas coincidan', async () => {
    const auth = fakeAuth({ signedIn: false })
    const signUp = vi.spyOn(auth.port, 'signUp')
    renderApp('/registro', { auth: auth.port })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Crear cuenta' }))
    expect(screen.getByText('Escriba su nombre y apellido.')).toBeInTheDocument()
    expect(screen.getByText('Escriba un correo válido.')).toBeInTheDocument()
    expect(screen.getByText('Use al menos 8 caracteres.')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Contraseña'), 'segura-2026')
    await user.type(screen.getByLabelText('Confirme la contraseña'), 'otra-cosa')
    await user.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(screen.getByText('Las contraseñas no coinciden.')).toBeInTheDocument()
    expect(signUp).not.toHaveBeenCalled()
  })

  it('con confirmación de correo: «Revise su correo» con la dirección, sin sesión', async () => {
    const auth = fakeAuth({ signedIn: false })
    renderApp('/registro', { auth: auth.port })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByRole('heading', { name: 'Revise su correo' })).toBeInTheDocument()
    expect(screen.getByText(/Le enviamos un enlace a maria@empresa\.cr/)).toBeInTheDocument()
    // El nombre viaja limpio: es el que Platform guarda al dar de alta al usuario.
    expect(auth.log).toContain('signUp:maria@empresa.cr:María Rojas')
  })

  it('con sesión inmediata va a crear la organización (pantalla 3)', async () => {
    const auth = fakeAuth({ signedIn: false, signUp: { ok: true, next: 'signedIn' } })
    const { router } = renderApp('/registro', { orgId: null, auth: auth.port })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/organizaciones/nueva'))
  })

  it('correo con cuenta: lo dice y ofrece ingresar o recuperar; contraseña débil cae en su campo', async () => {
    const exists = fakeAuth({ signedIn: false, signUp: { ok: false, reason: 'exists' } })
    const { unmount } = renderApp('/registro', { auth: exists.port })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByText(/Ya existe una cuenta con ese correo/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '¿Olvidó su contraseña?' })).toHaveAttribute('href', '/recuperar')
    unmount()

    const weak = fakeAuth({ signedIn: false, signUp: { ok: false, reason: 'weak-password' } })
    renderApp('/registro', { auth: weak.port })
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear cuenta' }))
    expect(await screen.findByText(/muy fácil de adivinar/)).toBeInTheDocument()
  })

  it('la pantalla 1 enlaza al registro y avisa cuando el correo quedó confirmado', async () => {
    renderApp('/ingresar?cuenta=confirmada', { auth: fakeAuth({ signedIn: false }).port })
    expect(
      await screen.findByText('Su correo quedó confirmado. Ingrese con su contraseña.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Crear cuenta' })).toHaveAttribute('href', '/registro')
  })
})

describe('registro con Supabase', () => {
  type Auth = Parameters<typeof fromClient>[0]['auth']
  const port = (signUp: unknown) => fromClient({ auth: { signUp } as unknown as Auth })

  it('manda el nombre en user_metadata y vuelve a la pantalla 1 al confirmar', async () => {
    const signUp = vi.fn(async () => ({ data: { session: null, user: {} }, error: null }))
    await expect(
      port(signUp).signUp({ fullName: 'María Rojas', email: 'm@e.cr', password: 'segura-2026' }),
    ).resolves.toEqual({ ok: true, next: 'confirmEmail' })
    expect(signUp).toHaveBeenCalledWith({
      email: 'm@e.cr',
      password: 'segura-2026',
      options: {
        data: { full_name: 'María Rojas' },
        emailRedirectTo: `${window.location.origin}/ingresar?cuenta=confirmada`,
      },
    })
  })

  it('con sesión en la respuesta (sin confirmación) entra directo', async () => {
    const signUp = async () => ({ data: { session: { access_token: 't' }, user: {} }, error: null })
    await expect(
      port(signUp).signUp({ fullName: 'A B', email: 'a@b.cr', password: 'x'.repeat(8) }),
    ).resolves.toEqual({ ok: true, next: 'signedIn' })
  })

  it('traduce los códigos de error de Supabase Auth', async () => {
    const cases: [unknown, string][] = [
      [{ code: 'user_already_exists', status: 422 }, 'exists'],
      [{ code: 'weak_password', status: 422 }, 'weak-password'],
      [{ code: 'signup_disabled', status: 422 }, 'disabled'],
      [{ code: 'over_email_send_rate_limit', status: 429 }, 'rate-limited'],
      [{ status: 500 }, 'unavailable'],
    ]
    for (const [error, reason] of cases) {
      const signUp = async () => ({ data: { session: null, user: null }, error })
      await expect(
        port(signUp).signUp({ fullName: 'A B', email: 'a@b.cr', password: 'x'.repeat(8) }),
      ).resolves.toEqual({ ok: false, reason })
    }
  })
})
