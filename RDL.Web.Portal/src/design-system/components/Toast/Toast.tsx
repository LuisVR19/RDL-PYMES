import { Check, CircleAlert, Info, TriangleAlert, X } from 'lucide-react'
import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react'
import { t } from '@/shared/i18n/t'
import styles from './Toast.module.css'

export type ToastTone = 'success' | 'danger' | 'info' | 'warning'

export interface ToastInput {
  tone: ToastTone
  title: string
  body?: string
  action?: { label: string; onClick: () => void }
}

interface ToastItem extends ToastInput {
  id: number
}

const ICON = { success: Check, danger: CircleAlert, info: Info, warning: TriangleAlert }

const ToastContext = createContext<((toast: ToastInput) => void) | null>(null)

/**
 * Toasts abajo a la derecha (`aria-live=polite`). Se cierran solos a los 5 s, salvo los de error, que esperan a
 * que el usuario los cierre (componentes.md).
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([])
  const next = useRef(1)

  const dismiss = useCallback((id: number) => setItems((all) => all.filter((x) => x.id !== id)), [])

  const push = useCallback(
    (toast: ToastInput) => {
      const id = next.current++
      setItems((all) => [...all.slice(-3), { ...toast, id }])
      if (toast.tone !== 'danger') window.setTimeout(() => dismiss(id), 5000)
    },
    [dismiss],
  )

  const value = useMemo(() => push, [push])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className={styles.region} aria-live="polite" aria-relevant="additions">
        {items.map((it) => {
          const Icon = ICON[it.tone]
          return (
            <div key={it.id} className={styles.toast} role={it.tone === 'danger' ? 'alert' : 'status'}>
              <Icon size={16} className={styles[it.tone]} aria-hidden />
              <div className={styles.text}>
                <span className={styles.title}>{it.title}</span>
                {it.body && <span className={styles.body}>{it.body}</span>}
                {it.action && (
                  <button type="button" className={styles.action} onClick={it.action.onClick}>
                    {it.action.label}
                  </button>
                )}
              </div>
              <button
                type="button"
                className={styles.close}
                onClick={() => dismiss(it.id)}
                aria-label={t('common.close')}
              >
                <X size={14} aria-hidden />
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast fuera de ToastProvider')
  return ctx
}
