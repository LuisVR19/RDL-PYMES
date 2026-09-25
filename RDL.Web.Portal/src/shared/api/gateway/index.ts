import type { Currency } from '@/shared/money/money'
import { isRole } from '@/shared/permissions/permissions'
import { ApiError, type Memberships, type Organization } from '../types'
import type { DataSource } from '../ports'
import {
  createCatalogsPort,
  createCustomersPort,
  createInvoicesPort,
  createProductsPort,
  createReceivablesPort,
} from './billing'
import type { GatewayHttp } from './http'

/**
 * Implementación de los puertos contra el Portal Gateway (`/portal/v1/...`). Las formas de las respuestas son
 * las de Platform (`openapi/platform.yaml` del repo de contratos): el gateway las reenvía sin cambios.
 */

// Platform · Me y MembershipList (solo lo que usa el portal).
interface MeDTO {
  id: string
  email: string
  fullName: string
}

interface MembershipListDTO {
  activeOrganizationId: string | null
  items: { organizationId: string; legalName: string; tradeName?: string; roles: string[] }[]
}

// Platform · Organization (GET /v1/organizations/current).
interface OrganizationDTO {
  id: string
  identificationNumber: string
  timezone: string
  defaultCurrencyCode: string
}

export function createGatewayDataSource(http: GatewayHttp): DataSource {
  return {
    session: {
      async currentUser() {
        const me = await http.get<MeDTO>('/portal/v1/me')
        return {
          id: me.id,
          email: me.email,
          fullName: me.fullName,
          initials: initialsOf(me.fullName || me.email),
        }
      },

      async organizations(): Promise<Memberships> {
        const list = await http.get<MembershipListDTO>('/portal/v1/me/memberships')
        // El detalle (zona horaria, moneda) solo existe para la organización activa: la del token.
        const detail = list.activeOrganizationId ? await currentOrganization(http) : null
        const items = list.items.flatMap(toOrganization).map((o) =>
          o.id === detail?.id
            ? {
                ...o,
                identification: detail.identificationNumber,
                timezone: detail.timezone,
                defaultCurrency: currencyOf(detail.defaultCurrencyCode),
              }
            : o,
        )
        return { items, activeOrganizationId: list.activeOrganizationId }
      },

      async activateOrganization(orgId) {
        // Única llamada que lleva un id de organización, y es la que el contrato define para eso: Platform
        // revalida la membresía (una ajena responde 404). El resto sale del token.
        await http.send('PUT', '/portal/v1/me/active-organization', { organizationId: orgId })
      },
    },

    access: {
      async createOrganization(input, idempotencyKey) {
        const org = await http.send<{ id: string }>('POST', '/portal/v1/organizations', input, {
          idempotencyKey,
        })
        return { id: org.id }
      },
      async acceptInvitation(token, idempotencyKey) {
        const r = await http.send<{ organizationId: string; role: string }>(
          'POST',
          `/portal/v1/invitations/${encodeURIComponent(token)}/accept`,
          undefined,
          { idempotencyKey },
        )
        if (!isRole(r.role)) {
          throw new ApiError({
            status: 0,
            type: 'urn:rdl:portal:problem:unknown-role',
            title: r.role,
            correlationId: '',
          })
        }
        return { organizationId: r.organizationId, role: r.role }
      },
    },

    customers: createCustomersPort(http),
    products: createProductsPort(http),
    catalogs: createCatalogsPort(http),
    receivables: createReceivablesPort(http),
    invoices: createInvoicesPort(http),

    notifications: {
      // TODO(api): /portal/v1/notifications es el incremento 6 del gateway, bloqueado por el transporte de
      // eventos (P2). Sin endpoint no se inventan avisos: la campana queda vacía.
      list: async () => [],
    },

    shell: {
      // TODO(api): el contador de la bandeja de Hacienda depende de E-Invoice (P5), que no existe. Un error hace
      // que el menú no muestre número, en vez de mostrar un 0 que diría «nada requiere atención».
      inboxAttentionCount: async () => {
        throw new ApiError({
          status: 503,
          type: 'urn:rdl:portal-gateway:problem:upstream-not-configured',
          title: 'E-Invoice todavía no está disponible',
          correlationId: '',
        })
      },
    },
  }
}

function toOrganization(m: MembershipListDTO['items'][number]): Organization[] {
  // V1 asigna un solo rol por membresía. Un rol que el portal no conoce no se adivina: la organización no se
  // ofrece hasta que el portal lo soporte.
  const role = m.roles.find(isRole)
  if (!role) return []
  const name = m.tradeName || m.legalName
  return [{ id: m.organizationId, legalName: m.legalName, initials: initialsOf(name), role }]
}

async function currentOrganization(http: GatewayHttp): Promise<OrganizationDTO | null> {
  try {
    return await http.get<OrganizationDTO>('/portal/v1/organizations/current')
  } catch (err) {
    // El token puede no traer todavía el org_id (recién creada o recién cambiada): la lista vale igual.
    if (err instanceof ApiError && err.status === 403) return null
    throw err
  }
}

/** Una moneda que el portal no sabe formatear queda sin definir: no se asume colones. */
function currencyOf(code: string): Currency | undefined {
  return code === 'CRC' || code === 'USD' ? code : undefined
}

/** Iniciales para el avatar: primeras letras de las dos primeras palabras («Comercial Los Almendros» → «CL»). */
export function initialsOf(name: string): string {
  const words = name
    .replace(/[^\p{L}\p{N}\s]/gu, ' ')
    .split(/\s+/)
    .filter(Boolean)
  return words
    .slice(0, 2)
    .map((w) => w[0]?.toLocaleUpperCase('es-CR') ?? '')
    .join('')
}
