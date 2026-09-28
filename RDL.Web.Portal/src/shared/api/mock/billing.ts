import { CUSTOMERS, PAYMENTS, RECEIVABLES } from '@/mocks/billing'
import { daysOverdue } from '@/shared/dates/dates'
import { sumMoney, type Currency } from '@/shared/money/money'
import type { Customer, Page, PageQuery, Receivable, ReceivablesSummary } from '../billing-types'
import type { CustomersPort, ReceivablesPort } from '../ports'
import { ApiError } from '../types'
import { fakeCorrelationId, simulate, simulateSecondary } from './simulate'

/**
 * Facturación simulada: imita las respuestas de Billing (paginación por cursor, búsqueda, 404, 409 por
 * identificación repetida) sobre datos en memoria que cambian durante la revisión.
 */
let customers: Customer[] = structuredClone(CUSTOMERS)

/** Solo pruebas: vuelve a los datos iniciales. */
export function resetMockBilling(): void {
  customers = structuredClone(CUSTOMERS)
}

/** Solo lectura, para otros simulados (un borrador valida su cliente; al emitir se copia el snapshot). */
export function mockCustomerById(id: string): Customer | undefined {
  return customers.find((c) => c.id === id)
}

const billingProblem = (status: number, code: string, title: string) =>
  new ApiError({ status, type: `urn:rdl:billing:problem:${code}`, title, correlationId: fakeCorrelationId() })

/** Cursor opaco simulado: la posición siguiente. */
export function paginate<T>(all: T[], q: PageQuery = {}): Page<T> {
  const limit = q.limit ?? 20
  const start = q.cursor ? Number.parseInt(q.cursor, 10) : 0
  const items = all.slice(start, start + limit)
  const next = start + limit < all.length ? String(start + limit) : null
  return { items, nextCursor: next }
}

const digits = (s: string) => s.replace(/\D/g, '')

export const mockCustomers: CustomersPort = {
  async list(q) {
    const text = q.q?.trim().toLocaleLowerCase('es-CR')
    const matches = customers.filter(
      (c) =>
        (q.active === undefined || c.isActive === q.active) &&
        (!text ||
          `${c.legalName} ${c.tradeName ?? ''} ${c.identification.number}`
            .toLocaleLowerCase('es-CR')
            .includes(text)),
    )
    return simulate(paginate(matches, q), { items: [], nextCursor: null })
  },
  async get(id) {
    await simulate(null, null)
    const found = customers.find((c) => c.id === id)
    if (!found) throw billingProblem(404, 'not-found', 'Recurso no encontrado')
    return structuredClone(found)
  },
  async create(input) {
    await simulate(null, null)
    if (customers.some((c) => digits(c.identification.number) === digits(input.identification.number))) {
      throw billingProblem(409, 'customer-identification-taken', 'Identificación repetida')
    }
    const now = new Date().toISOString()
    const created: Customer = {
      id: `c${Date.now()}`,
      ...input,
      isActive: true,
      createdAt: now,
      updatedAt: now,
    }
    customers = [created, ...customers]
    return structuredClone(created)
  },
  async update(id, patch) {
    await simulate(null, null)
    const i = customers.findIndex((c) => c.id === id)
    const current = customers[i]
    if (!current) throw billingProblem(404, 'not-found', 'Recurso no encontrado')
    const next: Customer = { ...current, ...patch, updatedAt: new Date().toISOString() }
    customers = customers.map((c) => (c.id === id ? next : c))
    return structuredClone(next)
  },
}

const empty = { items: [], nextCursor: null }

export const mockReceivables: ReceivablesPort = {
  // Cobranza es secundaria en facturación: en el escenario «datos parciales» falla sola.
  byCustomer: (customerId, q) =>
    simulateSecondary(
      paginate(
        RECEIVABLES.filter((x) => x.customerId === customerId),
        q,
      ),
      empty,
    ),
  paymentsByCustomer: (customerId, q) =>
    simulateSecondary(
      paginate(
        PAYMENTS.filter((x) => x.customerId === customerId),
        q,
      ),
      empty,
    ),
  balancesByCustomer: (ids) =>
    simulateSecondary(
      Object.fromEntries(ids.map((id) => [id, RECEIVABLES.filter((x) => x.customerId === id)])),
      {},
    ),
  payments: (q) =>
    simulateSecondary(
      paginate(
        PAYMENTS.toSorted(
          (a, b) => b.receivedOn.localeCompare(a.receivedOn) || b.createdAt.localeCompare(a.createdAt),
        ),
        q,
      ),
      empty,
    ),
  summary: (asOf) => {
    const open = RECEIVABLES.filter((r) => r.status === 'open' || r.status === 'partially_paid')
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
