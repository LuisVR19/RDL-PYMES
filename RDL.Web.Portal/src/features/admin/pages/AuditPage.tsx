import { Button } from '@/design-system/components/Button/Button'
import { InlineAlert } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { Tag } from '@/design-system/components/Surface/Surface'
import { t } from '@/shared/i18n/t'
import styles from './AuditPage.module.css'

// Propuesta del diseño (prototipo «32»): columnas del CSV y tipos de acción. No hay contrato que las respalde.
const COLUMNS = [
  'Fecha y hora',
  'Usuario',
  'Rol',
  'Acción',
  'Documento o recurso',
  'Detalle',
  'Resultado',
] as const

const ACTIONS = [
  'Emisión de documentos',
  'Anulaciones',
  'Pagos y aplicaciones',
  'Configuración fiscal',
  'Usuarios e invitaciones',
] as const

/**
 * Pantalla 32 · Exportar auditoría (prototipo «32»).
 *
 * TODO(api): el diseño lo marca «depende de una API todavía no definida». Ningún servicio expone el schema `audit`
 * (append-only, ownership del repo de contratos) ni hay una ruta de exportación en el gateway. La pantalla muestra
 * la propuesta con el formulario deshabilitado: no simula una descarga que no existe. Cuando el contrato defina la
 * ruta (filtros, formato y límites), se habilita aquí.
 */
export function AuditPage() {
  return (
    <div className={styles.page}>
      <PageHeader
        section={t('nav.section.admin')}
        title={t('nav.audit')}
        subtitle={t('admin.audit.subtitle')}
      />
      <InlineAlert tone="warning" title={t('admin.audit.unavailableTitle')}>
        {t('admin.audit.unavailableBody')}
      </InlineAlert>
      <fieldset className={styles.card} disabled aria-describedby="auditoria-propuesta">
        <legend className="sr-only">{t('admin.audit.filters')}</legend>
        <TextField label={t('admin.audit.from')} type="date" />
        <TextField label={t('admin.audit.to')} type="date" />
        <Select label={t('admin.audit.user')} options={[{ value: '', label: t('admin.audit.allUsers') }]} />
        <Select
          label={t('admin.audit.action')}
          options={[
            { value: '', label: t('admin.audit.allActions') },
            ...ACTIONS.map((a) => ({ value: a, label: a })),
          ]}
        />
        <div className={styles.columns}>
          <span id="auditoria-propuesta" className={styles.columnsTitle}>
            {t('admin.audit.columns')}
          </span>
          <div className={styles.tags}>
            {COLUMNS.map((c) => (
              <Tag key={c}>{c}</Tag>
            ))}
          </div>
        </div>
      </fieldset>
      <div className={styles.actions}>
        <Button variant="primary" disabled>
          {t('admin.audit.download')}
        </Button>
      </div>
    </div>
  )
}
