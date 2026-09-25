import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ConfirmDialog } from '@/design-system/components/Dialog/Dialog'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { BalancePart, Invoice } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { useSession } from '@/shared/session/SessionProvider'

/**
 * Pantalla 17 · Anular factura (prototipo «17»): motivo obligatorio de 10 caracteres y la explicación del ajuste de
 * la cuenta por cobrar. El ajuste lo hace Receivables al recibir `InvoiceCancelled`; aquí solo se explica.
 * Reintentar tras un error reutiliza la `Idempotency-Key` (mismo motivo = mismo envío).
 */
export function VoidInvoiceDialog({
  invoice,
  receivable,
  open,
  onOpenChange,
}: {
  invoice: Invoice
  receivable?: BalancePart
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const number = invoice.number ?? ''
  const balance = receivable?.balance

  async function confirm(reason?: string) {
    if (!reason) return
    setSending(true)
    setError(null)
    try {
      await ds.invoices.cancel(invoice.id, reason, keyFor({ id: invoice.id, reason }))
      await queryClient.invalidateQueries({ queryKey: ['invoices', activeOrg?.id] })
      onOpenChange(false)
      toast({ tone: 'success', title: t('void.done', { number }), body: t('void.doneBody') })
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

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={(o) => {
        if (!o) setError(null)
        onOpenChange(o)
      }}
      title={t('void.title', { number })}
      warning={
        balance
          ? t('void.body', {
              balance: formatMoney(balance.balanceAmount, balance.currency),
              zero: formatMoney('0', balance.currency),
            })
          : t('void.bodyNoBalance')
      }
      reasonRequired
      reasonPlaceholder={t('void.placeholder')}
      tone="danger"
      confirmLabel={t('void.confirm')}
      confirmingLabel={t('void.confirming')}
      sending={sending}
      errorMessage={error ? t('void.error') : undefined}
      errorRef={error?.correlationId || undefined}
      onConfirm={confirm}
    />
  )
}
