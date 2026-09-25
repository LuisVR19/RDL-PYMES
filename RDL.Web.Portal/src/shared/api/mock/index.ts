import { INVITED_ORGANIZATION, NOTIFICATIONS, ORGANIZATIONS, USER } from '@/mocks/session'
import type { DataSource } from '../ports'
import { ApiError, type Organization } from '../types'
import { mockCustomers, mockReceivables } from './billing'
import { mockInvoices } from './invoices'
import { mockCatalogs, mockProducts } from './products'
import { fakeCorrelationId, simulate } from './simulate'

// Las organizaciones simuladas cambian durante la revisión (crear una, aceptar una invitación).
let organizations: Organization[] = [...ORGANIZATIONS]

/** Solo pruebas: vuelve a la lista inicial. */
export function resetMockOrganizations(): void {
  organizations = [...ORGANIZATIONS]
}

const problem = (status: number, code: string, title: string) =>
  new ApiError({
    status,
    type: `urn:rdl:platform:problem:${code}`,
    title,
    correlationId: fakeCorrelationId(),
  })

/**
 * Tokens de invitación de la barra de revisión (pantalla 4): `vencida`, `no-pendiente` y `ajena` reproducen las
 * respuestas de Platform (410, 409 y 404); cualquier otro la acepta.
 */
async function acceptMockInvitation(token: string) {
  await simulate(null, null)
  if (token === 'vencida') throw problem(410, 'invitation-expired', 'Invitación vencida')
  if (token === 'no-pendiente')
    throw problem(409, 'invitation-not-pending', 'La invitación ya no está pendiente')
  if (token === 'ajena') throw problem(404, 'not-found', 'Recurso no encontrado')
  if (!organizations.some((o) => o.id === INVITED_ORGANIZATION.id))
    organizations = [INVITED_ORGANIZATION, ...organizations]
  return { organizationId: INVITED_ORGANIZATION.id, role: INVITED_ORGANIZATION.role }
}

/** Implementación simulada de los puertos: datos en memoria del prototipo de diseño. */
export const mockDataSource: DataSource = {
  session: {
    currentUser: async () => USER,
    // La sesión no sigue el escenario de revisión: así una lista en «error» no tumba el armazón. El selector de
    // organización (pantalla 2) tendrá su propio método con estados cuando se construya.
    organizations: async () => ({ items: organizations, activeOrganizationId: 'ca' }),
    activateOrganization: async () => {},
  },
  access: {
    async createOrganization(input) {
      await simulate(null, null)
      const id = `nueva-${organizations.length + 1}`
      const words = (input.tradeName || input.legalName).trim().split(/\s+/)
      const initials = words
        .slice(0, 2)
        .map((w) => w[0]?.toUpperCase() ?? '')
        .join('')
      organizations = [
        {
          id,
          legalName: input.legalName,
          initials,
          role: 'owner',
          timezone: input.timezone,
          defaultCurrency: 'CRC',
        },
        ...organizations,
      ]
      return { id }
    },
    acceptInvitation: (token) => acceptMockInvitation(token),
  },
  customers: mockCustomers,
  products: mockProducts,
  catalogs: mockCatalogs,
  receivables: mockReceivables,
  invoices: mockInvoices,
  notifications: {
    list: () => simulate(NOTIFICATIONS, []),
  },
  shell: {
    inboxAttentionCount: () => simulate(3, 0),
  },
}
