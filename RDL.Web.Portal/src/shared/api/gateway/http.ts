import { ApiError } from '../types'

/**
 * Cliente HTTP del Portal Gateway: el ÚNICO `fetch` del portal (ADR 0006). Cada llamada lleva:
 *   - `Authorization: Bearer <token del usuario>`, pedido a Supabase en cada llamada (supabase-js lo refresca);
 *   - `X-Correlation-Id`: un UUID nuevo, que el gateway propaga a todas las APIs y devuelve en los errores;
 *   - `Idempotency-Key` en los POST que la piden: la genera QUIEN LLAMA, una por intento de envío del usuario,
 *     y la reutiliza si el usuario reintenta ese mismo envío (P8b, paso 4.3).
 * La organización nunca viaja: sale del token (P8b, paso 4.4).
 */
export interface GatewayHttp {
  get<T>(path: string, query?: Record<string, string | undefined>): Promise<T>
  send<T>(
    method: 'POST' | 'PUT' | 'PATCH' | 'DELETE',
    path: string,
    body?: unknown,
    opts?: SendOptions,
  ): Promise<T>
}

export interface SendOptions {
  idempotencyKey?: string
}

export interface GatewayHttpDeps {
  baseUrl: string
  accessToken: () => Promise<string | null>
  /** Se llama ante un 401 (token vencido o rechazado): la sesión muestra el diálogo de la pantalla 35. */
  onUnauthorized?: () => void
  fetch?: typeof fetch
  newId?: () => string
}

/** `type` de los errores que nacen en el portal, sin respuesta del gateway (red caída, sin sesión). */
export const PORTAL_PROBLEM = {
  network: 'urn:rdl:portal:problem:network',
  noSession: 'urn:rdl:portal:problem:no-session',
} as const

export function createGatewayHttp({
  baseUrl,
  accessToken,
  fetch: doFetch = (...args) => globalThis.fetch(...args),
  newId = () => crypto.randomUUID(),
  onUnauthorized = () => {},
}: GatewayHttpDeps): GatewayHttp {
  async function request<T>(
    method: string,
    path: string,
    body: unknown,
    headers: Record<string, string>,
  ): Promise<T> {
    const correlationId = newId()
    const token = await accessToken()
    if (!token) {
      onUnauthorized()
      throw new ApiError({ status: 401, type: PORTAL_PROBLEM.noSession, title: 'Sin sesión', correlationId })
    }

    let res: Response
    try {
      res = await doFetch(baseUrl + path, {
        method,
        headers: {
          Accept: 'application/json',
          Authorization: `Bearer ${token}`,
          'X-Correlation-Id': correlationId,
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
          ...headers,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      })
    } catch {
      throw new ApiError({ status: 0, type: PORTAL_PROBLEM.network, title: 'Sin conexión', correlationId })
    }

    if (res.status === 401) onUnauthorized()
    if (!res.ok) throw await toApiError(res, correlationId)
    if (res.status === 204) return undefined as T
    return (await res.json()) as T
  }

  return {
    get(path, query) {
      const qs = new URLSearchParams()
      for (const [k, v] of Object.entries(query ?? {})) if (v !== undefined) qs.set(k, v)
      const suffix = qs.size > 0 ? `?${qs.toString()}` : ''
      return request('GET', path + suffix, undefined, {})
    },
    send(method, path, body, opts) {
      return request(
        method,
        path,
        body,
        opts?.idempotencyKey ? { 'Idempotency-Key': opts.idempotencyKey } : {},
      )
    },
  }
}

/** Problem Details (RFC 9457) → ApiError. El código de referencia es el que devolvió el gateway, si lo hay. */
async function toApiError(res: Response, sentCorrelationId: string): Promise<ApiError> {
  const correlationId = res.headers.get('X-Correlation-Id') ?? sentCorrelationId
  try {
    const p = (await res.json()) as {
      type?: string
      title?: string
      correlationId?: string
      errors?: { field?: string; message?: string }[]
    }
    return new ApiError({
      status: res.status,
      type: p.type ?? 'about:blank',
      title: p.title ?? res.statusText,
      correlationId: p.correlationId ?? correlationId,
      errors: (p.errors ?? []).flatMap((e) =>
        e.field && e.message ? [{ field: e.field, message: e.message }] : [],
      ),
    })
  } catch {
    return new ApiError({ status: res.status, type: 'about:blank', title: res.statusText, correlationId })
  }
}
