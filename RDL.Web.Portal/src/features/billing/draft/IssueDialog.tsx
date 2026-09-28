import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { ConfirmDialog } from '@/design-system/components/Dialog/Dialog'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Invoice } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'

/**
 * Pantalla 14 · Emitir (prototipo «14 Emitir»): confirmación con resumen. Al emitir, el documento recibe número y el
 * portal va a su detalle, donde Hacienda aparece «En proceso» (Billing responde sin esperar a E-Invoice).
 *
 * Reintentar es seguro: la `Idempotency-Key` depende solo del borrador, así que un reintento tras un error de red
 * devuelve el mismo documento en vez de emitir dos.
 */
export function IssueDialog({
  open,
  onOpenChange,
  draft,
  summary,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** El borrador tal como lo guardó Billing (el que se emite). */
  draft: Invoice
  summary: { label: string; value: string; strong?: boolean }[]
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const isInvoice = draft.documentType === 'invoice'

  async function confirm() {
    setSending(true)
    setError(null)
    try {
      const issued = await ds.invoices.issue(draft.id, keyFor({ issue: draft.id }))
      await queryClient.invalidateQueries({ queryKey: ['invoices', activeOrg?.id] })
      onOpenChange(false)
      toast({
        tone: 'success',
        title: isInvoice
          ? t('invoice.issued', { n: issued.number ?? '' })
          : t('issue.noteDone', { n: issued.number ?? '' }),
        body: t('invoice.issuedSub'),
      })
      navigate(`/facturas/${issued.id}`, { replace: true })
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err
          : new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }),
      )
    } finally {
      setSending(false)
    }
  }

  const message = !error
    ? undefined
    : error.is('invoice-not-draft')
      ? t('invoice.alreadyIssued')
      : error.is('invoice-without-lines')
        ? t('draft.issue.noLines')
        : t(isInvoice ? 'invoice.emit.error' : 'note.emit.error')

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) setError(null)
        onOpenChange(o)
      }}
      title={
        draft.documentType === 'invoice'
          ? t('invoice.emit.title')
          : t(`note.emit.title.${draft.documentType}`)
      }
      summary={summary.map((s) => ({
        label: s.label,
        value: s.strong ? <strong>{s.value}</strong> : s.value,
      }))}
      warning={t(isInvoice ? 'invoice.emit.warning' : 'note.emit.warning')}
      confirmLabel={error ? t('issue.retry') : t(isInvoice ? 'issue.confirm' : 'issue.confirmNote')}
      confirmingLabel={t('issue.confirming')}
      sending={sending}
      errorMessage={message}
      errorRef={error?.correlationId || undefined}
      onConfirm={() => void confirm()}
    />
  )
}
