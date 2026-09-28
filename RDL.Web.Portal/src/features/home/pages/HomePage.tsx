import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { clsx } from 'clsx'
import { TriangleAlert } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { Skeleton } from '@/design-system/components/Feedback/Feedback'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { Kbd } from '@/design-system/components/Surface/Surface'
import { FiscalBadge } from '@/features/billing/FiscalBadge'
import { totalsByCurrency } from '@/features/billing/shared'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { InvoiceListItem, Payment } from '@/shared/api/billing-types'
import {
  DEFAULT_TZ,
  formatBusinessDate,
  formatInstant,
  formatInstantDate,
  todayIn,
} from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney, type Currency } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { timezonePlace } from '@/shared/timezones'
import { issuedInvoicesBetween, latestIssued } from '../monthInvoices'
import styles from './HomePage.module.css'

type Totals = Partial<Record<Currency, string>>

/**
 * Pantalla 6 · Inicio (prototipo «06»). Cuatro cifras y dos paneles; cada bloque carga y falla por su cuenta, y si
 * alguno falla se avisa que hay datos parciales. Lo que se ve depende del rol (matriz de permisos):
 * - facturación: lo facturado en el mes y las últimas facturas (Billing);
 * - cobranza: saldo por cobrar, saldo vencido y los últimos pagos (Receivables, P6);
 * - Hacienda: documentos rechazados o en contingencia (E-Invoice, P5).
 */
