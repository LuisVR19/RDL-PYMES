import { useQuery } from '@tanstack/react-query'
import { clsx } from 'clsx'
import { useId, useState, type KeyboardEvent } from 'react'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import { identificationText, useDebouncedValue } from '../shared'
import type { DraftCustomer } from './model'
import styles from './draft.module.css'

/**
 * Cliente del borrador (prototipo «13»): el elegido con «Cambiar», o un buscador con la lista de coincidencias y, al
 * final, «+ Crear cliente…» (alta rápida en un panel). Combobox ARIA: ↑ ↓ recorren, Enter elige, Esc cierra.
 */
export function CustomerPicker({
  value,
  error,
  onChange,
  onCreate,
}: {
  value: DraftCustomer | null
  error?: string
  onChange: (c: DraftCustomer | null) => void
  onCreate: (text: string) => void
}) {
  const id = useId()
  if (value) {
    return (
      <div className={styles.field}>
        <span className={styles.label} id={`${id}-label`}>
          {t('draft.customer')} <span className={styles.required}>*</span>
        </span>
        <div className={styles.chosen} aria-labelledby={`${id}-label`} role="group">
          <span className={styles.chosenText}>
            <span className={styles.chosenName}>{value.legalName}</span>
            {value.identification && (
              <span className={styles.muted}>{identificationText(value.identification)}</span>
            )}
          </span>
          <button type="button" className={styles.linkButton} onClick={() => onChange(null)}>
            {t('draft.customer.change')}
          </button>
        </div>
      </div>
    )
  }
  return <CustomerSearch id={id} error={error} onPick={onChange} onCreate={onCreate} />
}

const toDraft = (c: Customer): DraftCustomer => ({
  id: c.id,
  legalName: c.legalName,
  identification: c.identification,
})

function CustomerSearch({
  id,
  error,
  onPick,
  onCreate,
}: {
  id: string
  error?: string
  onPick: (c: DraftCustomer) => void
  onCreate: (text: string) => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const [text, setText] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const q = useDebouncedValue(text.trim())
  const found = useQuery({
    queryKey: ['customers', activeOrg?.id, 'picker', q],
    queryFn: () => ds.customers.list({ q: q || undefined, active: true, limit: 6 }),
    enabled: open && !!activeOrg,
  })
  const rows = found.data?.items ?? []
  // La última opción siempre es crear uno nuevo con lo escrito.
  const count = rows.length + 1
  const listId = `${id}-list`
  const optionId = (i: number) => `${id}-opt-${i}`

  function choose(i: number) {
    const row = rows[i]
    setOpen(false)
    if (row) onPick(toDraft(row))
    else onCreate(text.trim())
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setOpen(true)
      setActive((a) => (a + 1) % count)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive((a) => (a - 1 + count) % count)
    } else if (e.key === 'Enter' && open) {
      e.preventDefault()
      choose(active)
    } else if (e.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <div className={clsx(styles.field, styles.comboWrap)}>
      <label htmlFor={id} className={styles.label}>
        {t('draft.customer')} <span className={styles.required}>*</span>
      </label>
      <input
        id={id}
        className={clsx(styles.control, error && styles.invalid)}
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={open ? optionId(active) : undefined}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${id}-err` : undefined}
        autoComplete="off"
        placeholder={t('draft.customer.placeholder')}
        value={text}
        onChange={(e) => {
          setText(e.target.value)
          setActive(0)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={onKeyDown}
      />
      {error && (
        <span id={`${id}-err`} className={styles.error} role="alert">
          ✕ {error}
        </span>
      )}
      {open && (
        <div id={listId} role="listbox" aria-label={t('draft.customer')} className={styles.popList}>
          {found.isError && <div className={styles.popNote}>{t('draft.customer.error')}</div>}
          {found.isSuccess && rows.length === 0 && (
            <div className={styles.popNote}>{t('draft.customer.none')}</div>
          )}
          {rows.map((c, i) => (
            <div
              key={c.id}
              id={optionId(i)}
              role="option"
              tabIndex={-1}
              aria-selected={active === i}
              className={styles.popOption}
              // mousedown y no click: el blur del campo cerraría la lista antes.
              onMouseDown={(e) => {
                e.preventDefault()
                choose(i)
              }}
            >
              <span>{c.legalName}</span>
              <span className={styles.muted}>{identificationText(c.identification)}</span>
            </div>
          ))}
          <div
            id={optionId(rows.length)}
            role="option"
            tabIndex={-1}
            aria-selected={active === rows.length}
            className={clsx(styles.popOption, styles.popCreate)}
            onMouseDown={(e) => {
              e.preventDefault()
              choose(rows.length)
            }}
          >
            {text.trim() ? t('draft.customer.create', { name: text.trim() }) : t('draft.customer.createNew')}
          </div>
        </div>
      )}
    </div>
  )
}
