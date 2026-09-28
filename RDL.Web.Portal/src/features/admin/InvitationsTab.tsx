import { useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { Copy, MailPlus } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { ConfirmDialog } from '@/design-system/components/Dialog/Dialog'
import { EmptyState, InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { Invitation, Member, NewInvitation } from '@/shared/api/admin-types'
import type { Page } from '@/shared/api/billing-types'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, formatInstantDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { can, isRole, ROLE_LABEL, ROLES, type Role } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './InvitationsTab.module.css'

type Pager = { hasPrev: boolean; hasNext: boolean; onPrev: () => void; onNext: () => void }

const EMAIL = /^\S+@\S+\.\S+$/

/**
 * Pantalla 31 · Invitaciones. Hoy el portal no envía correos (Platform: TODO(notificaciones)): al crear la invitación
 * se muestra el enlace para compartirlo a mano, una sola vez, porque Platform solo guarda el hash del token.
 */
export function InvitationsTab({
  query,
  members,
  pager,
}: {
  query: UseQueryResult<Page<Invitation>>
  members: Member[]
  pager: Pager
}) {
  const ds = useDataSource()
  const { role, activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const tz = activeOrg?.timezone ?? DEFAULT_TZ
  const actorOwner = role ? can(role, 'admin.owners') : false
  const roles = ROLES.filter((r) => r !== 'owner' || actorOwner)

  const [email, setEmail] = useState('')
  const [newRole, setNewRole] = useState<Role>('biller')
  const [formError, setFormError] = useState<string | undefined>()
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [sending, setSending] = useState(false)
  const [created, setCreated] = useState<{ email: string; role: Role; link?: string } | null>(null)
  const [revoking, setRevoking] = useState<Invitation | null>(null)
  const [revokeError, setRevokeError] = useState<ApiError | null>(null)
  const [revokeSending, setRevokeSending] = useState(false)

  const invitations = query.data?.items ?? []
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['invitations', activeOrg?.id] })

  async function invite(e: FormEvent) {
    e.preventDefault()
    const address = email.trim().toLowerCase()
    setServerError(null)
    if (!EMAIL.test(address)) return setFormError(t('admin.invite.emailInvalid'))
    if (members.some((m) => m.status === 'active' && m.email.toLowerCase() === address)) {
      return setFormError(t('admin.invite.alreadyMember'))
    }
    if (invitations.some((i) => i.status === 'pending' && i.email.toLowerCase() === address)) {
      return setFormError(t('admin.invite.alreadyPending'))
    }
    const input: NewInvitation = { email: address, role: newRole }
    setSending(true)
    try {
      const inv = await ds.invitations.create(input, keyFor(input))
      setCreated({
        email: inv.email,
        role: inv.role,
        link: inv.token ? `${window.location.origin}/invitacion/${encodeURIComponent(inv.token)}` : undefined,
      })
      setEmail('')
      await refresh()
      toast({ tone: 'success', title: t('admin.invite.created'), body: t('admin.invite.createdBody') })
    } catch (err) {
      const api = err instanceof ApiError ? err : null
      if (api?.status === 409) setFormError(t('admin.invite.conflict'))
      else if (api?.is('owner-required')) setFormError(t('admin.users.ownerRequired'))
      else if (api?.errors.some((fe) => fe.field === 'email')) setFormError(t('admin.invite.emailInvalid'))
      else
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    } finally {
      setSending(false)
    }
  }

  async function revoke(inv: Invitation) {
    setRevokeSending(true)
    setRevokeError(null)
    try {
      await ds.invitations.revoke(inv.id)
      await refresh()
      if (created?.email === inv.email) setCreated(null)
      setRevoking(null)
      toast({ tone: 'success', title: t('admin.invite.revoked'), body: inv.email })
    } catch (err) {
      setRevokeError(
        err instanceof ApiError
          ? err
          : new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }),
      )
    } finally {
      setRevokeSending(false)
    }
  }

  async function copy(link: string) {
    try {
      await navigator.clipboard.writeText(link)
      toast({ tone: 'success', title: t('admin.invite.copied') })
    } catch {
      toast({ tone: 'warning', title: t('admin.invite.copyFailed') })
    }
  }

  const columns = invitationColumns(tz, (i) => {
    setRevokeError(null)
    setRevoking(i)
  })

  return (
    <div className={styles.tab}>
      <form className={styles.form} onSubmit={invite} noValidate aria-label={t('admin.users.invite')}>
        <TextField
          className={styles.email}
          label={t('admin.invite.email')}
          required
          type="email"
          placeholder={t('auth.login.emailPlaceholder')}
          value={email}
          error={formError}
          onChange={(e) => {
            setEmail(e.target.value)
            setFormError(undefined)
          }}
        />
        <Select
          label={t('admin.users.col.role')}
          value={newRole}
          options={roles.map((r) => ({ value: r, label: ROLE_LABEL[r] }))}
          onChange={(e) => {
            const r = e.target.value
            if (isRole(r)) setNewRole(r)
          }}
        />
        <Button type="submit" variant="primary" className={styles.submit} loading={sending}>
          {t('admin.invite.submit')}
        </Button>
        <span className={styles.help}>{t('admin.invite.noEmail')}</span>
      </form>

      {serverError && (
        <InlineAlert tone="danger" refCode={serverError.correlationId || undefined}>
          {t('admin.invite.serverError')}
        </InlineAlert>
      )}

      {created?.link && (
        <div className={styles.linkBox} role="status">
          <strong className={styles.linkTitle}>
            {t('admin.invite.linkFor', { email: created.email, role: ROLE_LABEL[created.role] })}
          </strong>
          <div className={styles.linkRow}>
            <code className={styles.link}>{created.link}</code>
            <Button variant="secondary" size="sm" onClick={() => void copy(created.link ?? '')}>
              <Copy size={14} aria-hidden /> {t('admin.invite.copy')}
            </Button>
          </div>
          <span className={styles.linkWarning}>{t('admin.invite.linkOnce')}</span>
        </div>
      )}
      {created && !created.link && <InlineAlert tone="info">{t('admin.invite.noToken')}</InlineAlert>}

      <DataTable
        label={t('admin.users.tab.invitations')}
        columns={columns}
        rows={query.data?.items}
        rowKey={(i) => i.id}
        status={query.status}
        error={query.error}
        onRetry={() => void query.refetch()}
        minWidth={820}
        empty={
          <EmptyState icon={MailPlus} title={t('admin.invite.empty')} body={t('admin.invite.emptyBody')} />
        }
        cursor={pager}
      />

      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(o) => !o && setRevoking(null)}
        title={t('admin.invite.revokeTitle')}
        summary={
          revoking
            ? [
                { label: t('admin.users.col.email'), value: revoking.email },
                { label: t('admin.users.col.role'), value: ROLE_LABEL[revoking.role] },
              ]
            : undefined
        }
        warning={t('admin.invite.revokeWarning')}
        confirmLabel={t('admin.invite.revoke')}
        confirmingLabel={t('admin.invite.revoking')}
        tone="danger"
        sending={revokeSending}
        errorMessage={revokeError ? t('admin.invite.revokeError') : undefined}
        errorRef={revokeError?.correlationId || undefined}
        onConfirm={() => revoking && void revoke(revoking)}
      />
    </div>
  )
}

