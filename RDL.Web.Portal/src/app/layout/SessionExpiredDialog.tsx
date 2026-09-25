import { useState, type FormEvent } from 'react'
import { useLocation, useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { Dialog } from '@/design-system/components/Dialog/Dialog'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { TextField } from '@/design-system/components/Field/Field'
import { RETURN_PARAM } from '@/shared/auth/auth'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import styles from './SessionExpiredDialog.module.css'

/**
 * Pantalla 35 · Sesión vencida (prototipo «35 Sesión vencida»). Aparece encima de la página, sin sacar al usuario
 * de ella: pide la contraseña del MISMO correo y, al continuar, reintenta lo que había fallado. No se cierra con Esc
 * ni tocando afuera: la única salida sin contraseña es «Entrar con otra cuenta», que sí cierra la sesión.
 */
export function SessionExpiredDialog() {
  const { status, session, reauthenticate, signOut } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string>()
  const [failure, setFailure] = useState<'invalid-credentials' | 'unavailable' | null>(null)
  const [sending, setSending] = useState(false)

  if (status !== 'expired') return null

  async function submit(e: FormEvent) {
    e.preventDefault()
    setFailure(null)
    if (!password) return setError(t('auth.login.passwordRequired'))
    setSending(true)
    const r = await reauthenticate(password)
    setSending(false)
    setPassword('')
    if (!r.ok) setFailure(r.reason)
  }

  const otherAccount = () => {
    const back = location.pathname + location.search
    void signOut().then(() =>
      navigate(`/ingresar?${RETURN_PARAM}=${encodeURIComponent(back)}`, { replace: true }),
    )
  }

  return (
    <Dialog
      open
      onOpenChange={() => {}}
      title={t('auth.session.title')}
      description={t('auth.session.expired')}
      width={400}
    >
      <form className={styles.form} onSubmit={submit} noValidate>
        {failure && (
          <InlineAlert tone="danger">
            {t(failure === 'invalid-credentials' ? 'auth.login.error' : 'auth.login.unavailable')}
          </InlineAlert>
        )}
        <div className={styles.field}>
          <span className={styles.label}>{t('auth.login.email')}</span>
          <div className={styles.readOnly}>{session?.email}</div>
        </div>
        <TextField
          label={t('auth.login.password')}
          type="password"
          autoComplete="current-password"
          value={password}
          error={error}
          onChange={(e) => {
            setPassword(e.target.value)
            setError(undefined)
          }}
        />
        <Button type="submit" variant="primary" size="lg" disabled={sending} className={styles.submit}>
          {t(sending ? 'auth.session.continuing' : 'auth.session.continue')}
        </Button>
        <button type="button" className={styles.other} onClick={otherAccount}>
          {t('auth.session.other')}
        </button>
      </form>
    </Dialog>
  )
}
