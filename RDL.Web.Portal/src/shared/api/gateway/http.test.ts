import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../types'
import { createGatewayHttp, PORTAL_PROBLEM } from './http'

function setup(
  respond: (url: string, init: RequestInit) => Response | Promise<Response>,
  token: string | null = 'tok',
) {
  const calls: { url: string; init: RequestInit }[] = []
  let n = 0
  const http = createGatewayHttp({
    baseUrl: 'http://gw.test',
    accessToken: async () => token,
    newId: () => `cid-${++n}`,
    fetch: async (input, init) => {
      const url = String(input)
      calls.push({ url, init: init ?? {} })
      return respond(url, init ?? {})
    },
  })
  const headers = (i = 0) => new Headers(calls[i]?.init.headers)
  return { http, calls, headers }
}

const json = (body: unknown, status = 200, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })

describe('cliente del Portal Gateway', () => {
  it('manda el token del usuario y un X-Correlation-Id nuevo por llamada', async () => {
    const { http, calls, headers } = setup(() => json({ ok: true }))
    await http.get('/portal/v1/me')
    await http.get('/portal/v1/me')
    expect(calls[0]?.url).toBe('http://gw.test/portal/v1/me')
    expect(headers(0).get('Authorization')).toBe('Bearer tok')
    expect(headers(0).get('X-Correlation-Id')).toBe('cid-1')
    expect(headers(1).get('X-Correlation-Id')).toBe('cid-2')
    expect(headers(0).get('Idempotency-Key')).toBeNull()
  })

  it('nunca agrega una organización: ni en la query ni en un header', async () => {
    const { http, calls, headers } = setup(() => json({}))
    await http.get('/portal/v1/invoices', { status: 'issued', limit: undefined })
    expect(calls[0]?.url).toBe('http://gw.test/portal/v1/invoices?status=issued')
    for (const [k] of headers(0)) expect(k.toLowerCase()).not.toContain('organization')
  })

  it('manda la Idempotency-Key que le da quien llama, y el cuerpo como JSON', async () => {
    const { http, calls, headers } = setup(() => json({ id: 'x' }, 201))
    await http.send('POST', '/portal/v1/customers', { legalName: 'A' }, { idempotencyKey: 'k-1' })
    expect(calls[0]?.init.method).toBe('POST')
    expect(headers(0).get('Idempotency-Key')).toBe('k-1')
    expect(headers(0).get('Content-Type')).toBe('application/json')
    expect(calls[0]?.init.body).toBe('{"legalName":"A"}')
  })

  it('convierte un Problem Details en ApiError con su type y su código de referencia', async () => {
    const { http } = setup(() =>
      json(
        {
          type: 'urn:rdl:platform:problem:not-found',
          title: 'No existe',
          status: 404,
          correlationId: 'cid-srv',
        },
        404,
      ),
    )
    const err = await http.get('/portal/v1/x').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({
      status: 404,
      type: 'urn:rdl:platform:problem:not-found',
      correlationId: 'cid-srv',
    })
  })

  it('sin cuerpo legible usa el X-Correlation-Id de la respuesta', async () => {
    const { http } = setup(
      () => new Response('oops', { status: 502, headers: { 'X-Correlation-Id': 'cid-hdr' } }),
    )
    await expect(http.get('/portal/v1/x')).rejects.toMatchObject({ status: 502, correlationId: 'cid-hdr' })
  })

  it('red caída → ApiError de red con el código que se mandó', async () => {
    const { http } = setup(() => {
      throw new TypeError('Failed to fetch')
    })
    await expect(http.get('/portal/v1/x')).rejects.toMatchObject({
      status: 0,
      type: PORTAL_PROBLEM.network,
      correlationId: 'cid-1',
    })
  })

  it('sin sesión no llama al gateway', async () => {
    const respond = vi.fn(() => json({}))
    const { http } = setup(respond, null)
    await expect(http.get('/portal/v1/me')).rejects.toMatchObject({
      status: 401,
      type: PORTAL_PROBLEM.noSession,
    })
    expect(respond).not.toHaveBeenCalled()
  })

  it('204 no intenta leer un cuerpo', async () => {
    const { http } = setup(() => new Response(null, { status: 204 }))
    await expect(http.send('DELETE', '/portal/v1/invoices/x')).resolves.toBeUndefined()
  })
})
