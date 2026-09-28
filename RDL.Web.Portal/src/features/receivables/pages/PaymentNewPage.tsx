import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Big } from 'big.js'
import { Check } from 'lucide-react'
import { Fragment, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { MoneyField } from '@/design-system/components/Choice/Choice'
import { InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { Panel, PanelHeader } from '@/design-system/components/Surface/Surface'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { PaymentInput, Receivable } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, formatBusinessDate, todayIn } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney, parseMoneyInput, type Currency } from '@/shared/money/money'
import { DEFAULT_PAYMENT_METHOD, PAYMENT_METHODS, paymentMethodLabel } from '@/shared/paymentMethods'
import { useSession } from '@/shared/session/SessionProvider'
import { autoDistribute, checkApplications, collectable, lateText, overdueDays } from '../model'
import styles from '../receivables.module.css'

type Step = 1 | 2 | 3

interface Data {
  customerId: string
  receivedOn: string
  amount: string
  currency: Currency
  exchangeRate: string
  method: string
  reference: string
}

/**
 * Pantalla 26 · Registrar pago (prototipo «26»): tres pasos. 1) datos del pago; 2) aplicarlo a las cuentas abiertas
 * del cliente en la misma moneda, de la más antigua a la más nueva («Repartir automáticamente»); lo aplicado de más
 * bloquea y lo que sobra queda sin aplicar; 3) confirmar. Pago y aplicaciones van en UNA llamada (Receivables los
 * registra en una transacción) con una `Idempotency-Key` por intento: reintentar tras un error no duplica el pago.
 */
