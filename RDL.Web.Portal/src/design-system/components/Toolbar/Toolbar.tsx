import { clsx } from 'clsx'
import { Search } from 'lucide-react'
import type { InputHTMLAttributes, ReactNode } from 'react'
import styles from './Toolbar.module.css'

/** Buscador de listas (prototipo: alto de control, borde fuerte, icono a la izquierda). */
export function SearchField({
  className,
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & { 'aria-label': string }) {
  return (
    <div className={clsx(styles.search, className)}>
      <Search size={16} aria-hidden />
      <input type="search" {...rest} />
    </div>
  )
}

export interface SegmentOption<T extends string> {
  value: T
  label: string
  count?: number
}

/** Filtro segmentado («Activos / Inactivos / Todos»). */
export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: SegmentOption<T>[]
  value: T
  onChange: (v: T) => void
  label: string
}) {
  return (
    <div className={styles.segmented} role="radiogroup" aria-label={label}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={o.value === value}
          className={clsx(styles.segment, o.value === value && styles.active)}
          onClick={() => onChange(o.value)}
        >
          {o.label}
          {o.count !== undefined && <span className={styles.segCount}>{o.count}</span>}
        </button>
      ))}
    </div>
  )
}

export function Toolbar({ children }: { children: ReactNode }) {
  return <div className={styles.toolbar}>{children}</div>
}
