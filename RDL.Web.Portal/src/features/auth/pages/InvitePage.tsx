import { CircleAlert, CircleSlash, Clock } from 'lucide-react'
import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './LoginPage.module.css'

type Outcome = 'expired' | 'notPending' | 'other'

/**
 * Pantalla 4 · Aceptar invitación (prototipo «04»). Exige sesión: el correo del usuario tiene que ser el de la
 * invitación, y eso lo decide Platform (si no coincide responde 404 sin revelar nada).
 *
 * TODO(api): el diseño muestra quién invitó, la organización, el rol y el vencimiento ANTES de aceptar. Platform no
 * expone la invitación por token (solo `POST .../accept`); falta `GET /v1/invitations/{token}` (docs/ESTADO.md).
 */
export function InvitePage() {
  const { token = '' } = useParams()
  const ds = useDataSource()
  const { session, signOut } = useAuth()
  const { switchOrganization } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [accepting, setAccepting] = useState(false)
  const [outcome, setOutcome] = useState<Outcome | null>(null)
  const [failure, setFailure] = useState<ApiError | null>(null)

  async function accept() {
    setAccepting(true)
    setFailure(null)
    try {
      // La misma clave para el mismo token: reintentar tras un error de red no duplica la membresía.
      const r = await ds.access.acceptInvitation(token, keyFor(token))
      switchOrganization(r.organizationId, {
        force: true,
        onDone: (ok) => {
          setAccepting(false)
          navigate(ok ? '/' : '/organizaciones', { replace: true })
          if (ok)
            toast({
              tone: 'success',
              title: t('invite.done'),
              body: t('invite.doneBody', { role: ROLE_LABEL[r.role] }),
            })
        },
      })
    } catch (err) {
      setAccepting(false)
      const api = err instanceof ApiError ? err : null
      if (api?.is('invitation-expired') || api?.status === 410) return setOutcome('expired')
      if (api?.is('invitation-not-pending')) return setOutcome('notPending')
      if (api?.status === 404) return setOutcome('other')
      setFailure(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    }
  }

  if (outcome) {
    const Icon = { expired: Clock, notPending: CircleSlash, other: CircleAlert }[outcome]
    const toLogin = outcome === 'other'
    return (
      <div className={styles.wrap}>
        <div className={`${styles.card} ${styles.wide}`}>
          <Icon size={36} className={styles.mutedIcon} aria-hidden />
          <div className={styles.heading}>
            <h1 className={styles.titleSm}>{t(`invite.${outcome}.title`)}</h1>
            <p className={styles.body}>
              {outcome === 'other'
                ? t('invite.other.body', { email: session?.email ?? '' })
                : t(`invite.${outcome}.body`)}
            </p>
          </div>
          <Button
            variant="secondary"
            size="lg"
            className={styles.submit}
            onClick={() =>
              toLogin
                ? void signOut().then(() =>
                    navigate(`/ingresar?volver=${encodeURIComponent(`/invitacion/${token}`)}`, {
                      replace: true,
                    }),
                  )
                : navigate('/organizaciones')
            }
          >
            {t(toLogin ? 'invite.other.action' : 'invite.toOrgs')}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.wrap}>
      <div className={`${styles.card} ${styles.wide}`}>
        <div className={styles.heading}>
          <h1 className={styles.titleSm}>{t('invite.title')}</h1>
          <p className={styles.body}>{t('invite.body')}</p>
        </div>
        {session?.email && (
          <dl className={styles.facts}>
            <dt>{t('invite.as')}</dt>
            <dd>{session.email}</dd>
          </dl>
        )}
        {failure && (
          <InlineAlert tone="danger" refCode={failure.correlationId || undefined}>
            {t('sys.unexpected.body')}
          </InlineAlert>
        )}
        <Button variant="primary" size="lg" className={styles.submit} disabled={accepting} onClick={accept}>
          {t(accepting ? 'invite.accepting' : 'invite.accept')}
        </Button>
      </div>
    </div>
  )
}