export function HomePage() {
  const ds = useDataSource()
  const navigate = useNavigate()
  const { activeOrg, role } = useSession()
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ
  const main: Currency = activeOrg?.defaultCurrency ?? 'CRC'
  const today = todayIn(tz)
  const monthStart = `${today.slice(0, 8)}01`

  const allowed = (c: Parameters<typeof can>[1]) => (role ? can(role, c) : false)
  const billing = allowed('billing.view')
  const receivables = allowed('receivables.view')
  const fiscal = allowed('fiscal.inbox')

  const month = useQuery({
    queryKey: ['invoices', org, 'home', monthStart, today],
    queryFn: () => issuedInvoicesBetween(ds.invoices, monthStart, today),
    enabled: !!org && billing,
    retry: false,
  })
  const balances = useQuery({
    queryKey: ['receivables', org, 'summary', today],
    queryFn: () => ds.receivables.summary(today),
    enabled: !!org && receivables,
    retry: false,
  })
  const payments = useQuery({
    queryKey: ['payments', org, 'home'],
    queryFn: () => ds.receivables.payments({ limit: 5 }),
    enabled: !!org && receivables,
    retry: false,
  })
  // Misma clave que el contador del menú: una sola llamada para los dos.
  const inbox = useQuery({
    queryKey: ['shell', org, 'inboxCount'],
    queryFn: () => ds.shell.inboxAttentionCount(org ?? ''),
    enabled: !!org && fiscal,
    retry: false,
  })

  const blocks = [billing && month, receivables && balances, receivables && payments, fiscal && inbox]
  const partial = blocks.some((q) => q && q.isError)
  // «al …»: cuándo llegaron los datos más recientes; antes de que llegue alguno, cuándo se abrió la pantalla.
  const [openedAt] = useState(() => Date.now())
  const updatedAt = Math.max(openedAt, ...blocks.map((q) => (q ? q.dataUpdatedAt : 0)))

  const monthItems = month.data ?? []
  const invoiced = totalsByCurrency(
    monthItems.map((i) => ({ currency: i.invoice.currency, amount: i.invoice.total })),
  )

  return (
    <div className={styles.page}>
      <PageHeader
        title={t('nav.home')}
        subtitle={t('home.asOf', {
          org: activeOrg?.legalName ?? '',
          at: formatInstant(new Date(updatedAt).toISOString(), tz),
          place: timezonePlace(tz),
        })}
        actions={
          <>
            {allowed('receivables.edit') && (
              <Button variant="secondary" onClick={() => navigate('/cobranza/pagos/nuevo')}>
                {t('ar.payment.register')}
              </Button>
            )}
            {allowed('billing.edit') && (
              <Button variant="primary" onClick={() => navigate('/facturas/nueva')}>
                {t('invoice.new')} <Kbd hint>N</Kbd>
              </Button>
            )}
          </>
        }
      />

      {partial && (
        <div className={styles.partial} role="status">
          <TriangleAlert size={16} aria-hidden />
          <span>{t('home.partial')}</span>
        </div>
      )}

      <div className={styles.kpis}>
        {billing && (
          <Kpi label={t('home.kpi.invoiced', { month: monthName(today) })} query={month}>
            {() => ({
              value: formatMoney(invoiced[main] ?? '0', main),
              sub: joinParts(
                monthItems.length === 0
                  ? t('home.kpi.noInvoices')
                  : monthItems.length === 1
                    ? t('home.kpi.invoicesOne')
                    : t('home.kpi.invoicesMany', { n: monthItems.length }),
                otherCurrency(invoiced, main),
              ),
            })}
          </Kpi>
        )}
        {receivables && (
          <Kpi label={t('home.kpi.balance')} query={balances}>
            {(s) => ({
              value: formatMoney(s.open.totals[main] ?? '0', main),
              sub: joinParts(
                s.open.count === 0
                  ? t('home.kpi.noOpen')
                  : s.open.count === 1
                    ? t('home.kpi.openOne')
                    : t('home.kpi.openMany', { n: s.open.count }),
                otherCurrency(s.open.totals, main),
              ),
            })}
          </Kpi>
        )}
        {receivables && (
          <Kpi label={t('home.kpi.overdue')} query={balances}>
            {(s) => ({
              value: formatMoney(s.overdue.totals[main] ?? '0', main),
              sub: joinParts(
                s.overdue.count === 0
                  ? t('home.kpi.noOverdue')
                  : s.overdue.count === 1
                    ? t('home.kpi.overdueOne')
                    : t('home.kpi.overdueMany', { n: s.overdue.count }),
                otherCurrency(s.overdue.totals, main),
              ),
              alert: s.overdue.count > 0,
              link:
                s.overdue.count > 0
                  ? {
                      label: t('home.kpi.seeOverdue'),
                      onClick: () => navigate('/cobranza/cuentas?estado=vencidas'),
                    }
                  : undefined,
            })}
          </Kpi>
        )}
        {fiscal && (
          <Kpi label={t('home.kpi.fiscal')} query={inbox}>
            {(n) => ({
              value:
                n === 0
                  ? t('home.kpi.fiscalNone')
                  : n === 1
                    ? t('home.kpi.fiscalOne')
                    : t('home.kpi.fiscalMany', { n }),
              sub: n === 0 ? t('home.kpi.fiscalOk') : t('home.kpi.fiscalAttention'),
              alert: n > 0,
              link:
                n > 0
                  ? { label: t('home.kpi.seeInbox'), onClick: () => navigate('/hacienda/bandeja') }
                  : undefined,
            })}
          </Kpi>
        )}
      </div>

      <div className={styles.panels}>
        {billing && (
          <HomePanel
            title={t('home.panel.invoices')}
            onSeeAll={() => navigate('/documentos')}
            query={month}
            empty={t('home.panel.invoicesEmpty')}
            rows={latestIssued(monthItems, 5).map((it) =>
              invoiceRow(it, tz, () => navigate(`/facturas/${it.invoice.id}`)),
            )}
          />
        )}
        {receivables && (
          <HomePanel
            title={t('home.panel.payments')}
            onSeeAll={() => navigate('/cobranza/pagos')}
            query={payments}
            empty={t('home.panel.paymentsEmpty')}
            rows={(payments.data?.items ?? []).map((p) =>
              paymentRow(p, () => navigate(`/cobranza/pagos/${p.id}`)),
            )}
          />
        )}
      </div>
    </div>
  )
}

interface KpiView {
  value: string
  sub: string
  alert?: boolean
  link?: { label: string; onClick: () => void }
}

