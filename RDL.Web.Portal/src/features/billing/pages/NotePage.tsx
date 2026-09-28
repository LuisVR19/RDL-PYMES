import * as Checkbox from '@radix-ui/react-checkbox'
import { useQuery } from '@tanstack/react-query'
import { Big } from 'big.js'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, Navigate, useLocation, useParams, useSearchParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { TextArea, TextField } from '@/design-system/components/Field/Field'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Invoice, InvoiceLine, InvoiceOverview } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { daysBetween, DEFAULT_TZ, formatBusinessDate, formatInstantDate, todayIn } from '@/shared/dates/dates'
import { useHotkey } from '@/shared/hotkeys/useHotkey'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { CASH, CREDIT } from '@/shared/saleConditions'
import { useSession } from '@/shared/session/SessionProvider'
import { DraftLines } from '../draft/DraftLines'
import { IssueDialog } from '../draft/IssueDialog'
import {
  formFromInvoice,
  formIssues,
  issueList,
  LOCAL_CURRENCY,
  newLineKey,
  parseQuantity,
  toDraftInput,
  toInputText,
  type DraftForm,
  type DraftLine,
} from '../draft/model'
import { TotalsPanel } from '../draft/TotalsPanel'
import { useDraftSync } from '../draft/useDraftSync'
import styles from '../draft/draft.module.css'

type NoteType = 'credit_note' | 'debit_note'

/** Una línea de la factura de referencia que se puede acreditar, con la cantidad elegida. */
interface CreditLine {
  ref: InvoiceLine
  include: boolean
  quantity: string
}

const REASON_MIN = 10

/**
 * Pantalla 16 · Nota de crédito o débito (prototipo «16»). La factura de referencia es fija; el motivo es
 * obligatorio. La de crédito elige líneas y cantidades de la factura; la de débito agrega cargos del catálogo y un
 * vencimiento. Se guarda y se emite como cualquier borrador: los totales son los de Billing.
 *
 * Mientras la nota no se emite, su id queda en `?borrador=` para retomarla; Billing devuelve su factura y su motivo
 * (F5), así que se retoma igual desde Documentos.
 */
export function NotePage() {
  const { id = '' } = useParams()
  const { pathname } = useLocation()
  const [search] = useSearchParams()
  const type: NoteType = pathname.endsWith('/nota-debito') ? 'debit_note' : 'credit_note'
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const org = activeOrg?.id
  const noteId = search.get('borrador')

  const reference = useQuery({
    queryKey: ['invoices', org, 'detail', id],
    queryFn: () => ds.invoices.get(id),
    enabled: !!org,
  })
  const overview = useQuery({
    queryKey: ['invoices', org, 'overview', id],
    queryFn: () => ds.invoices.overview(id),
    enabled: !!org,
  })
  const note = useQuery({
    queryKey: ['invoices', org, 'detail', noteId],
    queryFn: () => ds.invoices.get(noteId ?? ''),
    enabled: Boolean(noteId) && !!org,
  })

  if (reference.isPending || (noteId && note.isPending)) return <SkeletonRows rows={8} columns={4} />
  if (reference.isError) {
    return <ErrorState onRetry={() => void reference.refetch()} refCode={refOf(reference.error)} />
  }
  const ref = reference.data
  if (ref.documentType !== 'invoice' || ref.status !== 'issued') {
    return (
      <div className={styles.page}>
        <InlineAlert tone="info">{t('note.notAllowed')}</InlineAlert>
      </div>
    )
  }
  const saved = noteId && note.isSuccess ? note.data : null
  if (saved && saved.status !== 'draft') return <Navigate to={`/facturas/${saved.id}`} replace />

  // `key` por tipo y no por borrador: al guardar la primera vez, la nota recibe id (`?borrador=`) y el editor debe
  // seguir montado, sin volver a leer lo que el usuario está escribiendo.
  return <NoteEditor key={type} type={type} reference={ref} overview={overview.data} initial={saved} />
}

