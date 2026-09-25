import { clsx } from 'clsx'
import { Menu, Search, TriangleAlert } from 'lucide-react'
import { useRef, useState } from 'react'
import { Outlet, useNavigate } from 'react-router'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useHotkey } from '@/shared/hotkeys/useHotkey'
import { t } from '@/shared/i18n/t'
import { can } from '@/shared/permissions/permissions'
import { useRealtime } from '@/shared/realtime/useRealtime'
import { useSession } from '@/shared/session/SessionProvider'
import { BrandMark } from './BrandMark'
import { EnvBadge } from './EnvBadge'
import { NotificationBell } from './NotificationBell'
import { OrgSwitcher } from './OrgSwitcher'
import { ShortcutSheet } from './ShortcutSheet'
import { MobileNav } from './MobileNav'
import { Sidebar } from './Sidebar'
import { ThemeToggle, UserMenu } from './UserMenu'
import styles from './AppShell.module.css'

const COLLAPSE_KEY = 'rdl.sidebar.collapsed'

function readCollapsed(): boolean {
  try {
    return window.localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {
    return false
  }
}

/**
 * Armazón del portal (prototipo `sApp`): menú lateral, barra superior con organización, ambiente, búsqueda,
 * notificaciones, tema y usuario; en móvil, hamburguesa y fila de organización. Atajos globales: `/`, `?`, `N`.
 */
export function AppShell() {
  const { activeOrg, role, switching } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const rt = useRealtime()
  const searchRef = useRef<HTMLInputElement>(null)
  const [collapsed, setCollapsed] = useState(readCollapsed)
  const [mobileNav, setMobileNav] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)

  const toggleCollapsed = () =>
    setCollapsed((c) => {
      try {
        window.localStorage.setItem(COLLAPSE_KEY, c ? '0' : '1')
      } catch {
        // Preferencia de conveniencia.
      }
      return !c
    })

  useHotkey('/', () => searchRef.current?.focus())
  useHotkey('?', () => setShortcuts(true))
  useHotkey('n', () => navigate('/facturas/nueva'), !!role && can(role, 'billing.edit'))

  const reconnect = () => {
    rt.reconnect()
    toast({ tone: 'success', title: t('rt.restored') })
  }

  return (
    <div className={styles.shell}>
      <a href="#contenido" className={styles.skip}>
        {t('app.skipToContent')}
      </a>
      <div className={styles.sidebarSlot}>
        <Sidebar collapsed={collapsed} onToggle={toggleCollapsed} />
      </div>

      <div className={styles.column}>
        <header className={styles.topbar}>
          <button
            type="button"
            className={styles.hamburger}
            onClick={() => setMobileNav(true)}
            aria-label={t('nav.menu')}
          >
            <Menu size={22} aria-hidden />
          </button>
          <span className={styles.mobileBrand}>
            <BrandMark size={28} />
          </span>

          <div className={styles.desktopOnly}>
            <OrgSwitcher variant="desktop" />
            {activeOrg?.environment && <EnvBadge environment={activeOrg.environment} />}
          </div>

          <form
            role="search"
            className={styles.search}
            onSubmit={(e) => {
              // La búsqueda global se implementa con el módulo de facturación (etapa de pantallas).
              e.preventDefault()
            }}
          >
            <Search size={16} aria-hidden />
            <input
              ref={searchRef}
              type="search"
              placeholder={t('app.search.placeholder')}
              aria-label={t('topbar.search')}
            />
            <kbd className={styles.kbd}>/</kbd>
          </form>

          <div className={styles.actions}>
            <NotificationBell />
            <ThemeToggle />
            <UserMenu onShortcuts={() => setShortcuts(true)} />
          </div>
        </header>

        <div className={styles.mobileOnly}>
          <OrgSwitcher variant="mobile" />
        </div>

        {rt.offline && (
          <div className={styles.rtBanner} role="status">
            <TriangleAlert size={16} aria-hidden />
            <span className={styles.rtText}>
              {t('rt.offline')} {t('rt.reconnectIn', { s: rt.secondsToRetry })}
            </span>
            <button type="button" className={styles.rtRetry} onClick={reconnect}>
              {t('notif.reconnect')}
            </button>
          </div>
        )}

        <main
          id="contenido"
          tabIndex={-1}
          className={clsx(styles.main, switching && styles.switching)}
          aria-busy={switching}
        >
          <div className={styles.content}>
            {/* Durante el cambio de organización no se ve ningún dato: ni de la anterior ni a medias de la nueva. */}
            {switching ? <SkeletonRows rows={6} columns={4} /> : <Outlet />}
          </div>
        </main>
      </div>

      <MobileNav open={mobileNav} onOpenChange={setMobileNav} />
      <ShortcutSheet open={shortcuts} onOpenChange={setShortcuts} />
    </div>
  )
}
