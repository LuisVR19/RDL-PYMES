import { createGatewayDataSource } from '@/shared/api/gateway'
import { createGatewayHttp } from '@/shared/api/gateway/http'
import { mockDataSource } from '@/shared/api/mock'
import type { DataSource } from '@/shared/api/ports'
import { mockAuth, type AuthPort } from '@/shared/auth/auth'
import { expireSession } from '@/shared/auth/expiry'
import { createSupabaseAuth } from '@/shared/auth/supabase'
import { config } from '@/shared/config'

/**
 * Composition root de los datos: `VITE_DATA_SOURCE` elige entre el portal simulado (sin sesión real) y el
 * cableado (Supabase Auth + Portal Gateway). Nada más en el portal decide esto.
 */
function build(): { auth: AuthPort; data: DataSource } {
  if (config.dataSource === 'mock') return { auth: mockAuth, data: mockDataSource }

  if (!config.supabaseUrl || !config.supabasePublishableKey) {
    throw new Error(
      'VITE_DATA_SOURCE=gateway necesita VITE_SUPABASE_URL y VITE_SUPABASE_PUBLISHABLE_KEY (ver .env.example).',
    )
  }
  const auth = createSupabaseAuth(config.supabaseUrl, config.supabasePublishableKey)
  const http = createGatewayHttp({
    baseUrl: config.gatewayUrl,
    accessToken: () => auth.accessToken(),
    onUnauthorized: expireSession,
  })
  return { auth, data: createGatewayDataSource(http) }
}

export const sources = build()
