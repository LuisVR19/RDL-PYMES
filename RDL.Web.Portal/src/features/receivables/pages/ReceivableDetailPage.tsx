import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Big } from 'big.js'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextArea, TextField } from '@/design-system/components/Field/Field'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { Panel, PanelHeader } from '@/design-system/components/Surface/Surface'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type {
  FollowUpType,
  PaymentApplication,
  PaymentPromise,
  ReceivableDetail,
} from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import {
  DEFAULT_TZ,
  formatBusinessDate,
  formatInstant,
  formatInstantDate,
  todayIn,
} from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney, parseMoneyInput } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { ReverseApplicationDialog } from '../dialogs'
import { collectable, displayStatus, lateText, overdueDays } from '../model'
import { Figures } from '../parts'
import styles from '../receivables.module.css'

const FOLLOW_UP_TYPES: FollowUpType[] = ['call', 'email', 'visit', 'message', 'note']

/**
 * Pantalla 24 · Cuenta por cobrar (prototipo «24»). Las tres cifras, los pagos aplicados (revertir con motivo),
 * los ajustes que hizo Receivables por notas o anulación, y la gestión de cobro con alta rápida: seguimientos y
 * promesas de pago. Seguimientos y promesas son secundarios: si fallan, el resto de la cuenta se ve igual.
 */
