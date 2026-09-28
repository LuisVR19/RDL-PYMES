import { useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { UsersRound } from 'lucide-react'
import { useState } from 'react'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { EmptyState, InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { Member, MemberPatch } from '@/shared/api/admin-types'
import type { Page } from '@/shared/api/billing-types'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { initialsOf } from '@/shared/api/gateway'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { can, isRole, ROLE_LABEL, ROLES, type Role } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './MembersTab.module.css'

type Pager = { hasPrev: boolean; hasNext: boolean; onPrev: () => void; onNext: () => void }

/**
 * Pantalla 30 · Miembros. Las reglas las hace cumplir Platform (403 `owner-required`, 409 `last-owner`); el portal
 * las anticipa para no ofrecer lo que va a fallar: nadie se cambia a sí mismo, solo un propietario toca a otro
 * propietario o asigna ese rol, y no se deja la organización sin propietario activo.
 */
export function MembersTab({ query, pager }: { query: UseQueryResult<Page<Member>>; pager: Pager }) {
  const ds = useDataSource()
  const { user, role, activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const [error, setError] = useState<{ message: string; ref?: string } | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  const members = query.data?.items ?? []
  const actorOwner = role ? can(role, 'admin.owners') : false
  const activeOwners = members.filter((m) => m.status === 'active' && m.roles.includes('owner'))
  const isLastOwner = (m: Member) =>
    m.status === 'active' && m.roles.includes('owner') && activeOwners.length <= 1

  async function update(m: Member, patch: MemberPatch, done: { title: string; body: string }) {
    setBusy(m.userId)
    setError(null)
    try {
      await ds.members.update(m.userId, patch)
      await queryClient.invalidateQueries({ queryKey: ['members', activeOrg?.id] })
      toast({ tone: 'success', ...done })
    } catch (err) {
      setError(messageOf(err))
    } finally {
      setBusy(null)
    }
  }

  function changeRole(m: Member, next: Role) {
    if ((m.roles.includes('owner') || next === 'owner') && !actorOwner) {
      return setError({ message: t('admin.users.ownerRequired') })
    }
    if (next !== 'owner' && isLastOwner(m)) return setError({ message: t('admin.users.lastOwner') })
    void update(
      m,
      { role: next },
      { title: t('admin.users.roleChanged'), body: `${m.fullName}: ${ROLE_LABEL[next]}` },
    )
  }

  function toggle(m: Member) {
    const suspend = m.status === 'active'
    if (suspend && isLastOwner(m)) return setError({ message: t('admin.users.lastOwnerSuspend') })
    void update(
      m,
      { status: suspend ? 'suspended' : 'active' },
      { title: t(suspend ? 'admin.users.suspended' : 'admin.users.reactivated'), body: m.fullName },
    )
  }

  const columns = memberColumns({ meId: user?.id, actorOwner, busy, changeRole, toggle })

  return (
    <div className={styles.tab}>
      <InlineAlert tone="info">{t('admin.users.ownerRule')}</InlineAlert>
      {error && (
        <InlineAlert tone="danger" refCode={error.ref}>
          {error.message}
        </InlineAlert>
      )}
      <DataTable
        label={t('admin.users.tab.members')}
        columns={columns}
        rows={query.data?.items}
        rowKey={(m) => m.userId}
        status={query.status}
        error={query.error}
        onRetry={() => void query.refetch()}
        minWidth={820}
        empty={<EmptyState icon={UsersRound} title={t('admin.users.empty')} />}
        cursor={pager}
      />
    </div>
  )
}

/** Columnas del prototipo «30». Fuera del componente: las celdas no son componentes que se creen en cada render. */
function memberColumns({
  meId,
  actorOwner,
  busy,
  changeRole,
  toggle,
}: {
  meId: string | undefined
  actorOwner: boolean
  busy: string | null
  changeRole: (m: Member, next: Role) => void
  toggle: (m: Member) => void
}): Column<Member>[] {
  return [
    {
      key: 'member',
      header: t('admin.users.col.member'),
      minWidth: 220,
      cell: (m) => (
        <span className={styles.who}>
          <span className={styles.avatar} aria-hidden>
            {initialsOf(m.fullName || m.email)}
          </span>
          <span className={styles.name}>{m.fullName || m.email}</span>
          {m.userId === meId && <span className={styles.you}>{t('admin.users.you')}</span>}
        </span>
      ),
    },
    {
      key: 'email',
      header: t('admin.users.col.email'),
      hideOnMobile: true,
      cell: (m) => <span className={styles.email}>{m.email}</span>,
    },
    {
      key: 'role',
      header: t('admin.users.col.role'),
      width: '170px',
      cell: (m) => {
        const current = m.roles[0]
        const touchesOwner = m.roles.includes('owner') && !actorOwner
        const locked = !current || m.userId === meId || touchesOwner || busy === m.userId
        const options = ROLES.filter((r) => r !== 'owner' || actorOwner || r === current)
        return (
          <select
            className={styles.role}
            aria-label={t('admin.users.roleOf', { name: m.fullName || m.email })}
            value={current ?? ''}
            disabled={locked}
            onChange={(e) => {
              const next = e.target.value
              if (isRole(next)) changeRole(m, next)
            }}
          >
            {!current && <option value="">{t('admin.users.unknownRole')}</option>}
            {options.map((r) => (
              <option key={r} value={r}>
                {ROLE_LABEL[r]}
              </option>
            ))}
          </select>
        )
      },
    },
    {
      key: 'status',
      header: t('admin.users.col.status'),
      width: '110px',
      cell: (m) => <StatusBadge domain="membership" status={m.status} />,
    },
    {
      key: 'actions',
      header: t('admin.users.col.actions'),
      width: '110px',
      align: 'right',
      cell: (m) => {
        const canToggle = m.userId !== meId && !(m.roles.includes('owner') && !actorOwner)
        if (!canToggle) return <span className={styles.none}>—</span>
        const suspend = m.status === 'active'
        return (
          <button
            type="button"
            className={styles.link}
            disabled={busy === m.userId}
            aria-label={t(suspend ? 'admin.users.suspendOf' : 'admin.users.reactivateOf', {
              name: m.fullName || m.email,
            })}
            onClick={() => toggle(m)}
          >
            {t(suspend ? 'admin.users.suspend' : 'admin.users.reactivate')}
          </button>
        )
      },
    },
  ]
}

function messageOf(err: unknown): { message: string; ref?: string } {
  const api = err instanceof ApiError ? err : null
  if (api?.is('last-owner')) return { message: t('admin.users.lastOwner') }
  if (api?.is('owner-required')) return { message: t('admin.users.ownerRequired') }
  return { message: t('admin.users.updateError'), ref: api?.correlationId || undefined }
}
