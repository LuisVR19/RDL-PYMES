import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Store } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import {
  EmptyState,
  ErrorState,
  InlineAlert,
  SkeletonRows,
} from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { BranchInput } from '@/shared/api/admin-types'
import type { Branch } from '@/shared/api/billing-types'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './BranchesPanel.module.css'

/**
 * El diseño pide 3 dígitos (como la sucursal del consecutivo de Hacienda). El contrato de Platform acepta más
 * (`^[A-Za-z0-9_-]{1,20}$`); el portal pide el subconjunto del diseño al crear, y muestra cualquier código que ya
 * exista.
 */
const CODE = /^\d{3}$/

/** Pantalla 29 · Sucursales (prototipo «29», pestaña de «28 Organización»). */
export function BranchesPanel() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<{ id: string; name: string } | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [rowError, setRowError] = useState<ApiError | null>(null)

  // Todas, activas e inactivas: aquí es donde se reactivan.
  const list = useQuery({
    queryKey: ['branches', activeOrg?.id, 'admin'],
    queryFn: () => ds.branches.list({ limit: 100 }),
    enabled: !!activeOrg,
  })
  const branches = list.data?.items ?? []
  const refresh = () => queryClient.invalidateQueries({ queryKey: ['branches', activeOrg?.id] })

  async function change(b: Branch, patch: { name?: string; isActive?: boolean }, done: string) {
    setBusy(b.id)
    setRowError(null)
    try {
      await ds.branches.update(b.id, patch)
      await refresh()
      setEditing(null)
      toast({ tone: 'success', title: done, body: `${b.code} · ${patch.name ?? b.name}` })
    } catch (err) {
      setRowError(err instanceof ApiError ? err : null)
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.head}>
        <span className={styles.rule}>{t('admin.branch.codeRule')}</span>
        {!adding && (
          <Button variant="primary" onClick={() => setAdding(true)}>
            {t('admin.branch.new')}
          </Button>
        )}
      </div>

      {adding && (
        <NewBranchForm
          existing={branches}
          onCancel={() => setAdding(false)}
          onCreated={async (b) => {
            setAdding(false)
            await refresh()
            toast({ tone: 'success', title: t('admin.branch.created'), body: `${b.code} · ${b.name}` })
          }}
        />
      )}

      {rowError && (
        <InlineAlert tone="danger" refCode={rowError.correlationId || undefined}>
          {t('admin.branch.updateError')}
        </InlineAlert>
      )}

      <div className={styles.list}>
        {list.isPending && <SkeletonRows rows={2} columns={3} />}
        {list.isError && <ErrorState onRetry={() => void list.refetch()} refCode={refOf(list.error)} />}
        {list.isSuccess && branches.length === 0 && (
          <EmptyState icon={Store} title={t('admin.branch.empty')} body={t('admin.branch.emptyBody')} />
        )}
        {list.isSuccess && branches.length > 0 && (
          <ul className={styles.rows} aria-label={t('admin.org.tab.branches')}>
            {branches.map((b) => {
              const isEditing = editing?.id === b.id
              return (
                <li key={b.id} className={b.isActive ? styles.row : styles.rowInactive}>
                  <span className={styles.code}>{b.code}</span>
                  <div className={styles.info}>
                    {isEditing ? (
                      <form
                        className={styles.editForm}
                        onSubmit={(e) => {
                          e.preventDefault()
                          const name = editing.name.trim()
                          if (name && name !== b.name) void change(b, { name }, t('admin.branch.updated'))
                          else setEditing(null)
                        }}
                      >
                        <input
                          className={styles.editInput}
                          aria-label={t('admin.branch.nameOf', { code: b.code })}
                          value={editing.name}
                          maxLength={150}
                          onChange={(e) => setEditing({ id: b.id, name: e.target.value })}
                        />
                        <Button type="submit" size="sm" variant="primary" loading={busy === b.id}>
                          {t('admin.branch.saveName')}
                        </Button>
                        <Button type="button" size="sm" variant="secondary" onClick={() => setEditing(null)}>
                          {t('common.cancel')}
                        </Button>
                      </form>
                    ) : (
                      <span className={styles.name}>{b.name}</span>
                    )}
                    <span className={styles.address}>{b.address || '—'}</span>
                  </div>
                  <StatusBadge domain="branch" status={b.isActive ? 'active' : 'inactive'} />
                  <div className={styles.actions}>
                    <button
                      type="button"
                      className={styles.link}
                      disabled={busy === b.id}
                      aria-label={t('admin.branch.editOf', { code: b.code })}
                      onClick={() => setEditing({ id: b.id, name: b.name })}
                    >
                      {t('admin.branch.edit')}
                    </button>
                    <button
                      type="button"
                      className={styles.link}
                      disabled={busy === b.id}
                      aria-label={t(b.isActive ? 'admin.branch.deactivateOf' : 'admin.branch.activateOf', {
                        code: b.code,
                      })}
                      onClick={() =>
                        void change(
                          b,
                          { isActive: !b.isActive },
                          t(b.isActive ? 'admin.branch.deactivated' : 'admin.branch.activated'),
                        )
                      }
                    >
                      {t(b.isActive ? 'admin.branch.deactivate' : 'admin.branch.activate')}
                    </button>
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    </div>
  )
}

type Form = { code: string; name: string; address: string }
type Errors = Partial<Record<keyof Form, string>>

function NewBranchForm({
  existing,
  onCancel,
  onCreated,
}: {
  existing: Branch[]
  onCancel: () => void
  onCreated: (b: Branch) => Promise<void>
}) {
  const ds = useDataSource()
  const keyFor = useIdempotencyKey()
  const [form, setForm] = useState<Form>({ code: '', name: '', address: '' })
  const [errors, setErrors] = useState<Errors>({})
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [saving, setSaving] = useState(false)

  const set = (k: keyof Form) => (value: string) => {
    setForm((f) => ({ ...f, [k]: value }))
    setErrors((e) => ({ ...e, [k]: undefined }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const code = form.code.trim()
    const next: Errors = {}
    if (!CODE.test(code)) next.code = t('admin.branch.codeInvalid')
    else if (existing.some((b) => b.code === code)) next.code = t('admin.branch.codeTaken', { code })
    if (!form.name.trim()) next.name = t('admin.branch.nameRequired')
    setErrors(next)
    setServerError(null)
    if (Object.keys(next).length > 0) return

    const input: BranchInput = {
      code,
      name: form.name.trim(),
      ...(form.address.trim() ? { address: form.address.trim() } : {}),
    }
    setSaving(true)
    try {
      await onCreated(await ds.branches.create(input, keyFor(input)))
    } catch (err) {
      const api = err instanceof ApiError ? err : null
      if (api?.status === 409) setErrors({ code: t('admin.branch.codeTaken', { code }) })
      else
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className={styles.newForm} onSubmit={submit} noValidate aria-label={t('admin.branch.new')}>
      <TextField
        label={t('admin.branch.code')}
        required
        inputMode="numeric"
        placeholder="003"
        className={styles.codeField}
        value={form.code}
        error={errors.code}
        onChange={(e) => set('code')(e.target.value)}
      />
      <TextField
        label={t('admin.branch.name')}
        required
        maxLength={150}
        value={form.name}
        error={errors.name}
        onChange={(e) => set('name')(e.target.value)}
      />
      <TextField
        className={styles.full}
        label={t('admin.branch.address')}
        maxLength={500}
        value={form.address}
        onChange={(e) => set('address')(e.target.value)}
      />
      {serverError && (
        <InlineAlert className={styles.full} tone="danger" refCode={serverError.correlationId || undefined}>
          {t('admin.branch.createError')}
        </InlineAlert>
      )}
      <div className={styles.newActions}>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={saving}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" variant="primary" loading={saving}>
          {t('admin.branch.create')}
        </Button>
      </div>
    </form>
  )
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
