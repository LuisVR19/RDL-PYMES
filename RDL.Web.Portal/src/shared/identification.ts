/**
 * Tipos de identificación que ofrece el portal al crear una organización (pantalla 3).
 *
 * TODO(fiscal): el catálogo es de Hacienda y el contrato lo deja pendiente (`schemas/common/identification.json`,
 * `fiscal.identification_types` vacío). FUENTE: borrador de la resolución MH-DGT-RES-000-2024, Anexo 1, Nota 4
 * (docs/Hacienda, página 74). Los códigos 05 «Extranjero No Domiciliado» y 06 «No Contribuyente» solo aplican a la
 * factura de compra y no se ofrecen aquí. Cuando el catálogo llegue al contrato, esta lista se reemplaza.
 */
export const IDENTIFICATION_TYPES = [
  { code: '02', label: 'Jurídica' },
  { code: '01', label: 'Física' },
  { code: '03', label: 'DIMEX' },
  { code: '04', label: 'NITE' },
] as const

export function identificationLabel(code: string): string | undefined {
  return IDENTIFICATION_TYPES.find((t) => t.code === code)?.label
}