export function PaymentNewPage() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()
  const keyFor = useIdempotencyKey()
  const [params] = useSearchParams()
  const org = activeOrg?.id
  const today = todayIn(activeOrg?.timezone ?? DEFAULT_TZ)
  const [step, setStep] = useState<Step>(1)
  const [data, setData] = useState<Data>({
    customerId: params.get('cliente') ?? '',
    receivedOn: today,
    amount: '',
    currency: activeOrg?.defaultCurrency ?? 'CRC',
    exchangeRate: '',
    method: DEFAULT_PAYMENT_METHOD,
    reference: '',
  })
  const [typed, setTyped] = useState<Record<string, string> | null>(null)
  const [touched, setTouched] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ApiError | null>(null)

  const customers = useQuery({
    queryKey: ['customers', org, { active: true, limit: 100 }],
    queryFn: () => ds.customers.list({ active: true, limit: 100 }),
    enabled: !!org,
    staleTime: 60_000,
  })
  const accounts = useQuery({
    queryKey: ['receivables', org, 'byCustomer', data.customerId],
    queryFn: () => ds.receivables.byCustomer(data.customerId, { limit: 100 }),
    enabled: !!org && !!data.customerId && step >= 2,
  })
  const open: Receivable[] = (accounts.data?.items ?? [])
    .filter((r) => collectable(r) && r.currency === data.currency && new Big(r.balanceAmount).gt(0))
    .toSorted((a, b) => a.dueOn.localeCompare(b.dueOn) || a.issuedOn.localeCompare(b.issuedOn))

  const amount = parseMoneyInput(data.amount)
  const rate = data.currency === 'USD' && data.exchangeRate.trim() ? parseMoneyInput(data.exchangeRate) : null
  const errors = {
    customerId: !data.customerId ? t('pay.customerError') : undefined,
    receivedOn: !data.receivedOn || data.receivedOn > today ? t('pay.dateError') : undefined,
    amount: amount === null || new Big(amount).lte(0) ? t('pay.amountError') : undefined,
    exchangeRate:
      data.currency === 'USD' && data.exchangeRate.trim() && (rate === null || new Big(rate).lte(0))
        ? t('pay.exchangeRateError')
        : undefined,
  }
  const step1Ok = Object.values(errors).every((e) => e === undefined)
  const values = typed ?? {}
  const check = checkApplications(amount ?? '0', open, values)
  const customerName = customers.data?.items.find((c) => c.id === data.customerId)?.legalName ?? ''

  const set = (patch: Partial<Data>) => {
    // Cambiar cliente, moneda o monto invalida lo repartido: se vuelve a repartir al pasar al paso 2.
    if ('customerId' in patch || 'currency' in patch || 'amount' in patch) setTyped(null)
    setData((d) => ({ ...d, ...patch }))
  }

  function toStep2() {
    setTouched(true)
    if (step1Ok) setStep(2)
  }

  // Al llegar al paso 2 con las cuentas cargadas y nada escrito todavía, se reparte solo (prototipo).
  if (step === 2 && typed === null && accounts.isSuccess && amount !== null) {
    setTyped(autoDistribute(amount, open))
  }

  async function register() {
    if (amount === null || !check.ok) return
    const applications = open
      .filter((r) => check.amounts[r.id] !== undefined)
      .map((r) => ({ receivableId: r.id, amount: check.amounts[r.id] as string }))
    const input: PaymentInput = {
      customerId: data.customerId,
      receivedOn: data.receivedOn,
      amount,
      currency: data.currency,
      ...(rate ? { exchangeRate: rate } : {}),
      paymentMethodCode: data.method,
      ...(data.reference.trim() ? { reference: data.reference.trim() } : {}),
      ...(applications.length ? { applications } : {}),
    }
    setSending(true)
    setError(null)
    try {
      const created = await ds.receivables.createPayment(input, keyFor(input))
      await queryClient.invalidateQueries({ queryKey: ['receivables', org] })
      toast({
        tone: 'success',
        title: t('ar.payment.registered', { n: formatMoney(created.amount, created.currency) }),
      })
      navigate(`/cobranza/pagos/${created.id}`, { replace: true })
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
    <div className={styles.form}>
      <div className={styles.titles}>
        <Link to="/cobranza/pagos" className={styles.back}>
          ‹ {t('pay.back')}
        </Link>
        <h1 className={styles.heading}>{t('ar.payment.register')}</h1>
      </div>
      <Stepper step={step} />

      {step === 1 && (
        <>
          <Panel padded={false}>
            <div className={styles.formGrid}>
              <Select
                label={t('pay.customer')}
                required
                className={styles.full}
                value={data.customerId}
                error={touched ? errors.customerId : undefined}
                onChange={(e) => set({ customerId: e.target.value })}
                options={[
                  { value: '', label: t('pay.customerPick') },
                  ...(customers.data?.items ?? []).map((c) => ({
                    value: c.id,
                    label: `${c.legalName} · ${c.identification.number}`,
                  })),
                ]}
              />
              <TextField
                label={t('pay.date')}
                required
                type="date"
                max={today}
                value={data.receivedOn}
                help={t('pay.dateHelp')}
                error={touched ? errors.receivedOn : undefined}
                onChange={(e) => set({ receivedOn: e.target.value })}
              />
              <MoneyField
                label={t('pay.amount')}
                required
                amount={data.amount}
                currency={data.currency}
                onAmountChange={(v) => set({ amount: v })}
                onCurrencyChange={(c) => set({ currency: c })}
                error={touched ? errors.amount : undefined}
              />
              {data.currency === 'USD' && (
                <TextField
                  label={t('pay.exchangeRate')}
                  inputMode="decimal"
                  value={data.exchangeRate}
                  help={t('pay.exchangeRateHelp')}
                  error={touched ? errors.exchangeRate : undefined}
                  onChange={(e) => set({ exchangeRate: e.target.value })}
                />
              )}
              <Select
                label={t('pay.method')}
                value={data.method}
                help={t('pay.methodHelp')}
                onChange={(e) => set({ method: e.target.value })}
                options={PAYMENT_METHODS.map((m) => ({ value: m.code, label: m.label }))}
              />
              <TextField
                label={t('pay.reference')}
                value={data.reference}
                maxLength={100}
                placeholder={t('pay.referencePlaceholder')}
                onChange={(e) => set({ reference: e.target.value })}
              />
            </div>
          </Panel>
          <div className={styles.formActions}>
            <Button variant="secondary" onClick={() => navigate('/cobranza/pagos')}>
              {t('pay.cancel')}
            </Button>
            <Button variant="primary" onClick={toStep2}>
              {t('pay.next1')}
            </Button>
          </div>
        </>
      )}

      {step === 2 && amount !== null && (
        <>
          <Sums
            amount={amount}
            applied={check.applied}
            remaining={check.remaining}
            currency={data.currency}
          />
          {check.overPayment && (
            <InlineAlert tone="danger">
              {t('ar.payment.over', {
                applied: formatMoney(check.applied, data.currency),
                diff: formatMoney(check.remaining.abs(), data.currency),
              })}
            </InlineAlert>
          )}
          <Panel padded={false} className={styles.listPanel}>
            <PanelHeader
              title={t('pay.openFor', { customer: customerName })}
              actions={
                open.length > 0 && (
                  <div className={styles.actions}>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => setTyped(autoDistribute(amount, open))}
                    >
                      {t('pay.auto')}
                    </Button>
                    <Button variant="tertiary" size="sm" onClick={() => setTyped({})}>
                      {t('pay.clear')}
                    </Button>
                  </div>
                )
              }
            >
              <span className={styles.muted}>{t('pay.openHelp')}</span>
            </PanelHeader>
            {accounts.isPending ? (
              <SkeletonRows rows={3} columns={3} />
            ) : accounts.isError ? (
              <InlineAlert
                tone="danger"
                refCode={
                  accounts.error instanceof ApiError ? accounts.error.correlationId || undefined : undefined
                }
                actions={
                  <Button variant="secondary" size="sm" onClick={() => void accounts.refetch()}>
                    {t('common.retry')}
                  </Button>
                }
              >
                {t('ar.secondaryError')}
              </InlineAlert>
            ) : open.length === 0 ? (
              <p className={styles.empty}>
                {t('pay.noOpen', { currency: t(`pay.currencyName.${data.currency}`) })}
              </p>
            ) : (
              open.map((r) => {
                const err = check.rowErrors[r.id]
                const late = overdueDays(r, today)
                return (
                  <div key={r.id} className={styles.applyRow}>
                    <div className={styles.applyDoc}>
                      <span className={styles.mono}>{r.documentNumber}</span>
                      <span className={styles.muted}>
                        {t('pay.dueOn', { date: formatBusinessDate(r.dueOn) })}
                        {late > 0 && (
                          <>
                            {' '}
                            · <span className={styles.late}>! {lateText(late)}</span>
                          </>
                        )}
                      </span>
                    </div>
                    <div className={styles.applyBalance}>
                      <span className={styles.muted}>{t('ar.col.balance')}</span>
                      <span>{formatMoney(r.balanceAmount, r.currency)}</span>
                    </div>
                    <div className={styles.applyInput}>
                      <input
                        inputMode="decimal"
                        placeholder="0,00"
                        aria-label={t('pay.applyAmount', { document: r.documentNumber })}
                        aria-invalid={err ? true : undefined}
                        value={values[r.id] ?? ''}
                        onChange={(e) => setTyped({ ...values, [r.id]: e.target.value })}
                      />
                    </div>
                    {err && (
                      <span className={styles.rowError} role="alert">
                        {err === 'invalid'
                          ? t('pay.rowInvalid')
                          : t('ar.payment.rowOver', { saldo: formatMoney(r.balanceAmount, r.currency) })}
                      </span>
                    )}
                  </div>
                )
              })
            )}
          </Panel>
          {!check.overPayment && check.remaining.gt(0) && (
            <p className={styles.warnText}>
              ▲ {t('ar.payment.unapplied', { amount: formatMoney(check.remaining, data.currency) })}
            </p>
          )}
          <div className={styles.formActions}>
            <Button variant="secondary" onClick={() => setStep(1)}>
              {t('pay.backStep')}
            </Button>
            <Button variant="primary" disabled={!check.ok || accounts.isPending} onClick={() => setStep(3)}>
              {t('pay.next2')}
            </Button>
          </div>
        </>
      )}

      {step === 3 && amount !== null && (
        <>
          <Panel padded={false}>
            <dl className={styles.summary}>
              <dt>{t('pay.confirm.customer')}</dt>
              <dd>{customerName || '—'}</dd>
              <dt>{t('pay.confirm.date')}</dt>
              <dd>{formatBusinessDate(data.receivedOn)}</dd>
              <dt>{t('pay.confirm.method')}</dt>
              <dd>{paymentMethodLabel(data.method)}</dd>
              <dt>{t('pay.confirm.reference')}</dt>
              <dd>{data.reference.trim() || '—'}</dd>
              <dt>{t('pay.confirm.amount')}</dt>
              <dd>{formatMoney(amount, data.currency)}</dd>
              {rate && (
                <>
                  <dt>{t('pay.exchangeRate')}</dt>
                  <dd>{rate}</dd>
                </>
              )}
            </dl>
            <div className={styles.appliesTo}>
              <span className={styles.appliesTitle}>{t('pay.confirm.appliesTo')}</span>
              {Object.keys(check.amounts).length === 0 ? (
                <span className={styles.muted}>{t('pay.confirm.none')}</span>
              ) : (
                open
                  .filter((r) => check.amounts[r.id] !== undefined)
                  .map((r) => (
                    <div key={r.id} className={styles.appliesRow}>
                      <span className={styles.mono}>{r.documentNumber}</span>
                      <span>{formatMoney(check.amounts[r.id] as string, data.currency)}</span>
                    </div>
                  ))
              )}
              <div
                className={
                  check.remaining.gt(0) ? `${styles.appliesTotal} ${styles.warnText}` : styles.appliesTotal
                }
              >
                <span>{t('pay.sum.unapplied')}</span>
                <span>{formatMoney(check.remaining, data.currency)}</span>
              </div>
            </div>
          </Panel>
          {error && (
            <InlineAlert tone="danger" refCode={error.correlationId || undefined}>
              {error.is('application-exceeds-balance')
                ? t('pay.error.exceedsBalance')
                : error.is('conflict')
                  ? t('pay.error.changed')
                  : t('pay.error')}
            </InlineAlert>
          )}
          <div className={styles.formActions}>
            <Button variant="secondary" onClick={() => setStep(2)} disabled={sending}>
              {t('pay.backStep')}
            </Button>
            <Button
              variant="primary"
              onClick={() => void register()}
              loading={sending}
              loadingLabel={t('pay.registering')}
            >
              {t('ar.payment.register')}
            </Button>
          </div>
        </>
      )}
    </div>
  )
}

