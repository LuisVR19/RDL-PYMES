import { FileText, Plus, Users } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { Link } from 'react-router'
import { MODULE_LABEL, OVERLAY_SCREENS, SCREENS } from '@/app/screens'
import { Button } from '@/design-system/components/Button/Button'
import { DataTable, type Column } from '@/design-system/components/DataTable/DataTable'
import { ConfirmDialog, Drawer } from '@/design-system/components/Dialog/Dialog'
import {
  EmptyState,
  ErrorState,
  InlineAlert,
  SkeletonRows,
} from '@/design-system/components/Feedback/Feedback'
import { Select, TextArea, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import { Kbd, Panel, PanelHeader, Tag, Tooltip } from '@/design-system/components/Surface/Surface'
import { TabPanel, Tabs } from '@/design-system/components/Tabs/Tabs'
import { useToast } from '@/design-system/components/Toast/Toast'
import { SearchField, SegmentedControl, Toolbar } from '@/design-system/components/Toolbar/Toolbar'
import { formatBusinessDate } from '@/shared/dates/dates'
import { t } from '@/shared/i18n/t'
import { formatMoney } from '@/shared/money/money'
import { STATUS, type StatusDomain } from '@/shared/status/status'
import styles from './CatalogPage.module.css'

interface DemoRow {
  id: string
  number: string
  client: string
  date: string
  total: string
  hacienda: 'accepted' | 'processing' | 'rejected' | 'contingency'
}

const ROWS: DemoRow[] = [
  {
    id: '1',
    number: 'FAC-0000040',
    client: 'Café Monteazul S.A.',
    date: '2026-09-24',
    total: '452500.00',
    hacienda: 'accepted',
  },
  {
    id: '2',
    number: 'FAC-0000039',
    client: 'Soluciones Ibis S.R.L.',
    date: '2026-09-24',
    total: '113000.00',
    hacienda: 'processing',
  },
  {
    id: '3',
    number: 'FAC-0000038',
    client: 'Laura Jiménez Solís',
    date: '2026-09-23',
    total: '28250.00',
    hacienda: 'rejected',
  },
  {
    id: '4',
    number: 'FAC-0000037',
    client: 'Transportes Cerro Verde S.A.',
    date: '2026-09-22',
    total: '-5650.00',
    hacienda: 'contingency',
  },
]

const COLUMNS: Column<DemoRow>[] = [
  { key: 'number', header: 'Número', width: '150px', cell: (r) => <span className="code">{r.number}</span> },
  { key: 'client', header: 'Cliente', cell: (r) => r.client },
  {
    key: 'date',
    header: 'Fecha',
    width: '120px',
    hideOnMobile: true,
    cell: (r) => formatBusinessDate(r.date),
  },
  {
    key: 'hacienda',
    header: 'Hacienda',
    width: '190px',
    cell: (r) => <StatusBadge domain="hacienda" status={r.hacienda} />,
  },
  {
    key: 'total',
    header: 'Total',
    width: '150px',
    align: 'right',
    cell: (r) => <span className="amount">{formatMoney(r.total, 'CRC')}</span>,
  },
]

/**
 * Catálogo del sistema de diseño (solo con datos simulados, ruta `/_catalogo`). Reemplaza a Storybook en esta
 * etapa: cada componente en sus variantes y estados, con los mismos tokens del producto.
 */
export function CatalogPage() {
  const toast = useToast()
  const [tab, setTab] = useState('all')
  const [segment, setSegment] = useState<'active' | 'inactive' | 'all'>('active')
  const [tableState, setTableState] = useState<'success' | 'pending' | 'error' | 'empty'>('success')
  const [confirm, setConfirm] = useState<null | 'emit' | 'void' | 'prod'>(null)
  const [drawer, setDrawer] = useState(false)

  return (
    <div className={styles.page}>
      <PageHeader
        section="Solo diseño"
        title="Catálogo de componentes"
        subtitle="Tokens, componentes y estados del sistema de diseño."
      />

      <Section title="Botones">
        <div className={styles.row}>
          <Button variant="primary" icon={<Plus size={16} aria-hidden />} kbd="N">
            Nueva factura
          </Button>
          <Button variant="secondary">Guardar borrador</Button>
          <Button variant="tertiary">Cancelar</Button>
          <Button variant="danger">Anular…</Button>
          <Button variant="primary" loading loadingLabel="Emitiendo…">
            Emitir factura…
          </Button>
          <Button variant="secondary" disabled>
            Deshabilitado
          </Button>
          <Tooltip content="Descargar XML">
            <Button variant="icon" aria-label="Descargar XML">
              <FileText size={18} aria-hidden />
            </Button>
          </Tooltip>
        </div>
        <div className={styles.row}>
          <Button size="sm">Pequeño 32</Button>
          <Button size="md">Mediano 36</Button>
          <Button size="lg">Grande 44 (táctil)</Button>
        </div>
      </Section>

      <Section title="Estados (StatusBadge)">
        {(Object.keys(STATUS) as StatusDomain[]).map((domain) => (
          <div key={domain} className={styles.badgeRow}>
            <span className={styles.badgeDomain}>{domain}</span>
            {Object.keys(STATUS[domain]).map((status) => (
              <StatusBadge key={status} domain={domain} status={status as never} />
            ))}
          </div>
        ))}
        <div className={styles.badgeRow}>
          <span className={styles.badgeDomain}>detalle</span>
          <StatusBadge domain="hacienda" status="signed" detail="Firmando" />
          <StatusBadge domain="receivable" status="overdue" detail="12 días" />
          <Tag>001 · Central</Tag>
          <Tag>Crédito 30 días</Tag>
        </div>
      </Section>

      <Section title="Montos y fechas">
        <dl className={styles.facts}>
          <dt>Colones</dt>
          <dd className="amount">{formatMoney('113000', 'CRC')}</dd>
          <dt>Dólares</dt>
          <dd className="amount">{formatMoney('1250.5', 'USD')}</dd>
          <dt>Negativo</dt>
          <dd className="amount">{formatMoney('-5650', 'CRC')}</dd>
          <dt>Redondeo half-up</dt>
          <dd className="amount">
            0,125 → {formatMoney('0.125', 'CRC')} · 0,135 → {formatMoney('0.135', 'CRC')}
          </dd>
          <dt>Fecha de negocio</dt>
          <dd>{formatBusinessDate('2026-10-24')}</dd>
          <dt>Atajo</dt>
          <dd>
            <Kbd>/</Kbd> buscar · <Kbd>?</Kbd> atajos
          </dd>
        </dl>
      </Section>

      <Section title="Campos">
        <div className={styles.grid2}>
          <TextField label="Razón social" required placeholder="Comercial Los Almendros S.A." />
          <TextField label="Correo" required defaultValue="compras@" error="Escriba un correo válido." />
          <Select
            label="Tipo de identificación"
            required
            options={[
              { value: 'juridica', label: 'Jurídica' },
              { value: 'fisica', label: 'Física' },
              { value: 'dimex', label: 'DIMEX' },
              { value: 'nite', label: 'NITE' },
            ]}
          />
          <TextField label="Teléfono" help="Opcional. 8 dígitos." />
          <TextArea label="Motivo" minLength={10} rows={3} className={styles.span2} />
        </div>
      </Section>

      <Section title="Barra de herramientas y pestañas">
        <Toolbar>
          <SearchField aria-label="Buscar clientes" placeholder="Buscar por nombre o identificación" />
          <SegmentedControl
            label="Filtro"
            value={segment}
            onChange={setSegment}
            options={[
              { value: 'active', label: 'Activos', count: 9 },
              { value: 'inactive', label: 'Inactivos', count: 1 },
              { value: 'all', label: 'Todos' },
            ]}
          />
        </Toolbar>
        <Tabs
          label="Bandeja"
          value={tab}
          onValueChange={setTab}
          items={[
            { value: 'rejected', label: 'Rechazados', count: 2, countTone: 'danger' },
            { value: 'contingency', label: 'Contingencia', count: 1, countTone: 'warning' },
            { value: 'error', label: 'Con error', count: 0 },
            { value: 'all', label: 'Todos' },
          ]}
        >
          {['rejected', 'contingency', 'error', 'all'].map((v) => (
            <TabPanel key={v} value={v} className={styles.tabNote}>
              Contenido de la pestaña «{v}».
            </TabPanel>
          ))}
        </Tabs>
      </Section>

      <Section title="Tabla de datos">
        <div className={styles.row}>
          {(['success', 'pending', 'empty', 'error'] as const).map((s) => (
            <Button
              key={s}
              size="sm"
              variant={tableState === s ? 'primary' : 'secondary'}
              onClick={() => setTableState(s)}
            >
              {{ success: 'Con datos', pending: 'Cargando', empty: 'Vacía', error: 'Error' }[s]}
            </Button>
          ))}
        </div>
        <DataTable
          label="Documentos de ejemplo"
          columns={COLUMNS}
          rows={tableState === 'empty' ? [] : ROWS}
          rowKey={(r) => r.id}
          status={tableState === 'empty' ? 'success' : tableState}
          onRowClick={(r) => toast({ tone: 'info', title: `Abrir ${r.number}` })}
          rowTone={(r) => (r.hacienda === 'rejected' ? 'danger' : undefined)}
          footer="4 documentos en esta página"
          cursor={{ hasPrev: false, hasNext: true, onPrev: () => {}, onNext: () => {} }}
          empty={
            <EmptyState
              icon={FileText}
              title="Todavía no hay documentos"
              body="Cuando emita su primera factura aparecerá aquí."
              action={<Button variant="primary">{t('invoice.new')}</Button>}
            />
          }
        />
      </Section>

      <Section title="Avisos y estados de carga">
        <div className={styles.stack}>
          <InlineAlert tone="info">{t('hacienda.processing')}</InlineAlert>
          <InlineAlert tone="success" title="Aceptada por Hacienda">
            FAC-0000040 · 24/09/2026 14:55
          </InlineAlert>
          <InlineAlert tone="warning" actions={<Button size="sm">Subir certificado</Button>}>
            {t('hacienda.cert.expiring', { date: '12/10/2026', days: 18 })}
          </InlineAlert>
          <InlineAlert
            tone="danger"
            title={t('hacienda.rejected')}
            refCode="3f2b8c1e-9d4a-4b7e-8c21-000000000001"
          >
            El receptor no está inscrito.
          </InlineAlert>
          <Panel padded={false}>
            <SkeletonRows rows={3} columns={4} />
          </Panel>
          <Panel padded={false}>
            <EmptyState
              icon={Users}
              title="Todavía no tiene clientes"
              body="Agregue su primer cliente para facturarle."
            />
          </Panel>
          <Panel padded={false}>
            <ErrorState onRetry={() => {}} refCode="3f2b8c1e-9d4a-4b7e-8c21-000000000002" />
          </Panel>
        </div>
      </Section>

      <Section title="Diálogos, panel lateral y toasts">
        <div className={styles.row}>
          <Button onClick={() => setConfirm('emit')}>Confirmar con resumen</Button>
          <Button onClick={() => setConfirm('void')}>Confirmar con motivo</Button>
          <Button onClick={() => setConfirm('prod')}>Confirmar escribiendo</Button>
          <Button onClick={() => setDrawer(true)}>Panel lateral</Button>
          <Button
            onClick={() =>
              toast({
                tone: 'success',
                title: t('invoice.issued', { n: 'FAC-0000041' }),
                body: t('invoice.issuedSub'),
              })
            }
          >
            Toast éxito
          </Button>
          <Button
            onClick={() =>
              toast({ tone: 'danger', title: 'No pudimos guardar', body: 'Se queda hasta que lo cierre.' })
            }
          >
            Toast error
          </Button>
        </div>
      </Section>

      <Section title={`Pantallas (${SCREENS.length} rutas + ${OVERLAY_SCREENS.length} diálogos/paneles)`}>
        <ul className={styles.screens}>
          {SCREENS.map((s) => (
            <li key={s.id}>
              <Link to={s.path.replace(/:\w+/g, 'demo')}>
                <span className={styles.screenN}>{s.n}</span> {s.name}
              </Link>
              <span className={styles.screenModule}>{MODULE_LABEL[s.module]}</span>
            </li>
          ))}
        </ul>
      </Section>

      <ConfirmDialog
        open={confirm === 'emit'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t('invoice.emit.title')}
        warning={t('invoice.emit.warning')}
        summary={[
          { label: 'Cliente', value: 'Café Monteazul S.A.' },
          { label: 'Líneas', value: '3' },
          { label: 'Total', value: <span className="amount">{formatMoney('452500', 'CRC')}</span> },
        ]}
        confirmLabel="Emitir factura"
        confirmingLabel="Emitiendo…"
        onConfirm={() => setConfirm(null)}
      />
      <ConfirmDialog
        open={confirm === 'void'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t('invoice.void.title', { n: 'FAC-0000040' })}
        warning={t('invoice.void.body', {
          saldo: formatMoney('452500', 'CRC'),
          cero: formatMoney('0', 'CRC'),
        })}
        reasonRequired
        tone="danger"
        confirmLabel="Anular factura"
        onConfirm={() => setConfirm(null)}
      />
      <ConfirmDialog
        open={confirm === 'prod'}
        onOpenChange={(o) => !o && setConfirm(null)}
        title="Cambiar a producción"
        warning="Los documentos que emita a partir de ahora tendrán validez fiscal."
        typeToConfirm="PRODUCCIÓN"
        tone="danger"
        confirmLabel="Cambiar a producción"
        onConfirm={() => setConfirm(null)}
      />
      <Drawer
        open={drawer}
        onOpenChange={setDrawer}
        title="Nuevo cliente"
        width={440}
        footer={
          <>
            <Button onClick={() => setDrawer(false)}>{t('common.cancel')}</Button>
            <Button variant="primary">Crear y usar en la factura</Button>
          </>
        }
      >
        <TextField label="Número de identificación" required placeholder="3-101-000000" />
        <TextField label="Razón social" required />
        <TextField label="Correo" required />
      </Drawer>
    </div>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Panel>
      <PanelHeader title={title} />
      <div className={styles.stack}>{children}</div>
    </Panel>
  )
}
