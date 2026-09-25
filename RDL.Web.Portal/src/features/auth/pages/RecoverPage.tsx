import { CircleCheck } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { TextField } from '@/design-system/components/Field/Field'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import styles from './LoginPage.module.css'

const EMAIL = /^\S+@\S+\.\S+$/

/**
 * Pantalla 1 · Recuperar contraseña (prototipo «01 Iniciar sesión», variantes recuperar y enviado). El mensaje final
 * es el mismo exista o no la cuenta: no revela quién está registrado.
 */
export function RecoverPage() {
  const { port } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string>()
  const [sending, setSending] = useState(false)
  const [sent, setSent] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (!EMAIL.test(email)) {
      setError(t('auth.login.emailInvalid'))
      return
    }
    setSending(true)
    // Cualquier respuesta (incluido un error) termina en el mismo mensaje genérico.
    await port.requestPasswordReset(email.trim()).catch(() => {})
    setSending(false)
    setSent(true)
  }

  if (sent) {
    return (
      <div className={styles.wrap}>
        <div className={styles.card}>
          <div className={styles.heading}>
            <CircleCheck size={36} className={styles.successIcon} aria-hidden />
            <h1 className={styles.titleSm}>{t('auth.recover.sentTitle')}</h1>
            <p className={styles.body}>{t('auth.recover.sent')}</p>
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
      </div>
    )
  }

  return (
    <div className={styles.wrap}>
      <form className={styles.card} onSubmit={submit} noValidate>
        <div className={styles.heading}>
          <h1 className={styles.title}>{t('auth.recover.title')}</h1>
          <span className={styles.subtitle}>{t('auth.recover.body')}</span>
        </div>
        <TextField
          label={t('auth.login.email')}
          type="email"
          autoComplete="username"
          value={email}
          error={error}
          onChange={(e) => {
            setEmail(e.target.value)
            setError(undefined)
          }}
        />
        <Button type="submit" variant="primary" size="lg" disabled={sending} className={styles.submit}>
          {t(sending ? 'auth.recover.submitting' : 'auth.recover.submit')}
        </Button>
        <Link to="/ingresar" className={styles.backLink}>
          {t('auth.recover.back')}
        </Link>
      </form>
    </div>
  )
}
