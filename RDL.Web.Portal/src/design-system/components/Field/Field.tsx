import { clsx } from 'clsx'
import {
  forwardRef,
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react'
import styles from './Field.module.css'

interface FieldFrameProps {
  label: string
  help?: string
  error?: string
  required?: boolean
  className?: string
  children: (ids: { id: string; describedBy: string | undefined; invalid: boolean }) => ReactNode
}

/** Marco común: etiqueta arriba, ayuda o error abajo (componentes.md). El error dice qué pasó y qué hacer. */
export function FieldFrame({ label, help, error, required, className, children }: FieldFrameProps) {
  const id = useId()
  const helpId = `${id}-help`
  const describedBy = error || help ? helpId : undefined
  return (
    <div className={clsx(styles.field, className)}>
      <label htmlFor={id} className={styles.label}>
        {label}
        {required && (
          <span className={styles.required} aria-hidden>
            {' '}
            *
          </span>
        )}
      </label>
      {children({ id, describedBy, invalid: Boolean(error) })}
      {(error || help) && (
        <span
          id={helpId}
          className={clsx(styles.help, error && styles.error)}
          role={error ? 'alert' : undefined}
        >
          {error ?? help}
        </span>
      )}
    </div>
  )
}

type Common = { label: string; help?: string; error?: string; className?: string }

export const TextField = forwardRef<HTMLInputElement, Common & InputHTMLAttributes<HTMLInputElement>>(
  function TextField({ label, help, error, required, className, ...rest }, ref) {
    return (
      <FieldFrame label={label} help={help} error={error} required={required} className={className}>
        {({ id, describedBy, invalid }) => (
          <input
            ref={ref}
            id={id}
            className={clsx(styles.control, invalid && styles.invalid)}
            aria-invalid={invalid || undefined}
            aria-describedby={describedBy}
            required={required}
            {...rest}
          />
        )}
      </FieldFrame>
    )
  },
)

export const TextArea = forwardRef<
  HTMLTextAreaElement,
  Common & TextareaHTMLAttributes<HTMLTextAreaElement> & { minLength?: number }
>(function TextArea({ label, help, error, required, className, minLength, value, ...rest }, ref) {
  const length = typeof value === 'string' ? value.length : 0
  const counter = minLength ? `${Math.min(length, minLength)}/${minLength} caracteres mínimos` : undefined
  return (
    <FieldFrame label={label} help={help ?? counter} error={error} required={required} className={className}>
      {({ id, describedBy, invalid }) => (
        <textarea
          ref={ref}
          id={id}
          className={clsx(styles.control, styles.textarea, invalid && styles.invalid)}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          required={required}
          minLength={minLength}
          value={value}
          {...rest}
        />
      )}
    </FieldFrame>
  )
})

export const Select = forwardRef<
  HTMLSelectElement,
  Common & SelectHTMLAttributes<HTMLSelectElement> & { options: { value: string; label: string }[] }
>(function Select({ label, help, error, required, className, options, ...rest }, ref) {
  return (
    <FieldFrame label={label} help={help} error={error} required={required} className={className}>
      {({ id, describedBy, invalid }) => (
        <select
          ref={ref}
          id={id}
          className={clsx(styles.control, styles.select, invalid && styles.invalid)}
          aria-invalid={invalid || undefined}
          aria-describedby={describedBy}
          required={required}
          {...rest}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      )}
    </FieldFrame>
  )
})
