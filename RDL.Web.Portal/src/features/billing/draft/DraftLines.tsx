import { clsx } from 'clsx'
import { X } from 'lucide-react'
import { Button } from '@/design-system/components/Button/Button'
import type { Invoice, InvoiceLine, Product } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'
import { formatMoney, type Currency } from '@/shared/money/money'
import { lineFromProduct, type DraftLine, type LineField, type LineIssues } from './model'
import { ProductPicker } from './ProductPicker'
import styles from './draft.module.css'

/**
 * Líneas del borrador (prototipo «13», panel «Líneas»): producto del catálogo, cantidad, precio, descuento y, si hay
 * descuento, su motivo obligatorio. El impuesto y el total de cada línea son los que devolvió Billing: mientras la
 * línea no se guardó, se ve «—».
 */
export function DraftLines({
  lines,
  currency,
  saved,
  stale,
  issues,
  onChange,
  title = t('draft.lines'),
  hint = t('draft.lines.hint'),
}: {
  lines: DraftLine[]
  currency: Currency
  saved: Invoice | null
  /** Hay cambios que Billing todavía no recalculó: los totales de línea se atenúan. */
  stale: boolean
  issues: Record<string, LineIssues>
  onChange: (lines: DraftLine[]) => void
  title?: string
  hint?: string
}) {
  const update = (key: string, field: LineField, value: string) =>
    onChange(lines.map((l) => (l.key === key ? { ...l, [field]: value } : l)))
  const add = (p: Product) => onChange([...lines, lineFromProduct(p, currency)])
  const remove = (key: string) => onChange(lines.filter((l) => l.key !== key))

  return (
    <section className={styles.card} aria-labelledby="draft-lines-title">
      <div className={styles.cardHead}>
        <h2 id="draft-lines-title" className={styles.cardTitle}>
          {title}
        </h2>
        <span className={styles.muted}>{hint}</span>
      </div>
      {/* Se desplaza de lado en pantallas angostas: enfocable para moverla con el teclado (WCAG 2.1.1). */}
      <div className={styles.linesScroll} role="region" aria-label={title} tabIndex={0}>
        <table className={styles.lines}>
          {/* Anchos del prototipo: minmax(200px, 1fr) 76 120 72 110 118 32. */}
          <colgroup>
            <col />
            <col className={styles.colQty} />
            <col className={styles.colPrice} />
            <col className={styles.colDiscount} />
            <col className={styles.colTax} />
            <col className={styles.colTotal} />
            <col className={styles.colRemove} />
          </colgroup>
          <thead>
            <tr>
              <th scope="col">{t('draft.col.product')}</th>
              <th scope="col" className={styles.num}>
                {t('draft.col.quantity')}
              </th>
              <th scope="col" className={styles.num}>
                {t('draft.col.price')}
              </th>
              <th scope="col" className={styles.num}>
                {t('draft.col.discount')}
              </th>
              <th scope="col">{t('draft.col.tax')}</th>
              <th scope="col" className={styles.num}>
                {t('draft.col.total')}
              </th>
              <th scope="col">
                <span className="sr-only">{t('draft.removeLine', { n: '' })}</span>
              </th>
            </tr>
          </thead>
          {lines.map((l, i) => {
            const server = serverLine(saved, l, i)
            const err = issues[l.key] ?? {}
            return (
              <tbody key={l.key} className={styles.lineGroup}>
                <tr>
                  <td>
                    <span className={styles.lineDesc}>{l.description}</span>
                    <span className={styles.mono}>
                      {l.code ? `${l.code} · ` : ''}CABYS {l.cabys}
                    </span>
                  </td>
                  <td className={styles.num}>
                    <CellInput
                      label={`${t('draft.col.quantity')} · ${t('draft.issue.line', { n: i + 1, msg: l.description })}`}
                      value={l.quantity}
                      error={err.quantity}
                      onChange={(v) => update(l.key, 'quantity', v)}
                    />
                  </td>
                  <td className={styles.num}>
                    <CellInput
                      label={`${t('draft.col.price')} · ${t('draft.issue.line', { n: i + 1, msg: l.description })}`}
                      value={l.unitPrice}
                      error={err.unitPrice}
                      onChange={(v) => update(l.key, 'unitPrice', v)}
                    />
                  </td>
                  <td className={styles.num}>
                    <CellInput
                      label={`${t('draft.col.discount')} · ${t('draft.issue.line', { n: i + 1, msg: l.description })}`}
                      value={l.discount}
                      placeholder="0"
                      error={err.discount}
                      onChange={(v) => update(l.key, 'discount', v)}
                    />
                  </td>
                  <td>{server ? taxText(server) : '—'}</td>
                  <td className={clsx(styles.num, styles.lineTotal, stale && styles.dimmedText)}>
                    {server ? formatMoney(server.total, currency) : '—'}
                  </td>
                  <td>
                    <Button
                      variant="icon"
                      aria-label={t('draft.removeLine', { n: i + 1 })}
                      onClick={() => remove(l.key)}
                    >
                      <X size={16} aria-hidden />
                    </Button>
                  </td>
                </tr>
                {(l.discount.trim() !== '' && l.discount.trim() !== '0') || err.discountReason ? (
                  <tr>
                    <td colSpan={7} className={styles.reasonCell}>
                      <label className={styles.reasonRow}>
                        <span className={styles.reasonLabel}>
                          {t('draft.discountReason')} <span className={styles.required}>*</span>
                        </span>
                        <input
                          className={clsx(
                            styles.control,
                            styles.reasonInput,
                            err.discountReason && styles.invalid,
                          )}
                          placeholder={t('draft.discountReason.placeholder')}
                          maxLength={80}
                          aria-invalid={err.discountReason ? true : undefined}
                          value={l.discountReason}
                          onChange={(e) => update(l.key, 'discountReason', e.target.value)}
                        />
                        {err.discountReason && (
                          <span className={styles.error} role="alert">
                            ✕ {err.discountReason}
                          </span>
                        )}
                      </label>
                    </td>
                  </tr>
                ) : null}
                {(err.quantity || err.unitPrice || err.discount) && (
                  <tr>
                    <td colSpan={7} className={styles.reasonCell}>
                      <span className={styles.error} role="alert">
                        ✕ {err.quantity ?? err.unitPrice ?? err.discount}
                      </span>
                    </td>
                  </tr>
                )}
              </tbody>
            )
          })}
        </table>
      </div>
      {lines.length === 0 && <p className={styles.empty}>{t('draft.lines.empty')}</p>}
      <div className={styles.linesFoot}>
        <ProductPicker onPick={add} />
      </div>
    </section>
  )
}

function CellInput({
  label,
  value,
  error,
  placeholder,
  onChange,
}: {
  label: string
  value: string
  error?: string
  placeholder?: string
  onChange: (v: string) => void
}) {
  return (
    <input
      className={clsx(styles.control, styles.cell, error && styles.invalid)}
      aria-label={label}
      aria-invalid={error ? true : undefined}
      inputMode="decimal"
      autoComplete="off"
      placeholder={placeholder}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

/** La línea que calculó Billing para esta fila (mismo orden, mismo producto), si ya se guardó. */
function serverLine(saved: Invoice | null, l: DraftLine, i: number): InvoiceLine | undefined {
  const s = saved?.lines[i]
  return s && s.productId === l.productId ? s : undefined
}

function taxText(l: InvoiceLine): string {
  return l.taxes.length === 0 ? '—' : l.taxes.map((x) => `${x.rate.replace('.', ',')} %`).join(', ')
}
