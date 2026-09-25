import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockBilling } from '@/shared/api/mock/billing'
import type { DataSource } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'
import { renderApp } from '@/test/renderApp'

afterEach(() => resetMockBilling())

const unavailable = () =>
  new ApiError({
    status: 503,
    type: 'urn:rdl:portal-gateway:problem:upstream-not-configured',
    title: 'x',
    correlationId: 'cid',
  })

function withSource(over: Partial<DataSource>): DataSource {
  return { ...mockDataSource, ...over }
}

describe('pantalla 7 · clientes', () => {
  it('lista los activos con su saldo por moneda y abre la ficha', async () => {
    const { router } = renderApp('/clientes')
    const table = await screen.findByRole('table', { name: 'Clientes' })
    expect(await within(table).findByText('Café Monteazul S.A.')).toBeInTheDocument()
    expect(within(table).queryByText('Panadería La Espiga')).not.toBeInTheDocument()
    // Hotel Brisas solo tiene saldo en dólares: no se mezcla con colones.
    expect(await within(table).findByText('US$2 730,00')).toBeInTheDocument()
    await userEvent.setup().click(within(table).getByText('Soluciones Ibis S.R.L.'))
    await waitFor(() => expect(router.state.location.pathname).toBe('/clientes/c9'))
  })

  it('filtra inactivos y busca', async () => {
    renderApp('/clientes')
    const user = userEvent.setup()
    await screen.findByText('Café Monteazul S.A.')
    await user.click(screen.getByRole('radio', { name: 'Inactivos' }))
    expect(await screen.findByText('Panadería La Espiga')).toBeInTheDocument()
    await user.click(screen.getByRole('radio', { name: 'Todos' }))
    await user.type(screen.getByRole('searchbox', { name: 'Buscar por nombre o identificación' }), 'zzz')
    expect(
      await screen.findByText('Ningún cliente coincide con la búsqueda o el filtro.'),
    ).toBeInTheDocument()
  })

  it('sin saldos de Receivables, la columna dice «No disponible» y la tabla sigue', async () => {
    const source = withSource({
      receivables: { ...mockDataSource.receivables, balancesByCustomer: () => Promise.reject(unavailable()) },
    })
    renderApp('/clientes', { source })
    expect(await screen.findByText('Café Monteazul S.A.')).toBeInTheDocument()
    expect((await screen.findAllByText('No disponible')).length).toBeGreaterThan(0)
  })

  it('sin clientes invita a crear el primero; solo lectura no ve el botón', async () => {
    const source = withSource({
      customers: { ...mockDataSource.customers, list: async () => ({ items: [], nextCursor: null }) },
    })
    renderApp('/clientes', { source, orgId: 'tp' })
    expect(await screen.findByText('Todavía no hay clientes')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Nuevo cliente|Crear cliente/ })).not.toBeInTheDocument()
  })
})

