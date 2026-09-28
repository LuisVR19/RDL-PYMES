import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import type { InvoiceListItem, InvoiceQuery } from '@/shared/api/billing-types'
import type { DataSource, InvoicesPort } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'
import { formatMoney, type Currency } from '@/shared/money/money'
import { renderApp } from '@/test/renderApp'
import { issuedInvoicesBetween, latestIssued } from './monthInvoices'

// Hoy, para las pruebas: 24/09/2026 a mediodía en Costa Rica. Solo se falsifica Date (no los temporizadores).
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-24T18:00:00Z'))
})
afterEach(() => vi.useRealTimers())

const item = (id: string, total: string, currency: 'CRC' | 'USD', issuedAt: string): InvoiceListItem => ({
  invoice: {
    id,
    documentType: 'invoice',
    number: `FAC-${id}`,
    status: 'issued',
    requiresCorrection: false,
    customerId: 'c1',
    customerLegalName: `Cliente ${id}`,
    issuedAt,
    currency,
    total,
    createdAt: issuedAt,
  },
  fiscal: { availability: 'unavailable' },
  receivable: { availability: 'unavailable' },
})

/** Dos páginas de Billing, en el orden en que las da (por creación, ascendente). */
function pagedInvoices(): { port: InvoicesPort; queries: InvoiceQuery[] } {
  const queries: InvoiceQuery[] = []
  const port: InvoicesPort = {
    ...mockDataSource.invoices,
    list: async (q) => {
      queries.push(q)
      return q.cursor === 'p2'
        ? {
            items: [
              item('3', '50', 'USD', '2026-09-20T15:00:00Z'),
              item('4', '13000.5', 'CRC', '2026-09-23T15:00:00Z'),
            ],
            nextCursor: null,
          }
        : { items: [item('1', '100000', 'CRC', '2026-09-02T15:00:00Z')], nextCursor: 'p2' }
    },
  }
  return { port, queries }
}

// Testing Library normaliza los espacios (el separador de miles es un espacio fino): se compara igual.
const money = (v: string, c: Currency) => formatMoney(v, c).replace(/\s/g, ' ')

const unavailable = () =>
  new ApiError({ status: 503, type: 'urn:rdl:portal-gateway:problem:x', title: 'x', correlationId: 'cid-1' })

describe('pantalla 6 · inicio', () => {
  it('suma lo facturado en el mes recorriendo todas las páginas, por moneda y sin convertir', async () => {
    const { port, queries } = pagedInvoices()
    renderApp('/', { source: { ...mockDataSource, invoices: port } })

    const invoiced = await screen.findByRole('region', { name: 'Facturado en septiembre' })
    expect(await within(invoiced).findByText(money('113000.5', 'CRC'))).toBeInTheDocument()
    expect(within(invoiced).getByText(`3 facturas · y ${money('50', 'USD')}`)).toBeInTheDocument()
    // Del 1 del mes a hoy, solo facturas emitidas.
    expect(queries[0]).toMatchObject({
      documentType: 'invoice',
      status: 'issued',
      issuedFrom: '2026-09-01',
      issuedTo: '2026-09-24',
    })
    expect(queries[1]?.cursor).toBe('p2')

    // Últimas facturas: las más recientes primero, aunque Billing las dé por creación.
    const panel = screen.getByRole('region', { name: 'Últimas facturas' })
    const rows = within(panel).getAllByRole('button', { name: /Cliente/ })
    expect(rows.map((r) => within(r).getByText(/^Cliente/).textContent)).toEqual([
      'Cliente 4',
      'Cliente 3',
      'Cliente 1',
    ])
  })

  it('muestra cobranza y Hacienda con sus enlaces cuando hay algo que atender', async () => {
    const { router } = renderApp('/')
    const overdue = await screen.findByRole('region', { name: 'Saldo vencido' })
    expect(await within(overdue).findByText(/cuentas vencidas/)).toBeInTheDocument()
    const fiscal = screen.getByRole('region', { name: 'Rechazados o en contingencia' })
    expect(await within(fiscal).findByText('3 documentos')).toBeInTheDocument()

    const payments = screen.getByRole('region', { name: 'Últimos pagos' })
    expect(await within(payments).findByText('Transferencia 88412')).toBeInTheDocument()

    await userEvent.setup().click(within(fiscal).getByRole('button', { name: 'Ver bandeja ›' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/hacienda/bandeja'))
  })

  it('cada bloque falla solo: avisa que hay datos parciales y reintenta ese bloque', async () => {
    let calls = 0
    const source: DataSource = {
      ...mockDataSource,
      receivables: {
        ...mockDataSource.receivables,
        summary: async (asOf) => {
          calls += 1
          if (calls === 1) throw unavailable()
          return mockDataSource.receivables.summary(asOf)
        },
      },
    }
    renderApp('/', { source })
    const balance = await screen.findByRole('region', { name: 'Saldo por cobrar' })
    expect(await within(balance).findByText('No pudimos cargar esta cifra.')).toBeInTheDocument()
    expect(screen.getByText(/Algunos datos no están disponibles/)).toBeInTheDocument()
    // El resto sigue: facturación y Hacienda cargan igual.
    expect(
      await within(screen.getByRole('region', { name: 'Rechazados o en contingencia' })).findByText(
        '3 documentos',
      ),
    ).toBeInTheDocument()

    await userEvent.setup().click(within(balance).getByRole('button', { name: 'Reintentar' }))
    expect(await within(balance).findByText(/cuentas abiertas/)).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.queryByText(/Algunos datos no están disponibles/)).not.toBeInTheDocument(),
    )
  })

  it('según el rol: un facturador no ve cobranza ni «Registrar pago»', async () => {
    renderApp('/', { orgId: 'si' })
    expect(await screen.findByRole('region', { name: 'Facturado en septiembre' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Saldo por cobrar' })).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Últimos pagos' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Registrar pago' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Nueva factura/ })).toBeInTheDocument()
  })

  it('una organización sin facturas en el mes lo dice en vez de mostrar una tabla vacía', async () => {
    const source: DataSource = {
      ...mockDataSource,
      invoices: { ...mockDataSource.invoices, list: async () => ({ items: [], nextCursor: null }) },
    }
    renderApp('/', { source })
    const invoiced = await screen.findByRole('region', { name: 'Facturado en septiembre' })
    expect(await within(invoiced).findByText('Sin facturas este mes')).toBeInTheDocument()
    expect(
      screen.getByText('No hay facturas emitidas este mes. Cree una con «Nueva factura».'),
    ).toBeInTheDocument()
  })
})

describe('facturas del mes', () => {
  it('no suma a medias: más de 2 000 facturas es un error, no una cifra incompleta', async () => {
    let pages = 0
    const port: InvoicesPort = {
      ...mockDataSource.invoices,
      list: async () => {
        pages += 1
        return { items: [item(String(pages), '1', 'CRC', '2026-09-02T15:00:00Z')], nextCursor: 'otra' }
      },
    }
    await expect(issuedInvoicesBetween(port, '2026-09-01', '2026-09-24')).rejects.toBeInstanceOf(ApiError)
    expect(pages).toBe(20)
  })

  it('las últimas se ordenan por el instante de emisión', () => {
    const items = [
      item('a', '1', 'CRC', '2026-09-02T15:00:00Z'),
      item('b', '1', 'CRC', '2026-09-10T06:00:00.5Z'),
      item('c', '1', 'CRC', '2026-09-10T06:00:00Z'),
    ]
    expect(latestIssued(items, 2).map((i) => i.invoice.id)).toEqual(['b', 'c'])
  })
})
