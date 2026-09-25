import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router'
import { TooltipProvider } from '@/design-system/components/Surface/Surface'
import { ToastProvider } from '@/design-system/components/Toast/Toast'
import { DataSourceProvider } from '@/shared/api/DataSourceProvider'
import { SessionProvider } from '@/shared/session/SessionProvider'
import { ThemeProvider } from '@/shared/theme/ThemeProvider'
import { router } from './router'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      // Los errores se muestran con su código de referencia; un solo reintento automático.
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

/** Proveedores en orden: datos → tema → sesión → tooltips → toasts → rutas. */
export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <DataSourceProvider>
        <ThemeProvider>
          <SessionProvider>
            <TooltipProvider delayDuration={300}>
              <ToastProvider>
                <RouterProvider router={router} />
              </ToastProvider>
            </TooltipProvider>
          </SessionProvider>
        </ThemeProvider>
      </DataSourceProvider>
    </QueryClientProvider>
  )
}
