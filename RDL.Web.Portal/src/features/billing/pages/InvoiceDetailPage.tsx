import * as Menu from '@radix-ui/react-dropdown-menu'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { Link, Navigate, useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Panel, PanelHeader } from '@/design-system/components/Surface/Surface'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { BalancePart, FiscalPart, Invoice, InvoiceLine, StatusChange } from '@/shared/api/billing-types'
import { ApiError } from '@/shared/api/types'
import { DEFAULT_TZ, daysOverdue, formatBusinessDate, formatInstant, todayIn } from '@/shared/dates/dates'
import { t, type MessageKey } from '@/shared/i18n/t'
import { formatMoney, MINUS } from '@/shared/money/money'
import { can } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { statusStyle } from '@/shared/status/status'
import { FiscalBadge } from '../FiscalBadge'
import { identificationText } from '../shared'
import { VoidInvoiceDialog } from '../VoidInvoiceDialog'
import styles from './InvoiceDetailPage.module.css'

/**
 * Pantalla 15 · Detalle de la factura (prototipo «15»). Tres cifras de tres dominios (arquitectura 2.2): el total de
 * Billing, el estado de Hacienda de E-Invoice y el saldo de Receivables, compuestos por el gateway. Si Hacienda o
 * Cobranza no responden, esa cifra dice que no está disponible y el resto de la pantalla sigue.
 * Solo lectura: una factura emitida no se edita; se corrige con una nota o se anula.
 */
