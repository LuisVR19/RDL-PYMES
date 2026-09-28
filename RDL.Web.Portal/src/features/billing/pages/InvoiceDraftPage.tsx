import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, Navigate, useLocation, useNavigate, useParams, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ConfirmDialog } from '@/design-system/components/Dialog/Dialog'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextArea, TextField } from '@/design-system/components/Field/Field'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Customer, Invoice } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { addDays, DEFAULT_TZ, formatBusinessDate, formatInstantTime, todayIn } from '@/shared/dates/dates'
import { useHotkey } from '@/shared/hotkeys/useHotkey'
import { t } from '@/shared/i18n/t'
import { formatMoney, type Currency } from '@/shared/money/money'
import { CREDIT, SALE_CONDITIONS } from '@/shared/saleConditions'
import { useSession } from '@/shared/session/SessionProvider'
import { CustomerPicker } from '../draft/CustomerPicker'
import { DraftLines } from '../draft/DraftLines'
import { IssueDialog } from '../draft/IssueDialog'
import {
  emptyForm,
  formFromInvoice,
  formIssues,
  hasIssues,
  issueList,
  LOCAL_CURRENCY,
  serverIssues,
  toDraftInput,
  type DraftCustomer,
  type DraftForm,
  type FormIssues,
} from '../draft/model'
import { QuickCustomerDrawer } from '../draft/QuickCustomerDrawer'
import { TotalsPanel } from '../draft/TotalsPanel'
import { useDraftSync } from '../draft/useDraftSync'
import styles from '../draft/draft.module.css'

/**
 * Estado de navegación al crear: el formulario tal cual, para no perder lo escrito al pasar a la ruta de edición.
 * `savedAt` evita usarlo si el borrador cambió después (el estado de navegación sobrevive a una recarga).
 */
interface CreatedState {
  form: DraftForm
  savedAt: string
}

/**
 * Pantalla 13 · Factura · borrador (prototipo «13 Factura · borrador»). `/facturas/nueva` crea (con `?cliente=` llega
 * con el cliente elegido); `/facturas/:id/editar` edita un borrador guardado. Un documento ya emitido va a su detalle.
 */
export function InvoiceDraftPage() {
  const { id } = useParams()
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const location = useLocation()
  const [search] = useSearchParams()
  const org = activeOrg?.id
  const preset = search.get('cliente')

  const invoice = useQuery({
    queryKey: ['invoices', org, 'detail', id],
    queryFn: () => ds.invoices.get(id ?? ''),
    enabled: Boolean(id) && !!org,
  })
  const customerId = id ? invoice.data?.customerId : (preset ?? undefined)
  const state = location.state as CreatedState | null
  const created = state && invoice.data?.updatedAt === state.savedAt ? state.form : undefined
  const customer = useQuery({
    queryKey: ['customers', org, 'detail', customerId],
    queryFn: () => ds.customers.get(customerId ?? ''),
    enabled: Boolean(customerId) && !!org && !created,
  })

  if (id && invoice.isPending) return <SkeletonRows rows={8} columns={4} />
  if (id && invoice.isError) {
    return <ErrorState onRetry={() => void invoice.refetch()} refCode={refOf(invoice.error)} />
  }
  const inv = invoice.data
  if (inv && inv.status !== 'draft') return <Navigate to={`/facturas/${inv.id}`} replace />
  if (inv && inv.documentType !== 'invoice') {
    return (
      <div className={styles.page}>
        <InlineAlert tone="info">
          {t('draft.isNote')} <Link to="/documentos">{t('invoice.back')}</Link>
        </InlineAlert>
      </div>
    )
  }
  // El cliente solo hace falta para mostrar su nombre: si falla, el borrador se edita igual con el nombre pendiente.
  if (customerId && !created && customer.isPending) return <SkeletonRows rows={8} columns={4} />

  const initialForm: DraftForm =
    created ??
    (inv
      ? formFromInvoice(inv, draftCustomer(customer.data, inv.customerId))
      : {
          ...emptyForm(activeOrg?.defaultCurrency ?? LOCAL_CURRENCY),
          customer: draftCustomer(customer.data),
        })

  // Recién creado (`created`), el editor sigue siendo el mismo que el de «nueva»: no se remonta, así no se pierde
  // nada de lo que estaba en curso (por ejemplo, el diálogo de emitir que pidió guardar primero).
  return (
    <DraftEditor
      key={created ? 'nueva' : (inv?.id ?? 'nueva')}
      initial={inv ?? null}
      initialForm={initialForm}
    />
  )
}

function draftCustomer(c: Customer | undefined, fallbackId?: string): DraftCustomer | null {
  if (c) return { id: c.id, legalName: c.legalName, identification: c.identification }
  return fallbackId ? { id: fallbackId, legalName: '—' } : null
}