export function ReceivableDetailPage() {
  const { id = '' } = useParams()
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [reversing, setReversing] = useState<PaymentApplication | null>(null)
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ
  const today = todayIn(tz)

  const account = useQuery({
    queryKey: ['receivables', org, 'detail', id],
    queryFn: () => ds.receivables.get(id),
    enabled: !!org,
  })

  if (account.isPending) return <SkeletonRows rows={8} columns={4} />
  if (account.isError) {
    return (
      <ErrorState
        onRetry={() => void account.refetch()}
        refCode={account.error instanceof ApiError ? account.error.correlationId || undefined : undefined}
      />
    )
  }
  const r = account.data
  const late = overdueDays(r, today)
  const canEdit = role ? can(role, 'receivables.edit') : false
  const canReverse = role ? can(role, 'receivables.void') : false
  const open = collectable(r) && new Big(r.balanceAmount).gt(0)

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titles}>
          <Link to="/cobranza/cuentas" className={styles.back}>
            ‹ {t('ar.back')}
          </Link>
          <div className={styles.titleRow}>
            <h1 className={styles.number}>{r.documentNumber}</h1>
            <StatusBadge domain="receivable" status={displayStatus(r, today)} />
          </div>
          {r.customerLegalName && (
            <Link to={`/clientes/${r.customerId}`} className={styles.customer}>
              {r.customerLegalName}
            </Link>
          )}
        </div>
        <div className={styles.actions}>
          <Button variant="secondary" onClick={() => navigate(`/facturas/${r.sourceInvoiceId}`)}>
            {t('ar.viewInvoice')}
          </Button>
          {canEdit && open && (
            <Button
              variant="primary"
              onClick={() => navigate(`/cobranza/pagos/nuevo?cliente=${encodeURIComponent(r.customerId)}`)}
            >
              {t('ar.payment.register')}
            </Button>
          )}
        </div>
      </div>

      <Figures
        items={[
          {
            label: t('ar.fig.original'),
            value: formatMoney(r.originalAmount, r.currency),
            sub: t('ar.fig.issued', { date: formatBusinessDate(r.issuedOn) }),
          },
          {
            label: t('ar.fig.balance'),
            value: formatMoney(r.balanceAmount, r.currency),
            sub: new Big(r.balanceAmount).gt(0) ? t('ar.fig.pending') : t('ar.fig.noBalance'),
            tone: late > 0 ? 'danger' : undefined,
          },
          {
            label: t('ar.fig.due'),
            value: formatBusinessDate(r.dueOn),
            sub: late > 0 ? t('ar.fig.lateDays', { days: lateText(late) }) : t('ar.fig.onTime'),
          },
        ]}
      />

      <div className={styles.columns}>
        <div className={styles.main}>
          <Panel padded={false} className={styles.listPanel}>
            <PanelHeader title={t('ar.apps.title')} />
            {r.applications.length === 0 ? (
              <p className={styles.empty}>{t('ar.apps.empty')}</p>
            ) : (
              <ul className={styles.list}>
                {r.applications.map((a) => (
                  <li key={a.id} className={styles.row}>
                    <div className={styles.rowMain}>
                      <Link to={`/cobranza/pagos/${a.paymentId}`} className={styles.link}>
                        {t('ar.apps.openPayment')}
                      </Link>
                      <span className={styles.muted}>
                        {t('ar.apps.appliedOn', { date: formatInstantDate(a.appliedAt, tz) })}
                      </span>
                      {a.reversalReason && (
                        <span className={styles.muted}>
                          {t('ar.apps.reason', { reason: a.reversalReason })}
                        </span>
                      )}
                    </div>
                    <StatusBadge domain="application" status={a.reversedAt ? 'reversed' : 'applied'} />
                    <span className={styles.rowAmount}>{formatMoney(a.amount, r.currency)}</span>
                    {canReverse && !a.reversedAt && (
                      <Button variant="secondary" size="sm" onClick={() => setReversing(a)}>
                        {t('pay.apps.reverse')}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
          </Panel>
          <FollowUps receivable={r} canEdit={canEdit} tz={tz} today={today} />
        </div>
        <div className={styles.side}>
          <Promises receivable={r} canEdit={canEdit && open} today={today} />
          <Panel padded={false} className={styles.listPanel}>
            <PanelHeader title={t('ar.adj.title')} />
            {r.adjustments.length === 0 ? (
              <p className={styles.empty}>{t('ar.adj.empty')}</p>
            ) : (
              <ul className={styles.list}>
                {r.adjustments.map((j) => (
                  <li key={j.id} className={styles.row}>
                    <div className={styles.rowMain}>
                      <span className={styles.amount}>
                        {t(`ar.adj.${j.adjustmentType}`, { amount: formatMoney(j.amount, r.currency) })}
                      </span>
                      <span className={styles.muted}>
                        {j.reason} · {formatInstantDate(j.createdAt, tz)}
                      </span>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>
      </div>

      <ReverseApplicationDialog
        application={reversing}
        document={r.documentNumber}
        currency={r.currency}
        onClose={() => setReversing(null)}
      />
    </div>
  )
}

function FollowUps({
  receivable,
  canEdit,
  tz,
  today,
}: {
  receivable: ReceivableDetail
  canEdit: boolean
  tz: string
  today: string
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [form, setForm] = useState<{ type: FollowUpType; next: string; notes: string } | null>(null)
  const [touched, setTouched] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)
  const queryKey = ['receivables', activeOrg?.id, 'followUps', receivable.id]
  const list = useQuery({
    queryKey,
    queryFn: () => ds.receivables.followUps(receivable.id),
    enabled: !!activeOrg?.id,
  })
  const notesError = touched && form && form.notes.trim().length < 3 ? t('ar.fu.notesError') : undefined

  async function save() {
    setTouched(true)
    if (!form || form.notes.trim().length < 3) return
    const input = {
      followupType: form.type,
      notes: form.notes.trim(),
      ...(form.next ? { nextActionOn: form.next } : {}),
    }
    setSending(true)
    setError(null)
    try {
      await ds.receivables.createFollowUp(receivable.id, input, keyFor({ id: receivable.id, ...input }))
      await queryClient.invalidateQueries({ queryKey })
      setForm(null)
      setTouched(false)
      toast({ tone: 'success', title: t('ar.fu.saved') })
    } catch (err) {
      setError(err instanceof ApiError ? err : null)
    } finally {
      setSending(false)
    }
  }

  return (
    <Panel padded={false} className={styles.listPanel}>
      <PanelHeader
        title={t('ar.fu.title')}
        actions={
          canEdit &&
          !form && (
            <button
              type="button"
              className={styles.textLink}
              onClick={() => setForm({ type: 'call', next: '', notes: '' })}
            >
              {t('ar.fu.new')}
            </button>
          )
        }
      />
      {form && (
        <div className={styles.quick}>
          <Select
            label={t('ar.fu.type')}
            value={form.type}
            onChange={(e) => setForm({ ...form, type: e.target.value as FollowUpType })}
            options={FOLLOW_UP_TYPES.map((x) => ({ value: x, label: t(`ar.fu.type.${x}`) }))}
          />
          <TextField
            label={t('ar.fu.next')}
            type="date"
            min={today}
            value={form.next}
            onChange={(e) => setForm({ ...form, next: e.target.value })}
          />
          <TextArea
            label={t('ar.fu.notes')}
            required
            className={styles.full}
            placeholder={t('ar.fu.notesPlaceholder')}
            value={form.notes}
            error={notesError}
            maxLength={2000}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
          />
          {error && (
            <InlineAlert tone="danger" className={styles.full} refCode={error.correlationId || undefined}>
              {t('ar.fu.error')}
            </InlineAlert>
          )}
          <div className={styles.quickActions}>
            <Button variant="secondary" size="sm" onClick={() => setForm(null)} disabled={sending}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => void save()}
              loading={sending}
              loadingLabel={t('ar.saving')}
            >
              {t('ar.save')}
            </Button>
          </div>
        </div>
      )}
      {list.isPending ? (
        <SkeletonRows rows={2} columns={2} />
      ) : list.isError ? (
        <p className={styles.empty}>{t('ar.secondaryError')}</p>
      ) : list.data.length === 0 ? (
        <p className={styles.empty}>{t('ar.fu.empty')}</p>
      ) : (
        <ul className={styles.list}>
          {list.data.map((f) => (
            <li key={f.id} className={styles.row}>
              <div className={styles.rowMain}>
                <div className={styles.titleRow}>
                  <span className={styles.chip}>{t(`ar.fu.type.${f.followupType}`)}</span>
                  <span className={styles.muted}>{formatInstant(f.performedAt, tz)}</span>
                </div>
                <span className={styles.noteText}>{f.notes}</span>
                {f.nextActionOn && (
                  <span className={styles.muted}>
                    {t('ar.fu.nextOn', { date: formatBusinessDate(f.nextActionOn) })}
                  </span>
                )}
              </div>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}

function Promises({
  receivable,
  canEdit,
  today,
}: {
  receivable: ReceivableDetail
  canEdit: boolean
  today: string
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [form, setForm] = useState<{ amount: string; date: string } | null>(null)
  const [touched, setTouched] = useState(false)
  const [sending, setSending] = useState<string | null>(null)
  const [error, setError] = useState<ApiError | null>(null)
  const queryKey = ['receivables', activeOrg?.id, 'promises', receivable.id]
  const list = useQuery({
    queryKey,
    queryFn: () => ds.receivables.promises(receivable.id),
    enabled: !!activeOrg?.id,
  })

  const amount = form ? parseMoneyInput(form.amount) : null
  const amountError = !touched
    ? undefined
    : amount === null || new Big(amount).lte(0)
      ? t('ar.pr.amountError')
      : new Big(amount).gt(receivable.balanceAmount)
        ? t('ar.pr.amountOver', { balance: formatMoney(receivable.balanceAmount, receivable.currency) })
        : undefined
  const dateError = touched && form && (!form.date || form.date < today) ? t('ar.pr.dateError') : undefined

  async function save() {
    setTouched(true)
    if (!form || amount === null || amountError || !form.date || form.date < today) return
    const input = { promisedAmount: amount, promisedOn: form.date }
    setSending('new')
    setError(null)
    try {
      await ds.receivables.createPromise(receivable.id, input, keyFor({ id: receivable.id, ...input }))
      await queryClient.invalidateQueries({ queryKey })
      setForm(null)
      setTouched(false)
      toast({ tone: 'success', title: t('ar.pr.saved') })
    } catch (err) {
      setError(err instanceof ApiError ? err : null)
    } finally {
      setSending(null)
    }
  }

  async function close(p: PaymentPromise, status: 'kept' | 'broken' | 'cancelled') {
    setSending(p.id)
    setError(null)
    try {
      await ds.receivables.closePromise(p.id, status, keyFor({ id: p.id, status }))
      await queryClient.invalidateQueries({ queryKey })
      toast({ tone: 'success', title: t('ar.pr.closed') })
    } catch (err) {
      setError(err instanceof ApiError ? err : null)
    } finally {
      setSending(null)
    }
  }

  return (
    <Panel padded={false} className={styles.listPanel}>
      <PanelHeader
        title={t('ar.pr.title')}
        actions={
          canEdit &&
          !form && (
            <button
              type="button"
              className={styles.textLink}
              onClick={() => setForm({ amount: '', date: '' })}
            >
              {t('ar.pr.new')}
            </button>
          )
        }
      />
      {form && (
        <div className={styles.quickStack}>
          <TextField
            label={t('ar.pr.amount')}
            required
            inputMode="decimal"
            placeholder="0,00"
            value={form.amount}
            error={amountError}
            onChange={(e) => setForm({ ...form, amount: e.target.value })}
          />
          <TextField
            label={t('ar.pr.date')}
            required
            type="date"
            min={today}
            value={form.date}
            error={dateError}
            onChange={(e) => setForm({ ...form, date: e.target.value })}
          />
          <div className={styles.quickActions}>
            <Button variant="secondary" size="sm" onClick={() => setForm(null)} disabled={sending !== null}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => void save()}
              loading={sending === 'new'}
              loadingLabel={t('ar.saving')}
            >
              {t('ar.save')}
            </Button>
          </div>
        </div>
      )}
      {error && (
        <InlineAlert tone="danger" refCode={error.correlationId || undefined}>
          {t('ar.pr.error')}
        </InlineAlert>
      )}
      {list.isPending ? (
        <SkeletonRows rows={2} columns={2} />
      ) : list.isError ? (
        <p className={styles.empty}>{t('ar.secondaryError')}</p>
      ) : list.data.length === 0 ? (
        <p className={styles.empty}>{t('ar.pr.empty')}</p>
      ) : (
        <ul className={styles.list}>
          {list.data.map((p) => (
            <li key={p.id} className={styles.row}>
              <div className={styles.rowMain}>
                <span className={styles.amount}>{formatMoney(p.promisedAmount, receivable.currency)}</span>
                <span className={styles.muted}>
                  {t('ar.pr.for', { date: formatBusinessDate(p.promisedOn) })}
                </span>
              </div>
              <StatusBadge domain="promise" status={p.status} />
              {canEdit && p.status === 'pending' && (
                <div className={styles.actions} role="group" aria-label={t('ar.pr.close')}>
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={sending !== null}
                    onClick={() => void close(p, 'kept')}
                  >
                    {t('ar.pr.kept')}
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={sending !== null}
                    onClick={() => void close(p, 'broken')}
                  >
                    {t('ar.pr.broken')}
                  </Button>
                  <Button
                    variant="tertiary"
                    size="sm"
                    disabled={sending !== null}
                    onClick={() => void close(p, 'cancelled')}
                  >
                    {t('ar.pr.cancel')}
                  </Button>
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}
