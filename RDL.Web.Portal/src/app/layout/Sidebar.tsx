import { useQuery } from '@tanstack/react-query'
import { clsx } from 'clsx'
import { NavLink } from 'react-router'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import { navFor, type NavItem } from '../nav'
import { BrandMark } from './BrandMark'
import styles from './Sidebar.module.css'

interface NavListProps {
  collapsed?: boolean
  onNavigate?: () => void
  variant?: 'sidebar' | 'drawer'
}

/** Lista de navegación filtrada por rol; la usan el menú lateral y el menú móvil. */
export function NavList({ collapsed = false, onNavigate, variant = 'sidebar' }: NavListProps) {
  const { role, activeOrg } = useSession()
  const ds = useDataSource()
  const inbox = useQuery({
    queryKey: ['shell', activeOrg?.id, 'inboxCount'],
    queryFn: () => ds.shell.inboxAttentionCount(activeOrg?.id ?? ''),
    enabled: !!activeOrg,
  })

  const badgeFor = (item: NavItem) => (item.badge === 'inbox' && inbox.data ? inbox.data : undefined)

  return (
    <nav aria-label={t('nav.menu')} className={clsx(styles.nav, variant === 'drawer' && styles.navDrawer)}>
      {navFor(role).map((section, i) => (
        <div key={section.label ?? i} className={styles.group}>
          {section.label && !collapsed && <div className={styles.section}>{t(section.label)}</div>}
          {section.label && collapsed && <div className={styles.sectionRule} aria-hidden />}
          <ul className={styles.list}>
            {section.items.map((item) => {
              const badge = badgeFor(item)
              const label = t(item.label)
              return (
                <li key={item.key}>
                  <NavLink
                    to={item.to}
                    end={item.to === '/'}
                    onClick={onNavigate}
                    title={collapsed ? label : undefined}
                    aria-label={collapsed ? label : undefined}
                    className={({ isActive }) =>
                      clsx(styles.item, collapsed && styles.itemCollapsed, isActive && styles.active)
                    }
                  >
                    <item.icon
                      size={variant === 'drawer' ? 20 : 18}
                      strokeWidth={1.8}
                      aria-hidden
                      className={styles.icon}
                    />
                    {!collapsed && <span className={styles.label}>{label}</span>}
                    {badge !== undefined && (
                      <span className={clsx(styles.badge, collapsed && styles.badgeDot)}>
                        {badge}
                        <span className="sr-only"> por atender</span>
                      </span>
                    )}
                  </NavLink>
                </li>
              )
            })}
          </ul>
        </div>
      ))}
    </nav>
  )
}

/** Menú lateral de escritorio y tableta: 232 px, contraído 72 px (solo iconos con tooltip nativo). */
export function Sidebar({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  return (
    <aside className={clsx(styles.sidebar, collapsed && styles.collapsed)}>
      <div className={styles.brand}>
        <BrandMark />
        {!collapsed && <span className={styles.brandText}>{t('app.brand')}</span>}
      </div>
      <NavList collapsed={collapsed} />
      <button type="button" className={styles.collapse} onClick={onToggle} aria-expanded={!collapsed}>
        {collapsed ? t('nav.expand') : t('nav.collapse')}
      </button>
    </aside>
  )
}
