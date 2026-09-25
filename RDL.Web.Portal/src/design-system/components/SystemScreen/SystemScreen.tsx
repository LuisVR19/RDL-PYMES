import { useNavigate } from 'react-router'
import { t } from '@/shared/i18n/t'
import { Button } from '../Button/Button'
import { RefCode } from '../Feedback/Feedback'
import styles from './SystemScreen.module.css'

export type SystemKind = '403' | '404' | 'unavailable' | 'unexpected'

const COPY: Record<SystemKind, { code: string; title: string; retry: boolean }> = {
  '403': { code: '403', title: t('sys.403.title'), retry: false },
  '404': { code: '404', title: t('sys.404.title'), retry: false },
  unavailable: { code: '503', title: t('sys.unavailable.title'), retry: true },
  unexpected: { code: 'ERROR', title: t('sys.error.title'), retry: true },
}

/**
 * Pantallas de sistema (pantalla 35). El 404 también cubre «es de otra organización» sin revelarlo.
 * `body` permite el texto con variables del 403 («Su rol en {org} es {role}…»).
 */
export function SystemScreen({
  kind,
  body,
  refCode,
  onRetry,
}: {
  kind: SystemKind
  body?: string
  refCode?: string
  onRetry?: () => void
}) {
  const navigate = useNavigate()
  const c = COPY[kind]
  const defaultBody =
    kind === '404'
      ? t('sys.404.body')
      : kind === 'unavailable'
        ? t('sys.unavailable.body')
        : kind === 'unexpected'
          ? t('sys.unexpected.body')
          : ''
  return (
    <div className={styles.wrap}>
      <div className={styles.box}>
        <span className={styles.code}>{c.code}</span>
        <h1 className={styles.title}>{c.title}</h1>
        <p className={styles.body}>{body ?? defaultBody}</p>
        <div className={styles.actions}>
          {c.retry && (
            <Button variant="primary" onClick={onRetry ?? (() => window.location.reload())}>
              {t('common.retry')}
            </Button>
          )}
          <Button variant="secondary" onClick={() => navigate('/')}>
            {t('common.goHome')}
          </Button>
        </div>
        {refCode && <RefCode code={refCode} />}
      </div>
    </div>
  )
}
