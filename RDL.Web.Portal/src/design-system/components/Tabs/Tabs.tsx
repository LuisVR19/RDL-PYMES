import * as RTabs from '@radix-ui/react-tabs'
import { clsx } from 'clsx'
import type { ReactNode } from 'react'
import styles from './Tabs.module.css'

export interface TabItem {
  value: string
  label: string
  count?: number
  /** Tono del contador (la bandeja resalta rechazados en rojo). */
  countTone?: 'danger' | 'warning' | 'neutral'
}

/**
 * Pestañas con subrayado (prototipo: separación 22 px, borde inferior de 2 px en la activa, contador en píldora).
 * Controladas: la pestaña suele vivir en la URL (`?estado=`).
 */
export function Tabs({
  items,
  value,
  onValueChange,
  label,
  children,
}: {
  items: TabItem[]
  value: string
  onValueChange: (v: string) => void
  label: string
  children?: ReactNode
}) {
  return (
    <RTabs.Root value={value} onValueChange={onValueChange} className={styles.root}>
      <RTabs.List className={styles.list} aria-label={label}>
        {items.map((it) => (
          <RTabs.Trigger key={it.value} value={it.value} className={styles.trigger}>
            {it.label}
            {it.count !== undefined && (
              <span className={clsx(styles.count, it.countTone && styles[it.countTone])}>{it.count}</span>
            )}
          </RTabs.Trigger>
        ))}
      </RTabs.List>
      {children}
    </RTabs.Root>
  )
}

export const TabPanel = RTabs.Content
