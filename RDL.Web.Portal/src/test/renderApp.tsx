import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { routes } from '@/app/router'
import { TooltipProvider } from '@/design-system/components/Surface/Surface'
import { ToastProvider } from '@/design-system/components/Toast/Toast'
import { setScenario } from '@/shared/api/scenario'
import { SessionProvider } from '@/shared/session/SessionProvider'
import { ThemeProvider } from '@/shared/theme/ThemeProvider'

/** Monta el portal completo en una ruta, con datos simulados sin latencia. */
export function renderApp(path: string, { orgId = 'ca' }: { orgId?: string } = {}) {
  setScenario({ scenario: 'ok', latencyMs: 0, realtime: 'ok' })
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <SessionProvider initialOrgId={orgId}>
          <TooltipProvider>
            <ToastProvider>
              <RouterProvider router={router} />
            </ToastProvider>
          </TooltipProvider>
        </SessionProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  )
  return { ...utils, router }
}