function NoteEditor({
  type,
  reference,
  overview,
  initial,
}: {
  type: NoteType
  reference: Invoice
  overview: InvoiceOverview | undefined
  initial: Invoice | null
}) {
  const { activeOrg, setDirty } = useSession()
  const [, setSearch] = useSearchParams()
  const tz = activeOrg?.timezone ?? DEFAULT_TZ
  const today = todayIn(tz)
  const credit = type === 'credit_note'
  const [reason, setReason] = useState(initial?.referenceReason ?? '')
  const [due, setDue] = useState(today)
  const [creditLines, setCreditLines] = useState<CreditLine[]>(() => initialCreditLines(reference, initial))
  const [debitLines, setDebitLines] = useState<DraftLine[]>(() =>
    initial && !credit ? formFromInvoice(initial, null).lines : [],
  )
  const [showIssues, setShowIssues] = useState(false)
  const [issuing, setIssuing] = useState(false)

  const form = useMemo(
    () =>
      buildForm(
        reference,
        credit ? toDraftLines(creditLines, reference.currency) : debitLines,
        credit,
        due,
        today,
      ),
    [reference, credit, creditLines, debitLines, due, today],
  )
  const creditProblems = credit ? creditIssues(creditLines) : []
  const reasonOk = reason.trim().length >= REASON_MIN
  const dueOk = credit || (safeDays(today, due) ?? -1) >= 0
  const input = useMemo(
    () =>
      creditProblems.length > 0 || !dueOk
        ? null
        : toDraftInput(form, type, { invoiceId: reference.id, reason: reason.trim() }),
    [creditProblems.length, dueOk, form, type, reference.id, reason],
  )
  const onCreated = useCallback(
    (inv: Invoice) => setSearch({ borrador: inv.id }, { replace: true }),
    [setSearch],
  )
  const sync = useDraftSync({ initial, input, onCreated })

  const unsaved = sync.dirty && (reason.trim() !== '' || form.lines.length > 0)
  useEffect(() => {
    setDirty(unsaved)
    return () => setDirty(false)
  }, [unsaved, setDirty])

  const local = formIssues(form)
  const errList = showIssues
    ? [
        ...(reasonOk ? [] : [t('common.reasonRequired')]),
        ...(dueOk ? [] : [t('note.issue.due')]),
        ...(credit ? creditProblems : issueList(form, local)),
      ]
    : []

  async function saveNow() {
    setShowIssues(true)
    if (input) await sync.save()
  }
  useHotkey('mod+s', () => void saveNow())

  async function tryIssue() {
    setShowIssues(true)
    if (!reasonOk || !dueOk || creditProblems.length > 0 || form.lines.length === 0) return
    if (!credit && Object.keys(local.lines).length > 0) return
    const current = sync.dirty || !sync.saved ? await sync.save() : sync.saved
    if (current) setIssuing(true)
  }

  const saved = sync.saved
  const balance = overview?.receivable.balance
  const money = (v: string) => formatMoney(v, reference.currency)

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titles}>
          <Link to={`/facturas/${reference.id}`} className={styles.back}>
            ‹ {reference.number}
          </Link>
          <div className={styles.titleRow}>
            <h1 className={styles.title}>{t(`note.title.${type}`)}</h1>
            <StatusBadge domain="invoice" status="draft" />
          </div>
        </div>
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
          <section className={styles.refBlock} aria-label={t('note.reference')}>
            <span className={styles.refTitle}>{t('note.reference')}</span>
            <RefItem label={t('note.ref.invoice')} value={reference.number ?? '—'} mono />
            <RefItem label={t('note.ref.customer')} value={reference.customerSnapshot?.legalName ?? '—'} />
            <RefItem
              label={t('note.ref.issued')}
              value={reference.issuedAt ? formatInstantDate(reference.issuedAt, tz) : '—'}
            />
            <RefItem
              label={t('note.ref.totalBalance')}
              value={`${money(reference.total)} · ${balance ? formatMoney(balance.balanceAmount, balance.currency) : t('common.notAvailable')}`}
            />
          </section>

          <section className={styles.headCard} aria-label={t('note.reason')}>
            <TextArea
              className={styles.full}
              label={t('note.reason')}
              required
              minLength={REASON_MIN}
              maxLength={500}
              placeholder={t(`note.reason.placeholder.${type}`)}
              error={showIssues && !reasonOk ? t('common.reasonRequired') : undefined}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={3}
            />
            {!credit && (
              <TextField
                label={t('note.due')}
                required
                type="date"
                min={today}
                value={due}
                error={dueOk ? undefined : t('note.issue.due')}
                help={due && dueOk ? formatBusinessDate(due) : undefined}
                onChange={(e) => setDue(e.target.value)}
              />
            )}
          </section>

          {credit ? (
            <CreditLines
              lines={creditLines}
              saved={saved}
              stale={sync.dirty || sync.status === 'busy'}
              currency={reference.currency}
              onChange={setCreditLines}
            />
          ) : (
            <DraftLines
              lines={debitLines}
              currency={reference.currency}
              saved={saved}
              stale={sync.dirty || sync.status === 'busy'}
              issues={local.lines}
              onChange={setDebitLines}
              hint={t('note.lines.hint.debit_note')}
            />
          )}
        </div>

        <TotalsPanel
          title={t('note.totals')}
          currency={reference.currency}
          saved={saved}
          status={sync.status}
          error={sync.error}
          dirty={sync.dirty}
          tz={tz}
          saveShortcut={false}
          onRetry={() => void sync.save()}
          onSave={() => void saveNow()}
          primary={
            <Button variant="primary" onClick={() => void tryIssue()} disabled={sync.status === 'busy'}>
              {t('note.emit')}
            </Button>
          }
        />
      </div>

      {saved && (
        <IssueDialog
          open={issuing}
          onOpenChange={setIssuing}
          draft={saved}
          summary={[
            { label: t('issue.row.invoice'), value: reference.number ?? '—' },
            { label: t('issue.row.customer'), value: reference.customerSnapshot?.legalName ?? '—' },
            {
              label: t('issue.row.noteTotal'),
              value: formatMoney(saved.total, saved.currency),
              strong: true,
            },
            { label: t('issue.row.reason'), value: reason.trim() },
          ]}
        />
      )}
    </div>
  )
}

function RefItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className={styles.refItem}>
      <span className={styles.muted}>{label}</span>
      <span className={mono ? styles.monoValue : undefined}>{value}</span>
    </div>
  )
}

function CreditLines({
  lines,
  saved,
  stale,
  currency,
  onChange,
}: {
  lines: CreditLine[]
  saved: Invoice | null
  stale: boolean
  currency: Invoice['currency']
  onChange: (lines: CreditLine[]) => void
}) {
  const set = (i: number, p: Partial<CreditLine>) =>
    onChange(lines.map((l, j) => (j === i ? { ...l, ...p } : l)))
  // Las líneas guardadas de la nota son solo las incluidas, en el mismo orden.
  let savedIndex = 0
  return (
    <section className={styles.card} aria-labelledby="credit-lines-title">
      <div className={styles.cardHead}>
        <h2 id="credit-lines-title" className={styles.cardTitle}>
          {t('draft.lines')}
        </h2>
        <span className={styles.muted}>{t('note.lines.hint.credit_note')}</span>
      </div>
      {lines.map((l, i) => {
        const server = l.include ? saved?.lines[savedIndex++] : undefined
        const over = l.include && exceeds(l)
        return (
          <div key={l.ref.lineNumber} className={styles.creditLine} data-included={l.include}>
            <Checkbox.Root
              className={styles.check}
              checked={l.include}
              onCheckedChange={(c) => set(i, { include: c === true })}
              aria-label={t('note.lineInclude', { description: l.ref.description })}
            >
              <Checkbox.Indicator>
                <Check size={12} aria-hidden />
              </Checkbox.Indicator>
            </Checkbox.Root>
            <span className={styles.creditDesc}>{l.ref.description}</span>
            <input
              className={clsx(styles.control, styles.cell, styles.creditQty, over && styles.invalid)}
              aria-label={`${t('draft.col.quantity')} · ${l.ref.description}`}
              aria-invalid={over || undefined}
              inputMode="decimal"
              disabled={!l.include}
              value={l.quantity}
              onChange={(e) => set(i, { quantity: e.target.value })}
            />
            <span className={styles.creditMax}>
              {t('note.lineOf', { n: toInputText(canon(l.ref.quantity)) })}
            </span>
            <span className={clsx(styles.creditTotal, stale && styles.dimmedText)}>
              {server ? formatMoney(server.total, currency) : '—'}
            </span>
          </div>
        )
      })}
    </section>
  )
}

