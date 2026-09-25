import { useQueryClient } from '@tanstack/react-query'
import { ChevronRight } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { EmptyState, ErrorState, Skeleton } from '@/design-system/components/Feedback/Feedback'
import { useToast } from '@/design-system/components/Toast/Toast'
import { ApiError, type Organization } from '@/shared/api/types'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './OrgSelectPage.module.css'

/**
 * Pantalla 2 · Selector de organización (prototipo «02»): la ve quien tiene varias organizaciones y ninguna activa,
 * o quien llega a propósito. Elegir una es el mismo cambio seguro de la pantalla 5 (token nuevo, caché vacía).
 */
export function OrgSelectPage() {
  const { user, organizations, organizationsStatus, activeOrg, switchOrganization, switching } = useSession()
  const { signOut } = useAuth()
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')

  const matches = useMemo(() => {
    const q = query.trim().toLocaleLowerCase('es-CR')
    if (!q) return organizations
    return organizations.filter(
      (o) => o.legalName.toLocaleLowerCase('es-CR').includes(q) || (o.identification ?? '').includes(q),
    )
  }, [organizations, query])

  const pick = (org: Organization) => {
    // Ya es la activa (el token la trae): no hace falta cambiar nada.
    if (org.id === activeOrg?.id) return navigate('/')
    switchOrganization(org.id, {
      force: true,
      onDone: (ok) => {
        if (!ok)
          return toast({ tone: 'danger', title: t('org.switch.failed', { org: activeOrg?.legalName ?? '' }) })
        navigate('/')
        toast({
          tone: 'info',
          title: t('org.switch.done', { org: org.legalName }),
          body: t('topbar.org.roleHere', { role: ROLE_LABEL[org.role] }),
        })
      },
    })
  }

  const firstName = user?.fullName.split(/\s+/)[0]
  const orgsError = queryClient.getQueryState(['session', 'organizations'])?.error

  return (
    <div className={styles.page}>
      <div className={styles.top}>
        <button
          type="button"
          className={styles.link}
          onClick={() => void signOut().then(() => navigate('/ingresar', { replace: true }))}
        >
          {t('user.logout')}
        </button>
      </div>
      <div className={styles.heading}>
        <h1 className={styles.title}>{t('org.select.title')}</h1>
        <span className={styles.subtitle}>
          {firstName ? t('org.select.hello', { name: firstName }) : t('org.select.helloAnon')}
        </span>
      </div>

      <div className={styles.card} aria-busy={organizationsStatus === 'pending' || switching}>
        {organizationsStatus === 'pending' && (
          <ul className={styles.list} aria-label={t('common.loading')}>
            {[0, 1, 2, 3].map((i) => (
              <li key={i} className={styles.skeletonRow}>
                <Skeleton width={36} height={36} radius={6} />
                <span className={styles.skeletonText}>
                  <Skeleton width="55%" height={12} />
                  <Skeleton width="30%" height={10} />
                </span>
              </li>
            ))}
          </ul>
        )}

        {organizationsStatus === 'error' && (
          <ErrorState
            title={t('org.select.errorTitle')}
            body={t('org.select.errorBody')}
            onRetry={() => void queryClient.refetchQueries({ queryKey: ['session', 'organizations'] })}
            refCode={orgsError instanceof ApiError ? orgsError.correlationId : undefined}
          />
        )}

        {organizationsStatus === 'success' && organizations.length === 0 && (
          <EmptyState
            title={t('org.select.empty')}
            body={t('org.select.emptyBody')}
            action={
              <Button variant="primary" onClick={() => navigate('/organizaciones/nueva')}>
                {t('org.select.emptyAction')}
              </Button>
            }
          />
        )}

        {organizationsStatus === 'success' && organizations.length > 0 && (
          <>
            <div className={styles.searchBox}>
              <input
                className={styles.search}
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('org.select.search')}
                aria-label={t('org.select.search')}
              />
            </div>
            <ul className={styles.list}>
              {matches.map((o) => (
                <li key={o.id}>
                  <button type="button" className={styles.row} onClick={() => pick(o)} disabled={switching}>
                    <span className={styles.initials}>{o.initials}</span>
                    <span className={styles.names}>
                      <span className={styles.name}>{o.legalName}</span>
                      {o.identification && <span className={styles.idn}>{o.identification}</span>}
                    </span>
                    <span className={styles.role}>{ROLE_LABEL[o.role]}</span>
                    <ChevronRight size={16} className={styles.chevron} aria-hidden />
                  </button>
                </li>
              ))}
            </ul>
            {matches.length === 0 && (
              <p className={styles.noMatch}>{t('org.select.noMatch', { q: query })}</p>
            )}
            <Link to="/organizaciones/nueva" className={styles.create}>
              {t('org.select.create')}
            </Link>
          </>
        )}
      </div>
    </div>
  )
}
