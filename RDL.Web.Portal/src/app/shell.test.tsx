import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { renderApp } from '@/test/renderApp'
import { SCREENS } from './screens'

describe('armazón y rutas', () => {
  it('el administrador ve todas las secciones del menú y el ambiente de pruebas', async () => {
    renderApp('/')
    const nav = await screen.findByRole('navigation', { name: 'Menú' })
    await within(nav).findByRole('link', { name: /Inicio/ }) // espera a que carguen las organizaciones
    for (const label of ['Inicio', 'Clientes', 'Bandeja', 'Cuentas por cobrar', 'Usuarios y roles']) {
      expect(within(nav).getByRole('link', { name: new RegExp(label) })).toBeInTheDocument()
    }
    expect(screen.getAllByText('Ambiente de pruebas').length).toBeGreaterThan(0)
  })

  it('el menú se filtra por rol: el facturador no ve cobranza ni administración', async () => {
    renderApp('/', { orgId: 'si' })
    const nav = await screen.findByRole('navigation', { name: 'Menú' })
    expect(await within(nav).findByRole('link', { name: /Documentos/ })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: /Cuentas por cobrar/ })).not.toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: /Usuarios y roles/ })).not.toBeInTheDocument()
  })

  it('una ruta sin permiso muestra 403 con el rol actual', async () => {
    renderApp('/admin/usuarios', { orgId: 'tp' })
    expect(await screen.findByRole('heading', { name: 'No tiene acceso a esta sección' })).toBeInTheDocument()
    expect(screen.getByText(/Su rol en Taller Mecánico Los Pinos es Solo lectura/)).toBeInTheDocument()
  })

  it('una ruta desconocida muestra 404', async () => {
    renderApp('/no-existe')
    expect(await screen.findByRole('heading', { name: 'No encontramos esta página' })).toBeInTheDocument()
  })

  it.each(SCREENS.map((s) => [s.path, s]))('%s tiene ruta y marcador provisional', async (_path, s) => {
    renderApp(s.path.replace(/:\w+/g, 'demo'), { orgId: 'fr' }) // propietario: acceso a todo
    expect(
      await screen.findByRole('heading', { level: 2, name: new RegExp(`Pantalla ${s.n} · `) }),
    ).toBeInTheDocument()
  })
})
