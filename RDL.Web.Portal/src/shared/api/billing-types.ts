import type { Currency } from '@/shared/money/money'

/**
 * Tipos de facturación y cobranza que usan las pantallas 7–17. Siguen el OpenAPI de Billing
 * (`RDL.Billing.API/api/openapi.yaml`) y el esqueleto de Receivables del repo de contratos: montos como string
 * decimal, instantes ISO en UTC, fechas de negocio YYYY-MM-DD.
 */

/** Una página por cursor (convenciones §9): `nextCursor` null = no hay más. */
export interface Page<T> {
  items: T[]
  nextCursor: string | null
}

export interface PageQuery {
  cursor?: string
  limit?: number
}

// --- Clientes (Billing) ---

export interface Identification {
  typeCode: string
  number: string
}

export interface Customer {
  id: string
  identification: Identification
  legalName: string
  tradeName?: string
  email?: string
  phone?: string
  address?: string
  isActive: boolean
  createdAt: string
  updatedAt: string
}

export interface CustomerInput {
  identification: Identification
  legalName: string
  tradeName?: string
  email?: string
  phone?: string
  address?: string
}

/** Campo ausente = no cambia; "" borra los opcionales. La identificación no se edita. */
export interface CustomerPatch {
  legalName?: string
  tradeName?: string
  email?: string
  phone?: string
  address?: string
  isActive?: boolean
}

export interface CustomerQuery extends PageQuery {
  /** Nombre legal, comercial o número de identificación (contiene). */
  q?: string
  /** undefined = todos. */
  active?: boolean
}

// --- Cobranza (Receivables · esqueleto del contrato) ---

export type ReceivableStatus = 'open' | 'partially_paid' | 'paid' | 'cancelled'

export interface Receivable {
  id: string
  sourceInvoiceId: string
  customerId: string
  documentNumber: string
  currency: Currency
  originalAmount: string
  balanceAmount: string
  issuedOn: string
  dueOn: string
  status: ReceivableStatus
}

export type PaymentStatus = 'posted' | 'voided'

export interface Payment {
  id: string
  customerId: string
  receivedOn: string
  amount: string
  currency: Currency
  reference?: string
  status: PaymentStatus
  createdAt: string
}

// --- Documentos (listado compuesto del Portal Gateway, pantalla 12) ---

export type DocumentType = 'invoice' | 'credit_note' | 'debit_note'
export type InvoiceStatus = 'draft' | 'issued' | 'cancelled'
export type FiscalStatus =
  'processing' | 'signed' | 'sent' | 'accepted' | 'rejected' | 'contingency' | 'error'

/**
 * Si el gateway pudo traer una parte secundaria (ADR 0004 del gateway): `absent` = la API dijo que no existe (un
 * borrador no tiene documento electrónico ni saldo); `unavailable` = no se pudo saber.
 */
export type Availability = 'available' | 'absent' | 'unavailable'

export interface FiscalPart {
  availability: Availability
  status?: { electronicDocumentId: string; status: FiscalStatus; haciendaStatusMessage?: string }
}

export interface BalancePart {
  availability: Availability
  balance?: {
    receivableId: string
    status: ReceivableStatus
    currency: Currency
    balanceAmount: string
    dueOn: string
  }
}

/** Una fila de `GET /portal/v1/invoices` (InvoicePage del OpenAPI del gateway). */
export interface InvoiceListItem {
  invoice: {
    id: string
    documentType: DocumentType
    number?: string
    status: InvoiceStatus
    requiresCorrection: boolean
    customerId: string
    customerLegalName?: string
    issuedAt?: string
    dueDate?: string
    currency: Currency
    total: string
    createdAt: string
  }
  fiscal: FiscalPart
  receivable: BalancePart
}

export interface InvoiceQuery extends PageQuery {
  documentType?: DocumentType
  status?: InvoiceStatus
  customerId?: string
  requiresCorrection?: boolean
  issuedFrom?: string
  issuedTo?: string
}

// --- Productos (Billing) ---

export interface ProductTax {
  taxTypeCode: string
  taxRateCode: string
}

export interface Product {
  id: string
  code: string
  description: string
  cabysCode: string
  unitOfMeasureCode: string
  unitPrice: string
  currency: Currency
  isService: boolean
  isActive: boolean
  taxes: ProductTax[]
  createdAt: string
  updatedAt: string
}

export interface ProductInput {
  code: string
  description: string
  cabysCode: string
  unitOfMeasureCode: string
  unitPrice: string
  currency: Currency
  isService: boolean
  taxes: ProductTax[]
}

/** Campo ausente = no cambia; `taxes` presente reemplaza la lista. */
export type ProductPatch = Partial<ProductInput> & { isActive?: boolean }

export interface ProductQuery extends PageQuery {
  /** Código o descripción (contiene). */
  q?: string
  active?: boolean
}

// --- Catálogos fiscales (E-Invoice · esqueleto `openapi/fiscal.yaml`) ---

export interface CatalogItem {
  code: string
  name: string
}

export interface CabysItem {
  code: string
  description: string
}

/** Una opción de impuesto del formulario: el par de códigos que guarda Billing y su nombre. */
export interface TaxOption {
  key: string
  label: string
  taxes: ProductTax[]
}

// --- Documento (detalle de Billing, pantallas 13–17) ---

export interface InvoiceLineTax {
  taxTypeCode: string
  taxRateCode?: string
  rate: string
  taxableBase: string
  amount: string
}

export interface InvoiceLine {
  lineNumber: number
  productId?: string
  productCode?: string
  cabysCode: string
  description: string
  unitOfMeasureCode: string
  isService: boolean
  quantity: string
  unitPrice: string
  discount: string
  discountReason?: string
  subtotal: string
  tax: string
  total: string
  taxes: InvoiceLineTax[]
}

/** CustomerSnapshot del contrato: los datos del cliente congelados al emitir. */
export interface CustomerSnapshot {
  customerId: string
  identification: Identification
  legalName: string
  email?: string
  phone?: string
  address?: string
}

export interface Invoice {
  id: string
  documentType: DocumentType
  number: string | null
  status: InvoiceStatus
  requiresCorrection: boolean
  fiscalRejectionReason?: string
  customerId: string
  customerSnapshot?: CustomerSnapshot
  branchId?: string
  saleConditionCode: string
  creditTermDays?: number
  issuedAt?: string
  dueDate?: string
  currency: Currency
  exchangeRate: string
  notes?: string
  lines: InvoiceLine[]
  subtotal: string
  discount: string
  tax: string
  exoneration: string
  total: string
  createdAt: string
  updatedAt: string
}

export interface StatusChange {
  fromStatus?: InvoiceStatus
  toStatus: InvoiceStatus
  reason?: string
  changedByUserId?: string
  changedAt: string
}

/** `GET /portal/v1/invoices/{id}/overview`: las tres cifras de la pantalla 15 (arquitectura 2.2). */
export interface InvoiceOverview {
  invoice: {
    id: string
    documentType: DocumentType
    number?: string
    status: InvoiceStatus
    requiresCorrection: boolean
    customerLegalName?: string
    currency: Currency
    total: string
  }
  fiscal: FiscalPart
  receivable: BalancePart
}
