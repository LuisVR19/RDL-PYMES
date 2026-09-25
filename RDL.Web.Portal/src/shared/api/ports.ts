import type { CurrentUser, Organization, PortalNotification } from './types'

/**
 * Puertos de datos del portal: lo único que las pantallas conocen. Cada módulo agrega su puerto aquí.
 * - `mock/` los implementa con datos simulados (esta etapa).
 * - En la etapa de cableado, `gateway/` los implementará contra el Portal Gateway (`/portal/v1/...`).
 * Las pantallas no cambian al cambiar de implementación.
 */
export interface SessionPort {
  currentUser(): Promise<CurrentUser>
  organizations(): Promise<Organization[]>
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
  notifications: NotificationsPort
  shell: ShellPort
}
