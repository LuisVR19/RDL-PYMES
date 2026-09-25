import { clsx } from 'clsx'
import { CircleDot, TriangleAlert } from 'lucide-react'
import type { FiscalEnvironment } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import styles from './EnvBadge.module.css'

/**
 * Ambiente de Hacienda de la organización activa (decisión del diseño: siempre visible). Pruebas en ámbar con ▲,
 * producción en verde con ●; nunca solo color.
 */
export function EnvBadge({
  environment,
  inline = false,
}: {
  environment: FiscalEnvironment
  inline?: boolean
}) {
  const test = environment === 'test'
  const Icon = test ? TriangleAlert : CircleDot
  const label = inline && test ? t('topbar.env.testShort') : t(test ? 'app.env.test' : 'app.env.prod')
  return (
    <span className={clsx(inline ? styles.inline : styles.badge, test ? styles.test : styles.prod)}>
      <Icon size={inline ? 12 : 14} strokeWidth={2} aria-hidden />
      {label}
    </span>
  )
}
