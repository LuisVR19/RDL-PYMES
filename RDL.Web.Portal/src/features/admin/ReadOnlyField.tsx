import { clsx } from 'clsx'
import styles from './ReadOnlyField.module.css'

/**
 * Campo de solo lectura con la forma de un campo deshabilitado del prototipo (fondo gris, sin borde activo), con
 * la explicación de por qué no se edita.
 */
export function ReadOnlyField({
  label,
  value,
  help,
  className,
  tabular,
}: {
  label: string
  value: string
  help?: string
  className?: string
  tabular?: boolean
}) {
  return (
    <div className={clsx(styles.field, className)}>
      <span className={styles.label}>{label}</span>
      <div className={clsx(styles.value, tabular && styles.tabular)}>{value}</div>
      {help && <span className={styles.help}>{help}</span>}
    </div>
  )
}
