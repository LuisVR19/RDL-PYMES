import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError, type NewOrganization } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { IDENTIFICATION_TYPES } from '@/shared/identification'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './OrgCreatePage.module.css'

const EMAIL = /^\S+@\S+\.\S+$/
const TIMEZONE = 'America/Costa_Rica'

type Form = {
  legalName: string
  tradeName: string
  typeCode: string
  number: string
  email: string
  phone: string
}
type Errors = Partial<Record<keyof Form, string>>

const EMPTY: Form = { legalName: '', tradeName: '', typeCode: '02', number: '', email: '', phone: '' }

// Campos JSON de Platform (errors[].field de un 422) → campo del formulario.
const SERVER_FIELD: Record<string, keyof Form> = {
  legalName: 'legalName',
  tradeName: 'tradeName',
  identificationTypeCode: 'typeCode',
  identificationNumber: 'number',
  email: 'email',
  phone: 'phone',
}

/**
 * Pantalla 3 · Crear organización (prototipo «03»). Quien la crea queda como propietario. Un error del servidor
 * conserva lo escrito; reintentar el mismo envío reutiliza la `Idempotency-Key` (no crea dos organizaciones).
 */
export function OrgCreatePage() {
  const ds = useDataSource()
  const { switchOrganization, organizations } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [form, setForm] = useState<Form>(EMPTY)
  const [errors, setErrors] = useState<Errors>({})
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [sending, setSending] = useState(false)

  const set = (k: keyof Form) => (value: string) => {
    setForm((f) => ({ ...f, [k]: value }))
    setErrors((e) => ({ ...e, [k]: undefined }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const next: Errors = {}
    if (!form.legalName.trim()) next.legalName = t('org.create.legalNameRequired')
    if (form.number.replace(/\D/g, '').length < 9) next.number = t('org.create.idNumberInvalid')
    if (!EMAIL.test(form.email)) next.email = t('org.create.emailInvalid')
    setErrors(next)
    setServerError(null)
    if (Object.keys(next).length > 0) return

    const input: NewOrganization = {
      legalName: form.legalName.trim(),
      ...(form.tradeName.trim() ? { tradeName: form.tradeName.trim() } : {}),
      identificationTypeCode: form.typeCode,
      identificationNumber: form.number.trim(),
      email: form.email.trim(),
      ...(form.phone.trim() ? { phone: form.phone.trim() } : {}),
      timezone: TIMEZONE,
    }
    setSending(true)
    try {
      const { id } = await ds.access.createOrganization(input, keyFor(input))
      // Platform solo la deja activa si el usuario no tenía otra; se activa siempre, con el cambio seguro.
      switchOrganization(id, {
        force: true,
        onDone: (ok) => {
          setSending(false)
          navigate(ok ? '/' : '/organizaciones', { replace: true })
          toast(
            ok
              ? { tone: 'success', title: t('org.create.done'), body: t('org.create.doneBody') }
              : { tone: 'danger', title: t('org.switch.failed', { org: input.legalName }) },
          )
        },
      })
    } catch (err) {
      setSending(false)
      const api = err instanceof ApiError ? err : null
      const fields: Errors = {}
      for (const fe of api?.errors ?? []) {
        const k = SERVER_FIELD[fe.field]
        if (k) fields[k] = fe.message
      }
      if (api?.status === 409) fields.number = t('org.create.idTaken')
      if (Object.keys(fields).length > 0) setErrors(fields)
      else
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.heading}>
        {organizations.length > 0 && (
          <Link to="/organizaciones" className={styles.back}>
            {t('org.create.back')}
          </Link>
        )}
        <h1 className={styles.title}>{t('org.create.title')}</h1>
        <span className={styles.subtitle}>{t('org.create.subtitle')}</span>
      </div>

      <form className={styles.card} onSubmit={submit} noValidate>
        {serverError && (
          <InlineAlert
            tone="danger"
            className={styles.alert}
            refCode={serverError.correlationId || undefined}
          >
            {t('org.create.serverError')}
          </InlineAlert>
        )}
        <div className={styles.grid}>
          <TextField
            className={styles.full}
            label={t('org.create.legalName')}
            required
            value={form.legalName}
            error={errors.legalName}
            onChange={(e) => set('legalName')(e.target.value)}
          />
          <TextField
            className={styles.full}
            label={t('org.create.tradeName')}
            help={t('org.create.tradeNameHelp')}
            value={form.tradeName}
            error={errors.tradeName}
            onChange={(e) => set('tradeName')(e.target.value)}
          />
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
            placeholder={t('org.create.idPlaceholder')}
            className={styles.tabular}
            value={form.number}
            error={errors.number}
            onChange={(e) => set('number')(e.target.value)}
          />
          <TextField
            label={t('org.create.email')}
            required
            type="email"
            value={form.email}
            error={errors.email}
            onChange={(e) => set('email')(e.target.value)}
          />
          <TextField
            label={t('org.create.phone')}
            type="tel"
            value={form.phone}
            error={errors.phone}
            onChange={(e) => set('phone')(e.target.value)}
          />
          <Select
            className={styles.full}
            label={t('org.create.timezone')}
            help={t('org.create.timezoneHelp')}
            value={TIMEZONE}
            options={[{ value: TIMEZONE, label: t('org.create.timezoneCR') }]}
            onChange={() => {}}
          />
        </div>
        <div className={styles.footer}>
          <Button
            type="button"
            variant="secondary"
            onClick={() => navigate(organizations.length > 0 ? '/organizaciones' : '/ingresar')}
          >
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={sending} className={styles.submit}>
            {t(sending ? 'org.create.submitting' : 'org.create.submit')}
          </Button>
        </div>
      </form>
    </div>
  )
}
