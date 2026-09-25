import type { Capability } from '@/shared/permissions/permissions'

/**
 * Registro único de las 36 pantallas de design/pantallas.md: número, nombre, ruta, módulo, permiso y referencia
 * del prototipo. El router, las pantallas provisionales y la barra de desarrollo salen de aquí; cuando una pantalla
 * se construye, su entrada apunta al componente real (`src/app/router.tsx`).
 */
export type ScreenModule = 'A' | 'B' | 'C' | 'D' | 'E' | 'F' | 'G'

export interface ScreenDef {
  /** Número en design/pantallas.md. Algunas pantallas tienen varias rutas (crear y editar). */
  n: number
  id: string
  name: string
  path: string
  module: ScreenModule
  /** `null`: pública o solo requiere sesión. */
  capability: Capability | null
  /** 📱 debe funcionar a 390 px. */
  mobile?: boolean
  /** Fragmento del prototipo (design/referencias, pantalla `data-screen-label`). */
  reference: string
  /** Fuera del armazón (pantallas de acceso). */
  outsideShell?: boolean
}

export const MODULE_LABEL: Record<ScreenModule, string> = {
  A: 'A · Acceso y organización',
  B: 'B · Inicio',
  C: 'C · Facturación',
  D: 'D · Hacienda',
  E: 'E · Cobranza',
  F: 'F · Administración',
  G: 'G · Transversales',
}