export function InvoiceDetailPage() {
  const { id = '' } = useParams()
  const ds = useDataSource()
  const { activeOrg, role } = useSession()
  const navigate = useNavigate()
  const [voidOpen, setVoidOpen] = useState(false)
  const org = activeOrg?.id
  const tz = activeOrg?.timezone ?? DEFAULT_TZ

  const invoice = useQuery({
    queryKey: ['invoices', org, 'detail', id],
    queryFn: () => ds.invoices.get(id),
    enabled: !!org,
  })
  const overview = useQuery({
    queryKey: ['invoices', org, 'overview', id],
    queryFn: () => ds.invoices.overview(id),
    enabled: !!org,
  })
  const history = useQuery({
    queryKey: ['invoices', org, 'history', id],
    queryFn: () => ds.invoices.history(id),
    enabled: !!org,
  })

  if (invoice.isPending) return <SkeletonRows rows={8} columns={4} />
  if (invoice.isError) {
    return (
      <ErrorState
        onRetry={() => void invoice.refetch()}
        refCode={invoice.error instanceof ApiError ? invoice.error.correlationId || undefined : undefined}
      />
    )
  }
  const inv = invoice.data
  // Un borrador no tiene detalle de solo lectura: se edita.
  if (inv.status === 'draft') return <Navigate to={`/facturas/${inv.id}/editar`} replace />

  const fiscal: FiscalPart = overview.data?.fiscal ?? {
    availability: overview.isError ? 'unavailable' : 'absent',
  }
  const receivable: BalancePart = overview.data?.receivable ?? {
    availability: overview.isError ? 'unavailable' : 'absent',
  }
  const fiscalStatus = fiscal.status?.status
  const canEdit = role ? can(role, 'billing.edit') : false
  const issuedInvoice = inv.status === 'issued' && inv.documentType === 'invoice'
  // Las notas corrigen un documento que Hacienda ya respondió (aceptado o rechazado).
  const canNotes = canEdit && issuedInvoice && (fiscalStatus === 'accepted' || fiscalStatus === 'rejected')
  const canVoid = (role ? can(role, 'billing.void') : false) && issuedInvoice
  const snapshot = inv.customerSnapshot

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div className={styles.titles}>
          <Link to="/documentos" className={styles.back}>
            ‹ {t('invoice.back')}
          </Link>
          <div className={styles.titleRow}>
            <h1 className={styles.number}>{inv.number ?? t('doc.draftNumber')}</h1>
            <StatusBadge domain="invoice" status={inv.status} />
            {inv.requiresCorrection && <StatusBadge domain="invoice" status="requires_correction" />}
          </div>
          {snapshot && (
            <span className={styles.customer}>
              <Link to={`/clientes/${inv.customerId}`} className={styles.customerLink}>
                {snapshot.legalName}
              </Link>{' '}
              <span className={styles.muted}>· {identificationText(snapshot.identification)}</span>
            </span>
          )}
          <span className={styles.meta}>{metaOf(inv, tz)}</span>
        </div>
        <div className={styles.actions}>
          <DownloadMenu />
          {canNotes && (
            <>
              <Button variant="secondary" onClick={() => navigate(`/facturas/${inv.id}/nota-credito`)}>
                {t('invoice.creditNote')}
              </Button>
              <Button variant="secondary" onClick={() => navigate(`/facturas/${inv.id}/nota-debito`)}>
                {t('invoice.debitNote')}
              </Button>
            </>
          )}
          {canVoid && (
            <Button variant="secondary" className={styles.danger} onClick={() => setVoidOpen(true)}>
              {t('invoice.void')}
            </Button>
          )}
        </div>
      </div>

      <div className={styles.figures}>
        <div className={styles.figure}>
          <span className={styles.figLabel}>{t('invoice.fig.total')}</span>
          <span className={styles.figValue}>{formatMoney(inv.total, inv.currency)}</span>
          <span className={styles.figSub}>
            {t(`invoice.currency.${inv.currency}`)} ·{' '}
            {inv.lines.length === 1 ? t('invoice.linesOne') : t('invoice.linesMany', { n: inv.lines.length })}
          </span>
        </div>
        <div className={styles.figure}>
          <span className={styles.figLabel}>{t('invoice.fig.fiscal')}</span>
          {overview.isPending ? (
            <span className={styles.figSub}>…</span>
          ) : fiscal.availability === 'absent' ? (
            <span className={styles.figSub}>{t('invoice.fiscal.absent')}</span>
          ) : (
            <>
              <FiscalBadge part={fiscal} size="md" />
              <span className={styles.figSub}>
                {t(
                  fiscal.availability === 'unavailable'
                    ? 'invoice.fiscal.unavailable'
                    : `invoice.fiscal.${fiscalStatus ?? 'processing'}`,
                )}
              </span>
            </>
          )}
        </div>
        <BalanceFigure
          part={receivable}
          pending={overview.isPending}
          cancelled={inv.status === 'cancelled'}
          tz={tz}
        />
      </div>

      <FiscalAlert
        inv={inv}
        fiscal={fiscal}
        history={history.data}
        tz={tz}
        canNotes={canNotes}
        onCreditNote={() => navigate(`/facturas/${inv.id}/nota-credito`)}
        onRetry={() => void overview.refetch()}
      />

      <div className={styles.columns}>
        <Panel padded={false} className={styles.linesPanel}>
          <h2 className={styles.linesTitle}>{t('invoice.lines')}</h2>
          <LinesTable lines={inv.lines} currency={inv.currency} />
          <Totals inv={inv} />
        </Panel>
        <div className={styles.side}>
          <Panel>
            <PanelHeader title={t('invoice.history')} />
            <Timeline inv={inv} history={history.data ?? []} fiscal={fiscal} tz={tz} />
          </Panel>
          <Panel>
            <PanelHeader title={t('invoice.payments')} />
            <p className={styles.muted}>{t('invoice.payments.unavailable')}</p>
          </Panel>
          <Panel>
            <PanelHeader title={t('invoice.edoc')} />
            {fiscal.availability === 'available' ? (
              <Link to="/hacienda/bandeja" className={styles.link}>
                {t('invoice.edoc.open')}
              </Link>
            ) : (
              <p className={styles.muted}>{t('invoice.edoc.none')}</p>
            )}
          </Panel>
        </div>
      </div>

      {canVoid && (
        <VoidInvoiceDialog invoice={inv} receivable={receivable} open={voidOpen} onOpenChange={setVoidOpen} />
      )}
    </div>
  )
}

/** «Emitida el 18/09/2026 15:00 · Vence 18/10/2026 · Crédito 30 días». */
function metaOf(inv: Invoice, tz: string): string {
  return [
    inv.issuedAt && t('invoice.issuedOn', { date: formatInstant(inv.issuedAt, tz) }),
    inv.dueDate && t('invoice.dueOn', { date: formatBusinessDate(inv.dueDate) }),
    inv.creditTermDays ? t('invoice.credit', { n: inv.creditTermDays }) : undefined,
  ]
    .filter(Boolean)
    .join(' · ')
}

/** El archivo XML y la respuesta de Hacienda son de E-Invoice. TODO(api): descarga cuando E-Invoice exista. */
function DownloadMenu() {
  return (
    <Menu.Root>
      <Menu.Trigger asChild>
        <Button variant="secondary">
          {t('invoice.download')} <ChevronDown size={14} aria-hidden />
        </Button>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className={styles.menu} align="start" sideOffset={6}>
          {(['invoice.download.xml', 'invoice.download.response'] as const).map((k) => (
            <Menu.Item key={k} className={styles.menuItem} disabled>
              <span>{t(k)}</span>
              <span className={styles.menuSub}>{t('invoice.download.unavailable')}</span>
            </Menu.Item>
          ))}
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  )
}

