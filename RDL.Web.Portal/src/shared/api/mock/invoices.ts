import { Big } from 'big.js'
import { CUSTOMERS, INVOICES } from '@/mocks/billing'
import type {
  BalancePart,
  FiscalPart,
  Invoice,
  InvoiceLine,
  InvoiceListItem,
  StatusChange,
} from '../billing-types'
import type { InvoicesPort } from '../ports'
import { ApiError } from '../types'
import { paginate } from './billing'
import { mockProductById } from './products'
import { fakeCorrelationId, simulate, simulateSecondary } from './simulate'

/**
 * Documentos simulados. Imitan a Billing: el SERVIDOR calcula líneas y totales (aquí, un cálculo de muestra con
 * decimales exactos y el redondeo D2: 5 decimales, mitad hacia arriba) y el gateway compone el estado de Hacienda y
 * el saldo. Las pantallas nunca calculan un total: lo muestran.
 */
export interface MockDoc {
  invoice: Invoice
  fiscal: FiscalPart
  receivable: BalancePart
  history: StatusChange[]
}

const round5 = (b: Big) => b.round(5, Big.roundHalfUp)
const s = (b: Big) => b.toFixed()

/** Una línea de muestra con impuesto de 13 % (tarifa ilustrativa del prototipo). */
export function mockLine(
  n: number,
  p: { id?: string; code?: string; cabys: string; description: string; unit: string; isService: boolean },
  quantity: string,
  unitPrice: string,
  discount = '0',
  discountReason?: string,
  rate = '13',
): InvoiceLine {
  const subtotal = round5(new Big(quantity).times(unitPrice).minus(discount))
  const tax = rate === '0' ? new Big(0) : round5(subtotal.times(rate).div(100))
  return {
    lineNumber: n,
    ...(p.id ? { productId: p.id } : {}),
    ...(p.code ? { productCode: p.code } : {}),
    cabysCode: p.cabys,
    description: p.description,
    unitOfMeasureCode: p.unit,
    isService: p.isService,
    quantity,
    unitPrice,
    discount,
    ...(discountReason ? { discountReason } : {}),
    subtotal: s(subtotal),
    tax: s(tax),
    total: s(subtotal.plus(tax)),
    taxes:
      rate === '0'
        ? []
        : [{ taxTypeCode: '01', taxRateCode: '08', rate, taxableBase: s(subtotal), amount: s(tax) }],
  }
}

export function withTotals(inv: Invoice, lines: InvoiceLine[]): Invoice {
  const sum = (k: 'subtotal' | 'tax' | 'discount') => lines.reduce((a, l) => a.plus(l[k]), new Big(0))
  return {
    ...inv,
    lines,
    subtotal: s(sum('subtotal')),
    discount: s(sum('discount')),
    tax: s(sum('tax')),
    exoneration: '0',
    total: s(sum('subtotal').plus(sum('tax'))),
  }
}

function productLine(n: number, productId: string, qty: string): InvoiceLine {
  const p = mockProductById(productId)
  if (!p) throw new Error(`producto simulado ${productId}`)
  return mockLine(
    n,
    {
      id: p.id,
      code: p.code,
      cabys: p.cabysCode,
      description: p.description,
      unit: p.unitOfMeasureCode,
      isService: p.isService,
    },
    qty,
    p.unitPrice,
  )
}

/** Una línea genérica cuyo total es exactamente el del documento del prototipo. */
function genericLine(total: string): InvoiceLine {
  const subtotal = round5(new Big(total).div('1.13'))
  const tax = new Big(total).minus(subtotal)
  return {
    lineNumber: 1,
    cabysCode: '0000000000140',
    description: 'Servicios de mantenimiento y reparación',
    unitOfMeasureCode: 'Unid',
    isService: true,
    quantity: '1',
    unitPrice: s(subtotal),
    discount: '0',
    subtotal: s(subtotal),
    tax: s(tax),
    total,
    taxes: [{ taxTypeCode: '01', taxRateCode: '08', rate: '13', taxableBase: s(subtotal), amount: s(tax) }],
  }
}

function fromListItem(it: InvoiceListItem): MockDoc {
  const i = it.invoice
  const customer = CUSTOMERS.find((c) => c.id === i.customerId)
  const base: Invoice = {
    id: i.id,
    documentType: i.documentType,
    number: i.number ?? null,
    status: i.status,
    requiresCorrection: i.requiresCorrection,
    ...(it.fiscal.status?.status === 'rejected'
      ? {
          fiscalRejectionReason:
            'La identificación del receptor no coincide con el registro (texto ilustrativo)',
        }
      : {}),
    customerId: i.customerId,
    ...(i.status !== 'draft' && customer
      ? {
          customerSnapshot: {
            customerId: customer.id,
            identification: customer.identification,
            legalName: customer.legalName,
            email: customer.email,
            phone: customer.phone,
            address: customer.address,
          },
        }
      : {}),
    saleConditionCode: i.dueDate && i.issuedAt && i.dueDate !== i.issuedAt.slice(0, 10) ? '02' : '01',
    ...(i.dueDate && i.issuedAt && i.dueDate !== i.issuedAt.slice(0, 10) ? { creditTermDays: 30 } : {}),
    ...(i.issuedAt ? { issuedAt: i.issuedAt } : {}),
    ...(i.dueDate ? { dueDate: i.dueDate } : {}),
    currency: i.currency,
    exchangeRate: i.currency === 'USD' ? '505.5' : '1',
    lines: [],
    subtotal: '0',
    discount: '0',
    tax: '0',
    exoneration: '0',
    total: i.total,
    createdAt: i.createdAt,
    updatedAt: i.createdAt,
  }
  const lines =
    i.id === 'd12'
      ? [productLine(1, 'p2', '2'), productLine(2, 'p3', '4'), productLine(3, 'p1', '40')]
      : [genericLine(i.total)]
  const history: StatusChange[] =
    i.status === 'draft'
      ? []
      : [
          {
            fromStatus: 'draft',
            toStatus: 'issued',
            changedByUserId: 'u1',
            changedAt: i.issuedAt ?? i.createdAt,
          },
          ...(i.status === 'cancelled'
            ? [
                {
                  fromStatus: 'issued' as const,
                  toStatus: 'cancelled' as const,
                  reason: 'Se facturó al cliente equivocado.',
                  changedByUserId: 'u1',
                  changedAt: '2026-09-16T16:00:00Z',
                },
              ]
            : []),
        ]
  return { invoice: withTotals(base, lines), fiscal: it.fiscal, receivable: it.receivable, history }
}

