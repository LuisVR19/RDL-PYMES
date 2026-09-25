/**
 * Configuración del portal, leída de las variables `VITE_*` al compilar.
 *
 * - `VITE_DATA_SOURCE`: `mock` (datos simulados, sin sesión real) o `gateway` (Portal Gateway + Supabase Auth).
 * - `VITE_GATEWAY_URL`: origen del Portal Gateway. El portal no habla con ninguna otra API de dominio.
 * - `VITE_SUPABASE_URL` y `VITE_SUPABASE_PUBLISHABLE_KEY`: Supabase Auth. La publishable key es pública por
 *   diseño; la `service_role` key nunca va en el portal.
 *
 * TODO(despliegue): el prompt P8b pide configuración en tiempo de ejecución (sin recompilar por entorno). Se
 * resuelve con el Dockerfile, que todavía no existe: hoy estos valores se fijan al compilar.
 */
export type DataSourceKind = 'mock' | 'gateway'

export interface PortalConfig {
  dataSource: DataSourceKind
  gatewayUrl: string
  supabaseUrl: string
  supabasePublishableKey: string
}

function readConfig(env: Record<string, string | undefined>): PortalConfig {
  const dataSource = env.VITE_DATA_SOURCE === 'gateway' ? 'gateway' : 'mock'
  return {
    dataSource,
    gatewayUrl: (env.VITE_GATEWAY_URL ?? 'http://localhost:8090').replace(/\/+$/, ''),
    supabaseUrl: (env.VITE_SUPABASE_URL ?? '').replace(/\/+$/, ''),
    supabasePublishableKey: env.VITE_SUPABASE_PUBLISHABLE_KEY ?? '',
  }
}

export const config: PortalConfig = readConfig(import.meta.env)

/** Solo para pruebas. */
export const readConfigForTest = readConfig
