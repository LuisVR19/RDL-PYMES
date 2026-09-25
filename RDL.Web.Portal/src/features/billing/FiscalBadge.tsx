import { StatusBadge } from '@/design-system/components/StatusBadge/StatusBadge'
import type { FiscalPart } from '@/shared/api/billing-types'
import { HACIENDA_DETAIL } from '@/shared/status/status'

/**
 * Estado del documento electrónico de una factura, tal como lo trajo el gateway (ADR 0004 del gateway):
 * - `available` → el estado de Hacienda (con «Firmando» o «Enviado» como detalle de «En proceso»);
 * - `unavailable` → «Estado no disponible»: no se pudo consultar, la fila se muestra igual (pantalla 12);
 * - `absent` → nada: un borrador todavía no tiene documento electrónico.
 */
export function FiscalBadge({ part, size }: { part: FiscalPart; size?: 'sm' | 'md' }) {
  if (part.availability === 'absent') return null
  if (part.availability === 'unavailable' || !part.status) {
    return <StatusBadge domain="hacienda" status="unavailable" size={size} />
  }
  const s = part.status.status
  return <StatusBadge domain="hacienda" status={s} detail={HACIENDA_DETAIL[s]} size={size} />
}
