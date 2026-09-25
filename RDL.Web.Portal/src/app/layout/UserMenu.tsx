import * as Menu from '@radix-ui/react-dropdown-menu'
import { Moon, Sun } from 'lucide-react'
import { useNavigate } from 'react-router'
import { Kbd } from '@/design-system/components/Surface/Surface'
import { useAuth } from '@/shared/auth/AuthProvider'
import { t } from '@/shared/i18n/t'
import { useSession } from '@/shared/session/SessionProvider'
import { useTheme } from '@/shared/theme/ThemeProvider'
import styles from './UserMenu.module.css'

export function ThemeToggle() {
  const { theme, toggle } = useTheme()
  const label = theme === 'dark' ? t('app.theme.toLight') : t('app.theme.toDark')
  return (
    <button type="button" className={styles.iconBtn} onClick={toggle} aria-label={label} title={label}>
      {theme === 'dark' ? (
        <Sun size={20} strokeWidth={1.8} aria-hidden />
      ) : (
        <Moon size={20} strokeWidth={1.8} aria-hidden />
      )}
    </button>
  )
}

/** Menú de usuario (prototipo): nombre y correo, Mi perfil, Atajos de teclado (?), Cerrar sesión. */
export function UserMenu({ onShortcuts }: { onShortcuts: () => void }) {
  const { user } = useSession()
  const { signOut } = useAuth()
  const navigate = useNavigate()
  if (!user) return null
  return (
    <Menu.Root>
      <Menu.Trigger asChild>
        <button type="button" className={styles.iconBtn} aria-label={t('topbar.userMenu')}>
          <span className={styles.avatar}>{user.initials}</span>
        </button>
      </Menu.Trigger>
      <Menu.Portal>
        <Menu.Content className={styles.menu} align="end" sideOffset={6}>
          <div className={styles.who}>
            <span className={styles.name}>{user.fullName}</span>
            <span className={styles.email}>{user.email}</span>
          </div>
          <Menu.Item className={styles.item} onSelect={() => navigate('/perfil')}>
            {t('user.profile')}
          </Menu.Item>
          <Menu.Item className={styles.item} onSelect={onShortcuts}>
            {t('user.shortcuts')} <Kbd>?</Kbd>
          </Menu.Item>
          <Menu.Separator className={styles.sep} />
          {/* Cierra la sesión de Supabase Auth y vacía la caché; con datos simulados, solo la sesión simulada. */}
          <Menu.Item
            className={styles.item}
            onSelect={() => {
              void signOut().then(() => navigate('/ingresar', { replace: true }))
            }}
          >
            {t('user.logout')}
          </Menu.Item>
        </Menu.Content>
      </Menu.Portal>
    </Menu.Root>
  )
}
