/**
 * Medios de pago para registrar un pago (pantalla 26). Receivables guarda el código (`paymentMethodCode`).
 *
 * TODO(fiscal): el catálogo es de Hacienda y el contrato lo deja como `FiscalCode` pendiente (`fiscal.payment_methods`
 * está vacío, así que Receivables solo valida el formato). FUENTE: Anexos y Estructuras v4.4, Nota 6 (docs/Hacienda,
 * página 71). «Otros» exige describir el medio en la representación gráfica: por eso va con la referencia.
 */
export const PAYMENT_METHODS = [
  { code: '04', label: 'Transferencia o depósito bancario' },
  { code: '06', label: 'SINPE Móvil' },
  { code: '01', label: 'Efectivo' },
  { code: '02', label: 'Tarjeta' },
  { code: '03', label: 'Cheque' },
  { code: '07', label: 'Plataforma digital' },
  { code: '05', label: 'Recaudado por terceros' },
  { code: '99', label: 'Otros' },
] as const

export const DEFAULT_PAYMENT_METHOD = '04'

/** Etiqueta del medio; un código que el portal no conoce se muestra tal cual. */
export function paymentMethodLabel(code: string): string {
  return PAYMENT_METHODS.find((m) => m.code === code)?.label ?? code
}
