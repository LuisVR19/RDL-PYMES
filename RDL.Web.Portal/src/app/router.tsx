import { lazy, Suspense, type ComponentType } from 'react'
import { createBrowserRouter, type RouteObject } from 'react-router'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { SystemScreen } from '@/design-system/components/SystemScreen/SystemScreen'
import { InvitePage } from '@/features/auth/pages/InvitePage'
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
 * Carga diferida por módulo: cada uno baja su bloque la primera vez que se entra a una de sus pantallas. Las de
 * acceso (1–4) van en el paquete inicial: el login es lo primero que se ve.
 */
const billing = () => import('@/features/billing/pages')
const admin = () => import('@/features/admin/pages')
const fromModule = <M,>(load: () => Promise<M>, pick: (m: M) => ComponentType) =>
  lazy(async () => ({ default: pick(await load()) }))

const HomePage = lazy(async () => ({ default: (await import('@/features/home/pages/HomePage')).HomePage }))
const ClientsPage = fromModule(billing, (m) => m.ClientsPage)
const ClientFormPage = fromModule(billing, (m) => m.ClientFormPage)
const ClientDetailPage = fromModule(billing, (m) => m.ClientDetailPage)
const ProductsPage = fromModule(billing, (m) => m.ProductsPage)
const ProductFormPage = fromModule(billing, (m) => m.ProductFormPage)
const DocumentsPage = fromModule(billing, (m) => m.DocumentsPage)
const InvoiceDraftPage = fromModule(billing, (m) => m.InvoiceDraftPage)
const InvoiceDetailPage = fromModule(billing, (m) => m.InvoiceDetailPage)
const NotePage = fromModule(billing, (m) => m.NotePage)
const ProfilePage = fromModule(admin, (m) => m.ProfilePage)
const OrganizationPage = fromModule(admin, (m) => m.OrganizationPage)
const UsersPage = fromModule(admin, (m) => m.UsersPage)
const AuditPage = fromModule(admin, (m) => m.AuditPage)

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
  home: HomePage,
  profile: ProfilePage,
  organization: OrganizationPage,
  branches: OrganizationPage,
  users: UsersPage,
  invitations: UsersPage,
  audit: AuditPage,
  clients: ClientsPage,
  clientNew: ClientFormPage,
  clientEdit: ClientFormPage,
  clientDetail: ClientDetailPage,
  products: ProductsPage,
  productNew: ProductFormPage,
  productEdit: ProductFormPage,
  documents: DocumentsPage,
  invoiceNew: InvoiceDraftPage,
  invoiceEdit: InvoiceDraftPage,
  invoiceDetail: InvoiceDetailPage,
  creditNote: NotePage,
  debitNote: NotePage,
}

/** Fuera del armazón pero con sesión: elegir o crear organización, aceptar una invitación. */
const NEEDS_SESSION = new Set(['orgSelect', 'orgCreate', 'invite'])

/** Pantallas que ya no son marcador provisional (las pruebas del armazón las saltan). */
export const builtScreenIds = new Set(Object.keys(BUILT))

function screenElement(s: ScreenDef) {
  const Built = BUILT[s.id]
  const page = Built ? (
    <Suspense fallback={<SkeletonRows rows={6} columns={4} />}>
      <Built />
    </Suspense>
  ) : (
    <ScreenPlaceholder screen={s} />
  )
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