let docs: MockDoc[] = INVOICES.map(fromListItem)

/** Solo pruebas: vuelve a los documentos iniciales. */
export function resetMockInvoices(): void {
  docs = INVOICES.map(fromListItem)
}

export const mockDocs = {
  all: () => docs,
  find: (id: string) => docs.find((d) => d.invoice.id === id),
  put: (d: MockDoc) => {
    docs = docs.some((x) => x.invoice.id === d.invoice.id)
      ? docs.map((x) => (x.invoice.id === d.invoice.id ? d : x))
      : [d, ...docs]
  },
  remove: (id: string) => {
    docs = docs.filter((x) => x.invoice.id !== id)
  },
}

export const notFound = () =>
  new ApiError({
    status: 404,
    type: 'urn:rdl:billing:problem:not-found',
    title: 'Recurso no encontrado',
    correlationId: fakeCorrelationId(),
  })

function toListItem(d: MockDoc): InvoiceListItem {
  const i = d.invoice
  const name = i.customerSnapshot?.legalName
  return {
    invoice: {
      id: i.id,
      documentType: i.documentType,
      ...(i.number ? { number: i.number } : {}),
      status: i.status,
      requiresCorrection: i.requiresCorrection,
      customerId: i.customerId,
      ...(name ? { customerLegalName: name } : {}),
      ...(i.issuedAt ? { issuedAt: i.issuedAt } : {}),
      ...(i.dueDate ? { dueDate: i.dueDate } : {}),
      currency: i.currency,
      total: i.total,
      createdAt: i.createdAt,
    },
    fiscal: d.fiscal,
    receivable: d.receivable,
  }
}

export const mockInvoices: InvoicesPort = {
  list(q) {
    const rows = docs
      .filter(
        (d) =>
          (!q.documentType || d.invoice.documentType === q.documentType) &&
          (!q.status || d.invoice.status === q.status) &&
          (!q.customerId || d.invoice.customerId === q.customerId) &&
          (q.requiresCorrection === undefined || d.invoice.requiresCorrection === q.requiresCorrection),
      )
      .map(toListItem)
    return simulate(paginate(rows, q), { items: [], nextCursor: null })
  },
  async get(id) {
    await simulate(null, null)
    const d = docs.find((x) => x.invoice.id === id)
    if (!d) throw notFound()
    return structuredClone(d.invoice)
  },
  async overview(id) {
    await simulate(null, null)
    const d = docs.find((x) => x.invoice.id === id)
    if (!d) throw notFound()
    // En «datos parciales» la parte de Hacienda sale no disponible, como cuando E-Invoice no responde.
    const fiscal = await simulateSecondary(d.fiscal, d.fiscal).catch((): FiscalPart => ({
      availability: 'unavailable',
    }))
    const i = toListItem(d).invoice
    return {
      invoice: {
        id: i.id,
        documentType: i.documentType,
        ...(i.number ? { number: i.number } : {}),
        status: i.status,
        requiresCorrection: i.requiresCorrection,
        ...(i.customerLegalName ? { customerLegalName: i.customerLegalName } : {}),
        currency: i.currency,
        total: i.total,
      },
      fiscal,
      receivable: d.receivable,
    }
  },
  async cancel(id, reason) {
    await simulate(null, null)
    const d = docs.find((x) => x.invoice.id === id)
    if (!d) throw notFound()
    if (d.invoice.status !== 'issued') {
      throw new ApiError({
        status: 409,
        type: 'urn:rdl:billing:problem:invoice-not-issued',
        title: 'La factura no está emitida',
        correlationId: fakeCorrelationId(),
      })
    }
    const now = new Date().toISOString()
    const next: MockDoc = {
      ...d,
      invoice: { ...d.invoice, status: 'cancelled', requiresCorrection: false, updatedAt: now },
      // Simula lo que haría Receivables al recibir InvoiceCancelled: la cuenta queda anulada en cero.
      receivable: d.receivable.balance
        ? {
            availability: 'available',
            balance: { ...d.receivable.balance, status: 'cancelled', balanceAmount: '0' },
          }
        : d.receivable,
      history: [
        ...d.history,
        { fromStatus: 'issued', toStatus: 'cancelled', reason, changedByUserId: 'u1', changedAt: now },
      ],
    }
    mockDocs.put(next)
    return structuredClone(next.invoice)
  },
  async history(id) {
    await simulate(null, null)
    const d = docs.find((x) => x.invoice.id === id)
    if (!d) throw notFound()
    return structuredClone(d.history)
  },
}
