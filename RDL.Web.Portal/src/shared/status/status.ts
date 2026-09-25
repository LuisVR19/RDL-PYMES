import {
  Check,
  Circle,
  CircleAlert,
  CircleDashed,
  CircleDot,
  CircleHelp,
  CircleSlash,
  Clock,
  Pause,
  TriangleAlert,
  X,
  type LucideIcon,
} from 'lucide-react'

/**
 * Mapeo único estado → etiqueta, tono e icono (design/pantallas.md, «Mapeo de estados»). Todas las pantallas usan
 * esta tabla vía `StatusBadge`: el color nunca va solo, siempre con texto e icono. Los valores son los de las
 * máquinas de estado del repo de contratos.
 */
export type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral'

export interface StatusStyle {
  label: string
  tone: Tone
  icon: LucideIcon
  /** «Anulada» se muestra tachada. */
  strike?: boolean
}

export const STATUS = {
  invoice: {
    draft: { label: 'Borrador', tone: 'neutral', icon: Circle },
    issued: { label: 'Emitida', tone: 'info', icon: CircleDot },
    cancelled: { label: 'Anulada', tone: 'neutral', icon: CircleSlash, strike: true },
    requires_correction: { label: 'Requiere corrección', tone: 'warning', icon: TriangleAlert },
  },
  hacienda: {
    processing: { label: 'En proceso', tone: 'info', icon: Clock },
    signed: { label: 'En proceso', tone: 'info', icon: Clock },
    sent: { label: 'En proceso', tone: 'info', icon: Clock },
    accepted: { label: 'Aceptada', tone: 'success', icon: Check },
    rejected: { label: 'Rechazada', tone: 'danger', icon: X },
    contingency: { label: 'En contingencia', tone: 'warning', icon: TriangleAlert },
    error: { label: 'Con error', tone: 'danger', icon: CircleAlert },
    unavailable: { label: 'Estado no disponible', tone: 'neutral', icon: CircleHelp },
  },
  receivable: {
    open: { label: 'Pendiente', tone: 'info', icon: CircleDot },
    partially_paid: { label: 'Pago parcial', tone: 'warning', icon: CircleDashed },
    paid: { label: 'Pagada', tone: 'success', icon: Check },
    cancelled: { label: 'Anulada', tone: 'neutral', icon: CircleSlash },
    overdue: { label: 'Vencida', tone: 'danger', icon: CircleAlert },
  },
  payment: {
    posted: { label: 'Registrado', tone: 'success', icon: Check },
    voided: { label: 'Anulado', tone: 'neutral', icon: CircleSlash },
  },
  membership: {
    active: { label: 'Activo', tone: 'success', icon: Check },
    suspended: { label: 'Suspendido', tone: 'neutral', icon: Pause },
  },
  invitation: {
    pending: { label: 'Pendiente', tone: 'info', icon: Clock },
    accepted: { label: 'Aceptada', tone: 'success', icon: Check },
    revoked: { label: 'Revocada', tone: 'neutral', icon: CircleSlash },
    expired: { label: 'Vencida', tone: 'neutral', icon: Circle },
  },
} as const satisfies Record<string, Record<string, StatusStyle>>

export type StatusDomain = keyof typeof STATUS
export type StatusOf<D extends StatusDomain> = keyof (typeof STATUS)[D]

/** Detalle que acompaña a «En proceso» en Hacienda (Firmando / Enviado). */
export const HACIENDA_DETAIL: Partial<Record<StatusOf<'hacienda'>, string>> = {
  signed: 'Firmando',
  sent: 'Enviado',
}

export function statusStyle<D extends StatusDomain>(domain: D, status: StatusOf<D>): StatusStyle {
  return STATUS[domain][status] as StatusStyle
}
