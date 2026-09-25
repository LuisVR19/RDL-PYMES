import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { routes } from '@/app/router'
import { TooltipProvider } from '@/design-system/components/Surface/Surface'
import { ToastProvider } from '@/design-system/components/Toast/Toast'
import { DataSourceProvider } from '@/shared/api/DataSourceProvider'
import type { DataSource } from '@/shared/api/ports'
import { setScenario } from '@/shared/api/scenario'
import type { AuthPort } from '@/shared/auth/auth'
import { AuthProvider } from '@/shared/auth/AuthProvider'
import { SessionProvider } from '@/shared/session/SessionProvider'
import { ThemeProvider } from '@/shared/theme/ThemeProvider'

/**
 * Monta el portal completo en una ruta, con datos simulados sin latencia. `auth` y `source` reemplazan la sesión
 * y los datos simulados (pruebas del cableado); `orgId: null` deja que la organización activa la diga el servidor.
 */
export function renderApp(
  path: string,
  { orgId = 'ca', auth, source }: { orgId?: string | null; auth?: AuthPort; source?: DataSource } = {},
) {
  setScenario({ scenario: 'ok', latencyMs: 0, realtime: 'ok' })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider port={auth}>
        <DataSourceProvider source={source}>
          <ThemeProvider>
            <SessionProvider initialOrgId={orgId ?? undefined}>
              <TooltipProvider>
                <ToastProvider>
                  <RouterProvider router={router} />
                </ToastProvider>
              </TooltipProvider>
            </SessionProvider>
          </ThemeProvider>
        </DataSourceProvider>
      </AuthProvider>
    </QueryClientProvider>,
  )
  return { ...utils, router, queryClient }
}
