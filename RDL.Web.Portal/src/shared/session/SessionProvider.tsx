import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { CurrentUser, Organization } from '@/shared/api/types'
import type { Role } from '@/shared/permissions/permissions'

/**
 * Sesión del portal: usuario, organizaciones y organización activa.
 *
 * Cambio de organización (pantalla 5 y regla de aislamiento del repo de contratos): se vacía TODA la caché de datos
 * para que ningún dato de la organización anterior quede en pantalla; la interfaz muestra la transición (atenuado y
 * esqueletos, 180 ms). En la etapa de cableado, además se llamará `PUT /v1/me/active-organization` y se refrescará
 * el token para que traiga el nuevo `org_id`.
 */
interface SessionValue {
  user: CurrentUser | undefined
  organizations: Organization[]
  organizationsStatus: 'pending' | 'error' | 'success'
  activeOrg: Organization | undefined
  role: Role | undefined
  switching: boolean
  /** Pide cambiar de organización; si hay cambios sin guardar, devuelve false y no cambia. */
  switchOrganization: (orgId: string, opts?: { force?: boolean }) => boolean
  /** Solo en la etapa de diseño: simula otro rol en la organización activa (barra de desarrollo). */
  setRoleOverride: (role: Role | undefined) => void
  /** Las pantallas con formularios registran si tienen cambios sin guardar. */
  setDirty: (dirty: boolean) => void
  isDirty: () => boolean
}

const SessionContext = createContext<SessionValue | null>(null)

export const SESSION_QUERY_KEYS = ['session'] as const

export function SessionProvider({
  children,
  initialOrgId = 'ca',
}: {
  children: ReactNode
  initialOrgId?: string
}) {
  const ds = useDataSource()
  const queryClient = useQueryClient()
  const [activeOrgId, setActiveOrgId] = useState(initialOrgId)
  const [roleOverride, setRoleOverride] = useState<Role | undefined>()
  const [switching, setSwitching] = useState(false)
  const dirty = useRef(false)

  const user = useQuery({ queryKey: ['session', 'user'], queryFn: () => ds.session.currentUser() })
  const orgs = useQuery({ queryKey: ['session', 'organizations'], queryFn: () => ds.session.organizations() })

  const switchOrganization = useCallback(
    (orgId: string, opts?: { force?: boolean }) => {
      if (dirty.current && !opts?.force) return false
      dirty.current = false
      setSwitching(true)
      // Nada de la organización anterior sobrevive: solo se conservan los datos de sesión.
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'session' })
      setRoleOverride(undefined)
      setActiveOrgId(orgId)
      window.setTimeout(() => setSwitching(false), 180)
      return true
    },
    [queryClient],
  )

  const value = useMemo<SessionValue>(() => {
    const organizations = orgs.data ?? []
    const activeOrg = organizations.find((o) => o.id === activeOrgId)
    return {
      user: user.data,
      organizations,
      organizationsStatus: orgs.status,
      activeOrg,
      role: roleOverride ?? activeOrg?.role,
      switching,
      switchOrganization,
      setRoleOverride,
      setDirty: (d) => {
        dirty.current = d
      },
      isDirty: () => dirty.current,
    }
  }, [user.data, orgs.data, orgs.status, activeOrgId, roleOverride, switching, switchOrganization])

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession fuera de SessionProvider')
  return ctx
}
