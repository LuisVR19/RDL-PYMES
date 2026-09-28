import type { ReactNode } from 'react'
import { lateText } from './model'
import styles from './receivables.module.css'

/** «! 18 días» (prototipo «22»), o «—» si está al día o ya no se cobra. */
export function LateBadge({ days }: { days: number }) {
  if (days <= 0) return <span className={styles.dash}>—</span>
  return <span className={styles.late}>! {lateText(days)}</span>
}

export interface Figure {
  label: string
  value: ReactNode
  sub?: ReactNode
  tone?: 'danger' | 'warning'
}

/** Las tres cifras de un detalle en una franja con separadores (prototipo «24» y «27»). */
export function Figures({ items }: { items: Figure[] }) {
  return (
    <div className={styles.figures}>
      {items.map((f) => (
        <div key={f.label} className={styles.figure}>
          <span className={styles.figLabel}>{f.label}</span>
          <span
            className={
              f.tone === 'danger'
                ? styles.figValueDanger
                : f.tone === 'warning'
                  ? styles.figValueWarning
                  : styles.figValue
            }
          >
            {f.value}
          </span>
          {f.sub && <span className={styles.figSub}>{f.sub}</span>}
        </div>
      ))}
    </div>
  )
}
