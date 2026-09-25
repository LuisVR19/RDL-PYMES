import { describe, expect, it } from 'vitest'
import { ApiError } from '../types'
import { createGatewayDataSource, initialsOf } from '.'
import type { GatewayHttp } from './http'

type Routes = Record<string, unknown | (() => unknown)>

/** GatewayHttp falso: responde por ruta y registra los comandos. */
function fakeHttp(routes: Routes) {
  const sent: { method: string; path: string; body: unknown }[] = []
  const http: GatewayHttp = {
    async get<T>(path: string) {
      const r = routes[path]
      if (r === undefined) throw new Error(`ruta no simulada: ${path}`)
      return (typeof r === 'function' ? r() : r) as T
    },
    async send<T>(method: string, path: string, body?: unknown) {
      sent.push({ method, path, body })
      return undefined as T
    },
  }
  return { http, sent }
}

const memberships = {
  activeOrganizationId: 'org-a',
  items: [
    { organizationId: 'org-a', legalName: 'Comercial Los Almendros S.A.', roles: ['admin'] },
    {
      organizationId: 'org-b',
      legalName: 'Café Monteazul S.A.',
      tradeName: 'Monteazul',
      roles: ['accountant'],
    },
    { organizationId: 'org-x', legalName: 'Rol nuevo S.A.', roles: ['auditor'] },
  ],
}
const current = {
  id: 'org-a',
  identificationNumber: '3101900001',
  timezone: 'America/Costa_Rica',
  defaultCurrencyCode: 'CRC',
}

describe('adaptador del Portal Gateway · sesión', () => {
  it('arma las organizaciones con el detalle de la activa', async () => {
    const { http } = fakeHttp({
      '/portal/v1/me/memberships': memberships,
      '/portal/v1/organizations/current': current,
    })
    const r = await createGatewayDataSource(http).session.organizations()

    expect(r.activeOrganizationId).toBe('org-a')
    expect(r.items.map((o) => o.id)).toEqual(['org-a', 'org-b'])
    expect(r.items[0]).toMatchObject({
      role: 'admin',
      timezone: 'America/Costa_Rica',
      defaultCurrency: 'CRC',
      initials: 'CL',
    })
    // De las demás no se sabe la zona ni la moneda: no se inventan.
    expect(r.items[1]).toEqual({
      id: 'org-b',
      legalName: 'Café Monteazul S.A.',
      initials: 'M',
      role: 'accountant',
    })
    // El ambiente de Hacienda sale de E-Invoice, que no existe: tampoco se inventa.
    expect(r.items[0]?.environment).toBeUndefined()
  })

  it('un rol que el portal no conoce no se adivina: esa organización no se ofrece', async () => {
    const { http } = fakeHttp({
      '/portal/v1/me/memberships': memberships,
      '/portal/v1/organizations/current': current,
    })
    const r = await createGatewayDataSource(http).session.organizations()
    expect(r.items.find((o) => o.id === 'org-x')).toBeUndefined()
  })

  it('si el token todavía no trae la organización (403), la lista vale igual', async () => {
    const { http } = fakeHttp({
      '/portal/v1/me/memberships': memberships,
      '/portal/v1/organizations/current': () => {
        throw new ApiError({
          status: 403,
          type: 'urn:rdl:platform:problem:no-active-organization',
          title: '',
          correlationId: '',
        })
      },
    })
    const r = await createGatewayDataSource(http).session.organizations()
    expect(r.items).toHaveLength(2)
    expect(r.items[0]?.timezone).toBeUndefined()
  })

  it('sin organización activa no pide el detalle', async () => {
    const { http } = fakeHttp({ '/portal/v1/me/memberships': { activeOrganizationId: null, items: [] } })
    await expect(createGatewayDataSource(http).session.organizations()).resolves.toEqual({
      items: [],
      activeOrganizationId: null,
    })
  })

  it('cambiar de organización es el PUT del contrato de Platform', async () => {
    const { http, sent } = fakeHttp({})
    await createGatewayDataSource(http).session.activateOrganization('org-b')
    expect(sent).toEqual([
      { method: 'PUT', path: '/portal/v1/me/active-organization', body: { organizationId: 'org-b' } },
    ])
  })

  it('el usuario sale de /me', async () => {
    const { http } = fakeHttp({
      '/portal/v1/me': { id: 'u1', email: 'a@b.cr', fullName: 'María Rojas Vargas' },
    })
    await expect(createGatewayDataSource(http).session.currentUser()).resolves.toEqual({
      id: 'u1',
      email: 'a@b.cr',
      fullName: 'María Rojas Vargas',
      initials: 'MR',
    })
  })

  it('lo que el gateway todavía no sirve no se inventa', async () => {
    const ds = createGatewayDataSource(fakeHttp({}).http)
    await expect(ds.notifications.list('org-a')).resolves.toEqual([])
    await expect(ds.shell.inboxAttentionCount('org-a')).rejects.toBeInstanceOf(ApiError)
  })
})

describe('initialsOf', () => {
  it.each([
    ['Comercial Los Almendros S.A.', 'CL'],
    ['Ñandú', 'Ñ'],
    ['  soluciones   ibis ', 'SI'],
    ['', ''],
  ])('%s → %s', (name, want) => expect(initialsOf(name)).toBe(want))
})
