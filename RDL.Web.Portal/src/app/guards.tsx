import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
import { SessionExpiredDialog } from '@/app/layout/SessionExpiredDialog'
import { RETURN_PARAM } from '@/shared/auth/auth'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL, can, type Capability } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'

/**
 * Guardia de permiso por ruta: sin la capacidad, pantalla 403 con el rol actual. Es solo experiencia de usuario;
 * la API responde 403 igual si alguien llega a la ruta.
 */
export function RequireCapability({
  capability,
  children,
}: {
  capability: Capability | null
  children: ReactNode
}) {
  const { activeOrg, role, organizationsStatus } = useSession()
  if (organizationsStatus === 'pending') return <SkeletonRows rows={4} columns={3} />
  if (!capability) return <>{children}</>
  if (!activeOrg || !role) return <SystemScreen kind="unavailable" />
  if (!can(role, capability)) {
    return (
      <SystemScreen
        kind="403"
        body={t('sys.403.body', { org: activeOrg.legalName, role: ROLE_LABEL[role] })}
      />
    )
  }
  return <>{children}</>
}

/** Ruta actual, para volver a ella después de iniciar sesión. */
function loginPath(pathname: string, search: string): string {
  const back = pathname + search
  return back === '/' ? '/ingresar' : `/ingresar?${RETURN_PARAM}=${encodeURIComponent(back)}`
}

/**
 * Pantallas fuera del armazón que igual exigen sesión (2, 3 y 4). Sin sesión → pantalla 1 recordando a dónde iba;
 * con la sesión vencida, el diálogo de la pantalla 35 encima de la página.
 */
export function RequireSession({ children }: { children: ReactNode }) {
  const { status } = useAuth()
  const location = useLocation()
  if (status === 'loading') return null
  if (status === 'signedOut') return <Navigate to={loginPath(location.pathname, location.search)} replace />
  return (
    <>
      {children}
      <SessionExpiredDialog />
    </>
  )
}

/**
 * El armazón solo se dibuja con sesión y con una organización activa:
 *   - sin sesión → pantalla 1, recordando a dónde iba;
 *   - sesión vencida → la página sigue y encima el diálogo de la pantalla 35;
 *   - con sesión y sin organizaciones → pantalla 3 (crear la primera);
 *   - con organizaciones pero ninguna activa → pantalla 2 (elegir).
 */
export function RequireAuth({ children }: { children: ReactNode }) {
  const { status } = useAuth()
  const { organizations, organizationsStatus, activeOrg } = useSession()
  const location = useLocation()

  if (status === 'loading') return null
  if (status === 'signedOut') return <Navigate to={loginPath(location.pathname, location.search)} replace />
  // Sin la lista de organizaciones el armazón no se dibuja con datos a medias. Las pantallas de acceso no pasan
  // por aquí: la 2 muestra su propio error, con código y reintento.
  if (status === 'signedIn' && organizationsStatus === 'error') return <SystemScreen kind="unavailable" />
  if (status === 'signedIn' && organizationsStatus === 'success' && !activeOrg) {
    return <Navigate to={organizations.length === 0 ? '/organizaciones/nueva' : '/organizaciones'} replace />
  }
  return (
    <>
      {children}
      <SessionExpiredDialog />
    </>
  )
}
