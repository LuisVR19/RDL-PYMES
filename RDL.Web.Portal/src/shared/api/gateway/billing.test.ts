import { describe, expect, it } from 'vitest'
import { ApiError } from '../types'
import {
  createCatalogsPort,
  createCustomersPort,
  createInvoicesPort,
  createProductsPort,
  createReceivablesPort,
} from './billing'
import type { GatewayHttp } from './http'

function recorder() {
  const calls: { method: string; path: string; query?: unknown; body?: unknown; key?: string }[] = []
  const http: GatewayHttp = {
    async get<T>(path: string, query?: Record<string, string | undefined>) {
      calls.push({ method: 'GET', path, query })
      return { items: [], nextCursor: null } as T
    },
    async send<T>(method: string, path: string, body?: unknown, opts?: { idempotencyKey?: string }) {
      calls.push({ method, path, body, key: opts?.idempotencyKey })
      return {} as T
    },
  }
  return { http, calls }
}

describe('adaptador del Portal Gateway · facturación', () => {
  it('clientes: filtros a query, POST con clave, PATCH sin clave', async () => {
    const { http, calls } = recorder()
    const port = createCustomersPort(http)
    await port.list({ q: '  ibis ', active: false, cursor: 'c2', limit: 20 })
    await port.list({ q: '' })
    await port.create({ identification: { typeCode: '02', number: '3101' }, legalName: 'X' }, 'k-1')
    await port.update('a/b', { phone: '' })
    expect(calls).toEqual([
      {
        method: 'GET',
        path: '/portal/v1/customers',
        query: { q: 'ibis', active: 'false', cursor: 'c2', limit: '20' },
      },
      {
        method: 'GET',
        path: '/portal/v1/customers',
        query: { q: undefined, active: undefined, cursor: undefined, limit: undefined },
      },
      {
        method: 'POST',
        path: '/portal/v1/customers',
        body: { identification: { typeCode: '02', number: '3101' }, legalName: 'X' },
        key: 'k-1',
      },
      { method: 'PATCH', path: '/portal/v1/customers/a%2Fb', body: { phone: '' }, key: undefined },
    ])
  })

  it('cobranza por cliente va a Receivables por el gateway; el saldo por lote no existe y no se inventa', async () => {
    const { http, calls } = recorder()
    const port = createReceivablesPort(http)
    await port.byCustomer('c9')
    await port.paymentsByCustomer('c9', { limit: 100 })
    expect(calls.map((c) => c.path)).toEqual(['/portal/v1/receivables', '/portal/v1/payments'])
    expect(calls[1]?.query).toMatchObject({ customerId: 'c9', limit: '100' })
    await expect(port.balancesByCustomer(['c1', 'c2'])).rejects.toBeInstanceOf(ApiError)
    expect(calls).toHaveLength(2)
  })

  it('documentos: el listado compuesto con sus filtros', async () => {
    const { http, calls } = recorder()
    await createInvoicesPort(http).list({
      documentType: 'credit_note',
      customerId: 'c9',
      requiresCorrection: true,
    })
    expect(calls[0]).toMatchObject({
      path: '/portal/v1/invoices',
      query: { documentType: 'credit_note', customerId: 'c9', requiresCorrection: 'true' },
    })
  })

  it('productos: mismas reglas que clientes (POST con clave, PATCH parcial)', async () => {
    const { http, calls } = recorder()
    const port = createProductsPort(http)
    await port.list({ q: 'pin', active: true })
    await port.update('p2', { taxes: [] })
    expect(calls[0]).toMatchObject({ path: '/portal/v1/products', query: { q: 'pin', active: 'true' } })
    expect(calls[1]).toEqual({
      method: 'PATCH',
      path: '/portal/v1/products/p2',
      body: { taxes: [] },
      key: undefined,
    })
  })

  it('catálogos van a E-Invoice; los pares de impuesto no se arman sin contrato', async () => {
    const { http, calls } = recorder()
    const port = createCatalogsPort(http)
    await port.searchCabys('tornillo')
    await port.unitsOfMeasure()
    expect(calls.map((c) => c.path)).toEqual([
      '/portal/v1/catalogs/cabys',
      '/portal/v1/catalogs/units-of-measure',
    ])
    await expect(port.taxOptions()).rejects.toBeInstanceOf(ApiError)
  })
})
