import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Big } from 'big.js'
import { clsx } from 'clsx'
import { useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { Panel } from '@/design-system/components/Surface/Surface'
import { SegmentedControl, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { AGING_BUCKETS, type AgingBucket, type AgingRow } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, formatBusinessDate, todayIn } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney, sumMoney, type Currency } from '@/shared/money/money'
import { useSession } from '@/shared/session/SessionProvider'
import styles from '../receivables.module.css'

const CURRENCIES: Currency[] = ['CRC', 'USD']
const DATE_RE = /^\d{4}-\d{2}-\d{2}$/

/**
 * Pantalla 23 · Antigüedad de saldos (prototipo «23»). Receivables devuelve el saldo cobrable por moneda y tramo a la
 * fecha de corte (R6: vencer hoy no es atraso); nunca se suman colones con dólares. Moneda y corte viajan en la URL
 * (`?moneda=&corte=`, pantallas.md). El gráfico repite sus cifras en una tabla (lectores de pantalla, impresión).
 *
 * Fuera del diseño por el contrato: el desglose por cliente (el aging de Receivables no trae clientes; sumar las
 * cuentas página por página no sería la cifra oficial) y los tramos configurables por organización (R6 los fija).
 * TODO(api): aging por cliente en Receivables.
 */
export function AgingPage() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const [params, setParams] = useSearchParams()
  const org = activeOrg?.id
  const today = todayIn(activeOrg?.timezone ?? DEFAULT_TZ)
  const currency = CURRENCIES.find((c) => c === params.get('moneda')) ?? activeOrg?.defaultCurrency ?? 'CRC'
  const corte = params.get('corte')
  const asOf = corte && DATE_RE.test(corte) ? corte : today

  const aging = useQuery({
    queryKey: ['receivables', org, 'aging', asOf, currency],
    queryFn: () => ds.receivables.aging(asOf, currency),
    placeholderData: keepPreviousData,
    enabled: !!org,
  })

  const setParam = (key: string, value: string | undefined) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  const buckets = bucketsOf(aging.data ?? [], currency)
  const total = sumMoney(buckets.map((b) => b.balance))
  const empty = new Big(total).eq(0)

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.receivables')}
        title={t('aging.title')}
        subtitle={t('aging.subtitle')}
        actions={
          <Button
            variant="secondary"
            disabled={!aging.data || empty}
            onClick={() => downloadCsv(buckets, currency, asOf)}
          >
            {t('aging.export')}
          </Button>
        }
      />
      <Toolbar>
        <SegmentedControl<Currency>
          label={t('aging.currency')}
          value={currency}
          onChange={(c) => setParam('moneda', c)}
          options={CURRENCIES.map((c) => ({ value: c, label: t(`aging.currency.${c}`) }))}
        />
        <TextField
          label={t('aging.asOf')}
          type="date"
          className={styles.dateFilter}
          value={asOf}
          max={today}
          onChange={(e) =>
            setParam('corte', e.target.value && e.target.value !== today ? e.target.value : undefined)
          }
        />
      </Toolbar>

      {aging.isPending ? (
        <SkeletonRows rows={6} columns={3} />
      ) : aging.isError ? (
        <ErrorState
          onRetry={() => void aging.refetch()}
          refCode={aging.error instanceof ApiError ? aging.error.correlationId || undefined : undefined}
        />
      ) : empty ? (
        <InlineAlert tone="info">
          {t('aging.empty', { currency: t(`pay.currencyName.${currency}`) })}
        </InlineAlert>
      ) : (
        <>
          <Panel className={styles.chart}>
            <div className={styles.chartHead}>
              <h2 className={styles.chartTitle}>{t('aging.chart')}</h2>
              <span className={styles.chartTotal}>
                {t('aging.total')} <strong>{formatMoney(total, currency)}</strong>
              </span>
            </div>
            <BucketBars buckets={buckets} total={total} currency={currency} />
          </Panel>
          <Panel padded={false}>
            <table className={styles.table}>
              <caption>{t('aging.table')}</caption>
              <thead>
                <tr>
                  <th scope="col">{t('aging.col.bucket')}</th>
                  <th scope="col" className={styles.right}>
                    {t('aging.col.balance')}
                  </th>
                  <th scope="col" className={styles.right}>
                    {t('aging.col.share')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {buckets.map((b) => (
                  <tr key={b.bucket}>
                    <th scope="row">
                      <span className={clsx(styles.swatch, styles[`bucket-${b.bucket}`])} aria-hidden />
                      {t(`aging.bucket.${b.bucket}`)}
                    </th>
                    <td className={styles.right}>{formatMoney(b.balance, currency)}</td>
                    <td className={styles.right}>{share(b.balance, total)}</td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr>
                  <td>{t('aging.total')}</td>
                  <td className={styles.right}>{formatMoney(total, currency)}</td>
                  <td className={styles.right}>100 %</td>
                </tr>
              </tfoot>
            </table>
          </Panel>
          <p className={styles.muted}>
            {t('aging.asOfNote', { date: formatBusinessDate(asOf) })} {t('aging.byCustomer')}
          </p>
        </>
      )}
    </div>
  )
}

interface Bucket {
  bucket: AgingBucket
  balance: string
}

/** Los cinco tramos en orden, en cero si la moneda no trae alguno. */
function bucketsOf(rows: AgingRow[], currency: Currency): Bucket[] {
  return AGING_BUCKETS.map((bucket) => ({
    bucket,
    balance: rows.find((r) => r.bucket === bucket && r.currency === currency)?.balance ?? '0',
  }))
}

/** Participación entera del tramo («32 %»): es para leer el gráfico, no una cifra contable. */
function share(balance: string, total: string): string {
  if (new Big(total).eq(0)) return '0 %'
  return `${new Big(balance).times(100).div(total).round(0, Big.roundHalfUp).toFixed(0)} %`
}

/** Alto de la barra respecto del tramo mayor; al menos 2 % para que un tramo en cero se vea como línea base. */
function heightOf(balance: string, max: Big): string {
  if (max.eq(0)) return '2%'
  const pct = new Big(balance).times(100).div(max).round(0, Big.roundHalfUp)
  return `${pct.lt(2) ? '2' : pct.toFixed(0)}%`
}

/** Columnas del prototipo: porcentaje arriba, barra anclada a la base, tramo y monto abajo. */
function BucketBars({ buckets, total, currency }: { buckets: Bucket[]; total: string; currency: Currency }) {
  const max = buckets.reduce((m, b) => (new Big(b.balance).gt(m) ? new Big(b.balance) : m), new Big(0))
  return (
    <>
      <ul className={styles.bars} aria-hidden>
        {buckets.map((b) => (
          <li key={b.bucket} className={styles.barSlot}>
            <span className={styles.barPct}>{share(b.balance, total)}</span>
            <span
              className={clsx(styles.bar, styles[`bucket-${b.bucket}`])}
              style={{ height: heightOf(b.balance, max) }}
              title={t('aging.bar', {
                bucket: t(`aging.bucket.${b.bucket}`),
                amount: formatMoney(b.balance, currency),
                pct: share(b.balance, total),
              })}
            />
          </li>
        ))}
      </ul>
      <div className={styles.barLabels} aria-hidden>
        {buckets.map((b) => (
          <div key={b.bucket} className={styles.barLabel}>
            <span className={styles.barName}>{t(`aging.bucket.${b.bucket}`)}</span>
            <span className={styles.barValue}>{formatMoney(b.balance, currency)}</span>
          </div>
        ))}
      </div>
    </>
  )
}

/** «Exportar» del prototipo: la tabla en CSV con los montos exactos que dio Receivables (punto decimal). */
function downloadCsv(buckets: Bucket[], currency: Currency, asOf: string) {
  const lines = [
    ['tramo', 'moneda', 'saldo'].join(','),
    ...buckets.map((b) => [b.bucket, currency, b.balance].join(',')),
  ]
  const url = URL.createObjectURL(new Blob([lines.join('\n') + '\n'], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a')
  a.href = url
  a.download = `aging-${currency}-${asOf}.csv`
  a.click()
  URL.revokeObjectURL(url)
}
