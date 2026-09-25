import { Construction } from 'lucide-react'
import { MODULE_LABEL, type ScreenDef } from '@/app/screens'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { Panel, Tag } from '@/design-system/components/Surface/Surface'
import { t } from '@/shared/i18n/t'
import { ROLES, ROLE_LABEL, can } from '@/shared/permissions/permissions'
import styles from './ScreenPlaceholder.module.css'

/**
 * Pantalla provisional: la ruta, el permiso y el lugar en el armazón ya existen; el diseño de la pantalla se
 * construye en el incremento de su módulo. Muestra de dónde sale (número y referencia del prototipo).
 */
export function ScreenPlaceholder({ screen }: { screen: ScreenDef }) {
  const roles = screen.capability
    ? ROLES.filter((r) => can(r, screen.capability!)).map((r) => ROLE_LABEL[r])
    : ['Usuario con sesión']
  return (
    <>
      <PageHeader section={MODULE_LABEL[screen.module]} title={screen.name} />
      <Panel className={styles.panel}>
        <div className={styles.icon} aria-hidden>
          <Construction size={22} />
        </div>
        <div className={styles.text}>
          <h2 className={styles.title}>{t('placeholder.title', { n: screen.n, name: screen.name })}</h2>
          <p className={styles.body}>{t('placeholder.body')}</p>
          <dl className={styles.facts}>
            <dt>{t('placeholder.route')}</dt>
            <dd>
              <code className="code">{screen.path}</code>
            </dd>
            <dt>{t('placeholder.roles')}</dt>
            <dd className={styles.tags}>
              {roles.map((r) => (
                <Tag key={r}>{r}</Tag>
              ))}
            </dd>
            <dt>{t('placeholder.reference')}</dt>
            <dd>
              design/referencias · «{screen.reference}»{screen.mobile ? ' · 📱 390 px' : ''}
            </dd>
          </dl>
        </div>
      </Panel>
    </>
  )
}
