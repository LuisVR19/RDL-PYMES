import type { ComponentType } from 'react'
import { createBrowserRouter, type RouteObject } from 'react-router'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
import { ScreenPlaceholder } from '@/features/system/pages/ScreenPlaceholder'
import { isMockDataSource } from '@/shared/api/DataSourceProvider'
import { RequireCapability } from './guards'
import { AppShell } from './layout/AppShell'
import { AuthLayout } from './layout/AuthLayout'
import { ForbiddenPage, RootFrame } from './RouteFrames'
import { SCREENS, type ScreenDef } from './screens'

/**
 * Pantallas ya construidas. Mientras una pantalla no está aquí, su ruta muestra el marcador provisional con su
 * número, permiso y referencia del prototipo. Cada incremento de módulo agrega sus pantallas a este mapa.
 */
const BUILT: Partial<Record<string, ComponentType>> = {}

function screenElement(s: ScreenDef) {
  const Built = BUILT[s.id]
  const page = Built ? <Built /> : <ScreenPlaceholder screen={s} />
  return s.outsideShell ? page : <RequireCapability capability={s.capability}>{page}</RequireCapability>
}

function toRoute(s: ScreenDef): RouteObject {
  return s.path === '/'
    ? { index: true, element: screenElement(s) }
    : { path: s.path.slice(1), element: screenElement(s) }
}

export const routes: RouteObject[] = [
  {
    element: <RootFrame />,
    errorElement: <SystemScreen kind="unexpected" />,
    children: [
      { element: <AuthLayout />, children: SCREENS.filter((s) => s.outsideShell).map(toRoute) },
      {
        path: '/',
        element: <AppShell />,
        children: [
          ...SCREENS.filter((s) => !s.outsideShell).map(toRoute),
          { path: '403', element: <ForbiddenPage /> },
          { path: '404', element: <SystemScreen kind="404" /> },
          { path: 'error', element: <SystemScreen kind="unexpected" /> },
          ...(isMockDataSource
            ? [
                {
                  path: '_catalogo',
                  lazy: async () => ({
                    Component: (await import('@/features/system/pages/CatalogPage')).CatalogPage,
                  }),
                },
              ]
            : []),
          { path: '*', element: <SystemScreen kind="404" /> },
        ],
      },
    ],
  },
]

export const router = createBrowserRouter(routes)
