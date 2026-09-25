import * as RTooltip from '@radix-ui/react-tooltip'
import { clsx } from 'clsx'
import type { HTMLAttributes, ReactNode } from 'react'
import styles from './Surface.module.css'

/** Tarjeta o panel: fondo de superficie, borde, radio 6 (prototipo). */
export function Panel({
  className,
  padded = true,
  ...rest
}: HTMLAttributes<HTMLDivElement> & { padded?: boolean }) {
  return <div className={clsx(styles.panel, padded && styles.padded, className)} {...rest} />
}

export function PanelHeader({
  title,
  actions,
  children,
}: {
  title: ReactNode
  actions?: ReactNode
  children?: ReactNode
}) {
  return (
    <div className={styles.panelHeader}>
      <div>
        <h2 className={styles.panelTitle}>{title}</h2>
        {children}
      </div>
      {actions}
    </div>
  )
}

/** Dato que no es estado (sucursal, condición de venta). */
export function Tag({ children }: { children: ReactNode }) {
  return <span className={styles.tag}>{children}</span>
}

export function Kbd({ children }: { children: ReactNode }) {
  return <kbd className={styles.kbd}>{children}</kbd>
}

export function Tooltip({ content, children }: { content: ReactNode; children: ReactNode }) {
  return (
    <RTooltip.Root delayDuration={300}>
      <RTooltip.Trigger asChild>{children}</RTooltip.Trigger>
      <RTooltip.Portal>
        <RTooltip.Content className={styles.tooltip} sideOffset={6}>
          {content}
        </RTooltip.Content>
      </RTooltip.Portal>
    </RTooltip.Root>
  )
}

export const TooltipProvider = RTooltip.Provider
