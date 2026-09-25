import { useQuery } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { Dialog } from '@/design-system/components/Dialog/Dialog'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { CabysItem } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import { useDebouncedValue } from './shared'
import styles from './CabysDialog.module.css'

const CABYS = /^\d{13}$/

/**
 * Buscador CABYS (prototipo «11 Buscador CABYS»). El catálogo es de E-Invoice: si no responde (hoy no existe), se
 * puede escribir el código de 13 dígitos a mano; Billing valida el formato y E-Invoice lo validará contra el oficial.
 */
export function CabysDialog({
  open,
  onOpenChange,
  onPick,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onPick: (item: CabysItem | { code: string; description?: undefined }) => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const [text, setText] = useState('')
  const [manual, setManual] = useState('')
  const [manualError, setManualError] = useState<string>()
  const q = useDebouncedValue(text.trim())

  const search = useQuery({
    queryKey: ['catalogs', activeOrg?.id, 'cabys', q],
    queryFn: () => ds.catalogs.searchCabys(q),
    enabled: open && q.length >= 2,
    retry: false,
  })

  const pick = (item: CabysItem | { code: string }) => {
    onPick(item)
    onOpenChange(false)
  }

  const useManual = () => {
    if (!CABYS.test(manual.trim())) return setManualError(t('cabys.manualInvalid'))
    pick({ code: manual.trim() })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('cabys.title')} width={620}>
      <div className={styles.body}>
        <TextField
          label={t('cabys.search')}
          placeholder={t('cabys.searchPlaceholder')}
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <p className={styles.illustrative}>
          <TriangleAlert size={14} aria-hidden /> {t('cabys.illustrative')}
        </p>

        {search.isError ? (
          <div className={styles.manual}>
            <p className={styles.unavailable}>{t('cabys.unavailable')}</p>
            <TextField
              label={t('cabys.manual')}
              inputMode="numeric"
              value={manual}
              error={manualError}
              onChange={(e) => {
                setManual(e.target.value)
                setManualError(undefined)
              }}
            />
            <Button variant="secondary" onClick={useManual}>
              {t('cabys.useCode')}
            </Button>
          </div>
        ) : search.isFetching ? (
          <SkeletonRows rows={3} columns={2} />
        ) : search.data && search.data.length === 0 ? (
          <p className={styles.none}>{t('cabys.none', { q })}</p>
        ) : (
          <ul className={styles.list} aria-label={t('cabys.results')}>
            {(search.data ?? []).map((c) => (
              <li key={c.code}>
                <button type="button" className={styles.row} onClick={() => pick(c)}>
                  <span className={styles.code}>{c.code}</span>
                  <span className={styles.desc}>{c.description}</span>
                  <span className={styles.choose}>{t('cabys.choose')}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Dialog>
  )
}
