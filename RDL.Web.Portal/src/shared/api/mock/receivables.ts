import { Big } from 'big.js'
import { ADJUSTMENTS, CUSTOMERS, FOLLOW_UPS, PAYMENTS, PROMISES, RECEIVABLES } from '@/mocks/billing'
import { daysOverdue, todayIn } from '@/shared/dates/dates'
import { sumMoney, type Currency } from '@/shared/money/money'
import {
  AGING_BUCKETS,
  type AgingBucket,
  type AgingRow,
  type FollowUp,
  type Payment,
  type PaymentApplication,
  type PaymentPromise,
  type Receivable,
  type ReceivableAdjustment,
  type ReceivablesSummary,
} from '../billing-types'
import type { ReceivablesPort } from '../ports'
import { ApiError, type FieldError } from '../types'
import { paginate } from './billing'
import { fakeCorrelationId, simulate, simulateSecondary } from './simulate'

/**
 * Cobranza simulada: imita a Receivables sobre datos en memoria. Aplica sus reglas (mismo cliente y moneda, nunca
 * más que el saldo ni que el pago, pago anulado → 409, aplicación revertida → 409), recalcula saldo y estado como
 * la base, y con la misma `Idempotency-Key` devuelve la misma respuesta sin repetir el efecto.
 */
let receivables: Receivable[] = structuredClone(RECEIVABLES)
let payments: Payment[] = structuredClone(PAYMENTS)
let followUps: FollowUp[] = structuredClone(FOLLOW_UPS)
let promises: PaymentPromise[] = structuredClone(PROMISES)
let adjustments: Record<string, ReceivableAdjustment[]> = structuredClone(ADJUSTMENTS)
const byKey = new Map<string, unknown>()
let seq = 100

/** Solo pruebas: vuelve a los datos iniciales. */
export function resetMockReceivables(): void {
  receivables = structuredClone(RECEIVABLES)
  payments = structuredClone(PAYMENTS)
  followUps = structuredClone(FOLLOW_UPS)
  promises = structuredClone(PROMISES)
  adjustments = structuredClone(ADJUSTMENTS)
  byKey.clear()
}

const problem = (status: number, code: string, title: string, errors?: FieldError[]) =>
  new ApiError({
    status,
    type: `urn:rdl:receivables:problem:${code}`,
    title,
    correlationId: fakeCorrelationId(),
    errors,
  })
const notFound = () => problem(404, 'not-found', 'Recurso no encontrado')
const invalid = (field: string, message: string) =>
  problem(422, 'validation', 'Datos inválidos', [{ field, message }])

const empty = { items: [], nextCursor: null }
const today = () => todayIn()
const nextId = (prefix: string) => `${prefix}${++seq}`
const now = () => new Date().toISOString()
const collectable = (r: Receivable) => r.status === 'open' || r.status === 'partially_paid'

/** Misma clave, misma respuesta: el efecto se aplica una sola vez. */
async function once<T>(key: string, run: () => T): Promise<T> {
  await simulate(null, null)
  if (byKey.has(key)) return structuredClone(byKey.get(key) as T)
  const out = run()
  byKey.set(key, structuredClone(out))
  return out
}

/** Saldo y estado como los recalcula la base (R1): 0 → paid; igual al original → open; si no, partially_paid. */
function setBalance(r: Receivable, balance: Big) {
  r.balanceAmount = balance.toFixed()
  if (r.status === 'cancelled') return
  if (balance.eq(0)) {
    r.status = 'paid'
    r.settledAt = now()
  } else {
    r.status = balance.eq(r.originalAmount) ? 'open' : 'partially_paid'
    delete r.settledAt
  }
}

const activeApps = (p: Payment) => p.applications.filter((a) => !a.reversedAt)
const available = (p: Payment) => activeApps(p).reduce((acc, a) => acc.minus(a.amount), new Big(p.amount))

function positive(field: string, amount: string) {
  if (!/^\d+(\.\d{1,5})?$/.test(amount) || new Big(amount).lte(0)) {
    throw invalid(field, 'Debe ser un monto mayor que cero.')
  }
}

/** Las reglas de una aplicación (settlement de Receivables): cliente, moneda, estado y saldo. */
function checkApplication(p: Payment, receivableId: string, amount: string, field: string): Receivable {
  const r = receivables.find((x) => x.id === receivableId)
  if (!r) throw notFound()
  positive(field, amount)
  if (r.customerId !== p.customerId) throw problem(422, 'customer-mismatch', 'Cliente distinto')
  if (r.currency !== p.currency) throw problem(422, 'currency-mismatch', 'Moneda distinta')
  if (!collectable(r)) throw problem(409, 'conflict', 'Conflicto con el estado actual')
  if (new Big(amount).gt(r.balanceAmount)) {
    throw problem(422, 'application-exceeds-balance', 'La aplicación supera el saldo')
  }
  return r
}

