import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { Panel, PanelHeader } from '@/design-system/components/Surface/Surface'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { PaymentApplication } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, formatBusinessDate, formatInstantDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { paymentMethodLabel } from '@/shared/paymentMethods'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { ApplyPaymentDialog, ReverseApplicationDialog, VoidPaymentDialog } from '../dialogs'
import { appliedOf, unappliedOf } from '../model'
import { Figures } from '../parts'
import styles from '../receivables.module.css'

/**
 * Pantalla 27 · Detalle del pago (prototipo «27»). Monto, aplicado y sin aplicar; cada aplicación con su cuenta,
 * revertir con motivo y anular el pago (revierte todo). Lo pendiente se puede aplicar a otra cuenta del cliente.
 * Revertir y anular son de propietario y administrador (`receivables.void`), como en Receivables.
 */
export function PaymentDetailPage() {
  const { id = '' } = useParams()
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const [voidOpen, setVoidOpen] = useState(false)
  const [applyOpen, setApplyOpen] = useState(false)
  const [reversing, setReversing] = useState<PaymentApplication | null>(null)
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ

  const payment = useQuery({
    queryKey: ['receivables', org, 'payment', id],
    queryFn: () => ds.receivables.payment(id),
    enabled: !!org,
  })
  const customerId = payment.data?.customerId
  // Número de documento de cada aplicación: las cuentas del cliente en una sola llamada (sin una por fila).
  const accounts = useQuery({
    queryKey: ['receivables', org, 'byCustomer', customerId],
    queryFn: () => ds.receivables.byCustomer(customerId ?? '', { limit: 100 }),
    enabled: !!org && !!customerId,
  })
  const customer = useQuery({
    queryKey: ['customers', org, 'detail', customerId],
    queryFn: () => ds.customers.get(customerId ?? ''),
    enabled: !!org && !!customerId,
  })

  if (payment.isPending) return <SkeletonRows rows={6} columns={4} />
  if (payment.isError) {
    return (
      <ErrorState
        onRetry={() => void payment.refetch()}
        refCode={payment.error instanceof ApiError ? payment.error.correlationId || undefined : undefined}
      />
    )
  }
  const p = payment.data
  const docs = new Map((accounts.data?.items ?? []).map((r) => [r.id, r]))
  const pending = unappliedOf(p)
  const posted = p.status === 'posted'
  const canVoid = (role ? can(role, 'receivables.void') : false) && posted
  const canApply = (role ? can(role, 'receivables.edit') : false) && posted && pending.gt(0)
  const meta = [
    t('pay.detail.meta', {
      date: formatBusinessDate(p.receivedOn),
      method: paymentMethodLabel(p.paymentMethodCode),
    }),
    p.reference && t('pay.detail.ref', { reference: p.reference }),
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div className={styles.narrow}>
      <div className={styles.header}>
        <div className={styles.titles}>
          <Link to="/cobranza/pagos" className={styles.back}>
            ‹ {t('pay.back')}
          </Link>
          <div className={styles.titleRow}>
            <h1 className={styles.heading}>{t('pay.title', { date: formatBusinessDate(p.receivedOn) })}</h1>
            <StatusBadge domain="payment" status={p.status} />
          </div>
          {customer.data && (
            <Link to={`/clientes/${p.customerId}`} className={styles.customer}>
              {customer.data.legalName}
            </Link>
          )}
          <span className={styles.meta}>{meta}</span>
        </div>
        <div className={styles.actions}>
          {canApply && (
            <Button variant="secondary" onClick={() => setApplyOpen(true)}>
              {t('pay.apps.apply')}
            </Button>
          )}
          {canVoid && (
            <Button variant="secondary" className={styles.danger} onClick={() => setVoidOpen(true)}>
              {t('pay.detail.void')}
            </Button>
          )}
        </div>
      </div>

      <Figures
        items={[
          { label: t('pay.fig.amount'), value: formatMoney(p.amount, p.currency) },
          { label: t('pay.fig.applied'), value: formatMoney(appliedOf(p), p.currency) },
          {
            label: t('pay.fig.unapplied'),
            value: formatMoney(pending, p.currency),
            tone: pending.gt(0) ? 'warning' : undefined,
          },
        ]}
      />

      {!posted && (
        <InlineAlert tone="neutral">{t('pay.voidedMsg', { reason: p.voidReason ?? '' })}</InlineAlert>
      )}
      {posted && pending.gt(0) && (
        <InlineAlert tone="warning">
          {t('pay.unappliedMsg', { amount: formatMoney(pending, p.currency) })}
        </InlineAlert>
      )}

      <Panel padded={false} className={styles.listPanel}>
        <PanelHeader title={t('pay.apps.title')} />
        {p.applications.length === 0 ? (
          <p className={styles.empty}>{t('pay.apps.empty')}</p>
        ) : (
          <ul className={styles.list}>
            {p.applications.map((a) => {
              const doc = docs.get(a.receivableId)
              return (
                <li key={a.id} className={styles.row}>
                  <div className={styles.rowMain}>
                    <Link to={`/cobranza/cuentas/${a.receivableId}`} className={styles.link}>
                      {doc?.documentNumber ?? t('pay.apply.account')}
                    </Link>
                    <span className={styles.muted}>
                      {doc
                        ? t('pay.dueOn', { date: formatBusinessDate(doc.dueOn) })
                        : t('ar.apps.appliedOn', { date: formatInstantDate(a.appliedAt, tz) })}
                    </span>
                    {a.reversalReason && (
                      <span className={styles.muted}>
                        {t('ar.apps.reason', { reason: a.reversalReason })}
                      </span>
                    )}
                  </div>
                  <StatusBadge domain="application" status={a.reversedAt ? 'reversed' : 'applied'} />
                  <span className={styles.rowAmount}>{formatMoney(a.amount, p.currency)}</span>
                  {canVoid && !a.reversedAt && (
                    <Button variant="secondary" size="sm" onClick={() => setReversing(a)}>
                      {t('pay.apps.reverse')}
                    </Button>
                  )}
                </li>
              )
            })}
          </ul>
        )}
      </Panel>

      <ReverseApplicationDialog
        application={reversing}
        document={(reversing && docs.get(reversing.receivableId)?.documentNumber) ?? t('pay.apply.account')}
        currency={p.currency}
        onClose={() => setReversing(null)}
      />
      <VoidPaymentDialog payment={p} open={voidOpen} onOpenChange={setVoidOpen} />
      <ApplyPaymentDialog payment={p} open={applyOpen} onOpenChange={setApplyOpen} />
    </div>
  )
}
