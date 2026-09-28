import { useQueryClient } from '@tanstack/react-query'
import { Big } from 'big.js'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Invoice, InvoiceDraftInput } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { useSession } from '@/shared/session/SessionProvider'
import { LOCAL_CURRENCY, toDraftPatch } from './model'

export type SyncStatus = 'idle' | 'busy' | 'error'

/** Tras dejar de escribir, cuánto se espera para recalcular (una llamada por pausa, no una por tecla). */
export const RECALC_DELAY_MS = 700

const asApiError = (err: unknown) =>
  err instanceof ApiError
    ? err
    : new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' })

/**
 * Guarda el borrador en Billing y trae de vuelta sus totales (pantallas 13 y 16).
 *
 * Billing no tiene una ruta para «calcular sin guardar», así que recalcular ES guardar: el primer guardado lo pide el
 * usuario («Guardar borrador» o Ctrl S) y crea el borrador; desde ahí, cada cambio válido se guarda solo tras una
 * pausa y la respuesta trae los totales del servidor. Nunca hay dos envíos a la vez: si el usuario sigue escribiendo
 * mientras uno viaja, al volver se manda lo último.
 */
export function useDraftSync({
  initial,
  input,
  onCreated,
}: {
  /** El borrador ya guardado (edición) o null (nuevo). */
  initial: Invoice | null
  /** Lo que está en pantalla como cuerpo del contrato, o null si todavía no se puede guardar. */
  input: InvoiceDraftInput | null
  onCreated?: (invoice: Invoice) => void
}) {
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const queryClient = useQueryClient()
  const keyFor = useIdempotencyKey()
  const [saved, setSaved] = useState<Invoice | null>(initial)
  const [status, setStatus] = useState<SyncStatus>('idle')
  const [error, setError] = useState<ApiError | null>(null)
  // Lo último que Billing aceptó: si coincide con la pantalla, no hay nada sin guardar.
  const [sentJson, setSentJson] = useState<string | null>(() =>
    initial ? JSON.stringify(normalized(inputOf(initial))) : null,
  )
  const orgId = activeOrg?.id
  const inFlight = useRef(false)
  const latest = useRef(input)
  const savedRef = useRef(saved)
  useEffect(() => {
    latest.current = input
    savedRef.current = saved
  })

  const inputJson = input ? JSON.stringify(normalized(input)) : null
  const dirty = inputJson === null ? saved === null || sentJson === null : inputJson !== sentJson

  const save = useCallback(async (): Promise<Invoice | null> => {
    const body = latest.current
    if (!body || inFlight.current) return null
    inFlight.current = true
    setStatus('busy')
    setError(null)
    const json = JSON.stringify(normalized(body))
    try {
      const current = savedRef.current
      const result = current
        ? await ds.invoices.updateDraft(current.id, toDraftPatch(body))
        : await ds.invoices.createDraft(body, keyFor(body))
      setSaved(result)
      savedRef.current = result
      setSentJson(json)
      setStatus('idle')
      queryClient.setQueryData(['invoices', orgId, 'detail', result.id], result)
      if (!current) {
        void queryClient.invalidateQueries({ queryKey: ['invoices', orgId, 'list'] })
        onCreated?.(result)
      }
      return result
    } catch (err) {
      setError(asApiError(err))
      setStatus('error')
      return null
    } finally {
      inFlight.current = false
    }
  }, [ds, keyFor, queryClient, orgId, onCreated])

  // Recalcular solo: ya guardado, algo cambió, se puede mandar y no hay un error esperando que el usuario lo vea.
  useEffect(() => {
    if (!saved || inputJson === null || inputJson === sentJson || status !== 'idle') return
    const id = window.setTimeout(() => void save(), RECALC_DELAY_MS)
    return () => window.clearTimeout(id)
  }, [saved, inputJson, sentJson, status, save])

  return { saved, status, error, dirty, save, setSaved }
}

/** «2.000» → «2»: Billing devuelve los decimales de la base; la pantalla los manda sin ceros de más. */
const canon = (v: string) => new Big(v).toFixed()

/**
 * El cuerpo que representa un borrador guardado, para comparar con la pantalla sin mandar nada. El motivo de una nota
 * no viene en la respuesta (el `Invoice` del contrato no lo trae): una nota recién abierta se vuelve a guardar al
 * escribirlo.
 */
function inputOf(inv: Invoice): InvoiceDraftInput {
  return {
    documentType: inv.documentType,
    customerId: inv.customerId,
    ...(inv.branchId ? { branchId: inv.branchId } : {}),
    saleConditionCode: inv.saleConditionCode,
    ...(inv.creditTermDays !== undefined ? { creditTermDays: inv.creditTermDays } : {}),
    currency: inv.currency,
    ...(inv.currency !== LOCAL_CURRENCY ? { exchangeRate: canon(inv.exchangeRate) } : {}),
    ...(inv.notes ? { notes: inv.notes } : {}),
    lines: inv.lines.flatMap((l) =>
      l.productId
        ? [
            {
              productId: l.productId,
              quantity: canon(l.quantity),
              unitPrice: canon(l.unitPrice),
              ...(new Big(l.discount).gt(0)
                ? { discount: canon(l.discount), discountReason: l.discountReason ?? '' }
                : {}),
            },
          ]
        : [],
    ),
  }
}

/** Para comparar: sin el tipo de documento ni la factura de referencia, que no cambian. */
function normalized(i: InvoiceDraftInput) {
  const { documentType: _d, referencedInvoiceId: _r, ...rest } = i
  return rest
}
