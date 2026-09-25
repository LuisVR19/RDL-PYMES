import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { Organization } from '@/shared/api/types'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './ProfilePage.module.css'

/**
 * Pantalla 33 · Mi perfil (prototipo «33»): quién es, en qué organizaciones está y cerrar sesión.
 *
 * TODO(api): el diseño deja editar el nombre y guardarlo, pero Platform no tiene cómo (`/v1/me` es solo GET). Hasta
 * que exista `PATCH /v1/me`, el nombre se muestra como el correo: de solo lectura, con su explicación.
 */
export function ProfilePage() {
  const { user, organizations, activeOrg, switchOrganization } = useSession()
  const { signOut } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()

  if (!user) return <SkeletonRows rows={4} columns={2} />

  const pick = (org: Organization) => {
    if (org.id === activeOrg?.id) return
    const from = activeOrg
    switchOrganization(org.id, {
      force: true,
      onDone: (ok) => {
        if (!ok)
          return toast({ tone: 'danger', title: t('org.switch.failed', { org: from?.legalName ?? '' }) })
        navigate('/')
        toast({
          tone: 'info',
          title: t('org.switch.done', { org: org.legalName }),
          body: t('topbar.org.roleHere', { role: ROLE_LABEL[org.role] }),
        })
      },
    })
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>{t('profile.title')}</h1>

      <section className={styles.card} aria-labelledby="perfil-datos">
        <div className={styles.who}>
          <span className={styles.avatar} aria-hidden>
            {user.initials}
          </span>
          <div className={styles.whoText}>
            <span id="perfil-datos" className={styles.name}>
              {user.fullName}
            </span>
            <span className={styles.email}>{user.email}</span>
          </div>
        </div>
        <ReadOnly label={t('profile.name')} value={user.fullName} help={t('profile.nameHelp')} />
        <ReadOnly label={t('profile.email')} value={user.email} help={t('profile.emailHelp')} />
      </section>

      <section className={styles.card} aria-labelledby="perfil-orgs">
        <h2 id="perfil-orgs" className={styles.sectionTitle}>
          {t('profile.orgs')}
        </h2>
        <ul className={styles.list}>
          {organizations.map((o) => {
            const active = o.id === activeOrg?.id
            return (
              <li key={o.id}>
                <button
                  type="button"
                  className={styles.row}
                  onClick={() => pick(o)}
                  aria-current={active || undefined}
                >
                  <span className={styles.initials}>{o.initials}</span>
                  <span className={styles.orgName}>{o.legalName}</span>
                  <span className={active ? styles.tagActive : styles.tag}>
                    {active ? t('profile.active', { role: ROLE_LABEL[o.role] }) : ROLE_LABEL[o.role]}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      </section>

      <div>
        <Button
          variant="secondary"
          className={styles.logout}
          onClick={() => void signOut().then(() => navigate('/ingresar', { replace: true }))}
        >
          {t('user.logout')}
        </Button>
      </div>
    </div>
  )
}

/** Campo de solo lectura con la forma de un campo deshabilitado del prototipo (fondo gris, sin borde activo). */
function ReadOnly({ label, value, help }: { label: string; value: string; help: string }) {
  return (
    <div className={styles.field}>
      <span className={styles.label}>{label}</span>
      <div className={styles.readOnly}>{value}</div>
      <span className={styles.help}>{help}</span>
    </div>
  )
}
