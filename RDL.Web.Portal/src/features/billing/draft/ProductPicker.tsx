import * as Popover from '@radix-ui/react-popover'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Product } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { useSession } from '@/shared/session/SessionProvider'
import { useDebouncedValue } from '../shared'
import styles from './draft.module.css'

/**
 * «+ Agregar del catálogo» (prototipo «13»): busca entre los productos activos por código o descripción y agrega
 * el elegido como línea. El precio que se ve es el del producto; el de la línea se puede cambiar después.
 */
export function ProductPicker({ onPick }: { onPick: (p: Product) => void }) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const q = useDebouncedValue(text.trim())
  const found = useQuery({
    queryKey: ['products', activeOrg?.id, 'picker', q],
    queryFn: () => ds.products.list({ q: q || undefined, active: true, limit: 20 }),
    enabled: open && !!activeOrg,
  })
  const rows = found.data?.items ?? []

  return (
    <Popover.Root
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (!o) setText('')
      }}
    >
      <Popover.Trigger asChild>
        <Button variant="secondary" size="sm">
          {t('draft.addFromCatalog')}
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className={styles.catalog} side="top" align="start" sideOffset={6}>
          <input
            className={styles.control}
            aria-label={t('draft.catalog.search')}
            placeholder={t('draft.catalog.search')}
            value={text}
            onChange={(e) => setText(e.target.value)}
          />
          <ul className={styles.catalogList} aria-label={t('draft.addFromCatalog')}>
            {found.isError && <li className={styles.popNote}>{t('draft.catalog.error')}</li>}
            {found.isSuccess && rows.length === 0 && (
              <li className={styles.popNote}>{t('draft.catalog.none')}</li>
            )}
            {rows.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  className={styles.catalogItem}
                  onClick={() => {
                    onPick(p)
                    setOpen(false)
                    setText('')
                  }}
                >
                  <span>{p.description}</span>
                  <span className={styles.amount}>{formatMoney(p.unitPrice, p.currency)}</span>
                  <span className={styles.mono}>
                    {p.code} · CABYS {p.cabysCode}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
