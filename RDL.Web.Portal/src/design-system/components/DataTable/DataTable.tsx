import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { useRef, type KeyboardEvent, type ReactNode } from 'react'
import { t } from '@/shared/i18n/t'
import { ApiError } from '@/shared/api/types'
import { ErrorState, SkeletonRows } from '../Feedback/Feedback'
import styles from './DataTable.module.css'

export interface Column<R> {
  key: string
  header: ReactNode
  cell: (row: R) => ReactNode
  /** Ancho CSS (`170px`, `1.6fr` se traduce a proporción con `minWidth`). */
  width?: string
  minWidth?: number
  align?: 'left' | 'right'
  /** Ocultar en móvil (<768). */
  hideOnMobile?: boolean
}

export interface DataTableProps<R> {
  columns: Column<R>[]
  rows: R[] | undefined
  rowKey: (row: R) => string
  label: string
  status: 'pending' | 'error' | 'success'
  error?: unknown
  onRetry?: () => void
  empty: ReactNode
  onRowClick?: (row: R) => void
  density?: 'comfortable' | 'compact'
  /** Texto del pie («8 clientes en esta página»). */
  footer?: ReactNode
  cursor?: { hasPrev: boolean; hasNext: boolean; onPrev: () => void; onNext: () => void }
  minWidth?: number
  rowTone?: (row: R) => 'danger' | undefined
}

/**
 * Tabla de datos (componentes.md): encabezado fijo, numéricas a la derecha, fila activa con ↑ ↓, Enter abre.
 * Paginación por cursor: solo «Anterior / Siguiente», nunca «página 7 de 20».
 */
export function DataTable<R>({
  columns,
  rows,
  rowKey,
  label,
  status,
  error,
  onRetry,
  empty,
  onRowClick,
  density = 'comfortable',
  footer,
  cursor,
  minWidth = 760,
  rowTone,
}: DataTableProps<R>) {
  const bodyRef = useRef<HTMLTableSectionElement>(null)

  const onKey = (e: KeyboardEvent<HTMLTableRowElement>, row: R) => {
    const tr = e.currentTarget
    if (e.key === 'Enter' && onRowClick) {
      e.preventDefault()
      onRowClick(row)
    } else if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const sibling = (
        e.key === 'ArrowDown' ? tr.nextElementSibling : tr.previousElementSibling
      ) as HTMLElement | null
      sibling?.focus()
    }
  }

  const refCode = error instanceof ApiError ? error.correlationId : undefined

  return (
    <div className={styles.wrap}>
      <div className={styles.scroll}>
        <table
          className={clsx(styles.table, styles[density])}
          style={{ minWidth }}
          aria-label={label}
          aria-busy={status === 'pending'}
        >
          <colgroup>
            {columns.map((c) => (
              <col key={c.key} style={{ width: c.width }} />
            ))}
            {onRowClick && <col style={{ width: 32 }} />}
          </colgroup>
          <thead>
            <tr>
              {columns.map((c) => (
                <th
                  key={c.key}
                  scope="col"
                  className={clsx(c.align === 'right' && styles.right, c.hideOnMobile && styles.hideMobile)}
                >
                  {c.header}
                </th>
              ))}
              {onRowClick && (
                <th aria-hidden>
                  <span className="sr-only">Abrir</span>
                </th>
              )}
            </tr>
          </thead>
          {status === 'success' && rows && rows.length > 0 && (
            <tbody ref={bodyRef}>
              {rows.map((row) => (
                <tr
                  key={rowKey(row)}
                  tabIndex={onRowClick ? 0 : undefined}
                  className={clsx(
                    onRowClick && styles.clickable,
                    rowTone?.(row) === 'danger' && styles.rowDanger,
                  )}
                  onClick={onRowClick ? () => onRowClick(row) : undefined}
                  onKeyDown={onRowClick ? (e) => onKey(e, row) : undefined}
                >
                  {columns.map((c) => (
                    <td
                      key={c.key}
                      className={clsx(
                        c.align === 'right' && styles.right,
                        c.hideOnMobile && styles.hideMobile,
                      )}
                    >
                      {c.cell(row)}
                    </td>
                  ))}
                  {onRowClick && (
                    <td className={styles.chevron} aria-hidden>
                      <ChevronRight size={16} />
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          )}
        </table>
        {status === 'pending' && <SkeletonRows columns={Math.min(columns.length, 5)} />}
        {status === 'error' && <ErrorState onRetry={onRetry} refCode={refCode} />}
        {status === 'success' && (!rows || rows.length === 0) && empty}
      </div>
      {status === 'success' && rows && rows.length > 0 && (footer || cursor) && (
        <div className={styles.footer}>
          <span>{footer}</span>
          {cursor && (
            <div className={styles.pager}>
              <button
                type="button"
                className={styles.pageBtn}
                disabled={!cursor.hasPrev}
                onClick={cursor.onPrev}
              >
                {t('common.prev')}
              </button>
              <button
                type="button"
                className={styles.pageBtn}
                disabled={!cursor.hasNext}
                onClick={cursor.onNext}
              >
                {t('common.next')}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
