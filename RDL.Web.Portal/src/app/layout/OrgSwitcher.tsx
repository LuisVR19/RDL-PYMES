import * as Popover from '@radix-ui/react-popover'
import { clsx } from 'clsx'
import { Check, ChevronDown } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { Dialog } from '@/design-system/components/Dialog/Dialog'
import { useToast } from '@/design-system/components/Toast/Toast'
import type { Organization } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { ROLE_LABEL } from '@/shared/permissions/permissions'
import { useSession } from '@/shared/session/SessionProvider'
import { EnvBadge } from './EnvBadge'
import styles from './OrgSwitcher.module.css'

/**
 * Cambio de organización (pantalla 5). Si la página tiene cambios sin guardar, pide confirmación; al cambiar se
 * vacía la caché, se atenúa el contenido 180 ms, se vuelve al inicio y un toast confirma el rol nuevo.
 */
export function OrgSwitcher({ variant }: { variant: 'desktop' | 'mobile' }) {
  const { activeOrg, organizations, role, switchOrganization } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [pending, setPending] = useState<Organization | null>(null)

  const matches = useMemo(() => {
    const q = query.trim().toLocaleLowerCase('es-CR')
    return q ? organizations.filter((o) => o.legalName.toLocaleLowerCase('es-CR').includes(q)) : organizations
  }, [organizations, query])

  if (!activeOrg || !role) return null

  const done = (org: Organization) => {
    navigate('/')
    toast({
      tone: 'info',
      title: t('org.switch.done', { org: org.legalName }),
      body: t('topbar.org.roleHere', { role: ROLE_LABEL[org.role] }),
    })
  }

  const pick = (org: Organization) => {
    setOpen(false)
    setQuery('')
    if (org.id === activeOrg.id) return
    if (switchOrganization(org.id)) done(org)
    else setPending(org)
  }

  const confirmDiscard = () => {
    if (!pending) return
    switchOrganization(pending.id, { force: true })
    done(pending)
    setPending(null)
  }

  const roleLabel = ROLE_LABEL[role]

  return (
    <>
      <Popover.Root open={open} onOpenChange={setOpen}>
        <Popover.Trigger asChild>
          {variant === 'desktop' ? (
            <button
              type="button"
              className={styles.trigger}
              aria-label={t('topbar.org.current', { org: activeOrg.legalName, role: roleLabel })}
            >
              <span className={styles.initials}>{activeOrg.initials}</span>
              <span className={styles.names}>
                <span className={styles.name}>{activeOrg.legalName}</span>
                <span className={styles.role}>{roleLabel}</span>
              </span>
              <ChevronDown size={14} aria-hidden className={styles.caret} />
            </button>
          ) : (
            <button
              type="button"
              className={styles.mobileRow}
              aria-label={t('topbar.org.current', { org: activeOrg.legalName, role: roleLabel })}
            >
              <span className={clsx(styles.initials, styles.initialsMobile)}>{activeOrg.initials}</span>
              <span className={styles.names}>
                <span className={styles.nameMobile}>{activeOrg.legalName}</span>
                <span className={styles.roleMobile}>
                  {roleLabel} · <EnvBadge environment={activeOrg.environment} inline />
                </span>
              </span>
              <span className={styles.change}>
                {t('topbar.org.change')} <ChevronDown size={14} aria-hidden />
              </span>
            </button>
          )}
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            className={clsx(styles.menu, variant === 'mobile' && styles.menuMobile)}
            align="start"
            sideOffset={variant === 'mobile' ? 0 : 6}
            aria-label={t('topbar.org.title')}
          >
            {variant === 'mobile' && <div className={styles.menuTitle}>{t('topbar.org.titleMobile')}</div>}
            <input
              className={styles.search}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('topbar.org.search')}
              aria-label={t('topbar.org.search')}
            />
            <ul className={styles.list}>
              {matches.map((o) => {
                const active = o.id === activeOrg.id
                return (
                  <li key={o.id}>
                    <button
                      type="button"
                      className={clsx(styles.row, active && styles.rowActive)}
                      onClick={() => pick(o)}
                      aria-current={active || undefined}
                    >
                      <span className={styles.rowInitials}>{o.initials}</span>
                      <span className={styles.rowName}>{o.legalName}</span>
                      {active ? (
                        <span className={styles.tagActive}>
                          <Check size={14} aria-hidden /> {t('topbar.org.active')}
                        </span>
                      ) : (
                        <span className={styles.tag}>{ROLE_LABEL[o.role]}</span>
                      )}
                    </button>
                  </li>
                )
              })}
              {matches.length === 0 && <li className={styles.noMatch}>{t('topbar.org.noMatch')}</li>}
            </ul>
            <div className={styles.sep} />
            <button
              type="button"
              className={styles.all}
              onClick={() => {
                setOpen(false)
                navigate('/organizaciones')
              }}
            >
              {t('topbar.org.all')}
            </button>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>

      <Dialog
        open={pending !== null}
        onOpenChange={(o) => !o && setPending(null)}
        title={t('topbar.dirty.title')}
        description={pending ? t('org.switch.dirty', { org: pending.legalName }) : undefined}
        footer={
          <>
            <Button variant="secondary" onClick={() => setPending(null)}>
              {t('topbar.dirty.stay')}
            </Button>
            <Button variant="primary" onClick={confirmDiscard}>
              {t('topbar.dirty.discard')}
            </Button>
          </>
        }
      />
    </>
  )
}
