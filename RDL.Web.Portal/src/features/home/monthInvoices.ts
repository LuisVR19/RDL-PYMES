import type { InvoiceListItem } from '@/shared/api/billing-types'
import type { InvoicesPort } from '@/shared/api/ports'
import { ApiError } from '@/shared/api/types'

const PAGE = 100
/** Tope de páginas: 2 000 facturas en el mes. Más que eso no se suma a medias: la cifra queda no disponible. */
const MAX_PAGES = 20

/**
 * Facturas emitidas entre dos fechas de negocio (inclusive), recorriendo todas las páginas de Billing.
 *
 * TODO(api): Billing no tiene un resumen por periodo ni ordena del más reciente al más antiguo (`listInvoices`
 * ordena por `createdAt` ascendente), así que Inicio suma y elige las últimas en el portal. Propuesta a contratos:
 * `GET /v1/invoices/summary?issuedFrom&issuedTo` (totales por moneda y cantidad) y `sort=-issuedAt` en el listado.
 * Solo se muestra: la cifra oficial de cada documento la da Billing.
 */
export async function issuedInvoicesBetween(
  invoices: InvoicesPort,
  issuedFrom: string,
  issuedTo: string,
): Promise<InvoiceListItem[]> {
  const out: InvoiceListItem[] = []
  let cursor: string | undefined
  for (let page = 0; page < MAX_PAGES; page++) {
    const res = await invoices.list({
      documentType: 'invoice',
      status: 'issued',
      issuedFrom,
      issuedTo,
      cursor,
      limit: PAGE,
    })
    out.push(...res.items)
    if (!res.nextCursor) return out
    cursor = res.nextCursor
  }
  throw new ApiError({
    status: 0,
    type: 'urn:rdl:portal:problem:too-many-invoices',
    title: 'Demasiadas facturas para sumarlas en el portal',
    correlationId: '',
  })
}

/** Las más recientes primero, por su instante de emisión. */
export function latestIssued(items: InvoiceListItem[], n: number): InvoiceListItem[] {
  return items.toSorted((a, b) => instantMs(b.invoice.issuedAt) - instantMs(a.invoice.issuedAt)).slice(0, n)
}

// Un instante RFC 3339 con zona (nunca una fecha de negocio): Date.parse no lo corre de día.
const instantMs = (iso: string | undefined) => (iso ? Date.parse(iso) : 0)
