import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextArea, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer, CustomerInput, CustomerPatch } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { IDENTIFICATION_TYPES } from '@/shared/identification'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './ClientFormPage.module.css'

const EMAIL = /^\S+@\S+\.\S+$/

type Form = {
  typeCode: string
  number: string
  legalName: string
  tradeName: string
  email: string
  phone: string
  address: string
}
type Errors = Partial<Record<keyof Form, string>>

const EMPTY: Form = {
  typeCode: '02',
  number: '',
  legalName: '',
  tradeName: '',
  email: '',
  phone: '',
  address: '',
}

const fromCustomer = (c: Customer): Form => ({
  typeCode: c.identification.typeCode,
  number: c.identification.number,
  legalName: c.legalName,
  tradeName: c.tradeName ?? '',
  email: c.email ?? '',
  phone: c.phone ?? '',
  address: c.address ?? '',
})

// errors[].field de Billing (campos JSON) → campo del formulario.
const SERVER_FIELD: Record<string, keyof Form> = {
  'identification.typeCode': 'typeCode',
  'identification.number': 'number',
  legalName: 'legalName',
  tradeName: 'tradeName',
  email: 'email',
  phone: 'phone',
  address: 'address',
}

/** Pantalla 8 · Nuevo cliente / Editar cliente (prototipo «08»). La ruta con `:id` edita. */
export function ClientFormPage() {
  const { id } = useParams()
  const editing = Boolean(id)
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const existing = useQuery({
    queryKey: ['customers', activeOrg?.id, 'detail', id],
    queryFn: () => ds.customers.get(id ?? ''),
    enabled: editing && !!activeOrg,
  })

  if (editing && existing.isPending) return <SkeletonRows rows={6} columns={2} />
  if (editing && existing.isError) {
    return <ErrorState onRetry={() => void existing.refetch()} refCode={refOf(existing.error)} />
  }
  // `key` reinicia el formulario si cambia el cliente.
  return <ClientForm key={id ?? 'nuevo'} customer={existing.data} />
}

