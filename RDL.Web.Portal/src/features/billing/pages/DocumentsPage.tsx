import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { FileText } from 'lucide-react'
import { useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { Kbd } from '@/design-system/components/Surface/Surface'
import { Tabs } from '@/design-system/components/Tabs/Tabs'
import { SegmentedControl, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { DocumentType, InvoiceListItem, InvoiceStatus } from '@/shared/api/billing-types'
import { DEFAULT_TZ, formatInstantDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { statusStyle } from '@/shared/status/status'
import { FiscalBadge } from '../FiscalBadge'
import { MoneyByCurrency, totalsByCurrency } from '../shared'
import { usePager } from '../usePager'
import styles from './DocumentsPage.module.css'

type Tab = 'all' | DocumentType
type Density = 'comfortable' | 'compact'
const TABS: Tab[] = ['all', 'invoice', 'credit_note', 'debit_note']
const STATUSES: InvoiceStatus[] = ['draft', 'issued', 'cancelled']

/**
 * Pantalla 12 · Documentos (prototipo «12»). Es el listado COMPUESTO del gateway: la página de Billing con el estado de
 * Hacienda y el saldo de cada fila (una llamada por API por página). Si Hacienda o Cobranza no responden, esa celda
 * dice que no está disponible y la tabla sigue entera.
 *
 * Fuera del diseño por el contrato: el filtro por estado de Hacienda (Billing no puede filtrar por un dato de
 * E-Invoice), los contadores de las pestañas (exigirían contar todo) y «Exportar» (sin endpoint). TODO(api).
 */
export function DocumentsPage() {
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const tab = (TABS.find((x) => x === params.get('tipo')) ?? 'all') as Tab
  const status = STATUSES.find((x) => x === params.get('estado'))
  const [customerId, setCustomerId] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [density, setDensity] = useState<Density>('comfortable')
  const pager = usePager([tab, status, customerId, from, to])
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ

  const query = {
    documentType: tab === 'all' ? undefined : tab,
    status,
    customerId: customerId || undefined,
    issuedFrom: from || undefined,
    issuedTo: to || undefined,
    cursor: pager.cursor,
    limit: 20,
  }
  const list = useQuery({
    queryKey: ['invoices', org, query],
    queryFn: () => ds.invoices.list(query),
    placeholderData: keepPreviousData,
    enabled: !!org,
  })
  // Nombres de cliente para los borradores (el snapshot solo existe desde la emisión) y para el filtro.
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
    (rows ?? []).flatMap((r) =>
      r.receivable.balance
        ? [{ currency: r.receivable.balance.currency, amount: r.receivable.balance.balanceAmount }]
        : [],
    ),
  )
  const canEdit = role ? can(role, 'billing.edit') : false

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.billing')}
        title={t('docs.title')}
        actions={
          canEdit && (
            <Button variant="primary" onClick={() => navigate('/facturas/nueva')}>
              {t('docs.new')} <Kbd>N</Kbd>
            </Button>
          )
        }
      />
      <Tabs
        label={t('docs.tabs')}
        value={tab}
        onValueChange={(v) => setParam('tipo', v === 'all' ? undefined : v)}
        items={TABS.map((x) => ({ value: x, label: t(`docs.tab.${x}`) }))}
      />
      <Toolbar>
        <Select
          label={t('docs.filter.status')}
          className={styles.filter}
          value={status ?? ''}
          onChange={(e) => setParam('estado', e.target.value || undefined)}
          options={[
            { value: '', label: t('docs.filter.all') },
            ...STATUSES.map((x) => ({ value: x, label: statusLabel(x) })),
          ]}
        />
        <Select
          label={t('docs.filter.customer')}
          className={styles.filterWide}
          value={customerId}
          onChange={(e) => setCustomerId(e.target.value)}
          options={[
            { value: '', label: t('docs.filter.allCustomers') },
            ...(customers.data?.items ?? []).map((c) => ({ value: c.id, label: c.legalName })),
          ]}
        />
        <TextField
          label={t('docs.filter.from')}
          type="date"
          className={styles.filter}
          value={from}
          onChange={(e) => setFrom(e.target.value)}
        />
        <TextField
          label={t('docs.filter.to')}
          type="date"
          className={styles.filter}
          value={to}
          onChange={(e) => setTo(e.target.value)}
        />
        <div className={styles.density}>
          <SegmentedControl<Density>
            label={t('docs.density')}
            value={density}
            onChange={setDensity}
            options={[
              { value: 'comfortable', label: t('docs.density.comfortable') },
              { value: 'compact', label: t('docs.density.compact') },
            ]}
          />
        </div>
      </Toolbar>
      <DataTable
        label={t('docs.title')}
        columns={documentColumns(names, tz)}
        rows={rows}
        rowKey={(r) => r.invoice.id}
        status={list.status}
        error={list.error}
        onRetry={() => void list.refetch()}
        onRowClick={(r) =>
          navigate(
            r.invoice.status === 'draft' ? `/facturas/${r.invoice.id}/editar` : `/facturas/${r.invoice.id}`,
          )
        }
        density={density}
        minWidth={1180}
        empty={
          <EmptyState
            icon={FileText}
            title={t('docs.empty.title')}
            body={t('docs.empty.body')}
            action={
              canEdit && (
                <Button variant="primary" onClick={() => navigate('/facturas/nueva')}>
                  {t('docs.new')}
                </Button>
              )
            }
          />
        }
        footer={
          rows && (
            <span className={styles.footer}>
              {t('docs.pageBalance')} <MoneyByCurrency totals={pageBalance} />
            </span>
          )
        }
        cursor={pager.controls(list.data?.nextCursor ?? null)}
      />
    </div>
  )
}

const statusLabel = (s: InvoiceStatus) => statusStyle('invoice', s).label

/** Columnas del prototipo «12», fuera del componente. */
function documentColumns(names: Map<string, string>, tz: string): Column<InvoiceListItem>[] {
  return [
    {
      key: 'number',
      header: t('docs.col.number'),
      width: '170px',
      cell: ({ invoice: i }) => (
        <span className={styles.number}>
          <span className={i.status === 'draft' ? styles.monoMuted : styles.mono}>
            {i.number ?? t('doc.draftNumber')}
          </span>
          {i.requiresCorrection && <StatusBadge domain="invoice" status="requires_correction" size="sm" />}
        </span>
      ),
    },
    {
      key: 'type',
      header: t('docs.col.type'),
      width: '48px',
      cell: ({ invoice: i }) => (
        <abbr className={styles.type} title={t(`doc.type.${i.documentType}`)}>
          {t(`docs.type.${i.documentType}`)}
        </abbr>
      ),
    },
    {
      key: 'customer',
      header: t('docs.col.customer'),
      minWidth: 200,
      cell: ({ invoice: i }) => (
        <span className={styles.ellipsis}>{i.customerLegalName ?? names.get(i.customerId) ?? '—'}</span>
      ),
    },
    {
      key: 'date',
      header: t('docs.col.date'),
      width: '112px',
      cell: ({ invoice: i }) => (
        <span className={styles.tabular}>{formatInstantDate(i.issuedAt ?? i.createdAt, tz)}</span>
      ),
    },
    {
      key: 'status',
      header: t('docs.col.status'),
      width: '120px',
      cell: ({ invoice: i }) => <StatusBadge domain="invoice" status={i.status} />,
    },
    {
      key: 'fiscal',
      header: t('docs.col.fiscal'),
      width: '190px',
      cell: ({ fiscal }) =>
        fiscal.availability === 'absent' ? (
          <span className={styles.muted}>{t('docs.noFiscal')}</span>
        ) : (
          <FiscalBadge part={fiscal} />
        ),
    },
    {
      key: 'total',
      header: t('docs.col.total'),
      width: '140px',
      align: 'right',
      cell: ({ invoice: i }) => <span className={styles.amount}>{formatMoney(i.total, i.currency)}</span>,
    },
    {
      key: 'balance',
      header: t('docs.col.balance'),
      width: '130px',
      align: 'right',
      cell: ({ receivable }) =>
        receivable.balance ? (
          <span className={styles.tabular}>
            {formatMoney(receivable.balance.balanceAmount, receivable.balance.currency)}
          </span>
        ) : receivable.availability === 'unavailable' ? (
          <span className={styles.muted}>{t('common.notAvailable')}</span>
        ) : (
          <span className={styles.muted}>—</span>
        ),
    },
  ]
}
