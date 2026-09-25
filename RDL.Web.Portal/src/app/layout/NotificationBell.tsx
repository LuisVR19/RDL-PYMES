import * as Popover from '@radix-ui/react-popover'
import { useQuery } from '@tanstack/react-query'
import { clsx } from 'clsx'
import { Bell, Check, CircleAlert, Info, TriangleAlert, X } from 'lucide-react'
import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { PortalNotification } from '@/shared/api/types'
import { formatRelative } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { useRealtime } from '@/shared/realtime/useRealtime'
import { useSession } from '@/shared/session/SessionProvider'
import styles from './NotificationBell.module.css'

const ICON = { success: Check, danger: X, info: Info, warning: TriangleAlert }

/**
 * Campana y panel de notificaciones (pantalla 34): contador de no leídas, punto ámbar si no hay tiempo real,
 * aviso de reconexión dentro del panel. En esta etapa «leído» es local; en la de cableado lo guarda el gateway.
 */
export function NotificationBell() {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const navigate = useNavigate()
  const rt = useRealtime()
  const [open, setOpen] = useState(false)
  const [read, setRead] = useState<Set<string>>(() => new Set())

  const q = useQuery({
    queryKey: ['notifications', activeOrg?.id],
    queryFn: () => ds.notifications.list(activeOrg?.id ?? ''),
    enabled: !!activeOrg,
  })

  const items = (q.data ?? []).map((n) => ({ ...n, unread: n.unread && !read.has(n.id) }))
  const unread = items.filter((n) => n.unread).length

  const markAll = () => setRead(new Set(items.map((n) => n.id)))
  const openItem = (n: PortalNotification) => {
    setRead((r) => new Set(r).add(n.id))
    if (n.action) {
      setOpen(false)
      navigate(n.action.to)
    }
  }

  const label =
    unread > 0
      ? `${t('topbar.notifications')}, ${t('notif.unreadCount', { n: unread })}`
      : t('topbar.notifications')

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button type="button" className={clsx(styles.bell, open && styles.bellOpen)} aria-label={label}>
          <Bell size={20} strokeWidth={1.8} aria-hidden />
          {unread > 0 && (
            <span className={styles.count} aria-hidden>
              {unread}
            </span>
          )}
          {rt.offline && <span className={styles.offlineDot} aria-hidden />}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className={styles.panel} align="end" sideOffset={8} aria-label={t('notif.title')}>
          <div className={styles.head}>
            <h2 className={styles.title}>{t('notif.title')}</h2>
            {unread > 0 && (
              <button type="button" className={styles.link} onClick={markAll}>
                {t('notif.markAllRead')}
              </button>
            )}
            <Popover.Close asChild>
              <Button variant="icon" aria-label={t('common.close')} className={styles.close}>
                <X size={18} aria-hidden />
              </Button>
            </Popover.Close>
          </div>
          {rt.offline && (
            <div className={styles.offline} role="status">
              <TriangleAlert size={14} aria-hidden /> {t('rt.offlineShort')}{' '}
              {t('rt.reconnectIn', { s: rt.secondsToRetry })}
            </div>
          )}
          <div className={styles.body} aria-live="polite">
            {q.status === 'pending' && <SkeletonRows rows={3} columns={1} />}
            {q.status === 'error' && (
              <div className={styles.empty}>
                <CircleAlert size={20} aria-hidden />
                <span>{t('sys.unavailable.body')}</span>
              </div>
            )}
            {q.status === 'success' && items.length === 0 && (
              <div className={styles.empty}>
                <span className={styles.emptyTitle}>{t('notif.empty')}</span>
                <span className={styles.emptyBody}>{t('notif.emptyBody')}</span>
              </div>
            )}
            {items.length > 0 && (
              <ul className={styles.list}>
                {items.map((n) => {
                  const Icon = ICON[n.tone]
                  return (
                    <li key={n.id}>
                      <button
                        type="button"
                        className={clsx(styles.item, n.unread && styles.itemUnread)}
                        onClick={() => openItem(n)}
                      >
                        <span className={clsx(styles.icon, styles[n.tone])}>
                          <Icon size={14} strokeWidth={2.4} aria-hidden />
                        </span>
                        <span className={styles.text}>
                          <span className={clsx(styles.itemTitle, n.unread && styles.bold)}>{n.title}</span>
                          <span className={styles.itemBody}>{n.body}</span>
                          <span className={styles.meta}>
                            <span>{formatRelative(n.occurredAt, new Date(), activeOrg?.timezone)}</span>
                            {n.action && <span className={styles.action}>{n.action.label} ›</span>}
                          </span>
                        </span>
                        {n.unread ? <span className={styles.dot} aria-label={t('notif.unread')} /> : <span />}
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
