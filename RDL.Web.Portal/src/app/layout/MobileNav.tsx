import * as RDialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { t } from '@/shared/i18n/t'
import { BrandMark } from './BrandMark'
import { NavList } from './Sidebar'
import styles from './MobileNav.module.css'

/** Menú móvil (prototipo): panel oscuro que deja 64 px a la derecha, ítems de 44 px, cierra al navegar. */
export function MobileNav({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <RDialog.Root open={open} onOpenChange={onOpenChange}>
      <RDialog.Portal>
        <RDialog.Overlay className={styles.overlay} />
        <RDialog.Content className={styles.panel}>
          <div className={styles.head}>
            <BrandMark />
            <RDialog.Title className="sr-only">{t('nav.menu')}</RDialog.Title>
            <RDialog.Description className="sr-only">{t('nav.menu')}</RDialog.Description>
            <RDialog.Close className={styles.close} aria-label={t('common.close')}>
              <X size={20} aria-hidden />
            </RDialog.Close>
          </div>
          <NavList variant="drawer" onNavigate={() => onOpenChange(false)} />
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  )
}
