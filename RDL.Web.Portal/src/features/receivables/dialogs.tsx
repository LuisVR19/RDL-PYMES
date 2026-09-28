import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Big } from 'big.js'
import { useState } from 'react'
import { Button } from '@/design-system/components/Button/Button'
import { ConfirmDialog, Dialog } from '@/design-system/components/Dialog/Dialog'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Payment, PaymentApplication } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { formatBusinessDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney, parseMoneyInput, type Currency } from '@/shared/money/money'
import { useSession } from '@/shared/session/SessionProvider'
import { collectable, unappliedOf } from './model'

const asApiError = (err: unknown) =>
  err instanceof ApiError
    ? err
    : new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' })

/** Después de un comando de cobranza, todo lo de Receivables de la organización se vuelve a leer. */
function useRefreshReceivables() {
  const queryClient = useQueryClient()
  const { activeOrg } = useSession()
  return () => queryClient.invalidateQueries({ queryKey: ['receivables', activeOrg?.id] })
}

/**
 * Revertir una aplicación (prototipo «27 Revertir aplicación»): motivo obligatorio; el saldo de la cuenta vuelve a
 * lo de antes y el monto queda sin aplicar en el pago. Reintentar reutiliza la `Idempotency-Key`.
 */
export function ReverseApplicationDialog({
  application,
  document,
  currency,
  onClose,
}: {
  application: PaymentApplication | null
  document: string
  currency: Currency
  onClose: () => void
}) {
  const ds = useDataSource()
  const toast = useToast()
  const refresh = useRefreshReceivables()
  const keyFor = useIdempotencyKey()
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  async function confirm(reason?: string) {
    if (!application || !reason) return
    setSending(true)
    setError(null)
    try {
      await ds.receivables.reverseApplication(application.id, reason, keyFor({ id: application.id, reason }))
      await refresh()
      onClose()
      toast({ tone: 'success', title: t('pay.reverse.done') })
    } catch (err) {
      setError(asApiError(err))
    } finally {
      setSending(false)
    }
  }

  return (
    <ConfirmDialog
      open={application !== null}
      onOpenChange={(o) => {
        if (!o) {
          setError(null)
          onClose()
        }
      }}
      title={t('ar.revert.title')}
      warning={
        application && t('pay.reverse.body', { amount: formatMoney(application.amount, currency), document })
      }
      reasonRequired
      reasonPlaceholder={t('pay.reverse.placeholder')}
      confirmLabel={t('pay.reverse.confirm')}
      confirmingLabel={t('pay.reverse.confirming')}
      sending={sending}
      errorMessage={error ? t('pay.reverse.error') : undefined}
      errorRef={error?.correlationId || undefined}
      onConfirm={confirm}
    />
  )
}

