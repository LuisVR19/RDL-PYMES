import * as RDialog from '@radix-ui/react-dialog'
import { clsx } from 'clsx'
import { X } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { t } from '@/shared/i18n/t'
import { Button } from '../Button/Button'
import { InlineAlert } from '../Feedback/Feedback'
import { TextArea, TextField } from '../Field/Field'
import styles from './Dialog.module.css'

export interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
  width?: number
}

/** Diálogo modal (aria-modal, foco atrapado, Esc cierra). Prototipo: 440 px, radio 8, pie separado. */
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  width = 440,
}: DialogProps) {
  return (
    <RDialog.Root open={open} onOpenChange={onOpenChange}>
      <RDialog.Portal>
        <RDialog.Overlay className={styles.overlay} />
        <RDialog.Content className={styles.content} style={{ maxWidth: width }}>
          <div className={styles.head}>
            <RDialog.Title className={styles.title}>{title}</RDialog.Title>
            {description ? (
              <RDialog.Description className={styles.description}>{description}</RDialog.Description>
            ) : (
              <RDialog.Description className="sr-only">{title}</RDialog.Description>
            )}
            {children}
          </div>
          {footer && <div className={styles.footer}>{footer}</div>}
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  )
}

export interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  summary?: { label: string; value: ReactNode }[]
  warning?: ReactNode
  confirmLabel: string
  confirmingLabel?: string
  tone?: 'primary' | 'danger'
  /** Motivo obligatorio (anular, revertir, anular pago): mínimo 10 caracteres. */
  reasonRequired?: boolean
  /** Confirmación escrita (cambiar a producción: «PRODUCCIÓN»). */
  typeToConfirm?: string
  onConfirm: (reason?: string) => void
  sending?: boolean
  errorMessage?: string
  errorRef?: string
}

/**
 * Confirmación proporcional al riesgo (README del diseño): emitir muestra un resumen; anular exige motivo;
 * pasar a producción exige escribir la palabra. En error conserva lo escrito y permite reintentar sin duplicar.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  summary,
  warning,
  confirmLabel,
  confirmingLabel,
  tone = 'primary',
  reasonRequired,
  typeToConfirm,
  onConfirm,
  sending,
  errorMessage,
  errorRef,
}: ConfirmDialogProps) {
  const [reason, setReason] = useState('')
  const [typed, setTyped] = useState('')
  const reasonOk = !reasonRequired || reason.trim().length >= 10
  const typedOk = !typeToConfirm || typed === typeToConfirm
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => !sending && onOpenChange(o)}
      title={title}
      footer={
        <>
          <Button variant="secondary" onClick={() => onOpenChange(false)} disabled={sending}>
            {t('common.cancel')}
          </Button>
          <Button
            variant={tone}
            loading={sending}
            loadingLabel={confirmingLabel}
            disabled={!reasonOk || !typedOk}
            onClick={() => onConfirm(reasonRequired ? reason.trim() : undefined)}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      {summary && (
        <dl className={styles.summary}>
          {summary.map((s) => (
            <div key={s.label} className={styles.summaryRow}>
              <dt>{s.label}</dt>
              <dd>{s.value}</dd>
            </div>
          ))}
        </dl>
      )}
      {warning && <p className={styles.warning}>{warning}</p>}
      {reasonRequired && (
        <TextArea
          label="Motivo"
          required
          minLength={10}
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          rows={3}
        />
      )}
      {typeToConfirm && (
        <TextField
          label={t('hacienda.envProd.confirm')}
          value={typed}
          onChange={(e) => setTyped(e.target.value)}
          autoComplete="off"
        />
      )}
      {errorMessage && (
        <InlineAlert tone="danger" refCode={errorRef}>
          {errorMessage}
        </InlineAlert>
      )}
    </Dialog>
  )
}

export interface DrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  children: ReactNode
  footer?: ReactNode
  width?: number
  side?: 'right' | 'left'
}

/** Panel lateral (alta rápida de cliente, menú móvil). 440–480 px a la derecha. */
export function Drawer({
  open,
  onOpenChange,
  title,
  children,
  footer,
  width = 460,
  side = 'right',
}: DrawerProps) {
  return (
    <RDialog.Root open={open} onOpenChange={onOpenChange}>
      <RDialog.Portal>
        <RDialog.Overlay className={styles.overlay} />
        <RDialog.Content
          className={clsx(styles.drawer, side === 'left' && styles.drawerLeft)}
          style={{ width }}
        >
          <div className={styles.drawerHead}>
            <RDialog.Title className={styles.title}>{title}</RDialog.Title>
            <RDialog.Description className="sr-only">{title}</RDialog.Description>
            <RDialog.Close asChild>
              <Button variant="icon" aria-label={t('common.close')}>
                <X size={18} aria-hidden />
              </Button>
            </RDialog.Close>
          </div>
          <div className={styles.drawerBody}>{children}</div>
          {footer && <div className={styles.footer}>{footer}</div>}
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  )
}