function BalanceFigure({
  part,
  pending,
  cancelled,
  tz,
}: {
  part: BalancePart
  pending: boolean
  cancelled: boolean
  tz: string
}) {
  const b = part.balance
  const late = b && b.status !== 'paid' && b.status !== 'cancelled' ? daysOverdue(b.dueOn, todayIn(tz)) : 0
  const sub = cancelled
    ? t('invoice.balance.cancelled')
    : !b
      ? t(part.availability === 'unavailable' ? 'invoice.balance.unavailable' : 'invoice.balance.absent')
      : late > 0
        ? t('invoice.balance.overdue', { n: late })
        : b.status === 'paid'
          ? t('invoice.balance.paid')
          : t('invoice.dueOn', { date: formatBusinessDate(b.dueOn) })
  return (
    <div className={styles.figure}>
      <span className={styles.figLabel}>{t('invoice.fig.balance')}</span>
      {pending ? (
        <span className={styles.figSub}>…</span>
      ) : (
        <>
          <span className={styles.figValueRow}>
            <span className={b ? styles.figValue : styles.figValueMuted}>
              {b ? formatMoney(b.balanceAmount, b.currency) : t('common.notAvailable')}
            </span>
            {b && <StatusBadge domain="receivable" status={b.status} />}
          </span>
          <span className={late > 0 ? styles.figSubAlert : styles.figSub}>{sub}</span>
        </>
      )}
    </div>
  )
}

function FiscalAlert({
  inv,
  fiscal,
  history,
  tz,
  canNotes,
  onCreditNote,
  onRetry,
}: {
  inv: Invoice
  fiscal: FiscalPart
  history: StatusChange[] | undefined
  tz: string
  canNotes: boolean
  onCreditNote: () => void
  onRetry: () => void
}) {
  if (inv.status === 'cancelled') {
    const change = history?.find((h) => h.toStatus === 'cancelled')
    return (
      <InlineAlert tone="neutral" title={t('invoice.alert.cancelled.title')}>
        {t('invoice.alert.cancelled.body', {
          date: change ? formatInstant(change.changedAt, tz) : '—',
          reason: change?.reason ?? '—',
        })}
      </InlineAlert>
    )
  }
  if (fiscal.availability === 'unavailable') {
    return (
      <InlineAlert
        tone="warning"
        title={t('invoice.alert.unavailable.title')}
        actions={
          <Button size="sm" variant="secondary" onClick={onRetry}>
            {t('common.retry')}
          </Button>
        }
      >
        {t('invoice.alert.unavailable.body')}
      </InlineAlert>
    )
  }
  const s = fiscal.status?.status
  if (s === 'rejected') {
    const reason = fiscal.status?.haciendaStatusMessage || inv.fiscalRejectionReason
    return (
      <InlineAlert
        tone="danger"
        title={t('invoice.alert.rejected.title')}
        actions={
          canNotes && (
            <Button size="sm" variant="primary" onClick={onCreditNote}>
              {t('invoice.alert.rejected.action')}
            </Button>
          )
        }
      >
        {reason && <>{t('invoice.alert.rejected.reason', { reason })} </>}
        {t('invoice.alert.rejected.body')}
      </InlineAlert>
    )
  }
  const simple: Partial<Record<string, [MessageKey, MessageKey, 'warning' | 'info' | 'danger']>> = {
    contingency: ['invoice.alert.contingency.title', 'invoice.alert.contingency.body', 'warning'],
    processing: ['invoice.alert.processing.title', 'invoice.alert.processing.body', 'info'],
    signed: ['invoice.alert.processing.title', 'invoice.alert.processing.body', 'info'],
    sent: ['invoice.alert.processing.title', 'invoice.alert.processing.body', 'info'],
    error: ['invoice.alert.error.title', 'invoice.alert.error.body', 'danger'],
  }
  const a = s ? simple[s] : undefined
  if (!a) return null
  return (
    <InlineAlert tone={a[2]} title={t(a[0])}>
      {t(a[1])}
    </InlineAlert>
  )
}