export const SCREENS: ScreenDef[] = [
  // A · Acceso y organización
  {
    n: 1,
    id: 'login',
    name: 'Iniciar sesión',
    path: '/ingresar',
    module: 'A',
    capability: null,
    mobile: true,
    reference: '01 Iniciar sesión',
    outsideShell: true,
  },
  {
    n: 1,
    id: 'recover',
    name: 'Recuperar contraseña',
    path: '/recuperar',
    module: 'A',
    capability: null,
    mobile: true,
    reference: '01 Iniciar sesión',
    outsideShell: true,
  },
  {
    n: 2,
    id: 'orgSelect',
    name: 'Selector de organización',
    path: '/organizaciones',
    module: 'A',
    capability: null,
    mobile: true,
    reference: '02 Selector de organización',
    outsideShell: true,
  },
  {
    n: 3,
    id: 'orgCreate',
    name: 'Crear organización',
    path: '/organizaciones/nueva',
    module: 'A',
    capability: null,
    reference: '03 Crear organización',
    outsideShell: true,
  },
  {
    n: 4,
    id: 'invite',
    name: 'Aceptar invitación',
    path: '/invitacion/:token',
    module: 'A',
    capability: null,
    reference: '04 Aceptar invitación',
    outsideShell: true,
  },

  // B · Inicio
  {
    n: 6,
    id: 'home',
    name: 'Inicio',
    path: '/',
    module: 'B',
    capability: 'home.view',
    mobile: true,
    reference: '06 Inicio',
  },

  // C · Facturación
  {
    n: 7,
    id: 'clients',
    name: 'Clientes',
    path: '/clientes',
    module: 'C',
    capability: 'billing.view',
    reference: '07 Clientes',
  },
  {
    n: 8,
    id: 'clientNew',
    name: 'Nuevo cliente',
    path: '/clientes/nuevo',
    module: 'C',
    capability: 'billing.edit',
    reference: '08 Cliente · crear/editar',
  },
  {
    n: 8,
    id: 'clientEdit',
    name: 'Editar cliente',
    path: '/clientes/:id/editar',
    module: 'C',
    capability: 'billing.edit',
    reference: '08 Cliente · crear/editar',
  },
  {
    n: 9,
    id: 'clientDetail',
    name: 'Ficha del cliente',
    path: '/clientes/:id',
    module: 'C',
    capability: 'billing.view',
    mobile: true,
    reference: '09 Cliente · ficha',
  },
  {
    n: 10,
    id: 'products',
    name: 'Productos y servicios',
    path: '/productos',
    module: 'C',
    capability: 'billing.view',
    reference: '10 Productos',
  },
  {
    n: 11,
    id: 'productNew',
    name: 'Nuevo producto',
    path: '/productos/nuevo',
    module: 'C',
    capability: 'billing.edit',
    reference: '11 Producto · crear/editar',
  },
  {
    n: 11,
    id: 'productEdit',
    name: 'Editar producto',
    path: '/productos/:id/editar',
    module: 'C',
    capability: 'billing.edit',
    reference: '11 Producto · crear/editar',
  },
  {
    n: 12,
    id: 'documents',
    name: 'Documentos',
    path: '/documentos',
    module: 'C',
    capability: 'billing.view',
    reference: '12 Documentos',
  },
  {
    n: 13,
    id: 'invoiceNew',
    name: 'Nueva factura',
    path: '/facturas/nueva',
    module: 'C',
    capability: 'billing.edit',
    reference: '13 Factura · borrador',
  },
  {
    n: 13,
    id: 'invoiceEdit',
    name: 'Editar borrador',
    path: '/facturas/:id/editar',
    module: 'C',
    capability: 'billing.edit',
    reference: '13 Factura · borrador',
  },
  {
    n: 15,
    id: 'invoiceDetail',
    name: 'Detalle de factura',
    path: '/facturas/:id',
    module: 'C',
    capability: 'billing.view',
    mobile: true,
    reference: '15 Factura · detalle',
  },
  {
    n: 16,
    id: 'creditNote',
    name: 'Nota de crédito',
    path: '/facturas/:id/nota-credito',
    module: 'C',
    capability: 'billing.edit',
    reference: '16 Nota de crédito / débito',
  },
  {
    n: 16,
    id: 'debitNote',
    name: 'Nota de débito',
    path: '/facturas/:id/nota-debito',
    module: 'C',
    capability: 'billing.edit',
    reference: '16 Nota de crédito / débito',
  },

  // D · Hacienda
  {
    n: 18,
    id: 'fiscalConfig',
    name: 'Configuración fiscal',
    path: '/hacienda/configuracion',
    module: 'D',
    capability: 'fiscal.config.view',
    reference: '18 Configuración fiscal',
  },
  {
    n: 19,
    id: 'establishments',
    name: 'Establecimientos y terminales',
    path: '/hacienda/establecimientos',
    module: 'D',
    capability: 'fiscal.config.view',
    reference: '19 Establecimientos y terminales',
  },
  {
    n: 20,
    id: 'inbox',
    name: 'Bandeja',
    path: '/hacienda/bandeja',
    module: 'D',
    capability: 'fiscal.inbox',
    reference: '20 Bandeja',
  },
  {
    n: 21,
    id: 'eDocument',
    name: 'Documento electrónico',
    path: '/hacienda/documentos/:clave',
    module: 'D',
    capability: 'fiscal.inbox',
    mobile: true,
    reference: '21 Documento electrónico',
  },

  // E · Cobranza
  {
    n: 22,
    id: 'receivables',
    name: 'Cuentas por cobrar',
    path: '/cobranza/cuentas',
    module: 'E',
    capability: 'receivables.view',
    reference: '22 Cuentas por cobrar',
  },
  {
    n: 23,
    id: 'aging',
    name: 'Aging',
    path: '/cobranza/aging',
    module: 'E',
    capability: 'receivables.view',
    reference: '23 Aging',
  },
  {
    n: 24,
    id: 'receivableDetail',
    name: 'Detalle de cuenta',
    path: '/cobranza/cuentas/:id',
    module: 'E',
    capability: 'receivables.view',
    mobile: true,
    reference: '24 Cuenta · detalle',
  },
  {
    n: 25,
    id: 'payments',
    name: 'Pagos',
    path: '/cobranza/pagos',
    module: 'E',
    capability: 'receivables.view',
    reference: '25 Pagos',
  },
  {
    n: 26,
    id: 'paymentNew',
    name: 'Registrar pago',
    path: '/cobranza/pagos/nuevo',
    module: 'E',
    capability: 'receivables.edit',
    mobile: true,
    reference: '26 Registrar pago',
  },
  {
    n: 27,
    id: 'paymentDetail',
    name: 'Detalle de pago',
    path: '/cobranza/pagos/:id',
    module: 'E',
    capability: 'receivables.view',
    reference: '27 Pago · detalle',
  },

  // F · Administración
  {
    n: 28,
    id: 'organization',
    name: 'Organización',
    path: '/admin/organizacion',
    module: 'F',
    capability: 'admin.view',
    reference: '28 Organización',
  },
  {
    n: 29,
    id: 'branches',
    name: 'Sucursales',
    path: '/admin/organizacion/sucursales',
    module: 'F',
    capability: 'admin.view',
    reference: '29 Sucursales',
  },
  {
    n: 30,
    id: 'users',
    name: 'Usuarios y roles',
    path: '/admin/usuarios',
    module: 'F',
    capability: 'admin.view',
    reference: '30 Usuarios y roles',
  },
  {
    n: 31,
    id: 'invitations',
    name: 'Invitaciones',
    path: '/admin/usuarios/invitaciones',
    module: 'F',
    capability: 'admin.view',
    reference: '31 Invitaciones',
  },
  {
    n: 32,
    id: 'audit',
    name: 'Exportar auditoría',
    path: '/admin/auditoria',
    module: 'F',
    capability: 'admin.view',
    reference: '32 Exportar auditoría',
  },
  {
    n: 33,
    id: 'profile',
    name: 'Mi perfil',
    path: '/perfil',
    module: 'F',
    capability: null,
    mobile: true,
    reference: '33 Mi perfil',
  },
]

/** Pantallas que no tienen ruta propia (diálogos y paneles), para la documentación y el catálogo. */
export const OVERLAY_SCREENS = [
  { n: 5, name: 'Cambio de organización', where: 'Barra superior · selector de organización' },
  { n: 14, name: 'Emitir', where: 'Diálogo desde el borrador' },
  { n: 17, name: 'Anular factura', where: 'Diálogo desde el detalle' },
  { n: 34, name: 'Notificaciones', where: 'Barra superior · campana' },
  { n: 35, name: 'Pantallas de sistema', where: '/403, /404, /error y sesión vencida en diálogo' },
  { n: 36, name: 'Atajos de teclado', where: 'Tecla ?' },
] as const

export function screenById(id: string): ScreenDef {
  const s = SCREENS.find((x) => x.id === id)
  if (!s) throw new Error(`Pantalla desconocida: ${id}`)
  return s
}
