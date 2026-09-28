import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { TabPanel, Tabs } from '@/design-system/components/Tabs/Tabs'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import { usePager } from '@/features/billing/usePager'
import { InvitationsTab } from '../InvitationsTab'
import { MembersTab } from '../MembersTab'
import styles from './UsersPage.module.css'

const INVITATIONS_PATH = '/admin/usuarios/invitaciones'

/**
 * Pantallas 30 · Usuarios y roles y 31 · Invitaciones (prototipo «30», con «31» como segunda pestaña). Cada pestaña
 * tiene su ruta. Las dos listas se piden aquí para mostrar los contadores de las pestañas.
 */
export function UsersPage() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const tab = pathname.startsWith(INVITATIONS_PATH) ? 'invitations' : 'members'
  const org = activeOrg?.id

  const membersPager = usePager([])
  const members = useQuery({
    queryKey: ['members', org, { cursor: membersPager.cursor }],
    queryFn: () => ds.members.list({ cursor: membersPager.cursor, limit: 100 }),
    placeholderData: keepPreviousData,
    enabled: !!org,
  })
  const invitationsPager = usePager([])
  const invitations = useQuery({
    queryKey: ['invitations', org, { cursor: invitationsPager.cursor }],
    queryFn: () => ds.invitations.list({ cursor: invitationsPager.cursor, limit: 100 }),
    placeholderData: keepPreviousData,
    enabled: !!org,
  })

  const pending = invitations.data?.items.filter((i) => i.status === 'pending').length

  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.admin')}
        title={t('nav.users')}
        actions={
          tab === 'members' && (
            <Button variant="primary" onClick={() => navigate(INVITATIONS_PATH)}>
              {t('admin.users.invite')}
            </Button>
          )
        }
      />
      <Tabs
        label={t('nav.users')}
        value={tab}
        onValueChange={(v) => navigate(v === 'invitations' ? INVITATIONS_PATH : '/admin/usuarios')}
        items={[
          { value: 'members', label: t('admin.users.tab.members'), count: members.data?.items.length },
          { value: 'invitations', label: t('admin.users.tab.invitations'), count: pending },
        ]}
      >
        <TabPanel value="members">
          <MembersTab query={members} pager={membersPager.controls(members.data?.nextCursor ?? null)} />
        </TabPanel>
        <TabPanel value="invitations">
          <InvitationsTab
            query={invitations}
            members={members.data?.items ?? []}
            pager={invitationsPager.controls(invitations.data?.nextCursor ?? null)}
          />
        </TabPanel>
      </Tabs>
    </div>
  )
}
