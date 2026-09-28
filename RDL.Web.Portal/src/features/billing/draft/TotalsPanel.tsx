import { clsx } from 'clsx'
import type { ReactNode } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Kbd } from '@/design-system/components/Surface/Surface'
import type { Invoice } from '@/shared/api/billing-types'
import type { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, formatInstantTime } from '@/shared/dates/dates'
import { t, type MessageKey } from '@/shared/i18n/t'
import { formatMoney, MINUS, type Currency } from '@/shared/money/money'
import type { SyncStatus } from './useDraftSync'
import styles from './draft.module.css'

/**
 * Totales del borrador (prototipo «13», columna derecha). Son los que devolvió Billing en el último guardado; mientras
 * recalcula se atenúan, y si falla se dice que no son definitivos. El portal no suma ni redondea nada aquí.
 */
export function TotalsPanel({
  title,
  currency,
  saved,
  status,
  error,
  dirty,
  tz = DEFAULT_TZ,
  onRetry,
  primary,
  onSave,
  saveShortcut = true,
}: {
  title: string
  currency: Currency
  saved: Invoice | null
  status: SyncStatus
  error: ApiError | null
  dirty: boolean
  tz?: string
  onRetry: () => void
  primary: ReactNode
  onSave: () => void
  saveShortcut?: boolean
}) {
  const cur = saved?.currency ?? currency
  const money = (v: string) => formatMoney(v, cur)
  const neg = (v: string) => (v === '0' || /^0(\.0+)?$/.test(v) ? money('0') : `${MINUS}${money(v)}`)
  const rows: [MessageKey, string][] = saved
    ? [
        ['invoice.subtotal', money(saved.subtotal)],
        ['invoice.discount', neg(saved.discount)],
        ['invoice.tax', money(saved.tax)],
        ['invoice.exoneration', neg(saved.exoneration)],
      ]
    : []
  const busy = status === 'busy'

  return (
    <aside className={styles.totalsCard} aria-label={title}>
      <div className={styles.totalsHead}>
        <h2 className={styles.cardTitle}>{title}</h2>
        <span className={styles.muted}>{t(`draft.totals.currency.${cur}`)}</span>
      </div>
      {saved ? (
        <dl className={clsx(styles.totals, busy && styles.dimmed)}>
          {rows.map(([k, v]) => (
            <div key={k} className={styles.totalRow}>
              <dt>{t(k)}</dt>
              <dd>{v}</dd>
            </div>
          ))}
          <div className={styles.grandTotal}>
            <dt>{t('invoice.total')}</dt>
            <dd>{money(saved.total)}</dd>
          </div>
        </dl>
      ) : (
        <p className={styles.muted}>{t('draft.totals.pending')}</p>
      )}
      {busy && (
        <div role="status" className={styles.recalc}>
          <span aria-hidden>◌</span> {t('invoice.totals.busy')}
        </div>
      )}
      {status === 'error' && (
        <InlineAlert
          tone="danger"
          refCode={error?.correlationId || undefined}
          actions={
            <Button size="sm" variant="secondary" onClick={onRetry}>
              {t('common.retry')}
            </Button>
          }
        >
          {saved ? t('invoice.totals.error') : t('draft.saveError')}
        </InlineAlert>
      )}
      {saved && status === 'idle' && !dirty && (
        <span className={styles.muted}>
          ✓ {t('invoice.totals.ok', { time: formatInstantTime(saved.updatedAt, tz) })}
        </span>
      )}
      {primary}
      <Button variant="secondary" onClick={onSave} loading={busy} loadingLabel={t('draft.saving')}>
        {t('invoice.saveDraft')} {saveShortcut && <Kbd hint>Ctrl S</Kbd>}
      </Button>
    </aside>
  )
}
