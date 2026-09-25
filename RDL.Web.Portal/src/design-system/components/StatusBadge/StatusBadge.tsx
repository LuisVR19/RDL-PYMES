import { clsx } from 'clsx'
import { statusStyle, type StatusDomain, type StatusOf } from '@/shared/status/status'
import styles from './StatusBadge.module.css'

export interface StatusBadgeProps<D extends StatusDomain> {
  domain: D
  status: StatusOf<D>
  /** Texto que acompaña a la etiqueta: «Firmando», «18 días». */
  detail?: string
  size?: 'sm' | 'md'
}

/** Insignia de estado: siempre icono + texto + tono (el color nunca va solo). Mapeo en `shared/status`. */
export function StatusBadge<D extends StatusDomain>({
  domain,
  status,
  detail,
  size = 'md',
}: StatusBadgeProps<D>) {
  const s = statusStyle(domain, status)
  const Icon = s.icon
  return (
    <span className={clsx(styles.badge, styles[s.tone], styles[size], s.strike && styles.strike)}>
      <Icon size={size === 'sm' ? 11 : 12} strokeWidth={2.5} aria-hidden />
      <span>
        {s.label}
        {detail && <> · {detail}</>}
      </span>
    </span>
  )
}
