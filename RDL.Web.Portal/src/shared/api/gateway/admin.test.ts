import { describe, expect, it } from 'vitest'
import { ApiError } from '../types'
import { createInvitationsPort, createMembersPort, createOrganizationPort } from './admin'
import { createBranchesPort, createReceivablesPort } from './billing'
import type { GatewayHttp } from './http'

function recorder(response: unknown = { items: [], nextCursor: null }) {
  const calls: { method: string; path: string; query?: unknown; body?: unknown; key?: string }[] = []
  const http: GatewayHttp = {
    async get<T>(path: string, query?: Record<string, string | undefined>) {
      calls.push({ method: 'GET', path, query })
      return response as T
    },
    async send<T>(method: string, path: string, body?: unknown, opts?: { idempotencyKey?: string }) {
      calls.push({ method, path, body, key: opts?.idempotencyKey })
      return response as T
    },
  }
  return { http, calls }
}

describe('adaptador del Portal Gateway · administración', () => {
  it('organización: GET y PATCH de la activa, sin id de organización en la ruta', async () => {
    const { http, calls } = recorder({})
    const port = createOrganizationPort(http)
    await port.current()
    await port.update({ tradeName: '', timezone: 'America/Panama' })
    expect(calls).toEqual([
      { method: 'GET', path: '/portal/v1/organizations/current', query: undefined },
      {
        method: 'PATCH',
        path: '/portal/v1/organizations/current',
        body: { tradeName: '', timezone: 'America/Panama' },
        key: undefined,
      },
    ])
  })

  it('miembros: filtros a query; un rol que el portal no conoce no se adivina', async () => {
    const member = {
      userId: 'u/1',
      email: 'a@b.cr',
      fullName: 'A',
      status: 'active',
      roles: ['auditor', 'biller'],
      joinedAt: '2026-09-01T00:00:00Z',
    }
    const { http, calls } = recorder({ items: [member], nextCursor: 'n2' })
    const port = createMembersPort(http)
    const page = await port.list({ status: 'suspended', cursor: 'c1', limit: 100 })
    expect(page).toEqual({ items: [{ ...member, roles: ['biller'] }], nextCursor: 'n2' })

    const { http: http2, calls: calls2 } = recorder(member)
    const updated = await createMembersPort(http2).update('u/1', { role: 'accountant' })
    expect(updated.roles).toEqual(['biller'])
    expect(calls).toEqual([
      {
        method: 'GET',
        path: '/portal/v1/organizations/current/users',
        query: { status: 'suspended', cursor: 'c1', limit: '100' },
      },
    ])
    expect(calls2).toEqual([
      {
        method: 'PATCH',
        path: '/portal/v1/organizations/current/users/u%2F1',
        body: { role: 'accountant' },
        key: undefined,
      },
    ])
  })

  it('invitaciones: alta con clave, revocar con DELETE; se descartan las de un rol desconocido', async () => {
    const pending = {
      id: 'i1',
      email: 'x@y.cr',
      role: 'collector',
      status: 'pending',
      createdAt: '2026-09-01T00:00:00Z',
      expiresAt: '2026-09-08T00:00:00Z',
    }
    const { http, calls } = recorder({
      items: [pending, { ...pending, id: 'i2', role: 'root' }],
      nextCursor: null,
    })
    const port = createInvitationsPort(http)
    expect((await port.list({ status: 'pending' })).items.map((i) => i.id)).toEqual(['i1'])
    await port.create({ email: 'x@y.cr', role: 'collector' }, 'k-9')
    await port.revoke('i1')
    expect(calls).toEqual([
      {
        method: 'GET',
        path: '/portal/v1/organizations/current/invitations',
        query: { status: 'pending', cursor: undefined, limit: undefined },
      },
      {
        method: 'POST',
        path: '/portal/v1/organizations/current/invitations',
        body: { email: 'x@y.cr', role: 'collector' },
        key: 'k-9',
      },
      {
        method: 'DELETE',
        path: '/portal/v1/organizations/current/invitations/i1',
        body: undefined,
        key: undefined,
      },
    ])
  })

  it('sucursales: todas las páginas por cursor, alta con clave, PATCH sin clave', async () => {
    const { http, calls } = recorder()
    const port = createBranchesPort(http)
    await port.list({ active: true })
    await port.list({ cursor: 'c2', limit: 50 })
    await port.create({ code: '003', name: 'Heredia' }, 'k-3')
    await port.update('b1', { isActive: false })
    expect(calls).toEqual([
      {
        method: 'GET',
        path: '/portal/v1/organizations/current/branches',
        query: { active: 'true', cursor: undefined, limit: '100' },
      },
      {
        method: 'GET',
        path: '/portal/v1/organizations/current/branches',
        query: { active: undefined, cursor: 'c2', limit: '50' },
      },
      {
        method: 'POST',
        path: '/portal/v1/organizations/current/branches',
        body: { code: '003', name: 'Heredia' },
        key: 'k-3',
      },
      {
        method: 'PATCH',
        path: '/portal/v1/organizations/current/branches/b1',
        body: { isActive: false },
        key: undefined,
      },
    ])
  })

  it('cobranza para Inicio: los pagos van al gateway; el resumen no existe en el contrato y no se deduce', async () => {
    const { http, calls } = recorder()
    const port = createReceivablesPort(http)
    await port.payments({ limit: 5 })
    await expect(port.summary('2026-09-24')).rejects.toBeInstanceOf(ApiError)
    expect(calls).toEqual([
      { method: 'GET', path: '/portal/v1/payments', query: { cursor: undefined, limit: '5' } },
    ])
  })
})
