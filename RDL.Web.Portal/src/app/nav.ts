import {
  BarChart3,
  Building2,
  CreditCard,
  FileDown,
  FileText,
  Home,
  Inbox,
  Package,
  Receipt,
  Settings2,
  Store,
  Users,
  UsersRound,
  type LucideIcon,
} from 'lucide-react'
import type { MessageKey } from '@/shared/i18n/t'
import { can, type Capability, type Role } from '@/shared/permissions/permissions'

export interface NavItem {
  key: string
  label: MessageKey
  to: string
  icon: LucideIcon
  capability: Capability
  /** Contador de la bandeja (documentos que requieren atención). */
  badge?: 'inbox'
}

export interface NavSection {
  label?: MessageKey
  items: NavItem[]
}

/** Menú lateral del prototipo (NAV en design/referencias). Se filtra por la matriz de permisos. */
export const NAV: NavSection[] = [
  { items: [{ key: 'home', label: 'nav.home', to: '/', icon: Home, capability: 'home.view' }] },
  {
    label: 'nav.section.billing',
    items: [
      { key: 'clients', label: 'nav.clients', to: '/clientes', icon: Users, capability: 'billing.view' },
      { key: 'products', label: 'nav.products', to: '/productos', icon: Package, capability: 'billing.view' },
      {
        key: 'documents',
        label: 'nav.documents',
        to: '/documentos',
        icon: FileText,
        capability: 'billing.view',
      },
    ],
  },
  {
    label: 'nav.section.fiscal',
    items: [
      {
        key: 'inbox',
        label: 'nav.inbox',
        to: '/hacienda/bandeja',
        icon: Inbox,
        capability: 'fiscal.inbox',
        badge: 'inbox',
      },
      {
        key: 'fiscalConfig',
        label: 'nav.fiscalConfig',
        to: '/hacienda/configuracion',
        icon: Settings2,
        capability: 'fiscal.config.view',
      },
      {
        key: 'establishments',
        label: 'nav.establishments',
        to: '/hacienda/establecimientos',
        icon: Store,
        capability: 'fiscal.config.view',
      },
    ],
  },
  {
    label: 'nav.section.receivables',
    items: [
      {
        key: 'receivables',
        label: 'nav.receivables',
        to: '/cobranza/cuentas',
        icon: Receipt,
        capability: 'receivables.view',
      },
      {
        key: 'aging',
        label: 'nav.aging',
        to: '/cobranza/aging',
        icon: BarChart3,
        capability: 'receivables.view',
      },
      {
        key: 'payments',
        label: 'nav.payments',
        to: '/cobranza/pagos',
        icon: CreditCard,
        capability: 'receivables.view',
      },
    ],
  },
  {
    label: 'nav.section.admin',
    items: [
      {
        key: 'organization',
        label: 'nav.organization',
        to: '/admin/organizacion',
        icon: Building2,
        capability: 'admin.view',
      },
      { key: 'users', label: 'nav.users', to: '/admin/usuarios', icon: UsersRound, capability: 'admin.view' },
      { key: 'audit', label: 'nav.audit', to: '/admin/auditoria', icon: FileDown, capability: 'admin.view' },
    ],
  },
]

export function navFor(role: Role | undefined): NavSection[] {
  if (!role) return []
  return NAV.map((s) => ({ ...s, items: s.items.filter((i) => can(role, i.capability)) })).filter(
    (s) => s.items.length > 0,
  )
}
