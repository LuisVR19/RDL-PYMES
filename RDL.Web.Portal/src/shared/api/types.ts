import type { Currency } from '@/shared/money/money'
import type { Role } from '@/shared/permissions/permissions'

/**
 * Tipos de vista que consumen las pantallas. En la etapa de cableado se generarán desde el OpenAPI del Portal
 * Gateway; hasta entonces se escriben a mano siguiendo los contratos (montos como string decimal, instantes ISO en
 * UTC, fechas de negocio YYYY-MM-DD).
 */

export type FiscalEnvironment = 'test' | 'prod'

export interface Organization {
  id: string
  legalName: string
  initials: string
  identification: string
  role: Role
  environment: FiscalEnvironment
  timezone: string
  defaultCurrency: Currency
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

  constructor(p: { status: number; type: string; title: string; correlationId: string }) {
    super(p.title)
    this.name = 'ApiError'
    this.status = p.status
    this.type = p.type
    this.correlationId = p.correlationId
  }
}
