import { ChevronLeft } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'
import styles from './PageHeader.module.css'

export interface PageHeaderProps {
  /** Sección («Facturación») o enlace de regreso («‹ Documentos»). */
  section?: string
  back?: { label: string; to: string }
  title: ReactNode
  /** Contador junto al título («Clientes 24»). */
  count?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
}

/** Encabezado de página del prototipo: sección 12 px, título 24/600 con contador y acciones a la derecha. */
export function PageHeader({ section, back, title, count, subtitle, actions }: PageHeaderProps) {
  return (
    <header className={styles.header}>
      <div className={styles.titles}>
        {back ? (
          <Link to={back.to} className={styles.back}>
            <ChevronLeft size={14} aria-hidden />
            {back.label}
          </Link>
        ) : (
          section && <span className={styles.section}>{section}</span>
        )}
        <h1 className={styles.title}>
          {title}
          {count !== undefined && <span className={styles.count}> {count}</span>}
        </h1>
        {subtitle && <div className={styles.subtitle}>{subtitle}</div>}
      </div>
      {actions && <div className={styles.actions}>{actions}</div>}
    </header>
  )
}
