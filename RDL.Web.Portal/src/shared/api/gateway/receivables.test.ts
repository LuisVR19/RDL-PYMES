import { describe, expect, it } from 'vitest'
import { ApiError } from '../types'
import type { GatewayHttp } from './http'
import { createReceivablesPort } from './receivables'

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

describe('adaptador del Portal Gateway · cobranza', () => {
  it('lecturas: filtros de la lista a query, ids escapados en la ruta', async () => {
    const { http, calls } = recorder()
    const port = createReceivablesPort(http)
    await port.list({ status: 'partially_paid', overdue: true, customerId: 'c9', limit: 20 })
    await port.get('r/1')
    await port.aging('2026-09-24', 'USD')
    await port.followUps('r1')
    await port.promises('r1')
    await port.payment('p1')
    expect(calls).toEqual([
      {
        method: 'GET',
        path: '/portal/v1/receivables',
        query: {
          status: 'partially_paid',
          customerId: 'c9',
          overdue: 'true',
          cursor: undefined,
          limit: '20',
        },
      },
      { method: 'GET', path: '/portal/v1/receivables/r%2F1', query: undefined },
      { method: 'GET', path: '/portal/v1/receivables/aging', query: { asOf: '2026-09-24', currency: 'USD' } },
      { method: 'GET', path: '/portal/v1/receivables/r1/follow-ups', query: undefined },
      { method: 'GET', path: '/portal/v1/receivables/r1/promises', query: undefined },
      { method: 'GET', path: '/portal/v1/payments/p1', query: undefined },
    ])
  })

  it('comandos: POST con la Idempotency-Key de quien llama y el cuerpo del contrato', async () => {
    const { http, calls } = recorder()
    const port = createReceivablesPort(http)
    const payment = {
      customerId: 'c9',
      receivedOn: '2026-09-24',
      amount: '150000',
      currency: 'CRC' as const,
      paymentMethodCode: '04',
      applications: [{ receivableId: 'r29', amount: '85000' }],
    }
    await port.createPayment(payment, 'k1')
    await port.applyPayment({ paymentId: 'p1', receivableId: 'r1', amount: '10' }, 'k2')
    await port.reverseApplication('a1', 'Se aplicó a la factura equivocada', 'k3')
    await port.voidPayment('p1', 'Transferencia devuelta', 'k4')
    await port.createFollowUp('r1', { followupType: 'call', notes: 'Llamó' }, 'k5')
    await port.createPromise('r1', { promisedAmount: '10', promisedOn: '2026-10-01' }, 'k6')
    await port.closePromise('pr1', 'kept', 'k7')
    expect(calls).toEqual([
      { method: 'POST', path: '/portal/v1/payments', body: payment, key: 'k1' },
      {
        method: 'POST',
        path: '/portal/v1/payment-applications',
        body: { paymentId: 'p1', receivableId: 'r1', amount: '10' },
        key: 'k2',
      },
      {
        method: 'POST',
        path: '/portal/v1/payment-applications/a1/reverse',
        body: { reason: 'Se aplicó a la factura equivocada' },
        key: 'k3',
      },
      {
        method: 'POST',
        path: '/portal/v1/payments/p1/void',
        body: { reason: 'Transferencia devuelta' },
        key: 'k4',
      },
      {
        method: 'POST',
        path: '/portal/v1/receivables/r1/follow-ups',
        body: { followupType: 'call', notes: 'Llamó' },
        key: 'k5',
      },
      {
        method: 'POST',
        path: '/portal/v1/receivables/r1/promises',
        body: { promisedAmount: '10', promisedOn: '2026-10-01' },
        key: 'k6',
      },
      { method: 'POST', path: '/portal/v1/payment-promises/pr1/status', body: { status: 'kept' }, key: 'k7' },
    ])
  })

  it('cobranza por cliente; el saldo por lote y el resumen no existen y no se inventan', async () => {
    const { http, calls } = recorder()
    const port = createReceivablesPort(http)
    await port.byCustomer('c9')
    await port.paymentsByCustomer('c9', { limit: 100 })
    expect(calls.map((c) => c.path)).toEqual(['/portal/v1/receivables', '/portal/v1/payments'])
    expect(calls[1]?.query).toMatchObject({ customerId: 'c9', limit: '100' })
    await expect(port.balancesByCustomer(['c1', 'c2'])).rejects.toBeInstanceOf(ApiError)
    await expect(port.summary('2026-09-24')).rejects.toBeInstanceOf(ApiError)
    expect(calls).toHaveLength(2)
  })
})