describe('pantalla 8 · crear y editar cliente', () => {
  it('valida y crea; vuelve a la ficha con un aviso', async () => {
    const { router } = renderApp('/clientes/nuevo')
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Guardar cliente' }))
    expect(screen.getByText('Escriba la razón social.')).toBeInTheDocument()
    expect(screen.getByText('Escriba un correo válido, por ejemplo compras@empresa.cr.')).toBeInTheDocument()

    await user.type(screen.getByLabelText(/Número de identificación/), '3101777888')
    await user.type(screen.getByLabelText(/Razón social/), 'Vivero Los Lirios S.A.')
    await user.type(screen.getByLabelText(/^Correo/), 'ventas@lirios.cr')
    await user.click(screen.getByRole('button', { name: 'Guardar cliente' }))
    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/clientes\/c\d+$/))
    expect(await screen.findByText('Cliente creado')).toBeInTheDocument()
    expect(
      await screen.findByRole('heading', { level: 1, name: /Vivero Los Lirios S.A./ }),
    ).toBeInTheDocument()
  })

  it('identificación repetida: enlaza al cliente existente', async () => {
    renderApp('/clientes/nuevo')
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText(/Número de identificación/), '3-101-900288')
    await user.type(screen.getByLabelText(/Razón social/), 'Otro')
    await user.type(screen.getByLabelText(/^Correo/), 'x@y.cr')
    await user.click(screen.getByRole('button', { name: 'Guardar cliente' }))
    const link = await screen.findByRole('link', { name: 'Café Monteazul S.A.' })
    expect(link).toHaveAttribute('href', '/clientes/c2')
  })

  it('editar: la identificación no se toca y solo viaja lo que cambió', async () => {
    const update = vi.fn(mockDataSource.customers.update)
    const source = withSource({ customers: { ...mockDataSource.customers, update } })
    renderApp('/clientes/c1/editar', { source })
    const user = userEvent.setup()
    expect(
      await screen.findByText(
        'Editar un cliente no cambia las facturas ya emitidas. Los cambios se usan en los documentos nuevos.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByLabelText(/Número de identificación/)).toBeDisabled()
    await user.clear(screen.getByLabelText(/Teléfono/))
    await user.type(screen.getByLabelText(/Teléfono/), '2222-9999')
    await user.click(screen.getByRole('button', { name: 'Guardar cliente' }))
    await waitFor(() => expect(update).toHaveBeenCalledWith('c1', { phone: '2222-9999' }))
  })

  it('un error del servidor conserva lo escrito y muestra el código', async () => {
    const source = withSource({
      customers: {
        ...mockDataSource.customers,
        create: () =>
          Promise.reject(
            new ApiError({
              status: 500,
              type: 'urn:rdl:billing:problem:internal',
              title: 'x',
              correlationId: 'cid-9',
            }),
          ),
      },
    })
    renderApp('/clientes/nuevo', { source })
    const user = userEvent.setup()
    await user.type(await screen.findByLabelText(/Número de identificación/), '3101777888')
    await user.type(screen.getByLabelText(/Razón social/), 'Vivero Los Lirios S.A.')
    await user.type(screen.getByLabelText(/^Correo/), 'ventas@lirios.cr')
    await user.click(screen.getByRole('button', { name: 'Guardar cliente' }))
    expect(
      await screen.findByText(
        'No pudimos guardar el cliente. Sus datos siguen en el formulario; intente de nuevo.',
      ),
    ).toBeInTheDocument()
    expect(screen.getByText(/cid-9/)).toBeInTheDocument()
    expect(screen.getByLabelText(/Razón social/)).toHaveValue('Vivero Los Lirios S.A.')
  })
})

describe('pantalla 9 · ficha del cliente', () => {
  it('muestra saldos, vencidas y facturado; cambia de pestaña', async () => {
    renderApp('/clientes/c9')
    const user = userEvent.setup()
    expect(
      await screen.findByRole('heading', { level: 1, name: /Soluciones Ibis S.R.L./ }),
    ).toBeInTheDocument()
    expect(await screen.findByText('₡188 000,00')).toBeInTheDocument()
    expect(screen.getByText('3 cuentas abiertas')).toBeInTheDocument()
    expect(await screen.findByText('FAC-0000034')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: 'Datos' }))
    expect(await screen.findByText('San José, Montes de Oca, San Pedro.')).toBeInTheDocument()
  })

  it('si Receivables falla, datos parciales: el resto de la ficha sigue', async () => {
    const source = withSource({
      receivables: {
        ...mockDataSource.receivables,
        byCustomer: () => Promise.reject(unavailable()),
        paymentsByCustomer: () => Promise.reject(unavailable()),
      },
    })
    renderApp('/clientes/c9', { source })
    expect(
      await screen.findByText(
        'No pudimos obtener las cuentas por cobrar de este cliente; el resto de la información está al día.',
      ),
    ).toBeInTheDocument()
    expect(screen.getAllByText('No disponible')).toHaveLength(2)
    expect(await screen.findByText('FAC-0000034')).toBeInTheDocument()
  })

  it('un cliente de otra organización (404) muestra el error con su código', async () => {
    renderApp('/clientes/no-existe')
    expect(await screen.findByRole('button', { name: 'Reintentar' })).toBeInTheDocument()
  })
})
