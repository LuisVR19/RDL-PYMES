import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockInvoices } from '@/shared/api/mock/invoices'
import type { DataSource } from '@/shared/api/ports'
import { renderApp } from '@/test/renderApp'

afterEach(() => resetMockInvoices())

describe('pantalla 12 · documentos', () => {
  it('lista con Hacienda y saldo por fila; «no disponible» no rompe la tabla', async () => {
    renderApp('/documentos')
    const table = await screen.findByRole('table', { name: 'Documentos' })
    expect(await within(table).findByText('FAC-0000040')).toBeInTheDocument()
    expect(within(table).getByText('Estado no disponible')).toBeInTheDocument()
    expect(within(table).getByText('Sin documento')).toBeInTheDocument()
    expect(within(table).getByText('Requiere corrección')).toBeInTheDocument()
    // El borrador no tiene snapshot: el nombre sale de la lista de clientes.
    expect(within(table).getByText('Ferretería El Roble S.A.')).toBeInTheDocument()
    expect(screen.getByText('Saldo en esta página:')).toBeInTheDocument()
  })

  it('las pestañas filtran por tipo y se guardan en la URL', async () => {
    const list = vi.fn(mockDataSource.invoices.list)
    const source: DataSource = { ...mockDataSource, invoices: { ...mockDataSource.invoices, list } }
    const { router } = renderApp('/documentos', { source })
    const user = userEvent.setup()
    await screen.findByText('FAC-0000040')
    await user.click(screen.getByRole('tab', { name: 'Notas de crédito' }))
    await waitFor(() => expect(router.state.location.search).toBe('?tipo=credit_note'))
    expect(await screen.findByText('NC-0000006')).toBeInTheDocument()
    expect(screen.queryByText('FAC-0000040')).not.toBeInTheDocument()
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ documentType: 'credit_note' }))
  })

  it('un borrador abre su edición; una emitida, su detalle', async () => {
    const { router } = renderApp('/documentos')
    const user = userEvent.setup()
    await user.click(await screen.findByText('FAC-0000034'))
    await waitFor(() => expect(router.state.location.pathname).toBe('/facturas/i34'))
  })
})

describe('pantalla 15 · detalle de la factura', () => {
  it('tres cifras de tres dominios, líneas y totales del servidor, historial', async () => {
    renderApp('/facturas/i34')
    expect(await screen.findByRole('heading', { level: 1, name: 'FAC-0000034' })).toBeInTheDocument()
    expect(screen.getAllByText('₡113 000,00').length).toBeGreaterThan(0)
    expect(await screen.findByText('Aceptada')).toBeInTheDocument()
    expect(screen.getByText('₡63 000,00')).toBeInTheDocument()
    expect(screen.getByText('Pago parcial')).toBeInTheDocument()
    expect(screen.getByText('₡100 000,00', { selector: 'dd' })).toBeInTheDocument()
    expect(await screen.findByText('Factura emitida')).toBeInTheDocument()
    expect(screen.getByText('Hacienda: Aceptada')).toBeInTheDocument()
  })

  it('rechazada: motivo y acción de nota de crédito', async () => {
    const { router } = renderApp('/facturas/i38')
    const user = userEvent.setup()
    expect(await screen.findByText('Hacienda rechazó el documento')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Crear nota de crédito' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/facturas/i38/nota-credito'))
  })

  it('si Hacienda no responde: datos parciales, el resto sigue', async () => {
    renderApp('/facturas/i36')
    expect(await screen.findByText('Datos parciales')).toBeInTheDocument()
    expect(screen.getByText('No pudimos consultarlo')).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 2, name: 'Líneas' })).toBeInTheDocument()
  })

  it('solo lectura (contador): sin notas ni anular', async () => {
    renderApp('/facturas/i34', { orgId: 'cm' })
    await screen.findByRole('heading', { level: 1, name: 'FAC-0000034' })
    expect(screen.queryByRole('button', { name: 'Nota de crédito' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Anular…' })).not.toBeInTheDocument()
  })

  it('un facturador puede hacer notas pero no anular (el contrato lo reserva a owner y admin)', async () => {
    renderApp('/facturas/i34', { orgId: 'si' })
    expect(await screen.findByRole('button', { name: 'Nota de crédito' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Anular…' })).not.toBeInTheDocument()
  })

  it('un borrador no tiene detalle: redirige a su edición', async () => {
    const { router } = renderApp('/facturas/d12')
    await waitFor(() => expect(router.state.location.pathname).toBe('/facturas/d12/editar'))
  })
})

describe('pantalla 17 · anular factura', () => {
  it('exige motivo de 10 caracteres, explica el ajuste y anula', async () => {
    const cancel = vi.fn(mockDataSource.invoices.cancel)
    const source: DataSource = { ...mockDataSource, invoices: { ...mockDataSource.invoices, cancel } }
    renderApp('/facturas/i34', { source })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Anular…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Anular FAC-0000034' })
    expect(within(dialog).getByText(/el saldo de ₡63 000,00 pasa a ₡0,00/)).toBeInTheDocument()
    const confirm = within(dialog).getByRole('button', { name: 'Anular factura' })
    expect(confirm).toBeDisabled()
    await user.type(within(dialog).getByLabelText(/Motivo/), 'corto')
    expect(within(dialog).getByText('Obligatorio · mínimo 10 caracteres (5/10)')).toBeInTheDocument()
    await user.type(within(dialog).getByLabelText(/Motivo/), ' pero ya no')
    expect(within(dialog).getByText('✓ Motivo suficiente')).toBeInTheDocument()
    await user.click(confirm)
    expect(await screen.findByText('FAC-0000034 anulada')).toBeInTheDocument()
    expect(cancel).toHaveBeenCalledWith('i34', 'corto pero ya no', expect.any(String))
    // En el aviso y en el historial.
    expect(await screen.findAllByText('Factura anulada')).toHaveLength(2)
  })
})
