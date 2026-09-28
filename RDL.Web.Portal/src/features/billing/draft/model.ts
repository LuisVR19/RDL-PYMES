import { Big } from 'big.js'
import type {
  DocumentType,
  Identification,
  Invoice,
  InvoiceDraftInput,
  InvoiceDraftPatch,
  InvoiceLineInput,
  Product,
} from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { parseMoneyInput, type Currency } from '@/shared/money/money'
import { CASH, CREDIT } from '@/shared/saleConditions'

/**
 * Modelo del borrador (pantallas 13 y 16): lo que el usuario escribe, cómo se valida y cómo se convierte en el
 * `InvoiceDraftInput` del contrato. El portal NUNCA calcula un total: los montos de línea y del documento son los que
 * devuelve Billing al guardar.
 */

/** Moneda local de Costa Rica: el tipo de cambio es 1 y Billing lo asume. */
export const LOCAL_CURRENCY: Currency = 'CRC'

export interface DraftCustomer {
  id: string
  legalName: string
  identification?: Identification
}

export interface DraftLine {
  /** Clave estable de la fila en pantalla (no viaja). */
  key: string
  productId: string
  description: string
  code?: string
  cabys: string
  /** Moneda y precio del producto: si la moneda no es la del documento, el precio es obligatorio. */
  productCurrency: Currency
  quantity: string
  unitPrice: string
  discount: string
  discountReason: string
}

export interface DraftForm {
  customer: DraftCustomer | null
  branchId: string
  currency: Currency
  exchangeRate: string
  saleCondition: string
  creditTermDays: string
  notes: string
  lines: DraftLine[]
}

export type LineField = 'quantity' | 'unitPrice' | 'discount' | 'discountReason'
export type LineIssues = Partial<Record<LineField, string>>

export interface FormIssues {
  customer?: string
  exchangeRate?: string
  creditTermDays?: string
  lines: Record<string, LineIssues>
}

export function emptyForm(currency: Currency = LOCAL_CURRENCY): DraftForm {
  return {
    customer: null,
    branchId: '',
    currency,
    exchangeRate: '',
    saleCondition: CASH,
    creditTermDays: '30',
    notes: '',
    lines: [],
  }
}

let keySeq = 0
export const newLineKey = () => `l${++keySeq}`

/** Número decimal del contrato → como se escribe en pantalla («18500.5» → «18500,5»). */
export const toInputText = (value: string) => value.replace('.', ',')

export function lineFromProduct(p: Product, currency: Currency): DraftLine {
  return {
    key: newLineKey(),
    productId: p.id,
    description: p.description,
    code: p.code,
    cabys: p.cabysCode,
    productCurrency: p.currency,
    quantity: '1',
    // En otra moneda el precio del producto no sirve: se pide el de la moneda del documento.
    unitPrice: p.currency === currency ? toInputText(p.unitPrice) : '',
    discount: '',
    discountReason: '',
  }
}

/** El formulario de un borrador guardado (o de una nota) a partir de lo que devolvió Billing. */
export function formFromInvoice(inv: Invoice, customer: DraftCustomer | null): DraftForm {
  return {
    customer,
    branchId: inv.branchId ?? '',
    currency: inv.currency,
    exchangeRate: inv.currency === LOCAL_CURRENCY ? '' : toInputText(inv.exchangeRate),
    saleCondition: inv.saleConditionCode,
    creditTermDays: inv.creditTermDays === undefined ? '30' : String(inv.creditTermDays),
    notes: inv.notes ?? '',
    lines: inv.lines.flatMap((l) =>
      l.productId
        ? [
            {
              key: newLineKey(),
              productId: l.productId,
              description: l.description,
              ...(l.productCode ? { code: l.productCode } : {}),
              cabys: l.cabysCode,
              productCurrency: inv.currency,
              quantity: toInputText(l.quantity),
              unitPrice: toInputText(l.unitPrice),
              discount: l.discount === '0' ? '' : toInputText(l.discount),
              discountReason: l.discountReason ?? '',
            },
          ]
        : [],
    ),
  }
}

const QUANTITY = /^(0|[1-9]\d{0,12})(\.\d{1,3})?$/

/** «2,5» → «2.5» (Quantity del contrato: hasta 3 decimales y distinta de cero), o null. */
export function parseQuantity(text: string): string | null {
  const s = text.replace(/[\s  ]/g, '').replace(',', '.')
  if (!/^\d+(\.\d+)?$/.test(s)) return null
  const normalized = new Big(s).toFixed()
  return QUANTITY.test(normalized) && new Big(normalized).gt(0) ? normalized : null
}

/** Tipo de cambio: como un monto, pero distinto de cero. */
export function parseRate(text: string): string | null {
  const v = parseMoneyInput(text)
  return v !== null && new Big(v).gt(0) ? v : null
}

export function lineIssues(l: DraftLine, currency: Currency): LineIssues {
  const out: LineIssues = {}
  if (parseQuantity(l.quantity) === null) out.quantity = t('draft.issue.quantity')
  if (l.unitPrice.trim() === '') {
    if (l.productCurrency !== currency) out.unitPrice = t('draft.issue.priceCurrency', { currency })
  } else if (parseMoneyInput(l.unitPrice) === null) {
    out.unitPrice = t('draft.issue.amount')
  }
  const discount = l.discount.trim() === '' ? '0' : parseMoneyInput(l.discount)
  if (discount === null) out.discount = t('draft.issue.amount')
  else if (new Big(discount).gt(0) && !l.discountReason.trim())
    out.discountReason = t('invoice.discountReason')
  return out
}