/** Columnas del prototipo «31». Fuera del componente: las celdas no son componentes que se creen en cada render. */
function invitationColumns(tz: string, onRevoke: (i: Invitation) => void): Column<Invitation>[] {
  return [
    {
      key: 'email',
      header: t('admin.users.col.email'),
      minWidth: 220,
      cell: (i) => <span className={styles.ellipsis}>{i.email}</span>,
    },
    { key: 'role', header: t('admin.users.col.role'), width: '130px', cell: (i) => ROLE_LABEL[i.role] },
    {
      key: 'created',
      header: t('admin.invite.col.created'),
      hideOnMobile: true,
      width: '116px',
      cell: (i) => <span className={styles.tabular}>{formatInstantDate(i.createdAt, tz)}</span>,
    },
    {
      key: 'expires',
      header: t('admin.invite.col.expires'),
      width: '116px',
      cell: (i) => <span className={styles.tabular}>{formatInstantDate(i.expiresAt, tz)}</span>,
    },
    {
      key: 'status',
      header: t('admin.users.col.status'),
      width: '120px',
      cell: (i) => <StatusBadge domain="invitation" status={i.status} />,
    },
    {
      key: 'actions',
      header: t('admin.users.col.actions'),
      width: '90px',
      align: 'right',
      cell: (i) =>
        i.status === 'pending' && (
          <button
            type="button"
            className={styles.revoke}
            aria-label={t('admin.invite.revokeOf', { email: i.email })}
            onClick={() => onRevoke(i)}
          >
            {t('admin.invite.revoke')}
          </button>
        ),
    },
  ]
}