function LinesTable({ lines, currency }: { lines: InvoiceLine[]; currency: Invoice['currency'] }) {
  return (
    <div className={styles.linesScroll}>
      <table className={styles.lines}>
        <thead>
          <tr>
            <th scope="col">{t('invoice.col.description')}</th>
            <th scope="col" className={styles.right}>
              {t('invoice.col.quantity')}
            </th>
            <th scope="col" className={styles.right}>
              {t('invoice.col.price')}
            </th>
            <th scope="col" className={styles.right}>
              {t('invoice.col.discount')}
            </th>
            <th scope="col">{t('invoice.col.tax')}</th>
            <th scope="col">{t('invoice.col.exoneration')}</th>
            <th scope="col" className={styles.right}>
              {t('invoice.col.total')}
            </th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l) => (
            <tr key={l.lineNumber}>
              <td>
                <span className={styles.lineDesc}>{l.description}</span>
                <span className={styles.lineMeta}>
                  {l.productCode
                    ? t('invoice.lineMeta', { code: l.productCode, cabys: l.cabysCode })
                    : t('invoice.lineCabys', { cabys: l.cabysCode })}
                </span>
              </td>
              <td className={styles.right}>
                {l.quantity.replace('.', ',')} {l.unitOfMeasureCode}
              </td>
              <td className={styles.right}>{formatMoney(l.unitPrice, currency)}</td>
              <td className={styles.right}>{l.discount === '0' ? '—' : formatMoney(l.discount, currency)}</td>
              <td>
                {l.taxes.length === 0 ? '—' : l.taxes.map((x) => `${x.rate.replace('.', ',')} %`).join(', ')}
              </td>
              <td>—</td>
              <td className={`${styles.right} ${styles.lineTotal}`}>{formatMoney(l.total, currency)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** Los totales tal como los calculó Billing: el portal no suma ni redondea. */
function Totals({ inv }: { inv: Invoice }) {
  const rows: [MessageKey, string][] = [
    ['invoice.subtotal', formatMoney(inv.subtotal, inv.currency)],
    [
      'invoice.discount',
      inv.discount === '0'
        ? formatMoney('0', inv.currency)
        : `${MINUS}${formatMoney(inv.discount, inv.currency)}`,
    ],
    ['invoice.tax', formatMoney(inv.tax, inv.currency)],
    ['invoice.exoneration', formatMoney(inv.exoneration, inv.currency)],
  ]
  return (
    <dl className={styles.totals}>
      {rows.map(([k, v]) => (
        <div key={k} className={styles.totalRow}>
          <dt>{t(k)}</dt>
          <dd>{v}</dd>
        </div>
      ))}
      <div className={styles.grandTotal}>
        <dt>{t('invoice.total')}</dt>
        <dd>{formatMoney(inv.total, inv.currency)}</dd>
      </div>
    </dl>
  )
}

/**
 * Historial: lo que sabe Billing (borrador creado, emitida, anulada) y, al final, el estado actual en Hacienda.
 * Los pasos intermedios de E-Invoice (firmado, enviado, intentos) llegan con su API (pantalla 21).
 */
function Timeline({
  inv,
  history,
  fiscal,
  tz,
}: {
  inv: Invoice
  history: StatusChange[]
  fiscal: FiscalPart
  tz: string
}) {
  const events: { key: string; label: string; detail?: string; at?: string; tone: string }[] = [
    { key: 'draft', label: t('invoice.history.draft'), at: inv.createdAt, tone: 'neutral' },
    ...history.map((h, i) => ({
      key: `h${i}`,
      label: t(h.toStatus === 'cancelled' ? 'invoice.history.cancelled' : 'invoice.history.issued'),
      detail:
        h.toStatus === 'issued' && inv.number
          ? t('invoice.history.issuedNumber', { number: inv.number })
          : h.reason,
      at: h.changedAt,
      tone: h.toStatus === 'cancelled' ? 'neutral' : 'info',
    })),
  ]
  if (fiscal.status) {
    const st = statusStyle('hacienda', fiscal.status.status)
    events.push({ key: 'fiscal', label: t('invoice.history.fiscal', { status: st.label }), tone: st.tone })
  }
  return (
    <ol className={styles.timeline}>
      {events.map((e) => (
        <li key={e.key} className={styles.event} data-tone={e.tone}>
          <span className={styles.dot} aria-hidden />
          <span className={styles.eventBody}>
            <span className={styles.eventLabel}>{e.label}</span>
            {e.detail && <span className={styles.muted}>{e.detail}</span>}
            {e.at && <span className={styles.eventTime}>{formatInstant(e.at, tz)}</span>}
          </span>
        </li>
      ))}
    </ol>
  )
}
