import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Users } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState } from '@/design-system/components/Feedback/Feedback'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { SearchField, SegmentedControl, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer, Receivable } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { identificationText, MoneyByCurrency, totalsByCurrency, useDebouncedValue } from '../shared'
import { usePager } from '../usePager'
import styles from './ClientsPage.module.css'

type Filter = 'active' | 'inactive' | 'all'
const ACTIVE: Record<Filter, boolean | undefined> = { active: true, inactive: false, all: undefined }

/**
 * Pantalla 7 · Clientes (prototipo «07»). La lista es de Billing; el saldo por cobrar de cada fila es de Receivables
 * y se pide de una sola vez para toda la página (si no se puede, la columna dice «No disponible» sin romper la tabla).
 */
export function ClientsPage() {
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState<Filter>('active')
  const q = useDebouncedValue(search)
  const pager = usePager([q, filter])

  const list = useQuery({
    queryKey: ['customers', activeOrg?.id, { q, filter, cursor: pager.cursor }],
    queryFn: () => ds.customers.list({ q, active: ACTIVE[filter], cursor: pager.cursor, limit: 20 }),
    placeholderData: keepPreviousData,
    enabled: !!activeOrg,
  })
  const ids = list.data?.items.map((c) => c.id) ?? []
  const balances = useQuery({
    queryKey: ['customers', activeOrg?.id, 'balances', ids],
    queryFn: () => ds.receivables.balancesByCustomer(ids),
    enabled: ids.length > 0,
    retry: false,
  })

  const columns = clientColumns(balances)

  const filtered = q !== '' || filter !== 'active'
  const canEdit = role ? can(role, 'billing.edit') : false

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.billing')}
        title={t('clients.title')}
        count={list.data ? String(list.data.items.length) : undefined}
        actions={
          canEdit && (
            <Button variant="primary" onClick={() => navigate('/clientes/nuevo')}>
              {t('clients.new')}
            </Button>
          )
        }
      />
      <Toolbar>
        <SearchField
          aria-label={t('clients.search')}
          placeholder={t('clients.search')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <SegmentedControl<Filter>
          label={t('clients.filter')}
          value={filter}
          onChange={setFilter}
          options={[
            { value: 'active', label: t('clients.filter.active') },
            { value: 'inactive', label: t('clients.filter.inactive') },
            { value: 'all', label: t('clients.filter.all') },
          ]}
        />
      </Toolbar>
      <DataTable
        label={t('clients.title')}
        columns={columns}
        rows={list.data?.items}
        rowKey={(c) => c.id}
        status={list.status}
        error={list.error}
        onRetry={() => void list.refetch()}
        onRowClick={(c) => navigate(`/clientes/${c.id}`)}
        minWidth={940}
        empty={
          filtered ? (
            <EmptyState icon={Users} title={t('clients.noMatch')} />
          ) : (
            <EmptyState
              icon={Users}
              title={t('clients.empty.title')}
              body={t('clients.empty.body')}
              action={
                canEdit && (
                  <Button variant="primary" onClick={() => navigate('/clientes/nuevo')}>
                    {t('clients.empty.action')}
                  </Button>
                )
              }
            />
          )
        }
        footer={list.data && t('clients.pageCount', { n: list.data.items.length })}
        cursor={pager.controls(list.data?.nextCursor ?? null)}
      />
    </div>
  )
}

type BalanceState = { isPending: boolean; isError: boolean; data?: Record<string, Receivable[]> }

/** Columnas del prototipo «07». Fuera del componente: las celdas no son componentes que se creen en cada render. */
function clientColumns(balances: BalanceState): Column<Customer>[] {
  return [
    {
      key: 'name',
      header: `${t('clients.col.customer')} ↑`,
      minWidth: 220,
      cell: (c) => (
        <span className={styles.name}>
          <span className={styles.legal}>{c.legalName}</span>
          <span className={styles.trade}>{c.tradeName || '—'}</span>
        </span>
      ),
    },
    {
      key: 'id',
      header: t('clients.col.identification'),
      width: '170px',
      cell: (c) => <span className={styles.tabular}>{identificationText(c.identification)}</span>,
    },
    {
      key: 'email',
      header: t('clients.col.email'),
      hideOnMobile: true,
      cell: (c) => <span className={styles.ellipsis}>{c.email || '—'}</span>,
    },
    {
      key: 'phone',
      header: t('clients.col.phone'),
      width: '110px',
      hideOnMobile: true,
      cell: (c) => <span className={styles.tabular}>{c.phone || '—'}</span>,
    },
    {
      key: 'balance',
      header: t('clients.col.balance'),
      width: '150px',
      align: 'right',
      cell: (c) => {
        if (balances.isPending) return <span className={styles.muted}>…</span>
        if (balances.isError) return <span className={styles.muted}>{t('common.notAvailable')}</span>
        const open = (balances.data?.[c.id] ?? []).filter(
          (r) => r.status === 'open' || r.status === 'partially_paid',
        )
        return (
          <MoneyByCurrency
            totals={totalsByCurrency(open.map((r) => ({ currency: r.currency, amount: r.balanceAmount })))}
          />
        )
      },
    },
    {
      key: 'status',
      header: t('clients.col.status'),
      width: '110px',
      cell: (c) => <StatusBadge domain="record" status={c.isActive ? 'active' : 'inactive'} />,
    },
  ]
}
