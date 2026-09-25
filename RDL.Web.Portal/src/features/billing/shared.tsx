import { useEffect, useState } from 'react'
import type { Identification } from '@/shared/api/billing-types'
import { identificationLabel } from '@/shared/identification'
import { formatMoney, sumMoney, type Currency } from '@/shared/money/money'
import styles from './shared.module.css'

/** «Jurídica 3-101-900412»: el tipo con su nombre si el portal lo conoce, si no el código tal cual. */
export function identificationText(id: Identification): string {
  return `${identificationLabel(id.typeCode) ?? id.typeCode} ${id.number}`
}

/**
 * Suma por moneda, sin convertir nunca de una a otra: colones y dólares son dos totales distintos. Es solo para
 * mostrar (la cifra oficial de cada documento la da su API dueña).
 */
export function totalsByCurrency(
  items: { currency: Currency; amount: string }[],
): Partial<Record<Currency, string>> {
  const out: Partial<Record<Currency, string>> = {}
  for (const cur of ['CRC', 'USD'] as const) {
    const amounts = items.filter((i) => i.currency === cur).map((i) => i.amount)
    if (amounts.length > 0) out[cur] = sumMoney(amounts)
  }
  return out
}

/** Montos por moneda uno debajo del otro (prototipo: «₡63 000,00» y abajo «US$1 480,00»). */
export function MoneyByCurrency({
  totals,
  zero = 'CRC',
}: {
  totals: Partial<Record<Currency, string>>
  /** Qué mostrar si no hay ningún monto. */
  zero?: Currency
}) {
  const lines = (['CRC', 'USD'] as const).filter((c) => totals[c] !== undefined)
  if (lines.length === 0) return <span className={styles.muted}>{formatMoney('0', zero)}</span>
  return (
    <span className={styles.stack}>
      {lines.map((c) => (
        <span key={c}>{formatMoney(totals[c] ?? '0', c)}</span>
      ))}
    </span>
  )
}

/** Espera a que el usuario deje de escribir antes de buscar: una llamada por búsqueda, no una por tecla. */
export function useDebouncedValue<T>(value: T, ms = 300): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(id)
  }, [value, ms])
  return v
}
