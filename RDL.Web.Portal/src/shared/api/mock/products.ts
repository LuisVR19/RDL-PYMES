import { MOCK_CABYS, MOCK_TAX_OPTIONS, MOCK_UNITS, PRODUCTS } from '@/mocks/catalogs'
import type { Product } from '../billing-types'
import type { CatalogsPort, ProductsPort } from '../ports'
import { ApiError } from '../types'
import { paginate } from './billing'
import { fakeCorrelationId, simulate, simulateSecondary } from './simulate'

/** Productos simulados: imitan a Billing (búsqueda por código o descripción, 404, 409 por código repetido). */
let products: Product[] = structuredClone(PRODUCTS)

/** Solo pruebas: vuelve a los datos iniciales. */
export function resetMockProducts(): void {
  products = structuredClone(PRODUCTS)
}

/** Solo lectura, para otros simulados (las líneas de un borrador copian el producto). */
export function mockProductById(id: string): Product | undefined {
  return products.find((x) => x.id === id)
}

const problem = (status: number, code: string, title: string) =>
  new ApiError({ status, type: `urn:rdl:billing:problem:${code}`, title, correlationId: fakeCorrelationId() })

const sameCode = (a: string, b: string) => a.toLocaleLowerCase('es-CR') === b.toLocaleLowerCase('es-CR')

export const mockProducts: ProductsPort = {
  async list(q) {
    const text = q.q?.trim().toLocaleLowerCase('es-CR')
    const matches = products.filter(
      (x) =>
        (q.active === undefined || x.isActive === q.active) &&
        (!text || `${x.code} ${x.description}`.toLocaleLowerCase('es-CR').includes(text)),
    )
    return simulate(paginate(matches, q), { items: [], nextCursor: null })
  },
  async get(id) {
    await simulate(null, null)
    const found = products.find((x) => x.id === id)
    if (!found) throw problem(404, 'not-found', 'Recurso no encontrado')
    return structuredClone(found)
  },
  async create(input) {
    await simulate(null, null)
    if (products.some((x) => sameCode(x.code, input.code))) {
      throw problem(409, 'product-code-taken', 'Código repetido')
    }
    const now = new Date().toISOString()
    const created: Product = {
      id: `p${Date.now()}`,
      ...input,
      isActive: true,
      createdAt: now,
      updatedAt: now,
    }
    products = [created, ...products]
    return structuredClone(created)
  },
  async update(id, patch) {
    await simulate(null, null)
    const current = products.find((x) => x.id === id)
    if (!current) throw problem(404, 'not-found', 'Recurso no encontrado')
    const code = patch.code
    if (code && products.some((x) => x.id !== id && sameCode(x.code, code))) {
      throw problem(409, 'product-code-taken', 'Código repetido')
    }
    const next: Product = { ...current, ...patch, updatedAt: new Date().toISOString() }
    products = products.map((x) => (x.id === id ? next : x))
    return structuredClone(next)
  },
}

// Catálogos ilustrativos (los del prototipo): el oficial es de E-Invoice, que no existe todavía.
export const mockCatalogs: CatalogsPort = {
  searchCabys: (q) => {
    const text = q.trim().toLocaleLowerCase('es-CR')
    return simulateSecondary(
      MOCK_CABYS.filter((c) => `${c.code} ${c.description}`.toLocaleLowerCase('es-CR').includes(text)),
      [],
    )
  },
  unitsOfMeasure: () => simulateSecondary(MOCK_UNITS, []),
  taxOptions: () => simulateSecondary(MOCK_TAX_OPTIONS, []),
}