function DraftEditor({ initial, initialForm }: { initial: Invoice | null; initialForm: DraftForm }) {
  const ds = useDataSource()
  const { activeOrg, setDirty } = useSession()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const toast = useToast()
  const tz = activeOrg?.timezone ?? DEFAULT_TZ
  const [form, setForm] = useState<DraftForm>(initialForm)
  const [showIssues, setShowIssues] = useState(false)
  const [quick, setQuick] = useState<{ text: string } | null>(null)
  const [issuing, setIssuing] = useState(false)
  const [discardOpen, setDiscardOpen] = useState(false)
  const [discarding, setDiscarding] = useState(false)
  const [discardError, setDiscardError] = useState<ApiError | null>(null)
  // Lo último escrito (no lo que se mandó): si el usuario siguió escribiendo mientras se creaba, no se pierde.
  const formRef = useRef(form)
  useEffect(() => {
    formRef.current = form
  })

  const input = useMemo(() => toDraftInput(form, 'invoice'), [form])
  const onCreated = useCallback(
    (inv: Invoice) =>
      navigate(`/facturas/${inv.id}/editar`, {
        replace: true,
        state: { form: formRef.current, savedAt: inv.updatedAt } satisfies CreatedState,
      }),
    [navigate],
  )
  const sync = useDraftSync({ initial, input, onCreated })

  // El cambio de organización (pantalla 5) pregunta antes de perder lo que no se guardó.
  const unsaved = sync.dirty && (form.customer !== null || form.lines.length > 0)
  useEffect(() => {
    setDirty(unsaved)
    return () => setDirty(false)
  }, [unsaved, setDirty])

  const local = formIssues(form)
  const server = sync.error?.errors.length ? serverIssues(sync.error.errors, form) : null
  // Los errores de formato se ven al escribir; «falta el cliente» o «sin líneas», solo al intentar emitir o guardar.
  const shown: FormIssues = {
    ...(showIssues ? { customer: local.customer } : {}),
    exchangeRate: local.exchangeRate ?? server?.exchangeRate,
    creditTermDays: local.creditTermDays ?? server?.creditTermDays,
    lines: { ...server?.lines, ...local.lines },
  }
  const errList = showIssues ? issueList(form, local) : []

  const patch = (p: Partial<DraftForm>) => setForm((f) => ({ ...f, ...p }))

  async function saveNow() {
    setShowIssues(true)
    if (!input) return
    await sync.save()
  }
  useHotkey('mod+s', () => void saveNow())

  async function tryIssue() {
    setShowIssues(true)
    if (hasIssues(local) || form.lines.length === 0) return
    const current = sync.dirty || !sync.saved ? await sync.save() : sync.saved
    if (current) setIssuing(true)
  }

  async function discard() {
    if (!sync.saved) return
    setDiscarding(true)
    setDiscardError(null)
    try {
      await ds.invoices.discardDraft(sync.saved.id)
      setDirty(false)
      await queryClient.invalidateQueries({ queryKey: ['invoices', activeOrg?.id, 'list'] })
      toast({ tone: 'success', title: t('draft.discard.done') })
      navigate('/documentos', { replace: true })
    } catch (err) {
      setDiscardError(
        err instanceof ApiError
          ? err
          : new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }),
      )
      setDiscarding(false)
    }
  }

  const saved = sync.saved
  const savedLabel = sync.dirty
    ? t('draft.unsaved')
    : saved
      ? t('draft.saved', { time: formatInstantTime(saved.updatedAt, tz) })
      : t('draft.notSaved')
  const today = todayIn(tz)
  const credit = form.saleCondition === CREDIT
  const days = /^\d{1,4}$/.test(form.creditTermDays.trim()) ? Number(form.creditTermDays.trim()) : null
  const due = credit
    ? days === null
      ? '—'
      : formatBusinessDate(addDays(today, days))
    : t('draft.due.cash', { date: formatBusinessDate(today) })

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titles}>
          <Link to="/documentos" className={styles.back}>
            ‹ {t('invoice.back')}
          </Link>
          <div className={styles.titleRow}>
            <h1 className={styles.title}>{t(saved ? 'draft.title.edit' : 'invoice.new')}</h1>
            <StatusBadge domain="invoice" status="draft" />
            <span className={sync.dirty ? styles.unsaved : styles.muted} aria-live="polite">
              {savedLabel}
            </span>
          </div>
        </div>
        {saved && (
          <Button variant="tertiary" onClick={() => setDiscardOpen(true)}>
            {t('draft.discard')}
          </Button>
        )}
      </div>

      {errList.length > 0 && (
        <InlineAlert tone="danger" title={t('draft.errors.title')}>
          <ul className={styles.errList}>
            {errList.map((e) => (
              <li key={e}>{e}</li>
            ))}
          </ul>
        </InlineAlert>
      )}

      <div className={styles.columns}>
        <div className={styles.main}>
          <section className={styles.headCard} aria-label={t('draft.title.edit')}>
            <div className={styles.full}>
              <CustomerPicker
                value={form.customer}
                error={shown.customer}
                onChange={(customer) => patch({ customer })}
                onCreate={(text) => setQuick({ text })}
              />
            </div>
            <BranchSelect value={form.branchId} onChange={(branchId) => patch({ branchId })} />
            <Select
              label={t('draft.currency')}
              value={form.currency}
              options={(['CRC', 'USD'] as const).map((c) => ({ value: c, label: t(`draft.currency.${c}`) }))}
              onChange={(e) => patch({ currency: e.target.value as Currency })}
            />
            {form.currency !== LOCAL_CURRENCY && (
              <TextField
                label={t('draft.exchangeRate')}
                required
                inputMode="decimal"
                className={styles.tabular}
                help={t('draft.exchangeRate.help')}
                value={form.exchangeRate}
                error={shown.exchangeRate}
                onChange={(e) => patch({ exchangeRate: e.target.value })}
              />
            )}
            <Select
              label={t('draft.saleCondition')}
              value={form.saleCondition}
              options={SALE_CONDITIONS.map((c) => ({ value: c.code, label: c.label }))}
              onChange={(e) => patch({ saleCondition: e.target.value })}
            />
            {credit && (
              <TextField
                label={t('draft.creditDays')}
                inputMode="numeric"
                className={styles.tabular}
                value={form.creditTermDays}
                error={shown.creditTermDays}
                onChange={(e) => patch({ creditTermDays: e.target.value })}
              />
            )}
            <div className={styles.field}>
              <span className={styles.label}>{t('draft.due')}</span>
              <span className={styles.readonly}>{due}</span>
              <span className={styles.muted}>{t('draft.due.help')}</span>
            </div>
          </section>

          <DraftLines
            lines={form.lines}
            currency={form.currency}
            saved={saved}
            stale={sync.dirty || sync.status === 'busy'}
            issues={shown.lines}
            onChange={(lines) => patch({ lines })}
          />

          <div className={styles.card}>
            <TextArea
              className={styles.notes}
              label={t('draft.notes')}
              placeholder={t('draft.notes.placeholder')}
              maxLength={2000}
              value={form.notes}
              onChange={(e) => patch({ notes: e.target.value })}
            />
          </div>
        </div>

        <TotalsPanel
          title={t('draft.totals')}
          currency={form.currency}
          saved={saved}
          status={sync.status}
          error={sync.error}
          dirty={sync.dirty}
          tz={tz}
          onRetry={() => void sync.save()}
          onSave={() => void saveNow()}
          primary={
            <Button variant="primary" onClick={() => void tryIssue()} disabled={sync.status === 'busy'}>
              {t('invoice.emit')}
            </Button>
          }
        />
      </div>

      <QuickCustomerDrawer
        open={quick !== null}
        initialText={quick?.text ?? ''}
        onOpenChange={(o) => !o && setQuick(null)}
        onCreated={(c) => {
          patch({ customer: { id: c.id, legalName: c.legalName, identification: c.identification } })
          setQuick(null)
        }}
      />

      {saved && (
        <IssueDialog
          open={issuing}
          onOpenChange={setIssuing}
          draft={saved}
          summary={[
            { label: t('issue.row.customer'), value: form.customer?.legalName ?? '—' },
            { label: t('issue.row.total'), value: formatMoney(saved.total, saved.currency), strong: true },
            { label: t('issue.row.currency'), value: t(`draft.currency.${saved.currency}`) },
            { label: t('issue.row.due'), value: due },
            { label: t('issue.row.lines'), value: String(saved.lines.length) },
          ]}
        />
      )}

      {saved && (
        <ConfirmDialog
          open={discardOpen}
          onOpenChange={(o) => {
            if (!o) setDiscardError(null)
            setDiscardOpen(o)
          }}
          title={t('draft.discard.title')}
          warning={t('draft.discard.body')}
          tone="danger"
          confirmLabel={t('draft.discard.confirm')}
          confirmingLabel={t('draft.discard.confirming')}
          sending={discarding}
          errorMessage={discardError ? t('draft.discard.error') : undefined}
          errorRef={discardError?.correlationId || undefined}
          onConfirm={() => void discard()}
        />
      )}
    </div>
  )
}

/** Sucursal (opcional en Billing). Si Platform no responde o no hay sucursales, el campo no se muestra. */
function BranchSelect({ value, onChange }: { value: string; onChange: (id: string) => void }) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const branches = useQuery({
    queryKey: ['branches', activeOrg?.id, 'active'],
    queryFn: () => ds.branches.list({ active: true }),
    enabled: !!activeOrg,
  })
  const items = branches.data?.items ?? []
  if (items.length === 0) return null
  return (
    <Select
      label={t('draft.branch')}
      value={value}
      options={[
        { value: '', label: t('draft.branch.none') },
        ...items.map((b) => ({ value: b.id, label: `${b.code} · ${b.name}` })),
      ]}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
