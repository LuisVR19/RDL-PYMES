import type { Currency } from '@/shared/money/money'
import type { Role } from '@/shared/permissions/permissions'

/**
 * Tipos de vista que consumen las pantallas. En la etapa de cableado se generarán desde el OpenAPI del Portal
 * Gateway; hasta entonces se escriben a mano siguiendo los contratos (montos como string decimal, instantes ISO en
 * UTC, fechas de negocio YYYY-MM-DD).
 */

export type FiscalEnvironment = 'test' | 'prod'

/**
 * Una organización del usuario. Los campos opcionales no los da la lista de membresías: salen del detalle de la
 * organización activa (`timezone`, `defaultCurrency`, `identification`) o de E-Invoice (`environment`), así que
 * pueden faltar. La interfaz no inventa un valor cuando falta: no lo muestra.
 */
export interface Organization {
  id: string
  legalName: string
  initials: string
  role: Role
  identification?: string
  environment?: FiscalEnvironment
  timezone?: string
  defaultCurrency?: Currency
}

export interface Memberships {
  items: Organization[]
  /** null: el usuario todavía no tiene organización activa (pantalla 3). */
  activeOrganizationId: string | null
}

export interface CurrentUser {
  id: string
  fullName: string
  email: string
  initials: string
}

export type NotificationTone = 'success' | 'danger' | 'info' | 'warning'

export interface PortalNotification {
  id: string
  tone: NotificationTone
  title: string
  body: string
  occurredAt: string
  unread: boolean
  action?: { label: string; to: string }
}

/** Error de la API con la forma de Problem Details (RFC 9457) que usan todas las APIs. */
export class ApiError extends Error {
  readonly status: number
  readonly type: string
  readonly correlationId: string
  /** Errores por campo de un 422 `validation` (convenciones §5): `field` es el nombre del campo JSON. */
  readonly errors: FieldError[]

  constructor(p: {
    status: number
    type: string
    title: string
    correlationId: string
    errors?: FieldError[]
  }) {
    super(p.title)
    this.name = 'ApiError'
    this.status = p.status
    this.type = p.type
    this.correlationId = p.correlationId
    this.errors = p.errors ?? []
  }

  /** El `type` termina en ese código (`urn:rdl:<servicio>:problem:<código>`), de cualquier servicio. */
  is(code: string): boolean {
    return this.type.endsWith(`:problem:${code}`)
  }
}

export interface FieldError {
  field: string
  message: string
}

/** Pantalla 3 · lo que se manda a `POST /portal/v1/organizations` (Platform · createOrganization). */
export interface NewOrganization {
  legalName: string
  tradeName?: string
  identificationTypeCode: string
  identificationNumber: string
  email: string
  phone?: string
  timezone: string
}

/** Pantalla 4 · respuesta de `POST /portal/v1/invitations/{token}/accept`. */
export interface AcceptedInvitation {
  organizationId: string
  role: Role
}
