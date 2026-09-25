import { useQuery } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { TabPanel, Tabs } from '@/design-system/components/Tabs/Tabs'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer, InvoiceListItem, Payment, Receivable } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, daysOverdue, formatBusinessDate, formatInstantDate, todayIn } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { FiscalBadge } from '../FiscalBadge'
import { identificationText, MoneyByCurrency, totalsByCurrency } from '../shared'
import styles from './ClientDetailPage.module.css'

type Tab = 'invoices' | 'receivables' | 'payments' | 'data'

// Una página grande basta para la ficha; si hay más, el total del año no se calcula a medias.
const PAGE = 100

/**
 * Pantalla 9 · Ficha del cliente (prototipo «09»). Los datos y las facturas son de Billing; las cuentas por cobrar
 * y los pagos, de Receivables. Si Receivables falla, la ficha se muestra igual con el aviso de datos parciales.
 */
export function ClientDetailPage() {
  const { id = '' } = useParams()
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [tab, setTab] = useState<Tab>('invoices')
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ

  const customer = useQuery({
    queryKey: ['customers', org, 'detail', id],
    queryFn: () => ds.customers.get(id),
    enabled: !!org,
  })
  const invoices = useQuery({
    queryKey: ['invoices', org, { customerId: id, limit: PAGE }],
    queryFn: () => ds.invoices.list({ customerId: id, limit: PAGE }),
    enabled: !!org,
  })
  const receivables = useQuery({
    queryKey: ['receivables', org, 'byCustomer', id],
    queryFn: () => ds.receivables.byCustomer(id, { limit: PAGE }),
    enabled: !!org,
    retry: false,
  })
  const payments = useQuery({
    queryKey: ['payments', org, 'byCustomer', id],
    queryFn: () => ds.receivables.paymentsByCustomer(id, { limit: PAGE }),
    enabled: !!org,
    retry: false,
  })

  if (customer.isPending) return <SkeletonRows rows={6} columns={3} />
  if (customer.isError) {
    return <ErrorState onRetry={() => void customer.refetch()} refCode={refOf(customer.error)} />
  }
  const c = customer.data
  const partial = receivables.isError
  const today = todayIn(tz)
  const open = (receivables.data?.items ?? []).filter(
    (r) => r.status === 'open' || r.status === 'partially_paid',
  )
  const overdue = open.filter((r) => daysOverdue(r.dueOn, today) > 0)
  const canEdit = role ? can(role, 'billing.edit') : false

  return (
    <div className={styles.page}>
      <PageHeader
        back={{ label: t('clients.title'), to: '/clientes' }}
        title={
          <span className={styles.title}>
            {c.legalName}
            <StatusBadge domain="record" status={c.isActive ? 'active' : 'inactive'} />
          </span>
        }
        subtitle={[identificationText(c.identification), c.email, c.phone].filter(Boolean).join(' · ')}
        actions={
          canEdit && (
            <>
              <Button variant="secondary" onClick={() => navigate(`/clientes/${c.id}/editar`)}>
                {t('client.detail.edit')}
              </Button>
              <Button variant="primary" onClick={() => navigate(`/facturas/nueva?cliente=${c.id}`)}>
                {t('client.detail.newInvoice')}
              </Button>
            </>
          )
        }
      />

      {partial && (
        <div className={styles.partial} role="status">
          <TriangleAlert size={16} aria-hidden />
          <span className={styles.partialText}>{t('client.detail.partial')}</span>
          <button
            type="button"
            className={styles.partialRetry}
            onClick={() => {
              void receivables.refetch()
              void payments.refetch()
            }}
          >
            {t('common.retry')}
          </button>
        </div>
      )}

      <div className={styles.kpis}>
        <Kpi
          label={t('client.kpi.balance')}
          value={
            partial ? (
              t('common.notAvailable')
            ) : receivables.isPending ? (
              '…'
            ) : (
              <MoneyByCurrency totals={totalsByCurrency(open.map(amountOf))} />
            )
          }
          sub={
            partial
              ? t('client.kpi.retryLater')
              : open.length === 1
                ? t('client.kpi.openOne')
                : t('client.kpi.openMany', { n: open.length })
          }
          muted={partial}
        />
        <Kpi
          label={t('client.kpi.overdue')}
          value={
            partial ? (
              t('common.notAvailable')
            ) : receivables.isPending ? (
              '…'
            ) : (
              <MoneyByCurrency totals={totalsByCurrency(overdue.map(amountOf))} />
            )
          }
          sub={
            partial
              ? '—'
              : overdue.length === 0
                ? t('client.kpi.noOverdue')
                : overdue.length === 1
                  ? t('client.kpi.overdueOne')
                  : t('client.kpi.overdueMany', { n: overdue.length })
          }
          alert={!partial && overdue.length > 0}
          muted={partial}
        />
        <InvoicedKpi invoices={invoices.data} pending={invoices.isPending} tz={tz} year={today.slice(0, 4)} />
      </div>

      <Tabs
        label={c.legalName}
        value={tab}
        onValueChange={(v) => setTab(v as Tab)}
        items={[
          { value: 'invoices', label: t('client.tab.invoices') },
          { value: 'receivables', label: t('client.tab.receivables') },
          { value: 'payments', label: t('client.tab.payments') },
          { value: 'data', label: t('client.tab.data') },
        ]}
      >
        <TabPanel value="invoices">
          <Rows
            loading={invoices.isPending}
            empty={t('client.empty.invoices')}
            rows={(invoices.data?.items ?? []).map((it) =>
              invoiceRow(it, tz, () => navigate(`/facturas/${it.invoice.id}`)),
            )}
          />
        </TabPanel>
        <TabPanel value="receivables">
          <Rows
            loading={receivables.isPending}
            empty={t(partial ? 'client.empty.receivablesPartial' : 'client.empty.receivables')}
            rows={
              partial
                ? []
                : open.map((r) => receivableRow(r, today, () => navigate(`/facturas/${r.sourceInvoiceId}`)))
            }
          />
        </TabPanel>
        <TabPanel value="payments">
          <Rows
            loading={payments.isPending}
            empty={t(payments.isError ? 'client.empty.receivablesPartial' : 'client.empty.payments')}
            rows={payments.isError ? [] : (payments.data?.items ?? []).map(paymentRow)}
          />
        </TabPanel>
        <TabPanel value="data">
          <CustomerData customer={c} />
        </TabPanel>
      </Tabs>
    </div>
  )
}

