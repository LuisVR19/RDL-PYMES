import * as RRadio from '@radix-ui/react-radio-group'
import { useId } from 'react'
import type { Currency } from '@/shared/money/money'
import styles from './Choice.module.css'

/**
 * Opciones excluyentes en línea (prototipo «11 Producto»: círculo de 16 px, la elegida con borde grueso de acento).
 * Sobre Radix: flechas para moverse, un solo punto de tabulación.
 */
export function RadioChoice<T extends string>({
  label,
  options,
  value,
  onChange,
  disabled,
}: {
  label: string
  options: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
  disabled?: boolean
}) {
  const id = useId()
  return (
    <div className={styles.field}>
      <span id={id} className={styles.label}>
        {label}
      </span>
      <RRadio.Root
        className={styles.radios}
        aria-labelledby={id}
        value={value}
        onValueChange={(v) => onChange(v as T)}
        disabled={disabled}
        orientation="horizontal"
      >
        {options.map((o) => (
          <label key={o.value} className={styles.option}>
            <RRadio.Item value={o.value} className={styles.radio}>
              <RRadio.Indicator className={styles.dot} />
            </RRadio.Item>
            {o.label}
          </label>
        ))}
      </RRadio.Root>
    </div>
  )
}

/**
 * Monto con su moneda (prototipo «11 Producto»: la moneda a la izquierda, el monto a la derecha con números
 * tabulares). El valor es el TEXTO que escribe el usuario; quien lo usa lo valida con `parseMoneyInput`.
 */
export function MoneyField({
  label,
  required,
  amount,
  currency,
  onAmountChange,
  onCurrencyChange,
  error,
  help,
  currencyLocked,
}: {
  label: string
  required?: boolean
  amount: string
  currency: Currency
  onAmountChange: (text: string) => void
  onCurrencyChange: (c: Currency) => void
  error?: string
  help?: string
  currencyLocked?: boolean
}) {
  const id = useId()
  const helpId = `${id}-help`
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {label}
        {required && (
          <span className={styles.required} aria-hidden>
            {' '}
            *
          </span>
        )}
      </label>
      <div className={error ? styles.moneyInvalid : styles.money}>
        <select
          aria-label={`${label}: moneda`}
          className={styles.currency}
          value={currency}
          disabled={currencyLocked}
          onChange={(e) => onCurrencyChange(e.target.value as Currency)}
        >
          <option value="CRC">₡ CRC</option>
          <option value="USD">US$ USD</option>
        </select>
        <input
          id={id}
          className={styles.amount}
          inputMode="decimal"
          placeholder="0,00"
          value={amount}
          aria-invalid={Boolean(error) || undefined}
          aria-describedby={error || help ? helpId : undefined}
          required={required}
          onChange={(e) => onAmountChange(e.target.value)}
        />
      </div>
      {(error || help) && (
        <span id={helpId} className={error ? styles.error : styles.help} role={error ? 'alert' : undefined}>
          {error ?? help}
        </span>
      )}
    </div>
  )
}