/** Una cifra del prototipo: esqueleto mientras carga, «No pudimos cargar esta cifra» con reintento si falla. */
function Kpi<T>({
  label,
  query,
  children,
}: {
  label: string
  query: UseQueryResult<T>
  children: (data: T) => KpiView
}) {
  const view = query.isSuccess ? children(query.data) : null
  return (
    <section className={styles.kpi} aria-label={label}>
      <span className={styles.kpiLabel}>{label}</span>
      {query.isPending && (
        <span className={styles.kpiLoading} role="status" aria-label={t('common.loading')}>
          <Skeleton width="70%" height={28} />
          <Skeleton width="45%" height={12} />
        </span>
      )}
      {query.isError && (
        <>
          <span className={styles.kpiError}>{t('home.kpi.error')}</span>
          <button type="button" className={styles.link} onClick={() => void query.refetch()}>
            {t('common.retry')}
          </button>
        </>
      )}
      {view && (
        <>
          <span className={clsx(styles.kpiValue, view.alert && styles.alert)}>{view.value}</span>
          <span className={clsx(styles.kpiSub, view.alert && styles.alert)}>{view.sub}</span>
          {view.link && (
            <button type="button" className={styles.link} onClick={view.link.onClick}>
              {view.link.label} ›
            </button>
          )}
        </>
      )}
    </section>
  )
}

interface Row {
  key: string
  title: string
  number?: string
  date: string
  badge?: ReactNode
  amount: string
  open: () => void
}

function HomePanel({
  title,
  onSeeAll,
  query,
  empty,
  rows,
}: {
  title: string
  onSeeAll: () => void
  query: UseQueryResult<unknown>
  empty: string
  rows: Row[]
}) {
  return (
    <section className={styles.panel} aria-label={title}>
      <div className={styles.panelHead}>
        <h2 className={styles.panelTitle}>{title}</h2>
        <button type="button" className={styles.link} onClick={onSeeAll}>
          {t('home.panel.seeAll')} ›
        </button>
      </div>
      {query.isPending && (
        <div role="status" aria-label={t('common.loading')}>
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className={styles.skeletonRow}>
              <Skeleton width="100%" height={12} />
              <Skeleton width={90} height={12} />
            </div>
          ))}
        </div>
      )}
      {query.isError && (
        <div className={styles.panelMessage} role="alert">
          <strong>{t('home.panel.error')}</strong>
          <button type="button" className={styles.link} onClick={() => void query.refetch()}>
            {t('common.retry')}
          </button>
        </div>
      )}
      {query.isSuccess && rows.length === 0 && <p className={styles.panelEmpty}>{empty}</p>}
      {query.isSuccess && rows.length > 0 && (
        <ul className={styles.rows}>
          {rows.map((r) => (
            <li key={r.key}>
              <button type="button" className={styles.row} onClick={r.open}>
                <span className={styles.rowMain}>
                  <span className={styles.rowTitle}>{r.title}</span>
                  <span className={styles.rowSub}>
                    {r.number && <span className={styles.mono}>{r.number}</span>}
                    {r.number && ' · '}
                    {r.date}
                  </span>
                </span>
                {r.badge}
                <span className={styles.rowAmount}>{r.amount}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

function invoiceRow(it: InvoiceListItem, tz: string, open: () => void): Row {
  const i = it.invoice
  return {
    key: i.id,
    title: i.customerLegalName ?? '—',
    number: i.number,
    date: i.issuedAt ? formatInstantDate(i.issuedAt, tz) : '—',
    badge: <FiscalBadge part={it.fiscal} size="sm" />,
    amount: formatMoney(i.total, i.currency),
    open,
  }
}

function paymentRow(p: Payment, open: () => void): Row {
  return {
    key: p.id,
    title: p.reference || t('home.panel.paymentNoReference'),
    date: formatBusinessDate(p.receivedOn),
    badge: p.status === 'voided' ? <StatusBadge domain="payment" status="voided" size="sm" /> : undefined,
    amount: formatMoney(p.amount, p.currency),
    open,
  }
}

/** «septiembre», de la fecha de negocio de hoy (sin pasar por la zona del navegador). */
function monthName(today: string): string {
  const year = Number(today.slice(0, 4))
  const month = Number(today.slice(5, 7))
  return new Intl.DateTimeFormat('es-CR', { month: 'long', timeZone: 'UTC' }).format(
    Date.UTC(year, month - 1, 15),
  )
}

/** El monto de la otra moneda va aparte, nunca convertido: «· y US$12 480,00». */
function otherCurrency(totals: Totals, main: Currency): string | undefined {
  const other: Currency = main === 'CRC' ? 'USD' : 'CRC'
  const amount = totals[other]
  return amount ? t('home.kpi.andOther', { amount: formatMoney(amount, other) }) : undefined
}

function joinParts(...parts: (string | undefined)[]): string {
  return parts.filter(Boolean).join(' · ')
}
