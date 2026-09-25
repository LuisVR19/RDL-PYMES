import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import type { DataSource } from '@/shared/api/ports'
import { fakeAuth } from '@/test/fakeAuth'
import { renderApp } from '@/test/renderApp'

// El selector de organización existe dos veces (escritorio y móvil): se usa el primero.
const orgButton = async (name: RegExp) => (await screen.findAllByRole('button', { name }))[0]!
const pickOrg = async (name: RegExp) =>
  (await screen.findAllByRole('button', { name })).find((b) => b.closest('li'))!

describe('acceso', () => {
  it('sin sesión, el armazón manda a la pantalla 1 recordando la ruta', async () => {
    const { router } = renderApp('/clientes?buscar=x', { auth: fakeAuth({ signedIn: false }).port })
    await waitFor(() => expect(router.state.location.pathname).toBe('/ingresar'))
    expect(router.state.location.search).toBe(`?volver=${encodeURIComponent('/clientes?buscar=x')}`)
    expect(await screen.findByText('/clientes?buscar=x')).toBeInTheDocument()
  })

  it('valida antes de llamar y muestra el mensaje genérico ante credenciales malas', async () => {
    const auth = fakeAuth({ signedIn: false })
    const signIn = vi.spyOn(auth.port, 'signIn')
    renderApp('/ingresar', { auth: auth.port })
    const user = userEvent.setup()

    await user.click(await screen.findByRole('button', { name: 'Ingresar' }))
    expect(screen.getByText('Escriba un correo válido.')).toBeInTheDocument()
    expect(screen.getByText('Escriba su contraseña.')).toBeInTheDocument()
    expect(signIn).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('Correo'), 'a@b.cr')
    await user.type(screen.getByLabelText('Contraseña'), 'mala')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    expect(
      await screen.findByText('El correo o la contraseña no son correctos. Revíselos e intente de nuevo.'),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('Contraseña')).toHaveValue('')
  })

  it('al ingresar vuelve a la ruta que pedía', async () => {
    const { router } = renderApp(`/ingresar?volver=${encodeURIComponent('/clientes')}`, {
      auth: fakeAuth({ signedIn: false }).port,
    })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Correo'), 'a@b.cr')
    await user.type(screen.getByLabelText('Contraseña'), 'secreta')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/clientes'))
  })

  it('un ?volver= externo no saca al usuario del portal', async () => {
    const { router } = renderApp(`/ingresar?volver=${encodeURIComponent('//evil.example/x')}`, {
      auth: fakeAuth({ signedIn: false }).port,
    })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText('Correo'), 'a@b.cr')
    await user.type(screen.getByLabelText('Contraseña'), 'secreta')
    await user.click(screen.getByRole('button', { name: 'Ingresar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })

  it('sin organizaciones, manda a crear la primera (pantalla 3)', async () => {
    const source: DataSource = {
      ...mockDataSource,
      session: {
        ...mockDataSource.session,
        organizations: async () => ({ items: [], activeOrganizationId: null }),
      },
    }
    const { router } = renderApp('/', { orgId: null, source, auth: fakeAuth({ signedIn: true }).port })
    await waitFor(() => expect(router.state.location.pathname).toBe('/organizaciones/nueva'))
  })
})

describe('cambio de organización', () => {
  it('PUT → refresco del token → caché vaciada → organización nueva', async () => {
    const auth = fakeAuth({ signedIn: true })
    const source: DataSource = {
      ...mockDataSource,
      session: {
        ...mockDataSource.session,
        activateOrganization: async (id) => {
          auth.log.push(`activate:${id}`)
        },
      },
    }
    const { queryClient } = renderApp('/', { orgId: null, source, auth: auth.port })
    const user = userEvent.setup()
    const current = await orgButton(/Comercial Los Almendros/)
    // Un dato de la organización anterior en caché.
    queryClient.setQueryData(['clientes', 'ca'], ['de la anterior'])

    await user.click(current)
    await user.click(await pickOrg(/Café Monteazul/))

    await waitFor(() => expect(auth.log).toEqual(['activate:cm', 'refresh']))
    expect(queryClient.getQueryData(['clientes', 'ca'])).toBeUndefined()
    expect(await orgButton(/Café Monteazul/)).toBeInTheDocument()
  })

  it('si el refresco falla, se queda en la organización anterior', async () => {
    const auth = fakeAuth({ signedIn: true })
    auth.port.refresh = async () => {
      throw new Error('refresh vencido')
    }
    renderApp('/', { orgId: null, auth: auth.port })
    const user = userEvent.setup()
    await user.click(await orgButton(/Comercial Los Almendros/))
    await user.click(await pickOrg(/Café Monteazul/))
    expect(
      await screen.findByText(
        'No pudimos cambiar de organización. Intente de nuevo; mientras tanto sigue en Comercial Los Almendros S.A.',
      ),
    ).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('main')).toHaveAttribute('aria-busy', 'false'))
    expect(await orgButton(/Comercial Los Almendros/)).toBeInTheDocument()
    expect(
      screen.queryAllByRole('button', { name: /Café Monteazul/ }).filter((b) => !b.closest('li')),
    ).toHaveLength(0)
  })
})