/** Anular un pago (prototipo «27 Anular pago»): motivo obligatorio; revierte todas sus aplicaciones vigentes. */
export function VoidPaymentDialog({
  payment,
  open,
  onOpenChange,
}: {
  payment: Payment
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const ds = useDataSource()
  const toast = useToast()
  const refresh = useRefreshReceivables()
  const keyFor = useIdempotencyKey()
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  async function confirm(reason?: string) {
    if (!reason) return
    setSending(true)
    setError(null)
    try {
      await ds.receivables.voidPayment(payment.id, reason, keyFor({ id: payment.id, reason }))
      await refresh()
      onOpenChange(false)
      toast({ tone: 'success', title: t('pay.void.done') })
    } catch (err) {
      setError(asApiError(err))
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
      title={t('pay.void.title', { date: formatBusinessDate(payment.receivedOn) })}
      warning={t('pay.void.body', { amount: formatMoney(payment.amount, payment.currency) })}
      reasonRequired
      reasonPlaceholder={t('pay.void.placeholder')}
      tone="danger"
      confirmLabel={t('pay.void.confirm')}
      confirmingLabel={t('pay.void.confirming')}
      sending={sending}
      errorMessage={error ? t('pay.void.error') : undefined}
      errorRef={error?.correlationId || undefined}
      onConfirm={confirm}
    />
  )
}

/**
 * Aplicar lo que quedó sin aplicar de un pago (pantalla 27). Solo ofrece cuentas del mismo cliente y moneda que se
 * puedan cobrar; el monto no supera ni lo pendiente del pago ni el saldo de la cuenta. Receivables lo vuelve a
 * validar (422 con su problem type).
 */
export function ApplyPaymentDialog({
  payment,
  open,
  onOpenChange,
}: {
  payment: Payment
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const toast = useToast()
  const refresh = useRefreshReceivables()
  const keyFor = useIdempotencyKey()
  const [receivableId, setReceivableId] = useState('')
  const [amount, setAmount] = useState('')
  const [touched, setTouched] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const pending = unappliedOf(payment)

  const accounts = useQuery({
    queryKey: ['receivables', activeOrg?.id, 'byCustomer', payment.customerId, 'open'],
    queryFn: () => ds.receivables.byCustomer(payment.customerId, { limit: 100 }),
    enabled: open && !!activeOrg?.id,
  })
  const options = (accounts.data?.items ?? [])
    .filter((r) => collectable(r) && r.currency === payment.currency)
    .toSorted((a, b) => a.dueOn.localeCompare(b.dueOn))
  const target = options.find((r) => r.id === receivableId)
  const value = parseMoneyInput(amount)
  const amountError = !touched
    ? undefined
    : value === null || new Big(value).lte(0)
      ? t('pay.amountError')
      : new Big(value).gt(pending)
        ? t('ar.payment.over', {
            applied: formatMoney(value, payment.currency),
            diff: formatMoney(new Big(value).minus(pending), payment.currency),
          })
        : target && new Big(value).gt(target.balanceAmount)
          ? t('ar.payment.rowOver', { saldo: formatMoney(target.balanceAmount, payment.currency) })
          : undefined

  async function submit() {
    setTouched(true)
    if (!target || value === null || amountError || new Big(value).lte(0)) return
    const input = { paymentId: payment.id, receivableId: target.id, amount: value }
    setSending(true)
    setError(null)
    try {
      await ds.receivables.applyPayment(input, keyFor(input))
      await refresh()
      close()
      toast({ tone: 'success', title: t('pay.apply.done') })
    } catch (err) {
      setError(asApiError(err))
    } finally {
      setSending(false)
    }
  }

  function close() {
    setReceivableId('')
    setAmount('')
    setTouched(false)
    setError(null)
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => (o ? onOpenChange(true) : !sending && close())}
      title={t('pay.apply.title')}
      width={480}
      footer={
        <>
          <Button variant="secondary" onClick={close} disabled={sending}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="primary"
            onClick={() => void submit()}
            loading={sending}
            loadingLabel={t('pay.apply.confirming')}
            disabled={options.length === 0}
          >
            {t('pay.apply.confirm')}
          </Button>
        </>
      }
    >
      {accounts.isSuccess && options.length === 0 ? (
        <InlineAlert tone="info">{t('pay.apply.none')}</InlineAlert>
      ) : (
        <>
          <Select
            label={t('pay.apply.account')}
            required
            value={receivableId}
            error={touched && !target ? t('pay.apply.pick') : undefined}
            onChange={(e) => {
              setReceivableId(e.target.value)
              const r = options.find((x) => x.id === e.target.value)
              if (r && amount === '') {
                const suggested = pending.gt(r.balanceAmount) ? new Big(r.balanceAmount) : pending
                setAmount(suggested.toFixed())
              }
            }}
            options={[
              { value: '', label: t('pay.apply.pick') },
              ...options.map((r) => ({
                value: r.id,
                label: `${r.documentNumber} · ${formatMoney(r.balanceAmount, r.currency)}`,
              })),
            ]}
          />
          <TextField
            label={t('pay.apply.amount')}
            required
            inputMode="decimal"
            value={amount}
            error={amountError}
            help={t('pay.unappliedMsg', { amount: formatMoney(pending, payment.currency) })}
            onChange={(e) => setAmount(e.target.value)}
            onBlur={() => setTouched(true)}
          />
        </>
      )}
      {error && (
        <InlineAlert tone="danger" refCode={error.correlationId || undefined}>
          {t('pay.apply.error')}
        </InlineAlert>
      )}
    </Dialog>
  )
}
