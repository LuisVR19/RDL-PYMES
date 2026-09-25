import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { CurrentUser, Organization } from '@/shared/api/types'
import { useAuth } from '@/shared/auth/AuthProvider'
import type { Role } from '@/shared/permissions/permissions'

/**
 * Sesión del portal: usuario, organizaciones y organización activa.
 *
 * Cambio de organización (pantalla 5, P8b paso 4.5 y la regla de aislamiento del repo de contratos):
 *   1. `PUT /portal/v1/me/active-organization` (Platform revalida la membresía);
 *   2. se refresca el token, para que el hook de Platform emita el `org_id` nuevo;
 *   3. se vacía TODA la caché de datos, antes y después, para que nada de la organización anterior quede;
 *   4. se vuelve a leer la lista de organizaciones (el detalle de la activa cambió).
 * Mientras dura, el contenido se reemplaza por esqueletos: no se ven datos de ninguna de las dos.
 * Si falla, se queda en la organización anterior y `switchError` lo dice.
 */
interface SessionValue {
  user: CurrentUser | undefined
  organizations: Organization[]
  organizationsStatus: 'pending' | 'error' | 'success'
  activeOrg: Organization | undefined
  role: Role | undefined
  switching: boolean
  switchError: unknown
  /**
   * Pide cambiar de organización; si hay cambios sin guardar, devuelve false y no cambia. `onDone` avisa al
   * terminar: true si quedó en la nueva, false si falló y siguió en la anterior.
   */
  switchOrganization: (orgId: string, opts?: SwitchOptions) => boolean
  /** Solo en la etapa de diseño: simula otro rol en la organización activa (barra de desarrollo). */
  setRoleOverride: (role: Role | undefined) => void
  /** Las pantallas con formularios registran si tienen cambios sin guardar. */
  setDirty: (dirty: boolean) => void
  isDirty: () => boolean
}

export interface SwitchOptions {
  force?: boolean
  onDone?: (ok: boolean) => void
}

const SessionContext = createContext<SessionValue | null>(null)

export const SESSION_QUERY_KEYS = ['session'] as const

/** Duración mínima de la transición, para que el cambio se note aunque la red sea instantánea. */
const MIN_TRANSITION_MS = 180

export function SessionProvider({
  children,
  initialOrgId,
}: {
  children: ReactNode
  /** Solo pruebas y datos simulados: fuerza la organización activa en vez de la del servidor. */
  initialOrgId?: string
}) {
  const ds = useDataSource()
  const auth = useAuth()
  const queryClient = useQueryClient()
  const [chosenOrgId, setChosenOrgId] = useState(initialOrgId)
  const [roleOverride, setRoleOverride] = useState<Role | undefined>()
  const [switching, setSwitching] = useState(false)
  const [switchError, setSwitchError] = useState<unknown>(null)
  const dirty = useRef(false)

  const signedIn = auth.status === 'signedIn'
  const user = useQuery({
    queryKey: ['session', 'user'],
    queryFn: () => ds.session.currentUser(),
    enabled: signedIn,
  })
  const orgs = useQuery({
    queryKey: ['session', 'organizations'],
    queryFn: () => ds.session.organizations(),
    enabled: signedIn,
  })

  const dropOrganizationData = useCallback(
    () => queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'session' }),
    [queryClient],
  )

  const switchOrganization = useCallback(
    (orgId: string, opts?: SwitchOptions) => {
      if (dirty.current && !opts?.force) return false
      dirty.current = false
      setSwitching(true)
      setSwitchError(null)
      setRoleOverride(undefined)
      dropOrganizationData()
      const started = Date.now()

      void (async () => {
        try {
          await ds.session.activateOrganization(orgId)
          await auth.port.refresh()
          // Lo que se haya pedido durante la transición salió con el token anterior: se descarta también.
          dropOrganizationData()
          setChosenOrgId(orgId)
          await queryClient.invalidateQueries({ queryKey: ['session', 'organizations'] })
          opts?.onDone?.(true)
        } catch (err) {
          setSwitchError(err)
          opts?.onDone?.(false)
        } finally {
          const wait = Math.max(0, MIN_TRANSITION_MS - (Date.now() - started))
          window.setTimeout(() => setSwitching(false), wait)
        }
      })()
      return true
    },
    [ds, auth.port, queryClient, dropOrganizationData],
  )

  const value = useMemo<SessionValue>(() => {
    const organizations = orgs.data?.items ?? []
    const activeOrgId = chosenOrgId ?? orgs.data?.activeOrganizationId ?? undefined
    const activeOrg = organizations.find((o) => o.id === activeOrgId)
    return {
      user: user.data,
      organizations,
      organizationsStatus: orgs.status,
      activeOrg,
      role: roleOverride ?? activeOrg?.role,
      switching,
      switchError,
      switchOrganization,
      setRoleOverride,
      setDirty: (d) => {
        dirty.current = d
      },
      isDirty: () => dirty.current,
    }
  }, [
    user.data,
    orgs.data,
    orgs.status,
    chosenOrgId,
    roleOverride,
    switching,
    switchError,
    switchOrganization,
  ])

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession fuera de SessionProvider')
  return ctx
}
