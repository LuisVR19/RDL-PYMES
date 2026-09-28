import { Big } from 'big.js'
import type { Payment, Receivable } from '@/shared/api/billing-types'
import { daysOverdue } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { parseMoneyInput } from '@/shared/money/money'

/**
 * Lógica de presentación de cobranza. Nada de esto es la cifra oficial: saldo, estado y lo aplicado los calcula
 * Receivables. Aquí solo se anticipa lo que la API va a rechazar (para no mandar un pago que no cabe) y se arma lo
 * que el prototipo muestra (atraso, «Vencida», «Sin aplicar»).
 */

/** Una cuenta que todavía se puede cobrar (la base: `open` o `partially_paid`). */
export const collectable = (r: Pick<Receivable, 'status'>) =>
  r.status === 'open' || r.status === 'partially_paid'

/** Días de atraso a la fecha de negocio `today`; 0 si está al día o ya no se cobra. */
export function overdueDays(r: Pick<Receivable, 'status' | 'dueOn'>, today: string): number {
  return collectable(r) ? Math.max(0, daysOverdue(r.dueOn, today)) : 0
}

/** «1 día», «18 días». */
export const lateText = (days: number) => (days === 1 ? t('ar.day') : t('ar.days', { n: days }))

/** Estado que se muestra: «Vencida» si se puede cobrar y pasó su vencimiento (prototipo `arSt`). */
export function displayStatus(r: Pick<Receivable, 'status' | 'dueOn'>, today: string) {
  return overdueDays(r, today) > 0 ? ('overdue' as const) : r.status
}

/** Lo aplicado vigente de un pago (las aplicaciones revertidas no cuentan). */
export function appliedOf(p: Pick<Payment, 'applications'>): Big {
  return p.applications.filter((a) => !a.reversedAt).reduce((acc, a) => acc.plus(a.amount), new Big(0))
}

/** Lo que queda sin aplicar; 0 en un pago anulado (sus aplicaciones se revirtieron). */
export function unappliedOf(p: Pick<Payment, 'applications' | 'amount' | 'status'>): Big {
  return p.status === 'voided' ? new Big(0) : new Big(p.amount).minus(appliedOf(p))
}

/**
 * «Repartir automáticamente» (prototipo `autoApps`): de la cuenta más antigua a la más nueva, cada una hasta su
 * saldo, mientras alcance el pago. Devuelve el texto que va en cada campo ('' = no aplicar).
 */
export function autoDistribute(amount: string, accounts: Pick<Receivable, 'id' | 'balanceAmount'>[]) {
  let left = new Big(amount)
  const out: Record<string, string> = {}
  for (const a of accounts) {
    const take = left.gt(a.balanceAmount) ? new Big(a.balanceAmount) : left
    out[a.id] = take.gt(0) ? take.toFixed() : ''
    left = left.minus(take)
  }
  return out
}

export interface ApplicationCheck {
  /** Montos válidos por cuenta, en el formato del contrato. */
  amounts: Record<string, string>
  /** Error por cuenta: monto inválido o mayor que su saldo. */
  rowErrors: Record<string, 'invalid' | 'overBalance'>
  applied: Big
  /** Pago menos lo aplicado; negativo si se aplicó de más. */
  remaining: Big
  overPayment: boolean
  ok: boolean
}

/** Revisa lo escrito en el paso 2 contra el monto del pago y el saldo de cada cuenta. */
export function checkApplications(
  paymentAmount: string,
  accounts: Pick<Receivable, 'id' | 'balanceAmount'>[],
  typed: Record<string, string>,
): ApplicationCheck {
  const amounts: Record<string, string> = {}
  const rowErrors: ApplicationCheck['rowErrors'] = {}
  let applied = new Big(0)
  for (const a of accounts) {
    const text = (typed[a.id] ?? '').trim()
    if (text === '') continue
    const value = parseMoneyInput(text)
    if (value === null) {
      rowErrors[a.id] = 'invalid'
      continue
    }
    if (new Big(value).eq(0)) continue
    if (new Big(value).gt(a.balanceAmount)) rowErrors[a.id] = 'overBalance'
    amounts[a.id] = value
    applied = applied.plus(value)
  }
  const remaining = new Big(paymentAmount).minus(applied)
  const overPayment = remaining.lt(0)
  return {
    amounts,
    rowErrors,
    applied,
    remaining,
    overPayment,
    ok: !overPayment && Object.keys(rowErrors).length === 0,
  }
}
