import type {
  Branch,
  CabysItem,
  CatalogItem,
  Customer,
  CustomerInput,
  CustomerPatch,
  CustomerQuery,
  Invoice,
  InvoiceDraftInput,
  InvoiceDraftPatch,
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
  ReceivablesSummary,
  StatusChange,
  TaxOption,
} from './billing-types'
import type {
  BranchInput,
  BranchPatch,
  Invitation,
  InvitationQuery,
  Member,
  MemberPatch,
  MemberQuery,
  NewInvitation,
  OrganizationDetail,
  OrganizationPatch,
} from './admin-types'
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
  /** Pagos registrados, los más recientes primero (Inicio y pantalla 25). */
  payments(query?: PageQuery): Promise<Page<Payment>>
  /** Saldo abierto y vencido a la fecha de corte `asOf` (fecha de negocio), para Inicio. */
  summary(asOf: string): Promise<ReceivablesSummary>
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
   * Anula una factura emitida con su motivo (`POST /v1/invoices/{id}/cancel`, owner y admin). La cuenta por cobrar
   * la ajusta Receivables al recibir `InvoiceCancelled`, no el portal.
   */
  cancel(id: string, reason: string, idempotencyKey: string): Promise<Invoice>
  /**
   * Crea un borrador (pantallas 13 y 16). La respuesta trae las líneas y los totales calculados por Billing: es lo
   * único que el portal muestra como total. Una nota lleva su factura (emitida, del mismo cliente y moneda) y motivo.
   */
  createDraft(input: InvoiceDraftInput, idempotencyKey: string): Promise<Invoice>
  /** Edita un borrador y recalcula (PATCH; `lines` presente las reemplaza). 409 si ya no es borrador. */
  updateDraft(id: string, patch: InvoiceDraftPatch): Promise<Invoice>
  /** Descarta un borrador (se borra; queda en la auditoría). Nunca un documento emitido (409). */
  discardDraft(id: string): Promise<void>
  /**
   * Emite: asigna número y deja el evento para Hacienda y Cobranza, sin esperarlos. La misma `Idempotency-Key`
   * responde lo mismo, así que reintentar tras un error no duplica el documento.
   */
  issue(id: string, idempotencyKey: string): Promise<Invoice>
}

/** Sucursales de la organización activa (Platform). El borrador las ofrece; la pantalla 29 las administra. */
export interface BranchesPort {
  list(query?: PageQuery & { active?: boolean }): Promise<Page<Branch>>
  /** 409 si el código ya existe en la organización. */
  create(input: BranchInput, idempotencyKey: string): Promise<Branch>
  update(id: string, patch: BranchPatch): Promise<Branch>
}

/** Organización activa (pantalla 28). Cualquier rol la lee; solo owner y admin la editan. */
export interface OrganizationPort {
  current(): Promise<OrganizationDetail>
  update(patch: OrganizationPatch): Promise<OrganizationDetail>
}

/**
 * Miembros de la organización activa (pantalla 30). Platform hace cumplir las reglas: solo un owner gestiona
 * owners (403 `owner-required`) y nunca queda la organización sin owner activo (409 `last-owner`).
 */
export interface MembersPort {
  list(query?: MemberQuery): Promise<Page<Member>>
  update(userId: string, patch: MemberPatch): Promise<Member>
}

/**
 * Invitaciones (pantalla 31). El token en claro solo viene en la primera respuesta de `create`; un reintento con
 * la misma `Idempotency-Key` devuelve la misma invitación sin token. `revoke` es idempotente.
 */
export interface InvitationsPort {
  list(query?: InvitationQuery): Promise<Page<Invitation>>
  create(input: NewInvitation, idempotencyKey: string): Promise<Invitation>
  revoke(id: string): Promise<void>
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
  branches: BranchesPort
  organization: OrganizationPort
  members: MembersPort
  invitations: InvitationsPort
  notifications: NotificationsPort
  shell: ShellPort
}
