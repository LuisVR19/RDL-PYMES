import type { ReactNode } from 'react'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
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

/** Mientras no hay sesión ni organizaciones, el armazón no se dibuja con datos a medias. */
export function SessionGate({ children }: { children: ReactNode }) {
  const { organizationsStatus } = useSession()
  if (organizationsStatus === 'error') return <SystemScreen kind="unavailable" />
  return <>{children}</>
}
