import type { CabysItem, CatalogItem, Product, TaxOption } from '@/shared/api/billing-types'

// Productos del prototipo (PRODUCTS0). Los códigos CABYS, de unidad y de impuesto son ILUSTRATIVOS, como en el
// diseño («catálogo oficial pendiente»): TODO(fiscal) cuando E-Invoice publique los catálogos oficiales.

export const MOCK_TAX_OPTIONS: TaxOption[] = [
  { key: 'iva13', label: 'IVA 13 %', taxes: [{ taxTypeCode: '01', taxRateCode: '08' }] },
  { key: 'iva4', label: 'IVA 4 %', taxes: [{ taxTypeCode: '01', taxRateCode: '04' }] },
]

export const MOCK_UNITS: CatalogItem[] = [
  { code: 'Unid', name: 'Unidad' },
  { code: 'h', name: 'Hora' },
  { code: 'Gal', name: 'Galón' },
  { code: 'Saco', name: 'Saco' },
  { code: 'm', name: 'Metro' },
]

export const MOCK_CABYS: CabysItem[] = [
  { code: '0000000000001', description: 'Tornillos de acero para madera' },
  { code: '0000000000014', description: 'Pinturas acrílicas a base de agua' },
  { code: '0000000000027', description: 'Cemento gris hidráulico' },
  { code: '0000000000033', description: 'Tubos y accesorios de PVC' },
  { code: '0000000000048', description: 'Lijas y abrasivos' },
  { code: '0000000000102', description: 'Servicios de instalación' },
  { code: '0000000000115', description: 'Servicios de asesoría técnica' },
  { code: '0000000000121', description: 'Servicios de transporte de carga' },
  { code: '0000000000136', description: 'Cargos e intereses financieros' },
  { code: '0000000000140', description: 'Servicios de mantenimiento y reparación' },
]

const at = '2026-09-01T15:00:00Z'
const iva13 = [{ taxTypeCode: '01', taxRateCode: '08' }]

const p = (
  id: string,
  code: string,
  description: string,
  cabysCode: string,
  isService: boolean,
  unitOfMeasureCode: string,
  unitPrice: string,
  currency: 'CRC' | 'USD',
  isActive = true,
): Product => ({
  id,
  code,
  description,
  cabysCode,
  unitOfMeasureCode,
  unitPrice,
  currency,
  isService,
  isActive,
  taxes: iva13,
  createdAt: at,
  updatedAt: at,
})

export const PRODUCTS: Product[] = [
  p('p1', 'TOR-001', 'Tornillo para madera 2"', '0000000000001', false, 'Unid', '75', 'CRC'),
  p('p2', 'PIN-020', 'Pintura acrílica blanca, galón', '0000000000014', false, 'Gal', '18500', 'CRC'),
  p('p3', 'SRV-INS', 'Servicio de instalación', '0000000000102', true, 'h', '15000', 'CRC'),
  p('p4', 'CEM-050', 'Cemento gris, saco 50 kg', '0000000000027', false, 'Saco', '7900', 'CRC'),
  p('p5', 'TUB-PVC', 'Tubo PVC 1/2" × 6 m', '0000000000033', false, 'Unid', '3250', 'CRC'),
  p('p6', 'SRV-ASE', 'Asesoría técnica', '0000000000115', true, 'h', '45', 'USD'),
  p('p7', 'LIJ-080', 'Lija de agua grano 80', '0000000000048', false, 'Unid', '450', 'CRC', false),
]
