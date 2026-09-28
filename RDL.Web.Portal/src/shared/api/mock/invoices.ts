import { Big } from 'big.js'
import { CUSTOMERS, INVOICES } from '@/mocks/billing'
import { addDays, DEFAULT_TZ, todayIn } from '@/shared/dates/dates'
import type {
  BalancePart,
  FiscalPart,
  Invoice,
  InvoiceDraftInput,
  InvoiceDraftPatch,
  InvoiceLine,
  InvoiceListItem,
  StatusChange,
} from '../billing-types'
import type { InvoicesPort } from '../ports'
import { ApiError } from '../types'
import { mockCustomerById, paginate } from './billing'
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
  createdByKey.clear()
  issuedByKey.clear()
  references.clear()
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
  async createDraft(input, idempotencyKey) {
    await simulate(null, null)
    const repeated = createdByKey.get(idempotencyKey)
    if (repeated) return structuredClone(repeated)
    const now = new Date().toISOString()
    const draft = draftFrom(
      {
        id: `b${Date.now()}`,
        documentType: input.documentType,
        number: null,
        status: 'draft',
        requiresCorrection: false,
        customerId: input.customerId,
        saleConditionCode: input.saleConditionCode,
        currency: input.currency,
        exchangeRate: input.exchangeRate ?? '1',
        lines: [],
        subtotal: '0',
        discount: '0',
        tax: '0',
        exoneration: '0',
        total: '0',
        createdAt: now,
        updatedAt: now,
      },
      input,
    )
    if (input.referencedInvoiceId) references.set(draft.id, input.referencedInvoiceId)
    mockDocs.put({ invoice: draft, ...NO_PARTS, history: [] })
    createdByKey.set(idempotencyKey, draft)
    return structuredClone(draft)
  },
  async updateDraft(id, patch) {
    await simulate(null, null)
    const d = draftOrThrow(id)
    const next = draftFrom({ ...d.invoice, updatedAt: new Date().toISOString() }, patch)
    mockDocs.put({ ...d, invoice: next })
    return structuredClone(next)
  },
  async discardDraft(id) {
    await simulate(null, null)
    draftOrThrow(id)
    mockDocs.remove(id)
  },
  async issue(id, idempotencyKey) {
    await simulate(null, null)
    const repeated = issuedByKey.get(idempotencyKey)
    if (repeated) return structuredClone(repeated)
    const d = draftOrThrow(id)
    if (d.invoice.lines.length === 0) {
      throw problem(422, 'invoice-without-lines', 'Documento sin líneas')
    }
    const customer = mockCustomerById(d.invoice.customerId)
    if (!customer) throw notFound()
    const now = new Date()
    const issuedAt = now.toISOString()
    const issueDate = todayIn(DEFAULT_TZ, now)
    const invoice: Invoice = {
      ...d.invoice,
      status: 'issued',
      number: nextNumber(d.invoice.documentType),
      issuedAt,
      dueDate: addDays(issueDate, d.invoice.creditTermDays ?? 0),
      customerSnapshot: {
        customerId: customer.id,
        identification: customer.identification,
        legalName: customer.legalName,
        ...(customer.email ? { email: customer.email } : {}),
        ...(customer.phone ? { phone: customer.phone } : {}),
        ...(customer.address ? { address: customer.address } : {}),
      },
      updatedAt: issuedAt,
    }
    // Lo que harían E-Invoice y Receivables al recibir el evento: un documento en proceso y, para una factura, su
    // cuenta por cobrar. Las notas ajustan la cuenta de la factura de referencia; aquí no se simula ese ajuste.
    mockDocs.put({
      invoice,
      fiscal: {
        availability: 'available',
        status: { electronicDocumentId: `e${invoice.id}`, status: 'processing' },
      },
      receivable:
        invoice.documentType === 'invoice'
          ? {
              availability: 'available',
              balance: {
                receivableId: `r${invoice.id}`,
                status: 'open',
                currency: invoice.currency,
                balanceAmount: invoice.total,
                dueOn: invoice.dueDate ?? issueDate,
              },
            }
          : { availability: 'absent' },
      history: [{ fromStatus: 'draft', toStatus: 'issued', changedByUserId: 'u1', changedAt: issuedAt }],
    })
    issuedByKey.set(idempotencyKey, invoice)
    return structuredClone(invoice)
  },
}

// --- Borradores simulados (pantallas 13, 14 y 16) ---

const NO_PARTS = {
  fiscal: { availability: 'absent' },
  receivable: { availability: 'absent' },
} satisfies Pick<MockDoc, 'fiscal' | 'receivable'>

