import { ApiError } from '../types'
import { getScenario } from '../scenario'

let counter = 0

/** Código de referencia con la forma de un correlationId (UUID corto para mostrar y copiar). */
export function fakeCorrelationId(): string {
  counter += 1
  return `3f2b8c1e-9d4a-4b7e-8c21-${String(counter).padStart(12, '0')}`
}

/**
 * Simula una llamada: espera la latencia configurada y aplica el escenario activo.
 * - `loading`: no responde nunca (para revisar esqueletos).
 * - `error`: falla con un Problem Details y código de referencia.
 * - `empty`: devuelve la versión vacía.
 * - `partial` y `ok`: devuelven los datos (cada pantalla decide qué bloque falla en `partial`).
 */
export async function simulate<T>(data: T, empty: T): Promise<T> {
  const { scenario, latencyMs } = getScenario()
  if (scenario === 'loading') return new Promise<T>(() => {})
  await new Promise((r) => setTimeout(r, latencyMs))
  if (scenario === 'error') {
    throw new ApiError({
      status: 503,
      type: 'urn:rdl:portal-gateway:problem:upstream-unavailable',
      title: 'Servicio no disponible',
      correlationId: fakeCorrelationId(),
    })
  }
  return scenario === 'empty' ? empty : data
}

/** Para bloques secundarios: en `partial` fallan aunque el resto de la pantalla cargue. */
export async function simulateSecondary<T>(data: T, empty: T): Promise<T> {
  if (getScenario().scenario === 'partial') {
    await new Promise((r) => setTimeout(r, getScenario().latencyMs))
    throw new ApiError({
      status: 503,
      type: 'urn:rdl:portal-gateway:problem:partial',
      title: 'Datos parciales',
      correlationId: fakeCorrelationId(),
    })
  }
  return simulate(data, empty)
}
