import type { ProductTax, TaxOption } from '@/shared/api/billing-types'
import { t } from '@/shared/i18n/t'

const sameTaxes = (a: ProductTax[], b: ProductTax[]) =>
  a.length === b.length &&
  a.every((x) => b.some((y) => y.taxTypeCode === x.taxTypeCode && y.taxRateCode === x.taxRateCode))

/**
 * Nombre del impuesto de un producto. Con el catálogo, su nombre; sin catálogo, los códigos tal cual (no se
 * adivina qué tarifa es); sin impuestos, «Sin impuesto».
 */
export function taxLabel(taxes: ProductTax[], options: TaxOption[] | undefined): string {
  if (taxes.length === 0) return t('products.tax.none')
  const known = options?.find((o) => sameTaxes(o.taxes, taxes))
  return known?.label ?? taxes.map((x) => `${x.taxTypeCode}/${x.taxRateCode}`).join(', ')
}

/** La opción del formulario que corresponde a los impuestos de un producto (`none` = sin impuesto). */
export function taxOptionKey(taxes: ProductTax[], options: TaxOption[] | undefined): string {
  if (taxes.length === 0) return 'none'
  return options?.find((o) => sameTaxes(o.taxes, taxes))?.key ?? 'current'
}
