import { Outlet } from 'react-router'
import { t } from '@/shared/i18n/t'
import { BrandMark } from './BrandMark'
import styles from './AuthLayout.module.css'

/** Marco de las pantallas de acceso (1–4): fondo de lienzo, marca arriba y tarjeta centrada. */
export function AuthLayout() {
  return (
    <div className={styles.page}>
      <div className={styles.brand}>
        <BrandMark size={36} />
        <span className={styles.brandText}>{t('app.brand')}</span>
      </div>
      <main className={styles.main}>
        <Outlet />
      </main>
    </div>
  )
}
