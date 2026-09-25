import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockProducts } from '@/shared/api/mock/products'
import type { DataSource } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'
import { renderApp } from '@/test/renderApp'

afterEach(() => resetMockProducts())

const unavailable = () =>
  new ApiError({
    status: 503,
    type: 'urn:rdl:portal-gateway:problem:upstream-not-configured',
    title: 'x',
    correlationId: 'cid',
  })

/** Como hoy con el gateway: E-Invoice no existe, así que ningún catálogo responde. */
const noCatalogs: DataSource['catalogs'] = {
  searchCabys: () => Promise.reject(unavailable()),
  unitsOfMeasure: () => Promise.reject(unavailable()),
  taxOptions: () => Promise.reject(unavailable()),
}

describe('pantalla 10 · productos y servicios', () => {
  it('lista con precio por unidad e impuesto; abre la edición', async () => {
    const { router } = renderApp('/productos')
    const table = await screen.findByRole('table', { name: 'Productos y servicios' })
    expect(await within(table).findByText('₡18 500,00 / Gal')).toBeInTheDocument()
    expect(within(table).getByText('US$45,00 / h')).toBeInTheDocument()
    expect(within(table).getAllByText('IVA 13 %').length).toBeGreaterThan(0)
    await userEvent.setup().click(within(table).getByText('Pintura acrílica blanca, galón'))
    await waitFor(() => expect(router.state.location.pathname).toBe('/productos/p2/editar'))
  })

  it('sin catálogo de impuestos muestra los códigos, no un nombre inventado', async () => {
    renderApp('/productos', { source: { ...mockDataSource, catalogs: noCatalogs } })
    const table = await screen.findByRole('table', { name: 'Productos y servicios' })
    expect((await within(table).findAllByText('01/08')).length).toBeGreaterThan(0)
  })

  it('solo lectura: sin botón de crear y las filas no abren edición', async () => {
    const { router } = renderApp('/productos', { orgId: 'tp' })
    const table = await screen.findByRole('table', { name: 'Productos y servicios' })
    expect(screen.queryByRole('button', { name: 'Nuevo producto' })).not.toBeInTheDocument()
    await userEvent.setup().click(await within(table).findByText('Tornillo para madera 2"'))
    expect(router.state.location.pathname).toBe('/productos')
  })
})

describe('pantalla 11 · crear y editar producto', () => {
  it('valida, elige CABYS en el buscador y crea', async () => {
    const create = vi.fn(mockDataSource.products.create)
    const { router } = renderApp('/productos/nuevo', {
      source: { ...mockDataSource, products: { ...mockDataSource.products, create } },
    })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Guardar producto' }))
    expect(screen.getByText('Escriba un código.')).toBeInTheDocument()
    expect(screen.getByText('Elija un código CABYS.')).toBeInTheDocument()
    expect(screen.getByText('Escriba un precio mayor que 0.')).toBeInTheDocument()

    await user.type(screen.getByLabelText(/Código interno/), 'VIV-001')
    await user.click(screen.getByRole('radio', { name: 'Servicio' }))
    await user.type(screen.getByLabelText(/Descripción/), 'Mantenimiento de jardín')
    await user.click(screen.getByRole('button', { name: 'Buscar CABYS' }))
    const dialog = await screen.findByRole('dialog', { name: 'Buscar código CABYS' })
    await user.type(within(dialog).getByLabelText('Buscar'), 'manten')
    await user.click(await within(dialog).findByRole('button', { name: /Servicios de mantenimiento/ }))
    expect(await screen.findByText('Servicios de mantenimiento y reparación')).toBeInTheDocument()
    await user.type(screen.getByRole('textbox', { name: /^Precio/ }), '12 500,5')
    await user.click(screen.getByRole('button', { name: 'Guardar producto' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/productos'))
    expect(create.mock.calls[0]?.[0]).toEqual({
      code: 'VIV-001',
      description: 'Mantenimiento de jardín',
      cabysCode: '0000000000140',
      unitOfMeasureCode: 'Unid',
      unitPrice: '12500.5',
      currency: 'CRC',
      isService: true,
      taxes: [],
    })
    expect(await screen.findByText('Producto creado')).toBeInTheDocument()
  })

  it('código repetido (409) cae en su campo', async () => {
    renderApp('/productos/p1/editar')
    const user = userEvent.setup()
    const code = await screen.findByLabelText(/Código interno/)
    await user.clear(code)
    await user.type(code, 'PIN-020')
    await user.click(screen.getByRole('button', { name: 'Guardar producto' }))
    expect(await screen.findByText('Ya existe un producto con este código.')).toBeInTheDocument()
  })

  it('editar solo manda lo que cambió y conserva la precisión del precio', async () => {
    const update = vi.fn(mockDataSource.products.update)
    renderApp('/productos/p1/editar', {
      source: { ...mockDataSource, products: { ...mockDataSource.products, update } },
    })
    const user = userEvent.setup()
    expect(await screen.findByText('Los cambios no afectan las facturas ya emitidas.')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('Impuesto'), 'IVA 4 %')
    await user.click(screen.getByRole('button', { name: 'Guardar producto' }))
    await waitFor(() =>
      expect(update).toHaveBeenCalledWith('p1', { taxes: [{ taxTypeCode: '01', taxRateCode: '04' }] }),
    )
  })

  it('sin catálogos: unidad como código, sin impuesto y CABYS a mano', async () => {
    const create = vi.fn(mockDataSource.products.create)
    renderApp('/productos/nuevo', {
      source: { ...mockDataSource, catalogs: noCatalogs, products: { ...mockDataSource.products, create } },
    })
    const user = userEvent.setup()
    expect(
      await screen.findByText(
        'Las tarifas de impuesto todavía no están disponibles. Por ahora el producto se guarda sin impuesto.',
      ),
    ).toBeInTheDocument()
    await user.type(screen.getByLabelText(/Código interno/), 'X-1')
    await user.type(screen.getByLabelText(/Descripción/), 'Algo')
    await user.clear(screen.getByLabelText(/Código de la unidad de medida/))
    await user.type(screen.getByLabelText(/Código de la unidad de medida/), 'Sp')
    await user.click(screen.getByRole('button', { name: 'Buscar CABYS' }))
    const dialog = await screen.findByRole('dialog', { name: 'Buscar código CABYS' })
    await user.type(within(dialog).getByLabelText('Buscar'), 'tornillo')
    await user.type(await within(dialog).findByLabelText('Código CABYS de 13 dígitos'), '123')
    await user.click(within(dialog).getByRole('button', { name: 'Usar este código' }))
    expect(within(dialog).getByText('El código CABYS tiene 13 dígitos.')).toBeInTheDocument()
    await user.clear(within(dialog).getByLabelText('Código CABYS de 13 dígitos'))
    await user.type(within(dialog).getByLabelText('Código CABYS de 13 dígitos'), '8314100000100')
    await user.click(within(dialog).getByRole('button', { name: 'Usar este código' }))
    await user.type(screen.getByRole('textbox', { name: /^Precio/ }), '1000')
    await user.click(screen.getByRole('button', { name: 'Guardar producto' }))
    await waitFor(() =>
      expect(create.mock.calls[0]?.[0]).toMatchObject({
        cabysCode: '8314100000100',
        unitOfMeasureCode: 'Sp',
        taxes: [],
      }),
    )
  })
})
