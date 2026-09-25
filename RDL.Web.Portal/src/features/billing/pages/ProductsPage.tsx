import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Package, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState } from '@/design-system/components/Feedback/Feedback'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { SearchField, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Product, TaxOption } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { taxLabel } from '../products'
import { useDebouncedValue } from '../shared'
import { usePager } from '../usePager'
import styles from './ProductsPage.module.css'

/** Pantalla 10 · Productos y servicios (prototipo «10»). */
export function ProductsPage() {
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const q = useDebouncedValue(search)
  const pager = usePager([q])

  const list = useQuery({
    queryKey: ['products', activeOrg?.id, { q, cursor: pager.cursor }],
    queryFn: () => ds.products.list({ q, cursor: pager.cursor, limit: 20 }),
    placeholderData: keepPreviousData,
    enabled: !!activeOrg,
  })
  const taxes = useQuery({
    queryKey: ['catalogs', activeOrg?.id, 'taxOptions'],
    queryFn: () => ds.catalogs.taxOptions(),
    enabled: !!activeOrg,
    retry: false,
    staleTime: Infinity,
  })
  const canEdit = role ? can(role, 'billing.edit') : false

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.billing')}
        title={t('products.title')}
        count={list.data ? String(list.data.items.length) : undefined}
        actions={
          canEdit && (
            <Button variant="primary" onClick={() => navigate('/productos/nuevo')}>
              {t('products.new')}
            </Button>
          )
        }
      />
      <Toolbar>
        <SearchField
          aria-label={t('products.search')}
          placeholder={t('products.search')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <span className={styles.illustrative}>
          <TriangleAlert size={14} aria-hidden /> {t('products.illustrative')}
        </span>
      </Toolbar>
      <DataTable
        label={t('products.title')}
        columns={productColumns(taxes.data)}
        rows={list.data?.items}
        rowKey={(p) => p.id}
        status={list.status}
        error={list.error}
        onRetry={() => void list.refetch()}
        onRowClick={canEdit ? (p) => navigate(`/productos/${p.id}/editar`) : undefined}
        minWidth={980}
        empty={
          q ? (
            <EmptyState icon={Package} title={t('products.noMatch')} />
          ) : (
            <EmptyState
              icon={Package}
              title={t('products.empty.title')}
              body={t('products.empty.body')}
              action={
                canEdit && (
                  <Button variant="primary" onClick={() => navigate('/productos/nuevo')}>
                    {t('products.empty.action')}
                  </Button>
                )
              }
            />
          )
        }
        footer={list.data && t('products.pageCount', { n: list.data.items.length })}
        cursor={pager.controls(list.data?.nextCursor ?? null)}
      />
    </div>
  )
}

/** Columnas del prototipo «10», fuera del componente. */
function productColumns(taxOptions: TaxOption[] | undefined): Column<Product>[] {
  return [
    {
      key: 'code',
      header: t('products.col.code'),
      width: '110px',
      cell: (p) => <span className={styles.mono}>{p.code}</span>,
    },
    {
      key: 'description',
      header: t('products.col.description'),
      minWidth: 220,
      cell: (p) => <span className={styles.desc}>{p.description}</span>,
    },
    {
      key: 'cabys',
      header: t('products.col.cabys'),
      width: '150px',
      hideOnMobile: true,
      cell: (p) => <span className={styles.mono}>{p.cabysCode}</span>,
    },
    {
      key: 'kind',
      header: t('products.col.kind'),
      width: '90px',
      hideOnMobile: true,
      cell: (p) => t(p.isService ? 'products.kind.service' : 'products.kind.good'),
    },
    {
      key: 'price',
      header: t('products.col.price'),
      width: '150px',
      align: 'right',
      cell: (p) => (
        <span className={styles.tabular}>
          {t('products.price', { price: formatMoney(p.unitPrice, p.currency), unit: p.unitOfMeasureCode })}
        </span>
      ),
    },
    {
      key: 'tax',
      header: t('products.col.tax'),
      width: '110px',
      cell: (p) => taxLabel(p.taxes, taxOptions),
    },
    {
      key: 'status',
      header: t('products.col.status'),
      width: '110px',
      cell: (p) => <StatusBadge domain="record" status={p.isActive ? 'active' : 'inactive'} />,
    },
  ]
}
