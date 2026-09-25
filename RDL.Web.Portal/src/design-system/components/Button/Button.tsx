import { clsx } from 'clsx'
import { Loader2 } from 'lucide-react'
import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react'
import styles from './Button.module.css'

export type ButtonVariant = 'primary' | 'secondary' | 'tertiary' | 'danger' | 'icon'
export type ButtonSize = 'sm' | 'md' | 'lg'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  /** Mantiene el ancho y muestra el texto en gerundio («Emitiendo…»). */
  loading?: boolean
  loadingLabel?: string
  icon?: ReactNode
  /** Atajo de teclado que se muestra dentro del botón. */
  kbd?: string
}

/** Botón del sistema de diseño (componentes.md): radio 6, texto 14/500, alturas 32/36/44. */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'secondary',
    size = 'md',
    loading = false,
    loadingLabel,
    icon,
    kbd,
    className,
    children,
    disabled,
    type = 'button',
    ...rest
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      className={clsx(styles.button, styles[variant], styles[size], loading && styles.loading, className)}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <Loader2 className={styles.spinner} size={16} aria-hidden /> : icon}
      {loading && loadingLabel ? loadingLabel : children}
      {kbd && !loading && <kbd className={styles.kbd}>{kbd}</kbd>}
    </button>
  )
})