export function formIssues(f: DraftForm): FormIssues {
  const lines: Record<string, LineIssues> = {}
  for (const l of f.lines) {
    const i = lineIssues(l, f.currency)
    if (Object.keys(i).length > 0) lines[l.key] = i
  }
  const days = f.creditTermDays.trim()
  return {
    ...(f.customer ? {} : { customer: t('draft.issue.customer') }),
    ...(f.currency !== LOCAL_CURRENCY && parseRate(f.exchangeRate) === null
      ? { exchangeRate: t('draft.issue.exchangeRate') }
      : {}),
    ...(f.saleCondition === CREDIT && !(/^\d{1,4}$/.test(days) && Number(days) <= 3650)
      ? { creditTermDays: t('draft.issue.creditDays') }
      : {}),
    lines,
  }
}

export function hasIssues(i: FormIssues): boolean {
  return Boolean(i.customer || i.exchangeRate || i.creditTermDays || Object.keys(i.lines).length > 0)
}

/** La lista de «Revise estos datos antes de emitir» (prototipo: `ff.errList`). */
export function issueList(f: DraftForm, i: FormIssues, requireLines = true): string[] {
  const out: string[] = []
  if (i.customer) out.push(i.customer)
  if (i.exchangeRate) out.push(i.exchangeRate)
  if (i.creditTermDays) out.push(i.creditTermDays)
  if (requireLines && f.lines.length === 0) out.push(t('draft.issue.noLines'))
  f.lines.forEach((l, n) => {
    for (const msg of Object.values(i.lines[l.key] ?? {})) out.push(t('draft.issue.line', { n: n + 1, msg }))
  })
  return out
}

function lineInput(l: DraftLine): InvoiceLineInput {
  const discount = l.discount.trim() === '' ? '0' : (parseMoneyInput(l.discount) ?? '0')
  const price = l.unitPrice.trim() === '' ? null : parseMoneyInput(l.unitPrice)
  return {
    productId: l.productId,
    quantity: parseQuantity(l.quantity) ?? '0',
    ...(price !== null ? { unitPrice: price } : {}),
    ...(discount !== '0' ? { discount, discountReason: l.discountReason.trim() } : {}),
  }
}

/**
 * El cuerpo que se manda a Billing, o null si todavía no se puede guardar (falta el cliente o hay un dato mal
 * escrito). Un borrador sin líneas sí se guarda: lo que exige al menos una es emitir.
 */
export function toDraftInput(
  f: DraftForm,
  documentType: DocumentType,
  reference?: { invoiceId: string; reason: string },
): InvoiceDraftInput | null {
  if (!f.customer || hasIssues(formIssues(f))) return null
  const credit = f.saleCondition === CREDIT
  return {
    documentType,
    customerId: f.customer.id,
    ...(f.branchId ? { branchId: f.branchId } : {}),
    ...(reference ? { referencedInvoiceId: reference.invoiceId } : {}),
    ...(reference?.reason ? { referenceReason: reference.reason } : {}),
    saleConditionCode: f.saleCondition,
    ...(credit ? { creditTermDays: Number(f.creditTermDays.trim()) } : {}),
    currency: f.currency,
    ...(f.currency !== LOCAL_CURRENCY ? { exchangeRate: parseRate(f.exchangeRate) ?? '1' } : {}),
    ...(f.notes.trim() ? { notes: f.notes.trim() } : {}),
    lines: f.lines.map(lineInput),
  }
}

/**
 * El PATCH que lleva un borrador ya guardado a lo que está en pantalla. Manda el encabezado completo y reemplaza las
 * líneas; lo que se quitó viaja como null o "" para que Billing lo borre.
 */
export function toDraftPatch(input: InvoiceDraftInput): InvoiceDraftPatch {
  return {
    ...(input.referenceReason !== undefined ? { referenceReason: input.referenceReason } : {}),
    customerId: input.customerId,
    branchId: input.branchId ?? null,
    saleConditionCode: input.saleConditionCode,
    creditTermDays: input.creditTermDays ?? null,
    currency: input.currency,
    exchangeRate: input.exchangeRate ?? '1',
    notes: input.notes ?? '',
    lines: input.lines ?? [],
  }
}

/**
 * Errores por campo de un 422 de Billing (`lines[2].discountReason`, `exchangeRate`) → los de la pantalla. Las líneas
 * se ubican por posición: Billing numera en el orden en que se mandaron.
 */
export function serverIssues(errors: { field: string; message: string }[], f: DraftForm): FormIssues {
  const out: FormIssues = { lines: {} }
  for (const e of errors) {
    const m = /^lines\[(\d+)\]\.(\w+)$/.exec(e.field)
    if (m) {
      const line = f.lines[Number(m[1])]
      const field = m[2] as LineField
      if (line && ['quantity', 'unitPrice', 'discount', 'discountReason'].includes(field)) {
        out.lines[line.key] = { ...out.lines[line.key], [field]: e.message }
      }
    } else if (e.field === 'customerId') out.customer = e.message
    else if (e.field === 'exchangeRate') out.exchangeRate = e.message
    else if (e.field === 'creditTermDays') out.creditTermDays = e.message
  }
  return out
}
