import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useIdempotencyKey } from '../idempotency'
import { ApiError } from '../types'
import { createGatewayDataSource } from '.'
import { createGatewayHttp, type GatewayHttp } from './http'

function recordingHttp(reply: unknown) {
  const sent: { method: string; path: string; body: unknown; key?: string }[] = []
  const http: GatewayHttp = {
    get: async () => {
      throw new Error('no se espera un GET')
    },
    async send<T>(method: string, path: string, body?: unknown, opts?: { idempotencyKey?: string }) {
      sent.push({ method, path, body, key: opts?.idempotencyKey })
      return reply as T
    },
  }
  return { http, sent }
}

describe('adaptador del Portal Gateway · acceso', () => {
  it('crear organización es el POST de Platform con la clave que da la pantalla', async () => {
    const { http, sent } = recordingHttp({ id: 'org-n', legalName: 'X' })
    const input = {
      legalName: 'Panadería La Espiga S.A.',
      identificationTypeCode: '02',
      identificationNumber: '3101123456',
      email: 'ventas@espiga.cr',
      timezone: 'America/Costa_Rica',
    }
    await expect(createGatewayDataSource(http).access.createOrganization(input, 'k-1')).resolves.toEqual({
      id: 'org-n',
    })
    expect(sent).toEqual([{ method: 'POST', path: '/portal/v1/organizations', body: input, key: 'k-1' }])
  })

  it('aceptar invitación escapa el token y manda la clave', async () => {
    const { http, sent } = recordingHttp({
      organizationId: 'org-t',
      role: 'collector',
      tokenRefreshRequired: true,
    })
    const r = await createGatewayDataSource(http).access.acceptInvitation('a/b?c', 'k-2')
    expect(r).toEqual({ organizationId: 'org-t', role: 'collector' })
    expect(sent[0]).toMatchObject({
      method: 'POST',
      path: '/portal/v1/invitations/a%2Fb%3Fc/accept',
      key: 'k-2',
    })
  })

  it('un rol que el portal no conoce no se adivina', async () => {
    const { http } = recordingHttp({ organizationId: 'org-t', role: 'auditor' })
    await expect(createGatewayDataSource(http).access.acceptInvitation('t', 'k')).rejects.toBeInstanceOf(
      ApiError,
    )
  })
})

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/problem+json' } })

describe('cliente del Portal Gateway · sesión y errores', () => {
  it('un 401 del gateway avisa a la sesión (pantalla 35); un 403 no', async () => {
    const onUnauthorized = vi.fn()
    let status = 401
    const http = createGatewayHttp({
      baseUrl: 'http://gw',
      accessToken: async () => 'tok',
      onUnauthorized,
      fetch: async () => json({ type: 'urn:rdl:portal-gateway:problem:unauthenticated', title: 'x' }, status),
    })
    await expect(http.get('/portal/v1/me')).rejects.toMatchObject({ status: 401 })
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
    status = 403
    await expect(http.get('/portal/v1/me')).rejects.toMatchObject({ status: 403 })
    expect(onUnauthorized).toHaveBeenCalledTimes(1)
  })

  it('sin token también es sesión vencida', async () => {
    const onUnauthorized = vi.fn()
    const http = createGatewayHttp({ baseUrl: 'http://gw', accessToken: async () => null, onUnauthorized })
    await expect(http.get('/portal/v1/me')).rejects.toBeInstanceOf(ApiError)
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('los errores por campo de un 422 llegan a la pantalla', async () => {
    const http = createGatewayHttp({
      baseUrl: 'http://gw',
      accessToken: async () => 'tok',
      fetch: async () =>
        json(
          {
            type: 'urn:rdl:platform:problem:validation',
            title: 'Datos inválidos',
            errors: [{ field: 'email', message: 'no válido' }, { message: 'sin campo' }],
          },
          422,
        ),
    })
    const err = (await http.send('POST', '/portal/v1/organizations', {}).catch((e: unknown) => e)) as ApiError
    expect(err.errors).toEqual([{ field: 'email', message: 'no válido' }])
    expect(err.is('validation')).toBe(true)
    expect(err.is('conflict')).toBe(false)
  })
})

describe('useIdempotencyKey', () => {
  it('misma clave para el mismo envío, otra si cambió el contenido', () => {
    let n = 0
    const { result } = renderHook(() => useIdempotencyKey(() => `k-${++n}`))
    const a = result.current({ x: 1 })
    expect(result.current({ x: 1 })).toBe(a)
    const b = result.current({ x: 2 })
    expect(b).not.toBe(a)
    expect(result.current({ x: 1 })).not.toBe(a)
  })
})
