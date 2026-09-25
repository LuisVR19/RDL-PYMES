import { useQueryClient } from '@tanstack/react-query'
import { FlaskConical, X } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { setScenario, useScenario, type Scenario } from '@/shared/api/scenario'
import { expireSession } from '@/shared/auth/expiry'
import { ROLES, ROLE_LABEL, type Role } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './DevToolbar.module.css'

const SCENARIOS: { value: Scenario; label: string }[] = [
  { value: 'ok', label: 'Con datos' },
  { value: 'loading', label: 'Cargando' },
  { value: 'empty', label: 'Vacío' },
  { value: 'error', label: 'Error' },
  { value: 'partial', label: 'Datos parciales' },
]

/**
 * Barra de revisión de la etapa de diseño. NO es parte del producto: solo existe con la fuente de datos simulada
 * (`VITE_DATA_SOURCE=mock`) y permite ver cada pantalla por rol, organización y estado.
 */
export function DevToolbar() {
  const [open, setOpen] = useState(false)
  const scenario = useScenario()
  const queryClient = useQueryClient()
  const { activeOrg, organizations, role, switchOrganization, setRoleOverride } = useSession()

  const apply = (patch: Parameters<typeof setScenario>[0]) => {
    setScenario(patch)
    // Rehace las consultas de datos (no las de sesión) con el escenario nuevo.
    queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'session' })
  }

  if (!open) {
    return (
      <button
        type="button"
        className={styles.fab}
        onClick={() => setOpen(true)}
        aria-label="Abrir barra de revisión (solo diseño)"
      >
        <FlaskConical size={18} aria-hidden />
      </button>
    )
  }

  return (
    <aside className={styles.bar} aria-label="Barra de revisión (solo diseño)">
      <div className={styles.head}>
        <strong>Revisión de diseño</strong>
        <span className={styles.note}>No es parte del producto</span>
        <button
          type="button"
          className={styles.close}
          onClick={() => setOpen(false)}
          aria-label="Cerrar barra de revisión"
        >
          <X size={16} aria-hidden />
        </button>
      </div>
      <label className={styles.field}>
        <span>Organización</span>
        <select
          value={activeOrg?.id ?? ''}
          onChange={(e) => switchOrganization(e.target.value, { force: true })}
        >
          {organizations.map((o) => (
            <option key={o.id} value={o.id}>
              {o.legalName}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.field}>
        <span>Ver como rol</span>
        <select value={role ?? ''} onChange={(e) => setRoleOverride(e.target.value as Role)}>
          {ROLES.map((r) => (
            <option key={r} value={r}>
              {ROLE_LABEL[r]}
              {activeOrg?.role === r ? ' (real)' : ''}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.field}>
        <span>Estado de los datos</span>
        <select value={scenario.scenario} onChange={(e) => apply({ scenario: e.target.value as Scenario })}>
          {SCENARIOS.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.field}>
        <span>Latencia simulada</span>
        <select value={scenario.latencyMs} onChange={(e) => apply({ latencyMs: Number(e.target.value) })}>
          {[0, 350, 1200, 3000].map((ms) => (
            <option key={ms} value={ms}>
              {ms} ms
            </option>
          ))}
        </select>
      </label>
      <label className={styles.check}>
        <input
          type="checkbox"
          checked={scenario.realtime === 'offline'}
          onChange={(e) => setScenario({ realtime: e.target.checked ? 'offline' : 'ok' })}
        />
        <span>Sin conexión en tiempo real</span>
      </label>
      <div className={styles.field}>
        <span>Acceso</span>
        <button type="button" onClick={expireSession}>
          Vencer la sesión (pantalla 35)
        </button>
        <span>
          Invitación: <Link to="/invitacion/valida">válida</Link> ·{' '}
          <Link to="/invitacion/vencida">vencida</Link> ·{' '}
          <Link to="/invitacion/no-pendiente">no pendiente</Link> ·{' '}
          <Link to="/invitacion/ajena">otro correo</Link>
        </span>
      </div>
    </aside>
  )
}
