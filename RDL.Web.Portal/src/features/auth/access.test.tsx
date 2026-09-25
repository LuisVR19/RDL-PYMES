import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource, resetMockOrganizations } from '@/shared/api/mock'
import type { DataSource } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'
import { expireSession } from '@/shared/auth/expiry'
import { fakeAuth } from '@/test/fakeAuth'
import { renderApp } from '@/test/renderApp'

afterEach(() => resetMockOrganizations())

const problem = (
  status: number,
  code: string,
  extra: Partial<{ errors: { field: string; message: string }[] }> = {},
) =>
  new ApiError({
    status,
    type: `urn:rdl:platform:problem:${code}`,
    title: code,
    correlationId: 'cid-srv',
    ...extra,
  })

function withAccess(
  access: Partial<DataSource['access']>,
  session: Partial<DataSource['session']> = {},
): DataSource {
  return {
    ...mockDataSource,
    access: { ...mockDataSource.access, ...access },
    session: { ...mockDataSource.session, ...session },
  }
}

describe('pantalla 1 · recuperar contraseña', () => {
  it('valida el correo y siempre responde con el mismo mensaje', async () => {
    const auth = fakeAuth({ signedIn: false })
    renderApp('/recuperar', { auth: auth.port })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Enviar enlace' }))
    expect(screen.getByText('Escriba un correo válido.')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Correo'), 'nadie@empresa.cr')
    await user.click(screen.getByRole('button', { name: 'Enviar enlace' }))
    expect(await screen.findByRole('heading', { name: 'Revise su correo' })).toBeInTheDocument()
    expect(auth.log).toContain('reset:nadie@empresa.cr')
  })
})

describe('pantalla 2 · selector de organización', () => {
  it('sin sesión manda a ingresar y vuelve aquí', async () => {
    const { router } = renderApp('/organizaciones', { auth: fakeAuth({ signedIn: false }).port })
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
    expect(router.state.location.search).toBe('?volver=%2Forganizaciones')
  })

  it('lista, busca y elige con el cambio seguro', async () => {
    const auth = fakeAuth({ signedIn: true })
    const source = withAccess(
      {},
      { activateOrganization: async (id) => void auth.log.push(`activate:${id}`) },
    )
    const { router } = renderApp('/organizaciones', { orgId: null, auth: auth.port, source })
    const user = userEvent.setup()

    expect(await screen.findByRole('heading', { name: 'Elija una organización' })).toBeInTheDocument()
    expect(
      await screen.findByText('Hola, María. Puede cambiarla después desde la barra superior.'),
    ).toBeInTheDocument()
    await user.type(screen.getByLabelText('Buscar por nombre o identificación'), 'zzz')
    expect(screen.getByText('Ninguna organización coincide con «zzz».')).toBeInTheDocument()
    await user.clear(screen.getByLabelText('Buscar por nombre o identificación'))
    await user.type(screen.getByLabelText('Buscar por nombre o identificación'), 'monteazul')
    await user.click(screen.getByRole('button', { name: /Café Monteazul/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(auth.log).toEqual(['activate:cm', 'refresh'])
    expect(await screen.findByText('Ahora trabaja en Café Monteazul S.A.')).toBeInTheDocument()
  })

  it('sin organizaciones ofrece crear la primera', async () => {
    const source = withAccess({}, { organizations: async () => ({ items: [], activeOrganizationId: null }) })
    renderApp('/organizaciones', { orgId: null, source })
    expect(await screen.findByText('Todavía no pertenece a ninguna organización')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Crear organización' })).toBeInTheDocument()
  })

  it('si no carga, muestra el error con su código y reintenta', async () => {
    let fail = true
    const source = withAccess(
      {},
      {
        organizations: async () => {
          if (fail) throw problem(503, 'unavailable')
          return mockDataSource.session.organizations()
        },
      },
    )
    renderApp('/organizaciones', { orgId: null, source })
    expect(
      await screen.findByText('No pudimos cargar sus organizaciones', {}, { timeout: 3000 }),
    ).toBeInTheDocument()
    expect(screen.getByText(/cid-srv/)).toBeInTheDocument()
    fail = false
    await userEvent.setup().click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(await screen.findByRole('button', { name: /Café Monteazul/ })).toBeInTheDocument()
  })
})

async function fill(user: ReturnType<typeof userEvent.setup>) {
  await user.type(await screen.findByLabelText(/Razón social/), 'Panadería La Espiga S.A.')
  await user.type(screen.getByLabelText(/Número de identificación/), '3101123456')
  await user.type(screen.getByLabelText(/^Correo/), 'ventas@espiga.cr')
}
describe('pantalla 3 · crear organización', () => {
  it('valida en línea antes de enviar', async () => {
    const create = vi.fn()
    renderApp('/organizaciones/nueva', { source: withAccess({ createOrganization: create }) })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Crear organización' }))
    expect(screen.getByText('Escriba la razón social.')).toBeInTheDocument()
    expect(screen.getByText('Revise el número de identificación.')).toBeInTheDocument()
    expect(screen.getByText('Escriba un correo válido, por ejemplo ventas@empresa.cr.')).toBeInTheDocument()
    expect(create).not.toHaveBeenCalled()
  })

  it('un error del servidor conserva lo escrito; reintentar lo mismo reutiliza la clave', async () => {
    const keys: string[] = []
    const create = vi.fn(async (_input: unknown, key: string) => {
      keys.push(key)
      throw problem(503, 'unavailable')
    })
    renderApp('/organizaciones/nueva', { source: withAccess({ createOrganization: create }) })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    expect(
      await screen.findByText(
        'No pudimos crear la organización. Sus datos siguen en el formulario; intente de nuevo.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByLabelText(/Razón social/)).toHaveValue('Panadería La Espiga S.A.')

    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    await waitFor(() => expect(keys).toHaveLength(2))
    expect(keys[0]).toBe(keys[1])

    await user.type(screen.getByLabelText(/Teléfono/), '2222-3333')
    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    await waitFor(() => expect(keys).toHaveLength(3))
    expect(keys[2]).not.toBe(keys[0])
    expect(create.mock.calls[0]?.[0]).toEqual({
      legalName: 'Panadería La Espiga S.A.',
      identificationTypeCode: '02',
      identificationNumber: '3101123456',
      email: 'ventas@espiga.cr',
      timezone: 'America/Costa_Rica',
    })
  })

  it('identificación repetida (409) y errores por campo (422) caen en su campo', async () => {
    let next: ApiError = problem(409, 'conflict')
    const create = vi.fn(async () => {
      throw next
    })
    renderApp('/organizaciones/nueva', { source: withAccess({ createOrganization: create }) })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    expect(await screen.findByText('Ya existe una organización con esa identificación.')).toBeInTheDocument()

    next = problem(422, 'validation', { errors: [{ field: 'email', message: 'Dominio no válido.' }] })
    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    expect(await screen.findByText('Dominio no válido.')).toBeInTheDocument()
  })

  it('al crear, la activa con el cambio seguro y entra como propietario', async () => {
    const auth = fakeAuth({ signedIn: true })
    const source = withAccess(
      {},
      {
        activateOrganization: async (id) => {
          auth.log.push(`activate:${id}`)
        },
      },
    )
    const { router } = renderApp('/organizaciones/nueva', { orgId: null, auth: auth.port, source })
    const user = userEvent.setup()
    await fill(user)
    await user.click(screen.getByRole('button', { name: 'Crear organización' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(auth.log).toEqual([expect.stringMatching(/^activate:nueva-/), 'refresh'])
    expect(await screen.findByText('Organización creada')).toBeInTheDocument()
    expect((await screen.findAllByRole('button', { name: /Panadería La Espiga/ }))[0]).toBeInTheDocument()
  })
})

describe('pantalla 4 · aceptar invitación', () => {
  it('sin sesión pide ingresar y vuelve a la invitación', async () => {
    const { router } = renderApp('/invitacion/abc', { auth: fakeAuth({ signedIn: false }).port })
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
    expect(router.state.location.search).toBe('?volver=%2Finvitacion%2Fabc')
  })

  it('aceptar entra a la organización con su rol', async () => {
    const auth = fakeAuth({ signedIn: true })
    const { router } = renderApp('/invitacion/valida', { orgId: null, auth: auth.port })
    const user = userEvent.setup()
    expect(await screen.findByText('a@b.cr')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Aceptar e ingresar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(await screen.findByText('Su rol es Cobrador.')).toBeInTheDocument()
    expect(auth.log).toContain('refresh')
  })

  it.each([
    ['vencida', 'Esta invitación venció', 'Ir a mis organizaciones'],
    ['no-pendiente', 'Esta invitación ya no está disponible', 'Ir a mis organizaciones'],
    ['ajena', 'Esta invitación no es para esta cuenta', 'Cerrar sesión y entrar con otro correo'],
  ])('%s → «%s»', async (token, title, action) => {
    renderApp(`/invitacion/${token}`, { orgId: null, auth: fakeAuth({ signedIn: true }).port })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Aceptar e ingresar' }))
    expect(await screen.findByRole('heading', { name: title })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: action })).toBeInTheDocument()
  })

  it('para otro correo: cierra sesión y vuelve a la invitación después de ingresar', async () => {
    const auth = fakeAuth({ signedIn: true })
    const { router } = renderApp('/invitacion/ajena', { orgId: null, auth: auth.port })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Aceptar e ingresar' }))
    expect(await screen.findByText(/Usted inició sesión como a@b\.cr/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cerrar sesión y entrar con otro correo' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
    expect(router.state.location.search).toBe('?volver=%2Finvitacion%2Fajena')
    expect(auth.log).toContain('signOut')
  })
})

describe('pantalla 33 · mi perfil', () => {
  it('muestra quién es, sus organizaciones y cierra sesión', async () => {
    const auth = fakeAuth({ signedIn: true })
    const { router } = renderApp('/perfil', { auth: auth.port })
    const user = userEvent.setup()
    expect(await screen.findByRole('heading', { level: 1, name: 'Mi perfil' })).toBeInTheDocument()
    expect(screen.getByText('Por ahora el nombre no se puede cambiar desde el portal.')).toBeInTheDocument()
    expect(screen.getByText('✓ Activa · Administrador')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cerrar sesión' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
  })
})

describe('pantalla 35 · sesión vencida', () => {
  it('pide la contraseña encima de la página y sigue en el mismo lugar', async () => {
    const auth = fakeAuth({ signedIn: true, password: 'secreta' })
    const { router } = renderApp('/perfil', { auth: auth.port })
    const user = userEvent.setup()
    await screen.findByRole('heading', { level: 1, name: 'Mi perfil' })

    act(() => expireSession())
    const dialog = await screen.findByRole('dialog', { name: 'Su sesión venció' })
    // La página sigue ahí, detrás del diálogo.
    expect(screen.getByRole('heading', { level: 1, hidden: true, name: 'Mi perfil' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('Contraseña'), 'mala')
    await user.click(screen.getByRole('button', { name: 'Continuar' }))
    expect(await screen.findByText(/El correo o la contraseña no son correctos/)).toBeInTheDocument()

    await user.type(screen.getByLabelText('Contraseña'), 'secreta')
    await user.click(screen.getByRole('button', { name: 'Continuar' }))
    await waitFor(() => expect(dialog).not.toBeInTheDocument())
    expect(router.state.location.pathname).toBe('/perfil')
    expect(auth.log.filter((l) => l.startsWith('signIn'))).toEqual(['signIn:a@b.cr', 'signIn:a@b.cr'])
  })

  it('si Supabase pierde la sesión solo, también es el diálogo y no una expulsión', async () => {
    const auth = fakeAuth({ signedIn: true })
    const { router } = renderApp('/perfil', { auth: auth.port })
    await screen.findByRole('heading', { level: 1, name: 'Mi perfil' })
    act(() => auth.lose())
    expect(await screen.findByRole('dialog', { name: 'Su sesión venció' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/perfil')
  })

  it('«Entrar con otra cuenta» cierra la sesión y recuerda la ruta', async () => {
    const auth = fakeAuth({ signedIn: true })
    const { router } = renderApp('/perfil', { auth: auth.port })
    const user = userEvent.setup()
    await screen.findByRole('heading', { level: 1, name: 'Mi perfil' })
    act(() => expireSession())
    await user.click(await screen.findByRole('button', { name: 'Entrar con otra cuenta' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
    expect(router.state.location.search).toBe('?volver=%2Fperfil')
    expect(auth.log).toContain('signOut')
  })
})