const amountOf = (r: Receivable) => ({ currency: r.currency, amount: r.balanceAmount })

function Kpi({
  label,
  value,
  sub,
  alert,
  muted,
}: {
  label: string
  value: ReactNode
  sub: string
  alert?: boolean
  muted?: boolean
}) {
  return (
    <div className={styles.kpi}>
      <span className={styles.kpiLabel}>{label}</span>
      <span className={alert ? styles.kpiValueAlert : muted ? styles.kpiValueMuted : styles.kpiValue}>
        {value}
      </span>
      <span className={alert ? styles.kpiSubAlert : styles.kpiSub}>{sub}</span>
    </div>
  )
}

/** «Facturado en 2026»: la suma por moneda de las facturas emitidas este año, solo si están todas en la página. */
function InvoicedKpi({
  invoices,
  pending,
  tz,
  year,
}: {
  invoices: { items: InvoiceListItem[]; nextCursor: string | null } | undefined
  pending: boolean
  tz: string
  year: string
}) {
  const label = t('client.kpi.invoiced', { year })
  if (pending || !invoices) return <Kpi label={label} value="…" sub="" />
  const thisYear = invoices.items.filter(
    (it) =>
      it.invoice.status === 'issued' &&
      it.invoice.documentType === 'invoice' &&
      it.invoice.issuedAt !== undefined &&
      todayIn(tz, new Date(it.invoice.issuedAt)).startsWith(year),
  )
  if (invoices.nextCursor) {
    return <Kpi label={label} value="—" sub={t('client.kpi.documentsMore', { n: invoices.items.length })} />
  }
  return (
    <Kpi
      label={label}
      value={
        <MoneyByCurrency
          totals={totalsByCurrency(
            thisYear.map((it) => ({ currency: it.invoice.currency, amount: it.invoice.total })),
          )}
        />
      }
      sub={t('client.kpi.documents', { n: thisYear.length })}
    />
  )
}

