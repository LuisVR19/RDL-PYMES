import { Outlet } from 'react-router'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
import { isMockDataSource } from '@/shared/api/DataSourceProvider'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { DevToolbar } from './DevToolbar'

/** Raíz de todas las rutas: solo con datos simulados, agrega la barra de revisión. */
export function RootFrame() {
  return (
    <>
      <Outlet />
      {isMockDataSource && <DevToolbar />}
    </>
  )
}

/** `/403` directo (p. ej. al volver de un enlace sin permiso). */
export function ForbiddenPage() {
  const { activeOrg, role } = useSession()
  return (
    <SystemScreen
      kind="403"
      body={
        activeOrg && role
          ? t('sys.403.body', { org: activeOrg.legalName, role: ROLE_LABEL[role] })
          : undefined
      }
    />
  )
}