function apply(p: Payment, r: Receivable, amount: string): PaymentApplication {
  const app: PaymentApplication = {
    id: nextId('ap'),
    paymentId: p.id,
    receivableId: r.id,
    amount,
    appliedAt: now(),
  }
  p.applications.push(app)
  setBalance(r, new Big(r.balanceAmount).minus(amount))
  return app
}

function reverse(app: PaymentApplication, reason: string) {
  const r = receivables.find((x) => x.id === app.receivableId)
  app.reversedAt = now()
  app.reversalReason = reason
  if (r) setBalance(r, new Big(r.balanceAmount).plus(app.amount))
}

function bucketOf(dueOn: string, asOf: string): AgingBucket {
  const days = daysOverdue(dueOn, asOf)
  if (days <= 0) return 'current'
  if (days <= 30) return '1_30'
  if (days <= 60) return '31_60'
  if (days <= 90) return '61_90'
  return '90_plus'
}

function reason(text: string) {
  if (text.trim().length === 0) throw invalid('reason', 'Es obligatorio.')
}

export const mockReceivables: ReceivablesPort = {
  list: (q) => {
    const asOf = today()
    const rows = receivables
      .filter(
        (r) =>
          (!q?.status || r.status === q.status) &&
          (!q?.customerId || r.customerId === q.customerId) &&
          (q?.overdue === undefined || (collectable(r) && daysOverdue(r.dueOn, asOf) > 0) === q.overdue),
      )
      .toSorted((a, b) => b.issuedOn.localeCompare(a.issuedOn) || b.id.localeCompare(a.id))
    return simulate(structuredClone(paginate(rows, q)), empty)
  },

  async get(id) {
    const r = receivables.find((x) => x.id === id)
    await simulate(null, null)
    if (!r) throw notFound()
    const applications = payments.flatMap((p) => p.applications.filter((a) => a.receivableId === id))
    return structuredClone({
      ...r,
      applications: applications.toSorted((a, b) => a.appliedAt.localeCompare(b.appliedAt)),
      adjustments: adjustments[id] ?? [],
    })
  },

  aging: (asOf, currency) => {
    const rows: AgingRow[] = []
    for (const cur of ['CRC', 'USD'] as const satisfies Currency[]) {
      if (currency && cur !== currency) continue
      const open = receivables.filter((r) => collectable(r) && r.currency === cur)
      if (open.length === 0) continue
      for (const bucket of AGING_BUCKETS) {
        const amounts = open.filter((r) => bucketOf(r.dueOn, asOf) === bucket).map((r) => r.balanceAmount)
        rows.push({ bucket, currency: cur, balance: sumMoney(amounts) })
      }
    }
    return simulate(rows, [])
  },

  followUps: (id) =>
    simulateSecondary(
      structuredClone(
        followUps
          .filter((f) => f.receivableId === id)
          .toSorted((a, b) => b.performedAt.localeCompare(a.performedAt)),
      ),
      [],
    ),

  createFollowUp: (id, input, key) =>
    once(key, () => {
      if (!receivables.some((r) => r.id === id)) throw notFound()
      if (input.notes.trim().length === 0) throw invalid('notes', 'Es obligatorio.')
      const at = now()
      const f: FollowUp = {
        id: nextId('fu'),
        receivableId: id,
        followupType: input.followupType,
        notes: input.notes.trim(),
        ...(input.nextActionOn ? { nextActionOn: input.nextActionOn } : {}),
        performedAt: at,
        createdAt: at,
      }
      followUps = [f, ...followUps]
      return structuredClone(f)
    }),

  promises: (id) =>
    simulateSecondary(
      structuredClone(
        promises
          .filter((p) => p.receivableId === id)
          .toSorted((a, b) => b.createdAt.localeCompare(a.createdAt)),
      ),
      [],
    ),

  createPromise: (id, input, key) =>
    once(key, () => {
      const r = receivables.find((x) => x.id === id)
      if (!r) throw notFound()
      if (!collectable(r)) throw problem(409, 'conflict', 'Conflicto con el estado actual')
      positive('promisedAmount', input.promisedAmount)
      if (new Big(input.promisedAmount).gt(r.balanceAmount)) {
        throw invalid('promisedAmount', 'No puede superar el saldo de la cuenta.')
      }
      if (input.promisedOn < today()) throw invalid('promisedOn', 'No puede ser anterior a hoy.')
      const at = now()
      const pr: PaymentPromise = {
        id: nextId('pr'),
        receivableId: id,
        promisedAmount: input.promisedAmount,
        promisedOn: input.promisedOn,
        status: 'pending',
        createdAt: at,
        updatedAt: at,
      }
      promises = [pr, ...promises]
      return structuredClone(pr)
    }),

  closePromise: (id, status, key) =>
    once(key, () => {
      const pr = promises.find((x) => x.id === id)
      if (!pr) throw notFound()
      if (pr.status !== 'pending') throw problem(409, 'conflict', 'Conflicto con el estado actual')
      pr.status = status
      pr.updatedAt = now()
      return structuredClone(pr)
    }),

  // Cobranza es secundaria en facturación: en el escenario «datos parciales» falla sola.
  byCustomer: (customerId, q) =>
    simulateSecondary(
      structuredClone(
        paginate(
          receivables.filter((x) => x.customerId === customerId),
          q,
        ),
      ),
      empty,
    ),
  paymentsByCustomer: (customerId, q) =>
    simulateSecondary(
      structuredClone(
        paginate(
          payments.filter((x) => x.customerId === customerId),
          q,
        ),
      ),
      empty,
    ),
  balancesByCustomer: (ids) =>
    simulateSecondary(
      structuredClone(
        Object.fromEntries(ids.map((id) => [id, receivables.filter((x) => x.customerId === id)])),
      ),
      {},
    ),

  payments: (q) =>
    simulateSecondary(
      structuredClone(
        paginate(
          payments.toSorted(
            (a, b) => b.receivedOn.localeCompare(a.receivedOn) || b.createdAt.localeCompare(a.createdAt),
          ),
          q,
        ),
      ),
      empty,
    ),

  async payment(id) {
    const p = payments.find((x) => x.id === id)
    await simulate(null, null)
    if (!p) throw notFound()
    return structuredClone(p)
  },

  createPayment: (input, key) =>
    once(key, () => {
      if (!CUSTOMERS.some((c) => c.id === input.customerId)) throw notFound()
      positive('amount', input.amount)
      if (input.receivedOn > today()) throw invalid('receivedOn', 'No puede ser posterior a hoy.')
      const p: Payment = {
        id: nextId('pg'),
        customerId: input.customerId,
        receivedOn: input.receivedOn,
        amount: input.amount,
        currency: input.currency,
        exchangeRate: input.exchangeRate ?? '1',
        paymentMethodCode: input.paymentMethodCode,
        ...(input.reference ? { reference: input.reference } : {}),
        ...(input.notes ? { notes: input.notes } : {}),
        status: 'posted',
        createdAt: now(),
        applications: [],
      }
      const apps = input.applications ?? []
      // Todo o nada, como la transacción de Receivables: primero se valida cada aplicación, después se aplica.
      const targets = apps.map((a, i) =>
        checkApplication(p, a.receivableId, a.amount, `applications[${i}].amount`),
      )
      if (apps.reduce((acc, a) => acc.plus(a.amount), new Big(0)).gt(p.amount)) {
        throw problem(422, 'application-exceeds-payment', 'La aplicación supera el pago')
      }
      apps.forEach((a, i) => apply(p, targets[i] as Receivable, a.amount))
      payments = [p, ...payments]
      return structuredClone(p)
    }),

  applyPayment: (input, key) =>
    once(key, () => {
      const p = payments.find((x) => x.id === input.paymentId)
      if (!p) throw notFound()
      if (p.status === 'voided') throw problem(409, 'payment-voided', 'Pago anulado')
      const r = checkApplication(p, input.receivableId, input.amount, 'amount')
      if (new Big(input.amount).gt(available(p))) {
        throw problem(422, 'application-exceeds-payment', 'La aplicación supera el pago')
      }
      return structuredClone(apply(p, r, input.amount))
    }),

  reverseApplication: (id, text, key) =>
    once(key, () => {
      reason(text)
      const app = payments.flatMap((p) => p.applications).find((a) => a.id === id)
      if (!app) throw notFound()
      if (app.reversedAt) throw problem(409, 'application-reversed', 'Aplicación revertida')
      reverse(app, text.trim())
      return structuredClone(app)
    }),

  voidPayment: (id, text, key) =>
    once(key, () => {
      reason(text)
      const p = payments.find((x) => x.id === id)
      if (!p) throw notFound()
      if (p.status === 'voided') throw problem(409, 'payment-voided', 'Pago anulado')
      activeApps(p).forEach((a) => reverse(a, text.trim()))
      p.status = 'voided'
      p.voidReason = text.trim()
      p.voidedAt = now()
      return structuredClone(p)
    }),

  summary: (asOf) => {
    const open = receivables.filter(collectable)
    const overdue = open.filter((r) => daysOverdue(r.dueOn, asOf) > 0)
    return simulateSecondary<ReceivablesSummary>(
      {
        open: { totals: byCurrency(open), count: open.length },
        overdue: { totals: byCurrency(overdue), count: overdue.length },
      },
      { open: { totals: {}, count: 0 }, overdue: { totals: {}, count: 0 } },
    )
  },
}

function byCurrency(rs: Receivable[]): Partial<Record<Currency, string>> {
  const out: Partial<Record<Currency, string>> = {}
  for (const cur of ['CRC', 'USD'] as const) {
    const amounts = rs.filter((r) => r.currency === cur).map((r) => r.balanceAmount)
    if (amounts.length > 0) out[cur] = sumMoney(amounts)
  }
  return out
}