const canon = (v: string) => new Big(v).toFixed()

function exceeds(l: CreditLine): boolean {
  const q = parseQuantity(l.quantity)
  return q === null || new Big(q).gt(l.ref.quantity)
}

function creditIssues(lines: CreditLine[]): string[] {
  const included = lines.filter((l) => l.include)
  if (included.length === 0) return [t('note.issue.none')]
  return lines.flatMap((l, i) =>
    l.include && exceeds(l)
      ? [t('note.issue.over', { n: i + 1, max: toInputText(canon(l.ref.quantity)) })]
      : [],
  )
}

/**
 * Líneas a acreditar. Por defecto, todas con su cantidad completa. Al retomar una nota guardada, las que ya tenía
 * (por producto, en orden).
 */
function initialCreditLines(reference: Invoice, note: Invoice | null): CreditLine[] {
  const pending = [...(note?.lines ?? [])]
  return reference.lines.map((ref) => {
    if (!note) return { ref, include: true, quantity: toInputText(canon(ref.quantity)) }
    const at = pending.findIndex((n) => n.productId === ref.productId)
    const match = at >= 0 ? pending.splice(at, 1)[0] : undefined
    return { ref, include: Boolean(match), quantity: toInputText(canon(match?.quantity ?? ref.quantity)) }
  })
}

/**
 * Cada línea incluida, con el precio de la factura. El descuento de la factura se repite solo si se acredita la
 * cantidad completa: con una cantidad parcial, el portal no reparte el descuento por su cuenta (lo definirá F5).
 */
function toDraftLines(lines: CreditLine[], currency: Invoice['currency']): DraftLine[] {
  return lines.flatMap((l) => {
    if (!l.include || !l.ref.productId) return []
    const full = parseQuantity(l.quantity) === canon(l.ref.quantity)
    const discounted = new Big(l.ref.discount).gt(0) && full
    return [
      {
        key: newLineKey(),
        productId: l.ref.productId,
        description: l.ref.description,
        ...(l.ref.productCode ? { code: l.ref.productCode } : {}),
        cabys: l.ref.cabysCode,
        // El precio viene de la factura, ya en su moneda.
        productCurrency: currency,
        quantity: l.quantity,
        unitPrice: toInputText(canon(l.ref.unitPrice)),
        discount: discounted ? toInputText(canon(l.ref.discount)) : '',
        discountReason: discounted ? (l.ref.discountReason ?? '') : '',
      },
    ]
  })
}

/**
 * El borrador de la nota: el cliente, la moneda y el tipo de cambio de la factura. La de crédito conserva su
 * condición de venta; la de débito es a crédito hasta el vencimiento elegido (o de contado si vence hoy).
 */
function buildForm(
  reference: Invoice,
  lines: DraftLine[],
  credit: boolean,
  due: string,
  today: string,
): DraftForm {
  const days = credit ? (reference.creditTermDays ?? 0) : Math.max(0, safeDays(today, due) ?? 0)
  const saleCondition = credit ? reference.saleConditionCode : days > 0 ? CREDIT : CASH
  return {
    customer: {
      id: reference.customerId,
      legalName: reference.customerSnapshot?.legalName ?? '—',
    },
    branchId: reference.branchId ?? '',
    currency: reference.currency,
    exchangeRate: reference.currency === LOCAL_CURRENCY ? '' : toInputText(canon(reference.exchangeRate)),
    saleCondition,
    creditTermDays: String(days),
    notes: '',
    lines,
  }
}

/** Días hasta una fecha escrita en el campo; null si el campo está vacío o incompleto. */
function safeDays(from: string, to: string): number | null {
  try {
    return daysBetween(from, to)
  } catch {
    return null
  }
}

function refOf(err: unknown): string | undefined {
  return err instanceof ApiError ? err.correlationId || undefined : undefined
}
