import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import { RETURN_PARAM } from '@/shared/auth/auth'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import styles from './LoginPage.module.css'

// Mismo criterio que el prototipo: algo@algo.algo. La validación de verdad la hace Supabase.
const EMAIL = /^\S+@\S+\.\S+$/

/**
 * Pantalla 1 · Iniciar sesión (design/referencias, «01 Iniciar sesión»). Mensaje genérico ante credenciales
 * malas (P8b: no se distingue «no existe» de «contraseña incorrecta»). Al entrar vuelve a la ruta de `?volver=`.
 */
export function LoginPage() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const back = safeReturn(params.get(RETURN_PARAM))

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<{ email?: string; password?: string }>({})
  const [failure, setFailure] = useState<'invalid-credentials' | 'unavailable' | null>(null)
  const [sending, setSending] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    const next: typeof errors = {}
    if (!EMAIL.test(email)) next.email = t('auth.login.emailInvalid')
    if (!password) next.password = t('auth.login.passwordRequired')
    setErrors(next)
    setFailure(null)
    if (next.email || next.password) return

    setSending(true)
    const r = await signIn(email.trim(), password)
    setSending(false)
    if (r.ok) {
      navigate(back ?? '/', { replace: true })
      return
    }
    setFailure(r.reason)
    setPassword('')
  }

  return (
    <div className={styles.wrap}>
      <form className={styles.card} onSubmit={submit} noValidate>
        <div className={styles.heading}>
          <h1 className={styles.title}>{t('auth.login.title')}</h1>
          <span className={styles.subtitle}>{t('auth.login.subtitle')}</span>
        </div>

        {back && !failure && (
          <InlineAlert tone="info">
            {t('auth.login.expired.before')} <span className={styles.path}>{back}</span>.
          </InlineAlert>
        )}
        {failure && (
          <InlineAlert tone="danger">
            {t(failure === 'invalid-credentials' ? 'auth.login.error' : 'auth.login.unavailable')}
          </InlineAlert>
        )}

        <TextField
          label={t('auth.login.email')}
          type="email"
          autoComplete="username"
          placeholder={t('auth.login.emailPlaceholder')}
          value={email}
          error={errors.email}
          onChange={(e) => {
            setEmail(e.target.value)
            setErrors((x) => ({ ...x, email: undefined }))
          }}
        />
        <div className={styles.passwordField}>
          <TextField
            label={t('auth.login.password')}
            type="password"
            autoComplete="current-password"
            value={password}
            error={errors.password}
            onChange={(e) => {
              setPassword(e.target.value)
              setErrors((x) => ({ ...x, password: undefined }))
            }}
          />
          <Link to="/recuperar" className={styles.forgot}>
            {t('auth.login.forgot')}
          </Link>
        </div>

        <Button type="submit" variant="primary" size="lg" disabled={sending} className={styles.submit}>
          {t(sending ? 'auth.login.submitting' : 'auth.login.submit')}
        </Button>
      </form>
      <span className={styles.hint}>{t('auth.login.invitationHint')}</span>
    </div>
  )
}

/** Solo rutas internas: un `?volver=https://otro.sitio` no puede sacar al usuario del portal. */
function safeReturn(value: string | null): string | null {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/ingresar')) return null
  return value
}
