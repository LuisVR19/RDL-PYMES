import type { Currency } from '@/shared/money/money'
import type {
  AgingRow,
  FollowUp,
  Page,
  Payment,
  PaymentApplication,
  PaymentPromise,
  Receivable,
  ReceivableDetail,
} from '../billing-types'
import type { ReceivablesPort } from '../ports'
import { ApiError } from '../types'
import type { GatewayHttp } from './http'

/**
 * Cobranza contra el Portal Gateway: paso directo a Receivables (`api/openapi.yaml` de RDL.Receivables.API). Las
 * formas de las respuestas son las de Receivables sin cambios. Sin `RECEIVABLES_API_URL` el gateway responde 503
 * `upstream-not-configured` y las pantallas lo muestran como error o datos parciales.
 */
const bool = (v: boolean | undefined) => (v === undefined ? undefined : String(v))
const num = (v: number | undefined) => (v === undefined ? undefined : String(v))
const seg = encodeURIComponent

export function createReceivablesPort(http: GatewayHttp): ReceivablesPort {
  return {
    list: (q) =>
      http.get<Page<Receivable>>('/portal/v1/receivables', {
        status: q?.status,
        customerId: q?.customerId,
        overdue: bool(q?.overdue),
        cursor: q?.cursor,
        limit: num(q?.limit),
      }),
    get: (id) => http.get<ReceivableDetail>(`/portal/v1/receivables/${seg(id)}`),
    aging: (asOf, currency?: Currency) =>
      http.get<AgingRow[]>('/portal/v1/receivables/aging', { asOf, currency }),
    followUps: (id) => http.get<FollowUp[]>(`/portal/v1/receivables/${seg(id)}/follow-ups`),
    createFollowUp: (id, input, idempotencyKey) =>
      http.send<FollowUp>('POST', `/portal/v1/receivables/${seg(id)}/follow-ups`, input, { idempotencyKey }),
    promises: (id) => http.get<PaymentPromise[]>(`/portal/v1/receivables/${seg(id)}/promises`),
    createPromise: (id, input, idempotencyKey) =>
      http.send<PaymentPromise>('POST', `/portal/v1/receivables/${seg(id)}/promises`, input, {
        idempotencyKey,
      }),
    closePromise: (id, status, idempotencyKey) =>
      http.send<PaymentPromise>(
        'POST',
        `/portal/v1/payment-promises/${seg(id)}/status`,
        { status },
        {
          idempotencyKey,
        },
      ),

    byCustomer: (customerId, q) =>
      http.get<Page<Receivable>>('/portal/v1/receivables', {
        customerId,
        cursor: q?.cursor,
        limit: num(q?.limit),
      }),
    paymentsByCustomer: (customerId, q) =>
      http.get<Page<Payment>>('/portal/v1/payments', { customerId, cursor: q?.cursor, limit: num(q?.limit) }),
    balancesByCustomer: async () => {
      // TODO(api): sin ruta por lote de saldos por cliente en el contrato. No se hace una llamada por fila.
      throw new ApiError({
        status: 501,
        type: 'urn:rdl:portal:problem:not-in-contract',
        title: 'Saldo por cliente por lote',
        correlationId: '',
      })
    },

    payments: (q) =>
      http.get<Page<Payment>>('/portal/v1/payments', { cursor: q?.cursor, limit: num(q?.limit) }),
    payment: (id) => http.get<Payment>(`/portal/v1/payments/${seg(id)}`),
    createPayment: (input, idempotencyKey) =>
      http.send<Payment>('POST', '/portal/v1/payments', input, { idempotencyKey }),
    applyPayment: (input, idempotencyKey) =>
      http.send<PaymentApplication>('POST', '/portal/v1/payment-applications', input, { idempotencyKey }),
    reverseApplication: (id, reason, idempotencyKey) =>
      http.send<PaymentApplication>(
        'POST',
        `/portal/v1/payment-applications/${seg(id)}/reverse`,
        { reason },
        {
          idempotencyKey,
        },
      ),
    voidPayment: (id, reason, idempotencyKey) =>
      http.send<Payment>('POST', `/portal/v1/payments/${seg(id)}/void`, { reason }, { idempotencyKey }),

    summary: async () => {
      // TODO(api): Receivables tiene el aging por moneda y tramo, pero no cuántas cuentas hay en cada cifra. Hasta
      // que el resumen propuesto llegue al contrato, Inicio muestra estas cifras como no disponibles en vez de
      // contar las cuentas página por página.
      throw new ApiError({
        status: 501,
        type: 'urn:rdl:portal:problem:not-in-contract',
        title: 'Resumen de cuentas por cobrar',
        correlationId: '',
      })
    },
  }
}
