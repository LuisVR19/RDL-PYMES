import type {
  CabysItem,
  CatalogItem,
  Customer,
  CustomerInput,
  CustomerPatch,
  CustomerQuery,
  Invoice,
  InvoiceListItem,
  InvoiceOverview,
  InvoiceQuery,
  Page,
  PageQuery,
  Payment,
  Product,
  ProductInput,
  ProductPatch,
  ProductQuery,
  Receivable,
  StatusChange,
  TaxOption,
} from './billing-types'
import type {
  AcceptedInvitation,
  CurrentUser,
  Memberships,
  NewOrganization,
  PortalNotification,
} from './types'

/**
 * Puertos de datos del portal: lo único que las pantallas conocen. Cada módulo agrega su puerto aquí.
 * - `mock/` los implementa con datos simulados (esta etapa).
 * - `gateway/` los implementa contra el Portal Gateway (`/portal/v1/...`); `VITE_DATA_SOURCE` elige.
 * Las pantallas no cambian al cambiar de implementación.
 */
export interface SessionPort {
  currentUser(): Promise<CurrentUser>
  /** Organizaciones del usuario y cuál es la activa (la del `org_id` del token). */
  organizations(): Promise<Memberships>
  /**
   * Cambia la organización activa en Platform. Quien llama refresca después el token: hasta entonces el token
   * sigue trayendo el `org_id` anterior y las APIs responden con esa organización.
   */
  activateOrganization(orgId: string): Promise<void>
}

/**
 * Acceso (pantallas 3 y 4). Los dos son comandos con `Idempotency-Key`: la genera la pantalla, una por intento de
 * envío, y la reutiliza si el usuario reintenta el mismo envío (P8b 4.3).
 */
export interface AccessPort {
  createOrganization(input: NewOrganization, idempotencyKey: string): Promise<{ id: string }>
  acceptInvitation(token: string, idempotencyKey: string): Promise<AcceptedInvitation>
}

/** Clientes (pantallas 7–9). `create` lleva `Idempotency-Key`; `update` es un PATCH (idempotente por diseño). */
export interface CustomersPort {
  list(query: CustomerQuery): Promise<Page<Customer>>
  get(id: string): Promise<Customer>
  create(input: CustomerInput, idempotencyKey: string): Promise<Customer>
  update(id: string, patch: CustomerPatch): Promise<Customer>
}

/** Productos y servicios (pantallas 10–11). */
export interface ProductsPort {
  list(query: ProductQuery): Promise<Page<Product>>
  get(id: string): Promise<Product>
  create(input: ProductInput, idempotencyKey: string): Promise<Product>
  update(id: string, patch: ProductPatch): Promise<Product>
}

/**
 * Catálogos fiscales para los formularios (E-Invoice, P5). Todavía no existen: con el gateway responden 503 y los
 * formularios degradan (CABYS a mano, unidad como código, sin impuesto). TODO(fiscal): contenido oficial.
 */
export interface CatalogsPort {
  searchCabys(q: string): Promise<CabysItem[]>
  unitsOfMeasure(): Promise<CatalogItem[]>
  taxOptions(): Promise<TaxOption[]>
}

/**
 * Cobranza vista desde facturación (pantallas 7 y 9). Es de Receivables (P6), que todavía no existe: con el
 * gateway, estas llamadas responden 503 y las pantallas muestran «datos parciales».
 */
export interface ReceivablesPort {
  byCustomer(customerId: string, query?: PageQuery): Promise<Page<Receivable>>
  paymentsByCustomer(customerId: string, query?: PageQuery): Promise<Page<Payment>>
  /**
   * Saldo por cobrar de varios clientes, para la columna de la pantalla 7.
   * TODO(api): no hay ruta por lote de saldos por cliente en el contrato (bff-internal solo la tiene por
   * factura). Con el gateway rechaza; la columna muestra «No disponible» en vez de hacer una llamada por fila.
   */
  balancesByCustomer(customerIds: string[]): Promise<Record<string, Receivable[]>>
}

/**
 * Documentos (pantallas 9, 12–17). `list` es la composición del gateway: la página de Billing con el estado de
 * Hacienda y el saldo de cada fila, una llamada por API por página (incremento 5 del gateway).
 */
export interface InvoicesPort {
  list(query: InvoiceQuery): Promise<Page<InvoiceListItem>>
  /** Detalle de Billing: líneas, impuestos y totales tal como los calculó el servidor. */
  get(id: string): Promise<Invoice>
  /** Vista transversal del gateway: total de Billing, estado de Hacienda y saldo (cada parte degrada sola). */
  overview(id: string): Promise<InvoiceOverview>
  history(id: string): Promise<StatusChange[]>
  /**
   * Anula una factura emitida con su motivo (`POST /v1/invoices/{id}/cancel` del contrato; Billing lo implementa en
   * F5). La cuenta por cobrar la ajusta Receivables al recibir `InvoiceCancelled`, no el portal.
   */
  cancel(id: string, reason: string, idempotencyKey: string): Promise<Invoice>
}

export interface NotificationsPort {
  list(orgId: string): Promise<PortalNotification[]>
}

/** Datos del armazón: contadores del menú lateral. */
export interface ShellPort {
  /** Documentos de la bandeja de Hacienda que requieren atención (rechazados, contingencia, con error). */
  inboxAttentionCount(orgId: string): Promise<number>
}

export interface DataSource {
  session: SessionPort
  access: AccessPort
  customers: CustomersPort
  products: ProductsPort
  catalogs: CatalogsPort
  receivables: ReceivablesPort
  invoices: InvoicesPort
  notifications: NotificationsPort
  shell: ShellPort
}
