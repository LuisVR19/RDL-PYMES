import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CUSTOMERS } from '@/mocks/billing'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockBilling } from '@/shared/api/mock/billing'
import { mockInvoices, resetMockInvoices } from '@/shared/api/mock/invoices'
import { resetMockProducts } from '@/shared/api/mock/products'
import type { DataSource } from '@/shared/api/ports'
import { setScenario } from '@/shared/api/scenario'
import { ApiError } from '@/shared/api/types'
import { renderApp } from '@/test/renderApp'

afterEach(() => {
  resetMockInvoices()
  resetMockBilling()
  resetMockProducts()
})

// El recálculo automático espera una pausa de 700 ms tras el último cambio.
const SLOW = { timeout: 3000 }

async function pickCustomer(user: ReturnType<typeof userEvent.setup>, text: string, name: string) {
  await user.type(await screen.findByRole('combobox', { name: /Cliente/ }), text)
  await user.click(await screen.findByRole('option', { name: new RegExp(name) }))
}

async function addProduct(user: ReturnType<typeof userEvent.setup>, description: RegExp) {
  await user.click(screen.getByRole('button', { name: '+ Agregar del catálogo' }))
  await user.click(await screen.findByRole('button', { name: description }))
}

describe('pantalla 13 · borrador de factura', () => {
  it('guardar crea el borrador y los totales llegan del servidor; al editar, recalcula solo', async () => {
    const { router } = renderApp('/facturas/nueva')
    const user = userEvent.setup()
    await screen.findByRole('heading', { level: 1, name: 'Nueva factura' })
    expect(
      screen.getByText('Guarde el borrador para que el sistema calcule los totales.'),
    ).toBeInTheDocument()

    await pickCustomer(user, 'Roble', 'Ferretería El Roble')
    await addProduct(user, /Tornillo para madera/)
    // Antes de guardar, la línea no tiene total: el portal no calcula.
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)

    await user.click(screen.getByRole('button', { name: /Guardar borrador/ }))
    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/facturas\/b\d+\/editar$/))
    expect(await screen.findByRole('heading', { level: 1, name: 'Borrador de factura' })).toBeInTheDocument()
    // 1 × 75 + IVA 13 % (tarifa ilustrativa) = 84,75.
    expect(await screen.findByText(/Calculado por el sistema/)).toBeInTheDocument()
    expect(screen.getAllByText('₡84,75').length).toBeGreaterThan(0)

    const qty = screen.getByRole('textbox', { name: /Cant\. · Línea 1/ })
    await user.clear(qty)
    await user.type(qty, '2')
    expect(screen.getByText('● Cambios sin guardar')).toBeInTheDocument()
    expect((await screen.findAllByText('₡169,50', {}, SLOW)).length).toBeGreaterThan(0)
  })

  it('un descuento sin motivo no se guarda y se explica', async () => {
    const createDraft = vi.fn(mockDataSource.invoices.createDraft)
    const source: DataSource = { ...mockDataSource, invoices: { ...mockDataSource.invoices, createDraft } }
    renderApp('/facturas/nueva', { source })
    const user = userEvent.setup()
    await pickCustomer(user, 'Roble', 'Ferretería El Roble')
    await addProduct(user, /Tornillo para madera/)
    await user.type(screen.getByRole('textbox', { name: /Desc\. · Línea 1/ }), '10')
    expect(screen.getByText('Motivo del descuento')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Emitir factura/ }))
    expect(screen.getByText('Revise estos datos antes de emitir:')).toBeInTheDocument()
    expect(screen.getByText('Línea 1: El motivo es obligatorio cuando hay descuento.')).toBeInTheDocument()
    expect(createDraft).not.toHaveBeenCalled()
  })

  it('sin cliente ni líneas: la lista de lo que falta', async () => {
    renderApp('/facturas/nueva')
    const user = userEvent.setup()
    await screen.findByRole('heading', { level: 1, name: 'Nueva factura' })
    await user.click(screen.getByRole('button', { name: /Emitir factura/ }))
    expect(screen.getByText('Revise estos datos antes de emitir:')).toBeInTheDocument()
    expect(screen.getAllByText('Elija un cliente.').length).toBeGreaterThan(0)
    expect(screen.getByText('Agregue al menos una línea.')).toBeInTheDocument()
  })

  it('llega con el cliente elegido desde su ficha (?cliente=)', async () => {
    const c2 = CUSTOMERS[1]
    renderApp(`/facturas/nueva?cliente=${c2?.id}`)
    expect(await screen.findByText(c2?.legalName ?? '')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cambiar' })).toBeInTheDocument()
  })

  it('alta rápida de cliente desde el buscador', async () => {
    renderApp('/facturas/nueva')
    const user = userEvent.setup()
    await user.type(await screen.findByRole('combobox', { name: /Cliente/ }), 'Taller Nuevo')
    await user.click(await screen.findByRole('option', { name: '+ Crear cliente «Taller Nuevo»' }))
    const drawer = await screen.findByRole('dialog', { name: 'Nuevo cliente' })
    expect(within(drawer).getByRole('textbox', { name: /Razón social/ })).toHaveValue('Taller Nuevo')
    await user.type(within(drawer).getByRole('textbox', { name: /Número de identificación/ }), '3101555444')
    await user.type(within(drawer).getByRole('textbox', { name: /Correo/ }), 'taller@nuevo.example')
    await user.click(within(drawer).getByRole('button', { name: 'Crear y usar en la factura' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByText('Taller Nuevo')).toBeInTheDocument()
  })
})

describe('pantalla 14 · emitir', () => {
  it('confirma con resumen, emite y va al detalle con número', async () => {
    const { router } = renderApp('/facturas/nueva')
    const user = userEvent.setup()
    await pickCustomer(user, 'Roble', 'Ferretería El Roble')
    await addProduct(user, /Tornillo para madera/)
    await user.click(screen.getByRole('button', { name: /Emitir factura/ }))

    const dialog = await screen.findByRole('dialog', { name: '¿Emitir la factura?' })
    expect(within(dialog).getByText('Ferretería El Roble S.A.')).toBeInTheDocument()
    expect(within(dialog).getByText('₡84,75')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Emitir factura' }))

    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/facturas\/b\d+$/))
    expect(await screen.findByRole('heading', { level: 1, name: 'FAC-0000041' })).toBeInTheDocument()
    expect(screen.getByText('Factura FAC-0000041 emitida')).toBeInTheDocument()
  })

  it('un error de red permite reintentar con la misma Idempotency-Key', async () => {
    const issue = vi
      .fn(mockDataSource.invoices.issue)
      .mockRejectedValueOnce(
        new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: 'ref-1' }),
      )
    const source: DataSource = { ...mockDataSource, invoices: { ...mockDataSource.invoices, issue } }
    renderApp('/facturas/nueva', { source })
    const user = userEvent.setup()
    await pickCustomer(user, 'Roble', 'Ferretería El Roble')
    await addProduct(user, /Tornillo para madera/)
    await user.click(screen.getByRole('button', { name: /Emitir factura/ }))
    const dialog = await screen.findByRole('dialog', { name: '¿Emitir la factura?' })
    await user.click(within(dialog).getByRole('button', { name: 'Emitir factura' }))

    expect(
      await within(dialog).findByText(/No pudimos emitir la factura\. Sigue como borrador/),
    ).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Reintentar' }))
    await screen.findByRole('heading', { level: 1, name: 'FAC-0000041' })
    expect(issue).toHaveBeenCalledTimes(2)
    expect(issue.mock.calls[0]?.[1]).toBe(issue.mock.calls[1]?.[1])
  })
})

