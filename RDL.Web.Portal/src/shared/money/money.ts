import { Big } from 'big.js'

/**
 * Dinero en el portal: los montos viajan como string decimal (contrato: `Money`, hasta 5 decimales, >= 0) y nunca
 * pasan por `number`. Aquí solo se formatean para mostrar y se interpretan desde lo que el usuario escribe.
 * Los cálculos definitivos (totales de factura, saldos) los hace el servidor.
 */
export type Currency = 'CRC' | 'USD'

/** Espacio fino no separable: separador de miles del diseño (README del handoff). */
export const THIN_NBSP = ' '
/** Signo menos tipográfico para negativos (diferencias, excedentes). */
export const MINUS = '−'

const PREFIX: Record<Currency, string> = { CRC: '₡', USD: 'US$' }

// big.js: redondeo de presentación mitad hacia arriba, igual que la regla de Hacienda.
Big.RM = Big.roundHalfUp

function toBig(value: string | Big): Big {
  return value instanceof Big ? value : new Big(value)
}

function group(intPart: string): string {
  return intPart.replace(/\B(?=(\d{3})+(?!\d))/g, THIN_NBSP)
}

/**
 * `₡113 000,00`, `US$1 250,00`, `−₡5 000,00`. Muestra 2 decimales (decisión 5 del diseño); el valor exacto sigue
 * siendo el string que llegó del servidor.
 */
export function formatMoney(value: string | Big, currency: Currency, decimals = 2): string {
  const b = toBig(value)
  const sign = b.lt(0) ? MINUS : ''
  const [intPart = '0', frac = ''] = b.abs().toFixed(decimals).split('.')
  return `${sign}${PREFIX[currency]}${group(intPart)}${frac ? ',' + frac : ''}`
}

/** Monto sin símbolo de moneda, para celdas donde la moneda está en el encabezado. */
export function formatAmount(value: string | Big, decimals = 2): string {
  const b = toBig(value)
  const sign = b.lt(0) ? MINUS : ''
  const [intPart = '0', frac = ''] = b.abs().toFixed(decimals).split('.')
  return `${sign}${group(intPart)}${frac ? ',' + frac : ''}`
}

const MONEY_RE = /^(0|[1-9]\d{0,12})(\.\d{1,5})?$/

/**
 * Interpreta lo que el usuario escribe o pega en un `MoneyInput` y devuelve el string decimal del contrato
 * (punto decimal, sin separadores), o `null` si no es un monto válido.
 * Acepta «113 000,00», «113000», «₡113 000,00», «US$1 250,5» y «113.000,50» (punto como separador de miles).
 */
export function parseMoneyInput(text: string): string | null {
  let s = text.replace(/₡|US\$|\$/g, '').replace(/[\s  ]/g, '')
  if (s === '') return null
  if (s.includes(',')) {
    s = s.replace(/\./g, '').replace(',', '.')
  }
  if (!/^\d+(\.\d+)?$/.test(s)) return null
  const normalized = new Big(s).toFixed()
  return MONEY_RE.test(normalized) ? normalized : null
}

/** Suma de montos string para vistas simuladas (por ejemplo, totales por moneda de una lista). */
export function sumMoney(values: string[]): string {
  return values.reduce((acc, v) => acc.plus(v), new Big(0)).toFixed()
}