/** Indicador de pasos del prototipo: círculo con número (✓ si ya pasó) y línea verde entre los hechos. */
function Stepper({ step }: { step: Step }) {
  const steps = [t('pay.step.data'), t('pay.step.apply'), t('pay.step.confirm')]
  return (
    <ol className={styles.steps} aria-label={t('pay.steps')}>
      {steps.map((label, i) => {
        const n = (i + 1) as Step
        const state = n < step ? 'done' : n === step ? 'current' : 'todo'
        return (
          <Fragment key={label}>
            <li
              className={
                state === 'current' ? styles.stepCurrent : state === 'done' ? styles.stepDone : styles.step
              }
              aria-current={state === 'current' ? 'step' : undefined}
            >
              <span className={styles.stepCircle} aria-hidden>
                {state === 'done' ? <Check size={14} strokeWidth={3} /> : n}
              </span>
              <span className={styles.stepLabel}>{label}</span>
            </li>
            {i < steps.length - 1 && (
              <li aria-hidden className={n < step ? styles.stepLineDone : styles.stepLine} />
            )}
          </Fragment>
        )
      })}
    </ol>
  )
}

/** Franja fija del paso 2: pago, aplicado y lo que queda (o el excedente, que bloquea). */
function Sums({
  amount,
  applied,
  remaining,
  currency,
}: {
  amount: string
  applied: Big
  remaining: Big
  currency: Currency
}) {
  const over = remaining.lt(0)
  return (
    <div className={styles.sums}>
      <div className={styles.sum}>
        <span className={styles.sumLabel}>{t('pay.sum.payment')}</span>
        <span className={styles.sumValue}>{formatMoney(amount, currency)}</span>
      </div>
      <div className={styles.sum}>
        <span className={styles.sumLabel}>{t('pay.sum.applied')}</span>
        <span className={styles.sumValue}>{formatMoney(applied, currency)}</span>
      </div>
      <div
        role="status"
        className={over ? styles.sumDanger : remaining.gt(0) ? styles.sumWarn : styles.sumOk}
      >
        <span className={styles.sumLabel}>{over ? t('pay.sum.excess') : t('pay.sum.unapplied')}</span>
        <span className={styles.sumValue}>{formatMoney(remaining.abs(), currency)}</span>
      </div>
    </div>
  )
}
