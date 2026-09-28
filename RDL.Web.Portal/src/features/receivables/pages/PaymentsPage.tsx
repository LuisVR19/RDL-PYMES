import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Banknote } from 'lucide-react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState } from '@/design-system/components/Feedback/Feedback'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { usePager } from '@/features/billing/usePager'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Payment } from '@/shared/api/billing-types'
import { formatBusinessDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { paymentMethodLabel } from '@/shared/paymentMethods'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { appliedOf, unappliedOf } from '../model'
import styles from '../receivables.module.css'

/**
 * Pantalla 25 · Pagos (prototipo «25»), del más nuevo al más viejo. Aplicado y sin aplicar salen de las aplicaciones
 * vigentes que trae cada pago. Receivables no numera los pagos: la fila se identifica por fecha, cliente y referencia.
 */
export function PaymentsPage() {
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const pager = usePager([])
  const org = activeOrg?.id

  const list = useQuery({
    queryKey: ['receivables', org, 'payments', pager.cursor],
    queryFn: () => ds.receivables.payments({ cursor: pager.cursor, limit: 20 }),
    placeholderData: keepPreviousData,
    enabled: !!org,
  })
  const customers = useQuery({
    queryKey: ['customers', org, { limit: 100 }],
    queryFn: () => ds.customers.list({ limit: 100 }),
    enabled: !!org,
    staleTime: 60_000,
  })
  const names = new Map((customers.data?.items ?? []).map((c) => [c.id, c.legalName]))
  const canPay = role ? can(role, 'receivables.edit') : false
  const register = canPay && (
    <Button variant="primary" onClick={() => navigate('/cobranza/pagos/nuevo')}>
      {t('ar.payment.register')}
    </Button>
  )

  return (
    <div className={styles.page}>
      <PageHeader section={t('nav.section.receivables')} title={t('pay.list.title')} actions={register} />
      <DataTable
        label={t('pay.list.title')}
        columns={paymentColumns(names)}
        rows={list.data?.items}
        rowKey={(p) => p.id}
        status={list.status}
        error={list.error}
        onRetry={() => void list.refetch()}
        onRowClick={(p) => navigate(`/cobranza/pagos/${p.id}`)}
        minWidth={1080}
        empty={
          <EmptyState
            icon={Banknote}
            title={t('pay.empty.title')}
            body={t('pay.empty.body')}
            action={register}
          />
        }
        cursor={pager.controls(list.data?.nextCursor ?? null)}
      />
    </div>
  )
}

function paymentColumns(names: Map<string, string>): Column<Payment>[] {
  return [
    {
      key: 'date',
      header: t('pay.col.date'),
      width: '96px',
      cell: (p) => <span className={styles.tabular}>{formatBusinessDate(p.receivedOn)}</span>,
    },
    {
      key: 'customer',
      header: t('pay.col.customer'),
      minWidth: 190,
      cell: (p) => <span className={styles.ellipsis}>{names.get(p.customerId) ?? '—'}</span>,
    },
    {
      key: 'method',
      header: t('pay.col.method'),
      width: '150px',
      cell: (p) => <span className={styles.ellipsis}>{paymentMethodLabel(p.paymentMethodCode)}</span>,
    },
    {
      key: 'reference',
      header: t('pay.col.reference'),
      width: '150px',
      hideOnMobile: true,
      cell: (p) => <span className={styles.ellipsis}>{p.reference ?? '—'}</span>,
    },
    {
      key: 'amount',
      header: t('pay.col.amount'),
      width: '120px',
      align: 'right',
      cell: (p) => <span className={styles.amount}>{formatMoney(p.amount, p.currency)}</span>,
    },
    {
      key: 'applied',
      header: t('pay.col.applied'),
      width: '120px',
      align: 'right',
      cell: (p) => <span className={styles.tabular}>{formatMoney(appliedOf(p), p.currency)}</span>,
    },
    {
      key: 'unapplied',
      header: t('pay.col.unapplied'),
      width: '120px',
      align: 'right',
      cell: (p) => {
        const left = unappliedOf(p)
        return (
          <span className={left.gt(0) ? styles.warnText : styles.tabular}>
            {formatMoney(left, p.currency)}
          </span>
        )
      },
    },
    {
      key: 'status',
      header: t('pay.col.status'),
      width: '120px',
      cell: (p) => <StatusBadge domain="payment" status={p.status} />,
    },
  ]
}
