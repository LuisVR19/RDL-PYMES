import type {
  CabysItem,
  CatalogItem,
  Customer,
  Invoice,
  InvoiceListItem,
  InvoiceOverview,
  Page,
  Payment,
  Product,
  Receivable,
  StatusChange,
} from '../billing-types'
import type { CatalogsPort, CustomersPort, InvoicesPort, ProductsPort, ReceivablesPort } from '../ports'
import { ApiError } from '../types'
import type { GatewayHttp } from './http'

/**
 * Facturación y cobranza contra el Portal Gateway. Clientes es paso directo a Billing (su OpenAPI); cobranza es
 * paso directo a Receivables, que hoy no existe: el gateway responde 503 `upstream-not-configured` y las
 * pantallas lo muestran como datos parciales.
 */

const bool = (v: boolean | undefined) => (v === undefined ? undefined : String(v))
const num = (v: number | undefined) => (v === undefined ? undefined : String(v))

export function createCustomersPort(http: GatewayHttp): CustomersPort {
  return {
    list: (q) =>
      http.get<Page<Customer>>('/portal/v1/customers', {
        q: q.q?.trim() || undefined,
        active: bool(q.active),
        cursor: q.cursor,
        limit: num(q.limit),
      }),
    get: (id) => http.get<Customer>(`/portal/v1/customers/${encodeURIComponent(id)}`),
    create: (input, idempotencyKey) =>
      http.send<Customer>('POST', '/portal/v1/customers', input, { idempotencyKey }),
    update: (id, patch) =>
      http.send<Customer>('PATCH', `/portal/v1/customers/${encodeURIComponent(id)}`, patch),
  }
}

export function createReceivablesPort(http: GatewayHttp): ReceivablesPort {
  return {
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
  }
}

export function createInvoicesPort(http: GatewayHttp): InvoicesPort {
  return {
    list: (q) =>
      http.get<Page<InvoiceListItem>>('/portal/v1/invoices', {
        documentType: q.documentType,
        status: q.status,
        customerId: q.customerId,
        requiresCorrection: bool(q.requiresCorrection),
        issuedFrom: q.issuedFrom,
        issuedTo: q.issuedTo,
        cursor: q.cursor,
        limit: num(q.limit),
      }),
    get: (id) => http.get<Invoice>(`/portal/v1/invoices/${encodeURIComponent(id)}`),
    overview: (id) => http.get<InvoiceOverview>(`/portal/v1/invoices/${encodeURIComponent(id)}/overview`),
    history: (id) => http.get<StatusChange[]>(`/portal/v1/invoices/${encodeURIComponent(id)}/history`),
    // TODO(api): Billing implementa `cancel` en F5 y el gateway todavía no declara la ruta: hoy responde 404.
    cancel: (id, reason, idempotencyKey) =>
      http.send<Invoice>(
        'POST',
        `/portal/v1/invoices/${encodeURIComponent(id)}/cancel`,
        { reason },
        { idempotencyKey },
      ),
  }
}

export function createProductsPort(http: GatewayHttp): ProductsPort {
  return {
    list: (q) =>
      http.get<Page<Product>>('/portal/v1/products', {
        q: q.q?.trim() || undefined,
        active: bool(q.active),
        cursor: q.cursor,
        limit: num(q.limit),
      }),
    get: (id) => http.get<Product>(`/portal/v1/products/${encodeURIComponent(id)}`),
    create: (input, idempotencyKey) =>
      http.send<Product>('POST', '/portal/v1/products', input, { idempotencyKey }),
    update: (id, patch) =>
      http.send<Product>('PATCH', `/portal/v1/products/${encodeURIComponent(id)}`, patch),
  }
}

/** Catálogos de E-Invoice (`openapi/fiscal.yaml`). Sin E-Invoice desplegada, el gateway responde 503. */
export function createCatalogsPort(http: GatewayHttp): CatalogsPort {
  return {
    searchCabys: (q) => http.get<CabysItem[]>('/portal/v1/catalogs/cabys', { q, limit: '20' }),
    unitsOfMeasure: () => http.get<CatalogItem[]>('/portal/v1/catalogs/units-of-measure'),
    taxOptions: async () => {
      // TODO(fiscal): Billing guarda el par (taxTypeCode, taxRateCode), pero los catálogos `tax-types` y
      // `tax-rates` del contrato no dicen qué tarifa va con qué tipo. Hasta que el contrato lo defina, el portal
      // no arma pares por su cuenta: el formulario solo ofrece «sin impuesto».
      throw new ApiError({
        status: 501,
        type: 'urn:rdl:portal:problem:not-in-contract',
        title: 'Pares de impuesto',
        correlationId: '',
      })
    },
  }
}
