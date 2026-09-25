import type { ComponentType } from 'react'
import { createBrowserRouter, type RouteObject } from 'react-router'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
import { ProfilePage } from '@/features/admin/pages/ProfilePage'
import { InvitePage } from '@/features/auth/pages/InvitePage'
import { ClientDetailPage } from '@/features/billing/pages/ClientDetailPage'
import { ClientFormPage } from '@/features/billing/pages/ClientFormPage'
import { ClientsPage } from '@/features/billing/pages/ClientsPage'
import { DocumentsPage } from '@/features/billing/pages/DocumentsPage'
import { InvoiceDetailPage } from '@/features/billing/pages/InvoiceDetailPage'
import { ProductFormPage } from '@/features/billing/pages/ProductFormPage'
import { ProductsPage } from '@/features/billing/pages/ProductsPage'
import { LoginPage } from '@/features/auth/pages/LoginPage'
import { OrgCreatePage } from '@/features/auth/pages/OrgCreatePage'
import { OrgSelectPage } from '@/features/auth/pages/OrgSelectPage'
import { RecoverPage } from '@/features/auth/pages/RecoverPage'
import { ScreenPlaceholder } from '@/features/system/pages/ScreenPlaceholder'
import { isMockDataSource } from '@/shared/api/DataSourceProvider'
import { RequireAuth, RequireCapability, RequireSession } from './guards'
import { AppShell } from './layout/AppShell'
import { AuthLayout } from './layout/AuthLayout'
import { ForbiddenPage, RootFrame } from './RouteFrames'
import { SCREENS, type ScreenDef } from './screens'

/**
 * Pantallas ya construidas. Mientras una pantalla no está aquí, su ruta muestra el marcador provisional con su
 * número, permiso y referencia del prototipo. Cada incremento de módulo agrega sus pantallas a este mapa.
 */
const BUILT: Partial<Record<string, ComponentType>> = {
  login: LoginPage,
  recover: RecoverPage,
  orgSelect: OrgSelectPage,
  orgCreate: OrgCreatePage,
  invite: InvitePage,
  profile: ProfilePage,
  clients: ClientsPage,
  clientNew: ClientFormPage,
  clientEdit: ClientFormPage,
  clientDetail: ClientDetailPage,
  products: ProductsPage,
  productNew: ProductFormPage,
  productEdit: ProductFormPage,
  documents: DocumentsPage,
  invoiceDetail: InvoiceDetailPage,
}

/** Fuera del armazón pero con sesión: elegir o crear organización, aceptar una invitación. */
const NEEDS_SESSION = new Set(['orgSelect', 'orgCreate', 'invite'])

/** Pantallas que ya no son marcador provisional (las pruebas del armazón las saltan). */
export const builtScreenIds = new Set(Object.keys(BUILT))

function screenElement(s: ScreenDef) {
  const Built = BUILT[s.id]
  const page = Built ? <Built /> : <ScreenPlaceholder screen={s} />
  if (s.outsideShell) return NEEDS_SESSION.has(s.id) ? <RequireSession>{page}</RequireSession> : page
  return <RequireCapability capability={s.capability}>{page}</RequireCapability>
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
        element: (
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        ),
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