function ClientForm({ customer }: { customer?: Customer }) {
  const ds = useDataSource()
  const { activeOrg, setDirty } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const keyFor = useIdempotencyKey()
  const initial = customer ? fromCustomer(customer) : EMPTY
  const [form, setForm] = useState<Form>(initial)
  const [errors, setErrors] = useState<Errors>({})
  const [duplicate, setDuplicate] = useState<{ id?: string; name?: string } | null>(null)
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [saving, setSaving] = useState(false)

  // El cambio de organización (pantalla 5) pregunta antes de perder lo escrito.
  const dirty = JSON.stringify(form) !== JSON.stringify(initial)
  useEffect(() => {
    setDirty(dirty)
    return () => setDirty(false)
  }, [dirty, setDirty])

  const set = (k: keyof Form) => (value: string) => {
    setForm((f) => ({ ...f, [k]: value }))
    setErrors((e) => ({ ...e, [k]: undefined }))
    if (k === 'number') setDuplicate(null)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (!form.legalName.trim()) next.legalName = t('org.create.legalNameRequired')
    if (!customer && form.number.replace(/\D/g, '').length < 9) next.number = t('org.create.idNumberInvalid')
    if (!EMAIL.test(form.email)) next.email = t('client.emailInvalid')
    setErrors(next)
    setServerError(null)
    setDuplicate(null)
    if (Object.keys(next).length > 0) return

    setSaving(true)
    try {
      const saved = customer
        ? await ds.customers.update(customer.id, patchOf(customer, form))
        : await create()
      setDirty(false)
      await queryClient.invalidateQueries({ queryKey: ['customers', activeOrg?.id] })
      toast({
        tone: 'success',
        title: t(customer ? 'client.updated' : 'client.created'),
        body: saved.legalName,
      })
      navigate(`/clientes/${saved.id}`, { replace: true })
    } catch (err) {
      await onError(err)
    } finally {
      setSaving(false)
    }
  }

  function create() {
    const input: CustomerInput = {
      identification: { typeCode: form.typeCode, number: form.number.trim() },
      legalName: form.legalName.trim(),
      ...optional('tradeName', form.tradeName),
      ...optional('email', form.email),
      ...optional('phone', form.phone),
      ...optional('address', form.address),
    }
    return ds.customers.create(input, keyFor(input))
  }

  async function onError(err: unknown) {
    const api = err instanceof ApiError ? err : null
    if (api?.is('customer-identification-taken')) {
      // Billing no dice cuál es: se busca por el número para ofrecer el enlace (diseño: «enlace al existente»).
      const found = await ds.customers
        .list({ q: form.number.trim(), limit: 5 })
        .then((p) =>
          p.items.find((c) => c.identification.number.replace(/\D/g, '') === form.number.replace(/\D/g, '')),
        )
        .catch(() => undefined)
      setDuplicate({ id: found?.id, name: found?.legalName })
      return
    }
    const fields: Errors = {}
    for (const fe of api?.errors ?? []) {
      const k = SERVER_FIELD[fe.field]
      if (k) fields[k] = fe.message
    }
    if (Object.keys(fields).length > 0) setErrors(fields)
    else setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
  }

  const back = customer ? `/clientes/${customer.id}` : '/clientes'

  return (
    <form className={styles.page} onSubmit={submit} noValidate>
      <PageHeader
        back={{ label: t('clients.title'), to: back }}
        title={t(customer ? 'client.edit.title' : 'client.new.title')}
      />
      {customer && <InlineAlert tone="info">{t('client.edit.notice')}</InlineAlert>}
      {serverError && (
        <InlineAlert tone="danger" refCode={serverError.correlationId || undefined}>
          {t('client.serverError')}
        </InlineAlert>
      )}

      <div className={styles.card}>
        <Select
          label={t('org.create.idType')}
          required
          disabled={Boolean(customer)}
          value={form.typeCode}
          options={IDENTIFICATION_TYPES.map((x) => ({ value: x.code, label: x.label }))}
          onChange={(e) => set('typeCode')(e.target.value)}
        />
        <div>
          <TextField
            label={t('org.create.idNumber')}
            required
            disabled={Boolean(customer)}
            inputMode="numeric"
            className={styles.tabular}
            value={form.number}
            error={errors.number}
            help={customer ? t('client.idLocked') : undefined}
            onChange={(e) => set('number')(e.target.value)}
          />
          {duplicate && (
            <p className={styles.duplicate} role="alert">
              {duplicate.id ? (
                <>
                  {t('client.duplicate')} <Link to={`/clientes/${duplicate.id}`}>{duplicate.name}</Link>.
                </>
              ) : (
                t('client.duplicateUnknown')
              )}
            </p>
          )}
        </div>
        <TextField
          className={styles.full}
          label={t('client.legalName')}
          required
          value={form.legalName}
          error={errors.legalName}
          onChange={(e) => set('legalName')(e.target.value)}
        />
        <TextField
          className={styles.full}
          label={t('client.tradeName')}
          value={form.tradeName}
          error={errors.tradeName}
          onChange={(e) => set('tradeName')(e.target.value)}
        />
        <TextField
          label={t('client.email')}
          required
          type="email"
          help={t('client.emailHelp')}
          value={form.email}
          error={errors.email}
          onChange={(e) => set('email')(e.target.value)}
        />
        <TextField
          label={t('client.phone')}
          type="tel"
          value={form.phone}
          error={errors.phone}
          onChange={(e) => set('phone')(e.target.value)}
        />
        <TextArea
          className={styles.full}
          label={t('client.address')}
          placeholder={t('client.addressPlaceholder')}
          value={form.address}
          error={errors.address}
          onChange={(e) => set('address')(e.target.value)}
        />
      </div>

      <div className={styles.actions}>
        <Button type="button" variant="secondary" onClick={() => navigate(back)}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" variant="primary" disabled={saving}>
          {t(saving ? 'client.saving' : 'client.save')}
        </Button>
      </div>
    </form>
  )
}

function optional<K extends string>(key: K, value: string): Partial<Record<K, string>> {
  const v = value.trim()
  return v ? ({ [key]: v } as Record<K, string>) : {}
}

/** Solo lo que cambió; un opcional vaciado viaja como "" (así lo borra Billing). */
function patchOf(before: Customer, form: Form): CustomerPatch {
  const patch: CustomerPatch = {}
  const was = fromCustomer(before)
  for (const k of ['legalName', 'tradeName', 'email', 'phone', 'address'] as const) {
    if (form[k].trim() !== was[k]) patch[k] = form[k].trim()
  }
  return patch
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