interface RowData {
  key: string
  title: string
  sub: string
  badge?: ReactNode
  amount: string
  amount2?: string
  onOpen?: () => void
}

function Rows({ loading, rows, empty }: { loading: boolean; rows: RowData[]; empty: string }) {
  if (loading) return <SkeletonRows rows={3} columns={3} />
  if (rows.length === 0) return <p className={styles.empty}>{empty}</p>
  return (
    <ul className={styles.rows}>
      {rows.map((r) => (
        <li key={r.key}>
          {r.onOpen ? (
            <button type="button" className={styles.row} onClick={r.onOpen}>
              <RowBody row={r} />
            </button>
          ) : (
            <div className={styles.row}>
              <RowBody row={r} />
            </div>
          )}
        </li>
      ))}
    </ul>
  )
}

function RowBody({ row }: { row: RowData }) {
  return (
    <>
      <span className={styles.rowMain}>
        <span className={styles.rowTitle}>{row.title}</span>
        <span className={styles.rowSub}>{row.sub}</span>
      </span>
      {row.badge}
      <span className={styles.rowAmounts}>
        <span className={styles.rowAmount}>{row.amount}</span>
        {row.amount2 && <span className={styles.rowSub}>{row.amount2}</span>}
      </span>
    </>
  )
}

function invoiceRow(it: InvoiceListItem, tz: string, onOpen: () => void): RowData {
  const inv = it.invoice
  const bal = it.receivable.balance
  return {
    key: inv.id,
    title: inv.number ?? t('doc.draftNumber'),
    sub: [inv.issuedAt ? formatInstantDate(inv.issuedAt, tz) : undefined, t(`doc.type.${inv.documentType}`)]
      .filter(Boolean)
      .join(' · '),
    badge:
      inv.status === 'cancelled' ? (
        <StatusBadge domain="invoice" status="cancelled" />
      ) : inv.status === 'draft' ? (
        <StatusBadge domain="invoice" status="draft" />
      ) : (
        <FiscalBadge part={it.fiscal} />
      ),
    amount: formatMoney(inv.total, inv.currency),
    amount2: bal
      ? t('client.row.balance', { amount: formatMoney(bal.balanceAmount, bal.currency) })
      : undefined,
    onOpen,
  }
}

function receivableRow(r: Receivable, today: string, onOpen: () => void): RowData {
  const late = daysOverdue(r.dueOn, today)
  return {
    key: r.id,
    title: r.documentNumber,
    sub: t('client.row.dueOn', { date: formatBusinessDate(r.dueOn) }),
    badge:
      late > 0 ? (
        <StatusBadge domain="receivable" status="overdue" detail={t('client.row.overdue', { n: late })} />
      ) : (
        <StatusBadge domain="receivable" status={r.status} />
      ),
    amount: formatMoney(r.balanceAmount, r.currency),
    amount2: t('client.row.of', { amount: formatMoney(r.originalAmount, r.currency) }),
    onOpen,
  }
}

function paymentRow(p: Payment): RowData {
  return {
    key: p.id,
    title: p.reference || t('client.row.payment', { date: formatBusinessDate(p.receivedOn) }),
    sub: formatBusinessDate(p.receivedOn),
    badge: <StatusBadge domain="payment" status={p.status} />,
    amount: formatMoney(p.amount, p.currency),
  }
}

function CustomerData({ customer: c }: { customer: Customer }) {
  const rows: [string, string][] = [
    [t('client.legalName'), c.legalName],
    [t('client.tradeName'), c.tradeName || '—'],
    [t('clients.col.identification'), identificationText(c.identification)],
    [t('client.email'), c.email || '—'],
    [t('client.phone'), c.phone || '—'],
    [t('client.address'), c.address || '—'],
  ]
  return (
    <dl className={styles.data}>
      {rows.map(([k, v]) => (
        <div key={k} className={styles.dataRow}>
          <dt>{k}</dt>
          <dd>{v}</dd>
        </div>
      ))}
    </dl>
  )
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
