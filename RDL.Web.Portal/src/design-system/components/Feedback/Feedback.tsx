import { clsx } from 'clsx'
import { Check, CircleAlert, Copy, Info, TriangleAlert, type LucideIcon } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { t } from '@/shared/i18n/t'
import type { Tone } from '@/shared/status/status'
import { Button } from '../Button/Button'
import styles from './Feedback.module.css'

const TONE_ICON: Record<Exclude<Tone, 'neutral'>, LucideIcon> = {
  success: Check,
  warning: TriangleAlert,
  danger: CircleAlert,
  info: Info,
}

/** Código de referencia copiable (correlationId), para que el usuario lo comparta con soporte. */
export function RefCode({ code }: { code: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      // Sin permiso de portapapeles: el código queda visible para copiarlo a mano.
    }
  }
  return (
    <button
      type="button"
      className={styles.refCode}
      onClick={copy}
      aria-label={`${t('common.refCode', { code })}. ${t('common.copy')}`}
    >
      <span>{t('common.refCode', { code })}</span>
      {copied ? <Check size={13} aria-hidden /> : <Copy size={13} aria-hidden />}
      <span>{copied ? 'Copiado' : t('common.copy')}</span>
    </button>
  )
}

export interface InlineAlertProps {
  tone: Exclude<Tone, 'neutral'>
  title?: ReactNode
  children?: ReactNode
  actions?: ReactNode
  refCode?: string
  className?: string
}

/** Aviso en línea (info, éxito, advertencia, error). Los errores de servidor traen código de referencia. */
export function InlineAlert({ tone, title, children, actions, refCode, className }: InlineAlertProps) {
  const Icon = TONE_ICON[tone]
  return (
    <div
      className={clsx(styles.alert, styles[tone], className)}
      role={tone === 'danger' ? 'alert' : 'status'}
    >
      <Icon size={16} className={styles.alertIcon} aria-hidden />
      <div className={styles.alertBody}>
        {title && <strong className={styles.alertTitle}>{title}</strong>}
        {children && <div>{children}</div>}
        {(actions || refCode) && (
          <div className={styles.alertActions}>
            {actions}
            {refCode && <RefCode code={refCode} />}
          </div>
        )}
      </div>
    </div>
  )
}

/** Esqueleto de carga: bloques atenuados con la forma del contenido. */
export function Skeleton({
  width = '100%',
  height = 14,
  radius = 4,
}: {
  width?: number | string
  height?: number
  radius?: number
}) {
  return <span className={styles.skeleton} style={{ width, height, borderRadius: radius }} aria-hidden />
}

export function SkeletonRows({ rows = 6, columns = 5 }: { rows?: number; columns?: number }) {
  return (
    <div className={styles.skeletonRows} role="status" aria-label={t('common.loading')}>
      {Array.from({ length: rows }, (_, r) => (
        <div key={r} className={styles.skeletonRow}>
          {Array.from({ length: columns }, (_, c) => (
            <Skeleton key={c} width={c === 0 ? '70%' : '55%'} />
          ))}
        </div>
      ))}
    </div>
  )
}

/** Estado vacío con acción principal. */
export function EmptyState({
  icon: Icon,
  title,
  body,
  action,
}: {
  icon?: LucideIcon
  title: string
  body?: string
  action?: ReactNode
}) {
  return (
    <div className={styles.empty}>
      {Icon && (
        <span className={styles.emptyIcon}>
          <Icon size={22} aria-hidden />
        </span>
      )}
      <strong className={styles.emptyTitle}>{title}</strong>
      {body && <p className={styles.emptyBody}>{body}</p>}
      {action}
    </div>
  )
}

/** Estado de error de un bloque o lista: qué pasó, reintentar y código de referencia. */
export function ErrorState({
  title,
  body,
  onRetry,
  refCode,
}: {
  title?: string
  body?: string
  onRetry?: () => void
  refCode?: string
}) {
  return (
    <div className={styles.empty} role="alert">
      <span className={clsx(styles.emptyIcon, styles.emptyIconDanger)}>
        <CircleAlert size={22} aria-hidden />
      </span>
      <strong className={styles.emptyTitle}>{title ?? 'No pudimos cargar esta información'}</strong>
      <p className={styles.emptyBody}>
        {body ?? 'Intente de nuevo. Si se repite, comparta el código de referencia con soporte.'}
      </p>
      <div className={styles.errorActions}>
        {onRetry && (
          <Button variant="secondary" onClick={onRetry}>
            {t('common.retry')}
          </Button>
        )}
        {refCode && <RefCode code={refCode} />}
      </div>
    </div>
  )
}
