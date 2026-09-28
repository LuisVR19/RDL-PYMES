import { INVITATIONS, MEMBERS, ORGANIZATION_DETAIL } from '@/mocks/admin'
import { BRANCHES } from '@/mocks/catalogs'
import type { Invitation, Member, OrganizationDetail } from '../admin-types'
import type { Branch } from '../billing-types'
import type { BranchesPort, InvitationsPort, MembersPort, OrganizationPort } from '../ports'
import { ApiError } from '../types'
import { paginate } from './billing'
import { fakeCorrelationId, simulate, simulateSecondary } from './simulate'

/**
 * Administración simulada: imita a Platform sobre datos en memoria (409 por código de sucursal repetido o
 * invitación pendiente, 409 `last-owner`, token solo en la primera respuesta, misma clave = misma invitación).
 */
let organization: OrganizationDetail = structuredClone(ORGANIZATION_DETAIL)
let branches: Branch[] = structuredClone(BRANCHES)
let members: Member[] = structuredClone(MEMBERS)
let invitations: Invitation[] = structuredClone(INVITATIONS)
const invitationByKey = new Map<string, string>()

/** Solo pruebas: vuelve a los datos iniciales. */
export function resetMockAdmin(): void {
  organization = structuredClone(ORGANIZATION_DETAIL)
  branches = structuredClone(BRANCHES)
  members = structuredClone(MEMBERS)
  invitations = structuredClone(INVITATIONS)
  invitationByKey.clear()
}

const problem = (status: number, code: string, title: string) =>
  new ApiError({
    status,
    type: `urn:rdl:platform:problem:${code}`,
    title,
    correlationId: fakeCorrelationId(),
  })

const DAY_MS = 86_400_000

/** "" borra un opcional (convención de los PATCH de Platform). */
function withOptional<T extends object>(base: T, patch: Partial<Record<keyof T, unknown>>): T {
  const next: Record<string, unknown> = { ...(base as Record<string, unknown>) }
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined) continue
    if (v === '') delete next[k]
    else next[k] = v
  }
  return next as T
}

export const mockOrganization: OrganizationPort = {
  current: () => simulate(structuredClone(organization), structuredClone(organization)),
  async update(patch) {
    await simulate(null, null)
    if (patch.email !== undefined && !/^\S+@\S+\.\S+$/.test(patch.email)) {
      throw new ApiError({
        status: 422,
        type: 'urn:rdl:platform:problem:validation',
        title: 'Datos inválidos',
        correlationId: fakeCorrelationId(),
        errors: [{ field: 'email', message: 'El correo no es válido.' }],
      })
    }
    organization = { ...withOptional(organization, patch), updatedAt: new Date().toISOString() }
    return structuredClone(organization)
  },
}

export const mockBranches: BranchesPort = {
  // En el borrador las sucursales son secundarias: en «datos parciales» fallan solas.
  list: (q) =>
    simulateSecondary(
      paginate(
        branches.filter((b) => q?.active === undefined || b.isActive === q.active),
        { cursor: q?.cursor, limit: q?.limit ?? 100 },
      ),
      { items: [], nextCursor: null },
    ),
  async create(input) {
    await simulate(null, null)
    if (branches.some((b) => b.code === input.code)) {
      throw problem(409, 'conflict', 'Ya existe una sucursal con ese código')
    }
    const created: Branch = { id: `b${Date.now()}`, ...input, isActive: true }
    branches = [...branches, created]
    return structuredClone(created)
  },
  async update(id, patch) {
    await simulate(null, null)
    const current = branches.find((b) => b.id === id)
    if (!current) throw problem(404, 'not-found', 'Recurso no encontrado')
    const next = withOptional(current, patch)
    branches = branches.map((b) => (b.id === id ? next : b))
    return structuredClone(next)
  },
}

export const mockMembers: MembersPort = {
  list: (q) =>
    simulate(
      paginate(
        members.filter((m) => !q?.status || m.status === q.status),
        q,
      ),
      { items: [], nextCursor: null },
    ),
  async update(userId, patch) {
    await simulate(null, null)
    const current = members.find((m) => m.userId === userId)
    if (!current) throw problem(404, 'not-found', 'Recurso no encontrado')
    const next: Member =
      'role' in patch ? { ...current, roles: [patch.role] } : { ...current, status: patch.status }
    const activeOwners = members
      .map((m) => (m.userId === userId ? next : m))
      .filter((m) => m.status === 'active' && m.roles.includes('owner'))
    if (activeOwners.length === 0) {
      throw problem(409, 'last-owner', 'La organización debe tener al menos un propietario activo')
    }
    members = members.map((m) => (m.userId === userId ? next : m))
    return structuredClone(next)
  },
}

export const mockInvitations: InvitationsPort = {
  list: (q) =>
    simulate(
      paginate(
        invitations.filter((i) => !q?.status || i.status === q.status),
        q,
      ),
      { items: [], nextCursor: null },
    ),
  async create(input, idempotencyKey) {
    await simulate(null, null)
    // Misma clave: la misma invitación, ya sin token (Platform guarda solo su hash).
    const repeated = invitations.find((i) => i.id === invitationByKey.get(idempotencyKey))
    if (repeated) return structuredClone(repeated)
    const email = input.email.trim().toLowerCase()
    const taken =
      members.some((m) => m.status === 'active' && m.email === email) ||
      invitations.some((i) => i.status === 'pending' && i.email === email)
    if (taken) throw problem(409, 'conflict', 'Ya es miembro o tiene una invitación pendiente')
    const now = Date.now()
    const created: Invitation = {
      id: `i${now}`,
      email,
      role: input.role,
      status: 'pending',
      createdAt: new Date(now).toISOString(),
      expiresAt: new Date(now + 7 * DAY_MS).toISOString(),
    }
    invitations = [created, ...invitations]
    invitationByKey.set(idempotencyKey, created.id)
    return { ...structuredClone(created), token: `demo${now.toString(36)}` }
  },
  async revoke(id) {
    await simulate(null, null)
    const current = invitations.find((i) => i.id === id)
    if (!current) throw problem(404, 'not-found', 'Recurso no encontrado')
    if (current.status === 'accepted') throw problem(409, 'conflict', 'La invitación ya fue aceptada')
    if (current.status === 'pending') {
      invitations = invitations.map((i) => (i.id === id ? { ...i, status: 'revoked' } : i))
    }
  },
}
