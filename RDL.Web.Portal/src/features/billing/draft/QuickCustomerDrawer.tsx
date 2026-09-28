import { useQueryClient } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { Drawer } from '@/design-system/components/Dialog/Dialog'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer, CustomerInput } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { IDENTIFICATION_TYPES } from '@/shared/identification'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './draft.module.css'

const EMAIL = /^\S+@\S+\.\S+$/
const digits = (s: string) => s.replace(/\D/g, '')

type Form = { typeCode: string; number: string; legalName: string; email: string }
type Errors = Partial<Record<keyof Form, string>>

/**
 * Alta rápida de cliente desde el borrador (prototipo «13 Alta rápida de cliente»): lo mínimo para facturar. Si la
 * identificación ya existe, ofrece usar ese cliente en vez de crear otro.
 */
export function QuickCustomerDrawer({
  open,
  initialText,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  /** Lo que el usuario escribió en el buscador: si son dígitos, es la identificación; si no, el nombre. */
  initialText: string
  onOpenChange: (open: boolean) => void
  onCreated: (c: Customer) => void
}) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange} title={t('quickClient.title')} width={440}>
      {/* `key` reinicia el formulario cada vez que se abre con otro texto. */}
      {open && (
        <QuickCustomerForm
          key={initialText}
          initialText={initialText}
          onCreated={onCreated}
          onCancel={() => onOpenChange(false)}
        />
      )}
    </Drawer>
  )
}

function QuickCustomerForm({
  initialText,
  onCreated,
  onCancel,
}: {
  initialText: string
  onCreated: (c: Customer) => void
  onCancel: () => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const keyFor = useIdempotencyKey()
  const looksLikeId = /^[\d\s-]+$/.test(initialText) && digits(initialText).length > 0
  const [form, setForm] = useState<Form>({
    typeCode: '02',
    number: looksLikeId ? initialText : '',
    legalName: looksLikeId ? '' : initialText,
    email: '',
  })
  const [errors, setErrors] = useState<Errors>({})
  const [existing, setExisting] = useState<Customer | null>(null)
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [saving, setSaving] = useState(false)

  const set = (k: keyof Form) => (value: string) => {
    setForm((f) => ({ ...f, [k]: value }))
    setErrors((e) => ({ ...e, [k]: undefined }))
    if (k === 'number') setExisting(null)
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (digits(form.number).length < 9) next.number = t('org.create.idNumberInvalid')
    if (!form.legalName.trim()) next.legalName = t('org.create.legalNameRequired')
    if (!EMAIL.test(form.email.trim())) next.email = t('client.emailInvalid')
    setErrors(next)
    setServerError(null)
    if (Object.keys(next).length > 0) return

    const input: CustomerInput = {
      identification: { typeCode: form.typeCode, number: form.number.trim() },
      legalName: form.legalName.trim(),
      email: form.email.trim(),
    }
    setSaving(true)
    try {
      const created = await ds.customers.create(input, keyFor(input))
      void queryClient.invalidateQueries({ queryKey: ['customers', activeOrg?.id] })
      onCreated(created)
    } catch (err) {
      const api = err instanceof ApiError ? err : null
      if (api?.is('customer-identification-taken')) {
        const found = await ds.customers
          .list({ q: form.number.trim(), limit: 5 })
          .then((p) => p.items.find((c) => digits(c.identification.number) === digits(form.number)))
          .catch(() => undefined)
        setExisting(found ?? null)
        setErrors({
          number: found ? t('client.duplicate', { name: found.legalName }) : t('client.duplicateUnknown'),
        })
      } else {
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className={styles.quick} onSubmit={submit} noValidate>
      <p className={styles.muted}>{t('quickClient.subtitle')}</p>
      {serverError && (
        <InlineAlert tone="danger" refCode={serverError.correlationId || undefined}>
          {t('client.serverError')}
        </InlineAlert>
      )}
      <Select
        label={t('org.create.idType')}
        required
        value={form.typeCode}
        options={IDENTIFICATION_TYPES.map((x) => ({ value: x.code, label: x.label }))}
        onChange={(e) => set('typeCode')(e.target.value)}
      />
      <TextField
        label={t('org.create.idNumber')}
        required
        inputMode="numeric"
        placeholder="3-101-000000"
        value={form.number}
        error={errors.number}
        onChange={(e) => set('number')(e.target.value)}
      />
      {existing && (
        <Button type="button" variant="secondary" size="sm" onClick={() => onCreated(existing)}>
          {t('quickClient.useExisting', { name: existing.legalName })}
        </Button>
      )}
      <TextField
        label={t('client.legalName')}
        required
        value={form.legalName}
        error={errors.legalName}
        onChange={(e) => set('legalName')(e.target.value)}
      />
      <TextField
        label={t('client.email')}
        required
        type="email"
        value={form.email}
        error={errors.email}
        onChange={(e) => set('email')(e.target.value)}
      />
      <div className={styles.quickActions}>
        <Button type="button" variant="secondary" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" variant="primary" loading={saving} loadingLabel={t('quickClient.submitting')}>
          {t('quickClient.submit')}
        </Button>
      </div>
    </form>
  )
}