/** Una factura emitida con líneas de producto (las del mock inicial no tienen producto). */
async function issuedInvoice() {
  setScenario({ scenario: 'ok', latencyMs: 0, realtime: 'ok' })
  const draft = await mockInvoices.createDraft(
    {
      documentType: 'invoice',
      customerId: 'c1',
      saleConditionCode: '01',
      currency: 'CRC',
      lines: [{ productId: 'p2', quantity: '2' }],
    },
    'k-draft',
  )
  return mockInvoices.issue(draft.id, 'k-issue')
}

describe('pantalla 16 · notas', () => {
  it('nota de crédito: referencia fija, motivo obligatorio, líneas de la factura y emisión', async () => {
    const inv = await issuedInvoice()
    const { router } = renderApp(`/facturas/${inv.id}/nota-credito`)
    const user = userEvent.setup()
    expect(await screen.findByRole('heading', { level: 1, name: 'Nota de crédito' })).toBeInTheDocument()
    expect(screen.getByText('Documento de referencia · no se puede cambiar')).toBeInTheDocument()

    // Sin motivo no se emite.
    await user.click(screen.getByRole('button', { name: 'Emitir nota…' }))
    expect(screen.getAllByText('Escriba el motivo (mínimo 10 caracteres).').length).toBeGreaterThan(0)

    // Más de lo facturado tampoco.
    const qty = screen.getByRole('textbox', { name: /Cant\. · Pintura/ })
    await user.clear(qty)
    await user.type(qty, '3')
    await user.type(
      screen.getByRole('textbox', { name: /Motivo/ }),
      'La identificación del receptor era incorrecta',
    )
    await user.click(screen.getByRole('button', { name: 'Emitir nota…' }))
    expect(screen.getByText('Línea 1: no puede acreditar más de 2.')).toBeInTheDocument()

    await user.clear(qty)
    await user.type(qty, '2')
    await user.click(screen.getByRole('button', { name: 'Guardar borrador' }))
    await waitFor(() => expect(router.state.location.search).toMatch(/^\?borrador=b\d+$/))
    // 2 × 18 500 + 13 % = 41 810.
    expect((await screen.findAllByText(/₡41.810,00/)).length).toBeGreaterThan(0)

    await user.click(screen.getByRole('button', { name: 'Emitir nota…' }))
    const dialog = await screen.findByRole('dialog', { name: '¿Emitir la nota de crédito?' })
    expect(within(dialog).getByText(inv.number ?? '')).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Emitir nota' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'NC-0000007' })).toBeInTheDocument()
  })

  it('solo sobre una factura emitida', async () => {
    renderApp('/facturas/d12/nota-credito')
    expect(await screen.findByText('Solo se crean notas sobre una factura emitida.')).toBeInTheDocument()
  })
})
