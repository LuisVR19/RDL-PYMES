import { MailCheck } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import type { SignUpResult } from '@/shared/auth/auth'
import { useAuth } from '@/shared/auth/AuthProvider'
import { config } from '@/shared/config'
import { t } from '@/shared/i18n/t'
import styles from './LoginPage.module.css'

// Mismo criterio que la pantalla 1. La validación de verdad la hace Supabase.
const EMAIL = /^\S+@\S+\.\S+$/
const MIN_PASSWORD = 8

type Failure = Extract<SignUpResult, { ok: false }>['reason']
type Field = 'fullName' | 'email' | 'password' | 'confirm'

/**
 * Crear cuenta (`/registro`, destino de «Crear cuenta» de la landing). El diseño no tiene esta pantalla: sigue la
 * tarjeta de la pantalla 1. Crea el usuario en Supabase Auth con su nombre; Platform lo da de alta en su primera
 * llamada. Después:
 * - si Supabase pide confirmar el correo, se muestra «Revise su correo» (el enlace vuelve a la pantalla 1);
 * - si ya hay sesión, va directo a crear la organización (pantalla 3).
 * TODO(diseño): pantalla de registro en el prototipo y texto legal definitivo (hoy los términos son borradores).
 */
export function SignupPage() {
  const { signUp } = useAuth()
  const navigate = useNavigate()
  const [values, setValues] = useState<Record<Field, string>>({
    fullName: '',
    email: '',
    password: '',
    confirm: '',
  })
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({})
  const [failure, setFailure] = useState<Failure | null>(null)
  const [sending, setSending] = useState(false)
  const [sentTo, setSentTo] = useState<string | null>(null)

  const set = (field: Field, value: string) => {
    setValues((v) => ({ ...v, [field]: value }))
    setErrors((e) => ({ ...e, [field]: undefined }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    const fullName = values.fullName.trim().replace(/\s+/g, ' ')
    const email = values.email.trim()
    const next: typeof errors = {}
    if (fullName.length < 3) next.fullName = t('auth.signup.nameRequired')
    if (!EMAIL.test(email)) next.email = t('auth.login.emailInvalid')
    if (values.password.length < MIN_PASSWORD) next.password = t('auth.signup.passwordShort')
    else if (values.confirm !== values.password) next.confirm = t('auth.signup.confirmMismatch')
    setErrors(next)
    setFailure(null)
    if (Object.keys(next).length > 0) return

    setSending(true)
    const r = await signUp({ fullName, email, password: values.password })
    setSending(false)
    if (r.ok) {
      if (r.next === 'signedIn') navigate('/organizaciones/nueva', { replace: true })
      else setSentTo(email)
      return
    }
    if (r.reason === 'weak-password') setErrors({ password: t('auth.signup.passwordWeak') })
    else if (r.reason === 'invalid-email') setErrors({ email: t('auth.login.emailInvalid') })
    else setFailure(r.reason)
  }

  if (sentTo) {
    return (
      <div className={styles.wrap}>
        <div className={styles.card}>
          <div className={styles.heading}>
            <MailCheck size={36} className={styles.successIcon} aria-hidden />
            <h1 className={styles.titleSm}>{t('auth.recover.sentTitle')}</h1>
            <p className={styles.body}>{t('auth.signup.sent', { email: sentTo })}</p>
          </div>
          <Button
            variant="secondary"
            size="lg"
            className={styles.submit}
            onClick={() => navigate('/ingresar')}
          >
            {t('auth.recover.backButton')}
          </Button>
        </div>
        <span className={styles.hint}>{t('auth.signup.sentHint')}</span>
      </div>
    )
  }

  return (
    <div className={styles.wrap}>
      <form className={styles.card} onSubmit={submit} noValidate>
        <div className={styles.heading}>
          <h1 className={styles.title}>{t('auth.signup.title')}</h1>
          <span className={styles.subtitle}>{t('auth.signup.subtitle')}</span>
        </div>

        {failure && (
          <InlineAlert tone="danger">
            {t(`auth.signup.error.${failure}`)}
            {failure === 'exists' && (
              <>
                {' '}
                <Link to="/ingresar" className={styles.inlineLink}>
                  {t('auth.login.submit')}
                </Link>{' '}
                ·{' '}
                <Link to="/recuperar" className={styles.inlineLink}>
                  {t('auth.login.forgot')}
                </Link>
              </>
            )}
          </InlineAlert>
        )}

        <TextField
          label={t('auth.signup.fullName')}
          autoComplete="name"
          value={values.fullName}
          error={errors.fullName}
          maxLength={120}
          onChange={(e) => set('fullName', e.target.value)}
        />
        <TextField
          label={t('auth.login.email')}
          type="email"
          autoComplete="email"
          placeholder={t('auth.login.emailPlaceholder')}
          value={values.email}
          error={errors.email}
          onChange={(e) => set('email', e.target.value)}
        />
        <TextField
          label={t('auth.login.password')}
          type="password"
          autoComplete="new-password"
          value={values.password}
          help={t('auth.signup.passwordHelp', { n: MIN_PASSWORD })}
          error={errors.password}
          onChange={(e) => set('password', e.target.value)}
        />
        <TextField
          label={t('auth.signup.confirm')}
          type="password"
          autoComplete="new-password"
          value={values.confirm}
          error={errors.confirm}
          onChange={(e) => set('confirm', e.target.value)}
        />

        <p className={styles.legal}>
          {t('auth.signup.legal.before')}{' '}
          <a
            href={`${config.landingUrl}/terminos`}
            target="_blank"
            rel="noreferrer"
            className={styles.inlineLink}
          >
            {t('auth.signup.legal.terms')}
          </a>{' '}
          {t('auth.signup.legal.and')}{' '}
          <a
            href={`${config.landingUrl}/privacidad`}
            target="_blank"
            rel="noreferrer"
            className={styles.inlineLink}
          >
            {t('auth.signup.legal.privacy')}
          </a>
          .
        </p>

        <Button type="submit" variant="primary" size="lg" disabled={sending} className={styles.submit}>
          {t(sending ? 'auth.signup.submitting' : 'auth.signup.submit')}
        </Button>
      </form>
      <span className={styles.hint}>
        {t('auth.signup.haveAccount')}{' '}
        <Link to="/ingresar" className={styles.inlineLink}>
          {t('auth.login.submit')}
        </Link>
      </span>
    </div>
  )
}
