import { isRole } from '@/shared/permissions/permissions'
import type { Invitation, Member, OrganizationDetail } from '../admin-types'
import type { Page } from '../billing-types'
import type { InvitationsPort, MembersPort, OrganizationPort } from '../ports'
import type { GatewayHttp } from './http'

/**
 * Administración contra el Portal Gateway: paso directo a Platform (`/v1/organizations/current/...`). Nada lleva
 * el id de la organización: Platform la toma del token.
 */

const num = (v: number | undefined) => (v === undefined ? undefined : String(v))

interface MemberDTO extends Omit<Member, 'roles'> {
  roles: string[]
}

interface InvitationDTO extends Omit<Invitation, 'role'> {
  role: string
}

export function createOrganizationPort(http: GatewayHttp): OrganizationPort {
  return {
    current: () => http.get<OrganizationDetail>('/portal/v1/organizations/current'),
    update: (patch) => http.send<OrganizationDetail>('PATCH', '/portal/v1/organizations/current', patch),
  }
}

export function createMembersPort(http: GatewayHttp): MembersPort {
  return {
    async list(q) {
      const page = await http.get<Page<MemberDTO>>('/portal/v1/organizations/current/users', {
        status: q?.status,
        cursor: q?.cursor,
        limit: num(q?.limit),
      })
      return { items: page.items.map(toMember), nextCursor: page.nextCursor }
    },
    async update(userId, patch) {
      const m = await http.send<MemberDTO>(
        'PATCH',
        `/portal/v1/organizations/current/users/${encodeURIComponent(userId)}`,
        patch,
      )
      return toMember(m)
    },
  }
}

export function createInvitationsPort(http: GatewayHttp): InvitationsPort {
  return {
    async list(q) {
      const page = await http.get<Page<InvitationDTO>>('/portal/v1/organizations/current/invitations', {
        status: q?.status,
        cursor: q?.cursor,
        limit: num(q?.limit),
      })
      return { items: page.items.flatMap(toInvitation), nextCursor: page.nextCursor }
    },
    async create(input, idempotencyKey) {
      const inv = await http.send<InvitationDTO>(
        'POST',
        '/portal/v1/organizations/current/invitations',
        input,
        { idempotencyKey },
      )
      // Se crea con un rol que el portal envió: siempre lo conoce.
      return { ...inv, role: input.role }
    },
    revoke: (id) =>
      http.send<void>('DELETE', `/portal/v1/organizations/current/invitations/${encodeURIComponent(id)}`),
  }
}

/** Un rol que el portal no conoce no se adivina: el miembro queda sin rol y la pantalla no deja cambiarlo. */
function toMember(m: MemberDTO): Member {
  return { ...m, roles: m.roles.filter(isRole) }
}

function toInvitation(i: InvitationDTO): Invitation[] {
  const role = i.role
  return isRole(role) ? [{ ...i, role }] : []
}
