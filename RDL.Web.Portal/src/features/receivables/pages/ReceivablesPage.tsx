import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Wallet } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState } from '@/design-system/components/Feedback/Feedback'
import { Select } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { SegmentedControl, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { MoneyByCurrency, totalsByCurrency } from '@/features/billing/shared'
import { usePager } from '@/features/billing/usePager'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Receivable, ReceivableQuery } from '@/shared/api/billing-types'
import { DEFAULT_TZ, formatBusinessDate, todayIn } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { collectable, displayStatus, overdueDays } from '../model'
import { LateBadge } from '../parts'
import styles from '../receivables.module.css'

type Filter = 'all' | 'open' | 'partially_paid' | 'overdue' | 'paid'
const FILTERS: Filter[] = ['all', 'open', 'partially_paid', 'overdue', 'paid']

/** Cada filtro es una sola consulta de Receivables (su `GET /v1/receivables` filtra por un estado o por vencida). */
const QUERY: Record<Filter, ReceivableQuery> = {
  all: {},
  open: { status: 'open' },
  partially_paid: { status: 'partially_paid' },
  overdue: { overdue: true },
  paid: { status: 'paid' },
}

/**
 * Pantalla 22 · Cuentas por cobrar (prototipo «22»). Cada factura a crédito emitida con su saldo, días de atraso y
 * estado. Saldo y estado son de Receivables; el atraso y «Vencida» se muestran contra el día de negocio de la
 * organización.
 *
 * Fuera del diseño por el contrato: el orden por vencimiento (Receivables ordena de la más nueva a la más vieja) y
 * «Pendientes» sin las de pago parcial (el filtro es por un estado): por eso «Pago parcial» es su propio filtro.
 */
export function ReceivablesPage() {
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const filter = FILTERS.find((f) => f === params.get('estado')) ?? 'all'
  const customerId = params.get('cliente') ?? ''
  const pager = usePager([filter, customerId])
  const org = activeOrg?.id
  const today = todayIn(activeOrg?.timezone ?? DEFAULT_TZ)

  const query: ReceivableQuery = {
    ...QUERY[filter],
    customerId: customerId || undefined,
    cursor: pager.cursor,
    limit: 20,
  }
  const list = useQuery({
    queryKey: ['receivables', org, 'list', query],
    queryFn: () => ds.receivables.list(query),
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

  const setParam = (key: string, value: string | undefined) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  const rows = list.data?.items
  const pageBalance = totalsByCurrency(
    (rows ?? []).filter(collectable).map((r) => ({ currency: r.currency, amount: r.balanceAmount })),
  )
  const canPay = role ? can(role, 'receivables.edit') : false
  const filtered = filter !== 'all' || customerId !== ''

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.receivables')}
        title={t('ar.list.title')}
        actions={
          canPay && (
            <Button variant="primary" onClick={() => navigate('/cobranza/pagos/nuevo')}>
              {t('ar.payment.register')}
            </Button>
          )
        }
      />
      <Toolbar>
        <SegmentedControl<Filter>
          label={t('ar.filter.label')}
          value={filter}
          onChange={(v) => setParam('estado', v === 'all' ? undefined : v)}
          options={FILTERS.map((f) => ({ value: f, label: t(`ar.filter.${f}`) }))}
        />
        <Select
          label={t('ar.filter.customer')}
          className={styles.filter}
          value={customerId}
          onChange={(e) => setParam('cliente', e.target.value || undefined)}
          options={[
            { value: '', label: t('ar.filter.allCustomers') },
            ...(customers.data?.items ?? []).map((c) => ({ value: c.id, label: c.legalName })),
          ]}
        />
      </Toolbar>
      <DataTable
        label={t('ar.list.title')}
        columns={receivableColumns(names, today)}
        rows={rows}
        rowKey={(r) => r.id}
        status={list.status}
        error={list.error}
        onRetry={() => void list.refetch()}
        onRowClick={(r) => navigate(`/cobranza/cuentas/${r.id}`)}
        minWidth={1040}
        empty={
          filtered ? (
            <EmptyState icon={Wallet} title={t('ar.empty.filtered')} />
          ) : (
            <EmptyState icon={Wallet} title={t('ar.empty.title')} body={t('ar.empty.body')} />
          )
        }
        footer={
          rows && (
            <span className={styles.footer}>
              {t('ar.pageBalance')} <MoneyByCurrency totals={pageBalance} />
            </span>
          )
        }
        cursor={pager.controls(list.data?.nextCursor ?? null)}
      />
    </div>
  )
}

function receivableColumns(names: Map<string, string>, today: string): Column<Receivable>[] {
  return [
    {
      key: 'document',
      header: t('ar.col.document'),
      width: '130px',
      cell: (r) => <span className={styles.mono}>{r.documentNumber}</span>,
    },
    {
      key: 'customer',
      header: t('ar.col.customer'),
      minWidth: 200,
      cell: (r) => (
        <span className={styles.ellipsis}>{r.customerLegalName ?? names.get(r.customerId) ?? '—'}</span>
      ),
    },
    {
      key: 'issued',
      header: t('ar.col.issued'),
      width: '92px',
      cell: (r) => <span className={styles.tabular}>{formatBusinessDate(r.issuedOn)}</span>,
    },
    {
      key: 'due',
      header: t('ar.col.due'),
      width: '92px',
      cell: (r) => <span className={styles.tabular}>{formatBusinessDate(r.dueOn)}</span>,
    },
    {
      key: 'late',
      header: t('ar.col.overdue'),
      width: '100px',
      cell: (r) => <LateBadge days={overdueDays(r, today)} />,
    },
    {
      key: 'original',
      header: t('ar.col.original'),
      width: '130px',
      align: 'right',
      cell: (r) => <span className={styles.tabular}>{formatMoney(r.originalAmount, r.currency)}</span>,
    },
    {
      key: 'balance',
      header: t('ar.col.balance'),
      width: '130px',
      align: 'right',
      cell: (r) => <span className={styles.amount}>{formatMoney(r.balanceAmount, r.currency)}</span>,
    },
    {
      key: 'status',
      header: t('ar.col.status'),
      width: '124px',
      cell: (r) => <StatusBadge domain="receivable" status={displayStatus(r, today)} />,
    },
  ]
}
