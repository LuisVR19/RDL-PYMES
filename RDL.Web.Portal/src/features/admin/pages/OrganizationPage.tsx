import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { TabPanel, Tabs } from '@/design-system/components/Tabs/Tabs'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { OrganizationDetail, OrganizationPatch } from '@/shared/api/admin-types'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { identificationLabel } from '@/shared/identification'
import { useSession } from '@/shared/session/SessionProvider'
import { TIMEZONES } from '@/shared/timezones'
import { BranchesPanel } from '../BranchesPanel'
import { ReadOnlyField } from '../ReadOnlyField'
import styles from './OrganizationPage.module.css'

const EMAIL = /^\S+@\S+\.\S+$/
const BRANCHES_PATH = '/admin/organizacion/sucursales'

/**
 * Pantallas 28 · Organización y 29 · Sucursales (prototipo «28», con «29» como segunda pestaña). Cada pestaña tiene
 * su ruta, así que un enlace a sucursales abre esa pestaña.
 */
export function OrganizationPage() {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const tab = pathname.startsWith(BRANCHES_PATH) ? 'branches' : 'data'

  return (
    <div className={styles.page}>
      <PageHeader section={t('nav.section.admin')} title={t('nav.organization')} />
      <Tabs
        label={t('nav.organization')}
        value={tab}
        onValueChange={(v) => navigate(v === 'branches' ? BRANCHES_PATH : '/admin/organizacion')}
        items={[
          { value: 'data', label: t('admin.org.tab.data') },
          { value: 'branches', label: t('admin.org.tab.branches') },
        ]}
      >
        <TabPanel value="data">
          <OrganizationData />
        </TabPanel>
        <TabPanel value="branches">
          <BranchesPanel />
        </TabPanel>
      </Tabs>
    </div>
  )
}

function OrganizationData() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const detail = useQuery({
    queryKey: ['organization', activeOrg?.id, 'detail'],
    queryFn: () => ds.organization.current(),
    enabled: !!activeOrg,
  })
  if (detail.isPending) return <SkeletonRows rows={4} columns={2} />
  if (detail.isError) {
    return <ErrorState onRetry={() => void detail.refetch()} refCode={refOf(detail.error)} />
  }
  // `key`: al guardar llega el detalle nuevo y el formulario parte de él.
  return <OrganizationForm key={detail.data.updatedAt} org={detail.data} />
}

type Form = { tradeName: string; email: string; phone: string; timezone: string }
type Errors = Partial<Record<keyof Form, string>>

const fromOrg = (o: OrganizationDetail): Form => ({
  tradeName: o.tradeName ?? '',
  email: o.email,
  phone: o.phone ?? '',
  timezone: o.timezone,
})

function OrganizationForm({ org }: { org: OrganizationDetail }) {
  const ds = useDataSource()
  const { setDirty } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const initial = fromOrg(org)
  const [form, setForm] = useState<Form>(initial)
  const [errors, setErrors] = useState<Errors>({})
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [saving, setSaving] = useState(false)

  const patch = patchOf(initial, form)
  const dirty = Object.keys(patch).length > 0
  useEffect(() => {
    setDirty(dirty)
    return () => setDirty(false)
  }, [dirty, setDirty])

  const set = (k: keyof Form) => (value: string) => {
    setForm((f) => ({ ...f, [k]: value }))
    setErrors((e) => ({ ...e, [k]: undefined }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setServerError(null)
    if (!EMAIL.test(form.email.trim())) {
      setErrors({ email: t('admin.org.emailInvalid') })
      return
    }
    setSaving(true)
    try {
      await ds.organization.update(patch)
      setDirty(false)
      // La zona horaria cambia cómo se ven todas las fechas: la sesión la vuelve a leer.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['organization', org.id] }),
        queryClient.invalidateQueries({ queryKey: ['session', 'organizations'] }),
      ])
      toast({ tone: 'success', title: t('admin.org.saved') })
    } catch (err) {
      const api = err instanceof ApiError ? err : null
      const fields: Errors = {}
      for (const fe of api?.errors ?? []) {
        if (fe.field in initial) fields[fe.field as keyof Form] = fe.message
      }
      if (Object.keys(fields).length > 0) setErrors(fields)
      else
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    } finally {
      setSaving(false)
    }
  }

  const timezones: { value: string; label: string }[] = TIMEZONES.map((z) => ({
    value: z.value,
    label: z.label,
  }))
  if (!timezones.some((z) => z.value === org.timezone))
    timezones.push({ value: org.timezone, label: org.timezone })

  return (
    <form className={styles.form} onSubmit={submit} noValidate>
      {serverError && (
        <InlineAlert tone="danger" refCode={serverError.correlationId || undefined}>
          {t('admin.org.serverError')}
        </InlineAlert>
      )}
      <div className={styles.card}>
        <ReadOnlyField
          className={styles.full}
          label={t('org.create.legalName')}
          value={org.legalName}
          help={t('admin.org.legalNameHelp')}
        />
        <TextField
          label={t('org.create.tradeName')}
          value={form.tradeName}
          error={errors.tradeName}
          maxLength={200}
          onChange={(e) => set('tradeName')(e.target.value)}
        />
        <ReadOnlyField
          label={t('admin.org.identification')}
          value={`${identificationLabel(org.identificationTypeCode) ?? org.identificationTypeCode} ${org.identificationNumber}`}
          help={t('admin.org.identificationHelp')}
          tabular
        />
        <TextField
          label={t('admin.org.email')}
          required
          type="email"
          value={form.email}
          error={errors.email}
          onChange={(e) => set('email')(e.target.value)}
        />
        <TextField
          label={t('admin.org.phone')}
          type="tel"
          value={form.phone}
          error={errors.phone}
          maxLength={30}
          onChange={(e) => set('phone')(e.target.value)}
        />
        <Select
          label={t('admin.org.timezone')}
          help={errors.timezone ? undefined : t('admin.org.timezoneHelp')}
          error={errors.timezone}
          value={form.timezone}
          options={timezones}
          onChange={(e) => set('timezone')(e.target.value)}
        />
        <ReadOnlyField
          label={t('admin.org.currency')}
          value={currencyLabel(org.defaultCurrencyCode)}
          help={t('admin.org.currencyHelp')}
        />
      </div>
      <div className={styles.actions}>
        <Button type="submit" variant="primary" loading={saving} disabled={!dirty}>
          {t('admin.org.save')}
        </Button>
      </div>
    </form>
  )
}

/** Solo lo que cambió; un opcional vaciado viaja como "" (así lo borra Platform). El correo no se puede vaciar. */
function patchOf(before: Form, form: Form): OrganizationPatch {
  const patch: OrganizationPatch = {}
  for (const k of ['tradeName', 'email', 'phone', 'timezone'] as const) {
    const v = form[k].trim()
    if (v !== before[k]) patch[k] = v
  }
  return patch
}

function currencyLabel(code: string): string {
  if (code === 'CRC') return t('admin.org.currencyCRC')
  if (code === 'USD') return t('admin.org.currencyUSD')
  return code
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
