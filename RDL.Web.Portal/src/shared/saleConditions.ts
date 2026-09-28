/**
 * Condiciones de venta que ofrece el borrador de factura (pantalla 13).
 *
 * TODO(fiscal): el catálogo es de Hacienda y el contrato lo deja como `FiscalCode` pendiente (ADR 0007 de contratos,
 * `fiscal.*` vacío). FUENTE: borrador de la resolución MH-DGT-RES-000-2024, Anexo 1, Nota 5 (docs/Hacienda,
 * página 66). El diseño solo ofrece «Contado» y «Crédito»; los demás códigos (consignación, apartado,
 * arrendamientos, ventas al Estado…) se agregan cuando el catálogo oficial llegue al contrato.
 */
export const SALE_CONDITIONS = [
  { code: '01', label: 'Contado' },
  { code: '02', label: 'Crédito' },
] as const

export const CASH = '01'
export const CREDIT = '02'

export function saleConditionLabel(code: string): string | undefined {
  return SALE_CONDITIONS.find((c) => c.code === code)?.label
}