/** La misma Idempotency-Key responde lo mismo, como en Billing. */
const createdByKey = new Map<string, Invoice>()
const issuedByKey = new Map<string, Invoice>()
/** Factura de referencia de cada nota (el `Invoice` del contrato no la trae). */
const references = new Map<string, string>()

/** Solo pruebas y revisión: la factura de referencia de una nota simulada. */
export function mockReferenceOf(noteId: string): string | undefined {
  return references.get(noteId)
}

const PREFIX = { invoice: 'FAC', credit_note: 'NC', debit_note: 'ND' } as const

function nextNumber(type: Invoice['documentType']): string {
  const used = docs
    .filter((d) => d.invoice.documentType === type && d.invoice.number)
    .map((d) => Number.parseInt((d.invoice.number ?? '').replace(/\D/g, ''), 10))
  return `${PREFIX[type]}-${String(Math.max(0, ...used) + 1).padStart(7, '0')}`
}

const problem = (
  status: number,
  code: string,
  title: string,
  errors?: { field: string; message: string }[],
) =>
  new ApiError({
    status,
    type: `urn:rdl:billing:problem:${code}`,
    title,
    correlationId: fakeCorrelationId(),
    ...(errors ? { errors } : {}),
  })

function draftOrThrow(id: string): MockDoc {
  const d = docs.find((x) => x.invoice.id === id)
  if (!d) throw notFound()
  if (d.invoice.status !== 'draft') {
    throw problem(409, 'invoice-not-draft', 'El documento no es un borrador')
  }
  return d
}

/** Tarifas ilustrativas por código (las de `MOCK_TAX_OPTIONS`). TODO(fiscal): catálogo oficial. */
const RATE: Record<string, string> = { '08': '13', '04': '4' }

/**
 * Aplica un alta o un PATCH a un borrador y recalcula como lo haría Billing: valida cliente y productos, toma la
 * descripción, el CABYS y los impuestos del producto, y redondea cada campo de línea a 5 decimales (D2).
 */
function draftFrom(base: Invoice, change: InvoiceDraftInput | InvoiceDraftPatch): Invoice {
  const customerId = change.customerId ?? base.customerId
  const customer = mockCustomerById(customerId)
  if (!customer) throw notFound()
  if (!customer.isActive) throw problem(422, 'customer-inactive', 'Cliente inactivo')

  const next: Invoice = { ...base, customerId }
  if (change.saleConditionCode !== undefined) next.saleConditionCode = change.saleConditionCode
  if (change.currency !== undefined) next.currency = change.currency
  if (change.exchangeRate !== undefined) next.exchangeRate = change.exchangeRate
  if (change.notes !== undefined) {
    if (change.notes) next.notes = change.notes
    else delete next.notes
  }
  if (change.branchId !== undefined) {
    if (change.branchId) next.branchId = change.branchId
    else delete next.branchId
  }
  if (change.creditTermDays !== undefined) {
    if (change.creditTermDays === null) delete next.creditTermDays
    else next.creditTermDays = change.creditTermDays
  }
  if (next.currency !== 'CRC' && next.exchangeRate === '1') {
    throw problem(422, 'validation', 'Datos inválidos', [
      { field: 'exchangeRate', message: 'Indique el tipo de cambio.' },
    ])
  }
  if (!change.lines) return withTotals(next, base.lines)

  const errors: { field: string; message: string }[] = []
  const lines = change.lines.map((l, i) => {
    const p = mockProductById(l.productId)
    if (!p || !p.isActive) throw notFound()
    if (!l.unitPrice && p.currency !== next.currency) {
      errors.push({
        field: `lines[${i}].unitPrice`,
        message: 'Indique el precio en la moneda del documento.',
      })
    }
    const discount = l.discount ?? '0'
    if (new Big(discount).gt(0) && !l.discountReason?.trim()) {
      errors.push({
        field: `lines[${i}].discountReason`,
        message: 'El motivo es obligatorio cuando hay descuento.',
      })
    }
    const rate = RATE[p.taxes[0]?.taxRateCode ?? ''] ?? '0'
    return mockLine(
      i + 1,
      {
        id: p.id,
        code: p.code,
        cabys: p.cabysCode,
        description: p.description,
        unit: p.unitOfMeasureCode,
        isService: p.isService,
      },
      l.quantity,
      l.unitPrice ?? p.unitPrice,
      discount,
      l.discountReason?.trim() || undefined,
      rate,
    )
  })
  if (errors.length > 0) throw problem(422, 'validation', 'Datos inválidos', errors)
  return withTotals(next, lines)
}
