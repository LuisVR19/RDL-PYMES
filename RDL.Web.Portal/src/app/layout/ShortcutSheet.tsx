import { Fragment } from 'react'
import { Dialog } from '@/design-system/components/Dialog/Dialog'
import { t } from '@/shared/i18n/t'
import styles from './ShortcutSheet.module.css'

/** Hoja de atajos (pantalla 36), textos y agrupación del prototipo. */
const GROUPS: { title: string; rows: { label: string; keys: string[] }[] }[] = [
  {
    title: 'General',
    rows: [
      { label: t('shortcuts.search'), keys: ['/'] },
      { label: 'Ver esta hoja de atajos', keys: ['?'] },
      { label: 'Cerrar menú, panel o diálogo', keys: ['Esc'] },
    ],
  },
  {
    title: 'Documentos',
    rows: [
      { label: t('shortcuts.newInvoice'), keys: ['N'] },
      { label: 'Guardar (borrador o formulario)', keys: ['Ctrl', 'S'] },
    ],
  },
  {
    title: 'Tablas',
    rows: [
      { label: 'Fila anterior / siguiente', keys: ['↑', '↓'] },
      { label: 'Abrir la fila activa', keys: ['Enter'] },
      { label: 'Seleccionar la fila activa', keys: ['Espacio'] },
      { label: 'Expandir / contraer fila', keys: ['→', '←'] },
    ],
  },
]

export function ShortcutSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange} title={t('shortcuts.title')} width={560}>
      <div className={styles.body}>
        {GROUPS.map((g) => (
          <Fragment key={g.title}>
            <h3 className={styles.section}>{g.title}</h3>
            <dl className={styles.list}>
              {g.rows.map((r) => (
                <div key={r.label} className={styles.row}>
                  <dt>{r.label}</dt>
                  <dd className={styles.keys}>
                    {r.keys.map((k) => (
                      <kbd key={k} className={styles.key}>
                        {k}
                      </kbd>
                    ))}
                  </dd>
                </div>
              ))}
            </dl>
          </Fragment>
        ))}
      </div>
    </Dialog>
  )
}
