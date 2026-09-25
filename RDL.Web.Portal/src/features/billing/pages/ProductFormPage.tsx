import { useQuery, useQueryClient } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useNavigate, useParams } from 'react-router'
import { Button } from '@/design-system/components/Button/Button'
import { MoneyField, RadioChoice } from '@/design-system/components/Choice/Choice'
import { ErrorState, InlineAlert, SkeletonRows } from '@/design-system/components/Feedback/Feedback'
import { Select, TextField } from '@/design-system/components/Field/Field'
import { PageHeader } from '@/design-system/components/PageHeader/PageHeader'
import { useToast } from '@/design-system/components/Toast/Toast'
import { useDataSource } from '@/shared/api/DataSourceProvider'
import type { Product, ProductInput, ProductPatch, ProductTax, TaxOption } from '@/shared/api/billing-types'
import { useIdempotencyKey } from '@/shared/api/idempotency'
import { ApiError } from '@/shared/api/types'
import { t } from '@/shared/i18n/t'
import { parseMoneyInput, type Currency } from '@/shared/money/money'
import { useSession } from '@/shared/session/SessionProvider'
import { CabysDialog } from '../CabysDialog'
import { taxLabel, taxOptionKey } from '../products'
import styles from './ProductFormPage.module.css'

type Kind = 'good' | 'service'
type Form = {
  code: string
  kind: Kind
  description: string
  cabysCode: string
  cabysDescription?: string
  unit: string
  price: string
  currency: Currency
  taxKey: string
}
type Errors = Partial<Record<'code' | 'description' | 'cabysCode' | 'unit' | 'price', string>>

const EMPTY: Form = {
  code: '',
  kind: 'good',
  description: '',
  cabysCode: '',
  unit: 'Unid',
  price: '',
  currency: 'CRC',
  taxKey: 'none',
}

const SERVER_FIELD: Record<string, keyof Errors> = {
  code: 'code',
  description: 'description',
  cabysCode: 'cabysCode',
  unitOfMeasureCode: 'unit',
  unitPrice: 'price',
}

/** Pantalla 11 · Nuevo producto o servicio / Editar producto (prototipo «11»). La ruta con `:id` edita. */
export function ProductFormPage() {
  const { id } = useParams()
  const ds = useDataSource()
  const { activeOrg } = useSession()
  const org = activeOrg?.id
  const existing = useQuery({
    queryKey: ['products', org, 'detail', id],
    queryFn: () => ds.products.get(id ?? ''),
    enabled: Boolean(id) && !!org,
  })
  // Catálogos de E-Invoice: si no responden, el formulario degrada (unidad como código, sin impuesto).
  const units = useQuery({
    queryKey: ['catalogs', org, 'units'],
    queryFn: () => ds.catalogs.unitsOfMeasure(),
    enabled: !!org,
    retry: false,
    staleTime: Infinity,
  })
  const taxes = useQuery({
    queryKey: ['catalogs', org, 'taxOptions'],
    queryFn: () => ds.catalogs.taxOptions(),
    enabled: !!org,
    retry: false,
    staleTime: Infinity,
  })

  if ((id && existing.isPending) || units.isPending || taxes.isPending)
    return <SkeletonRows rows={6} columns={2} />
  if (id && existing.isError) {
    return (
      <ErrorState
        onRetry={() => void existing.refetch()}
        refCode={existing.error instanceof ApiError ? existing.error.correlationId || undefined : undefined}
      />
    )
  }
  return (
    <ProductForm key={id ?? 'nuevo'} product={existing.data} units={units.data} taxOptions={taxes.data} />
  )
}

function ProductForm({
  product,
  units,
  taxOptions,
}: {
  product?: Product
  units?: { code: string; name: string }[]
  taxOptions?: TaxOption[]
}) {
  const ds = useDataSource()
  const { activeOrg, setDirty } = useSession()
  const navigate = useNavigate()
  const toast = useToast()
  const queryClient = useQueryClient()
  const keyFor = useIdempotencyKey()
  const initial: Form = product
    ? {
        code: product.code,
        kind: product.isService ? 'service' : 'good',
        description: product.description,
        cabysCode: product.cabysCode,
        unit: product.unitOfMeasureCode,
        // Tal cual, con coma decimal: formatear a 2 decimales perdería precisión (Billing admite 5).
        price: product.unitPrice.replace('.', ','),
        currency: product.currency,
        taxKey: taxOptionKey(product.taxes, taxOptions),
      }
    : EMPTY
  const [form, setForm] = useState<Form>(initial)
  const [errors, setErrors] = useState<Errors>({})
  const [serverError, setServerError] = useState<ApiError | null>(null)
  const [saving, setSaving] = useState(false)
  const [cabysOpen, setCabysOpen] = useState(false)
  // El producto guarda solo el código: su descripción sale del catálogo, si responde.
  const cabysLookup = useQuery({
    queryKey: ['catalogs', activeOrg?.id, 'cabys', form.cabysCode],
    queryFn: () => ds.catalogs.searchCabys(form.cabysCode),
    enabled: Boolean(form.cabysCode) && form.cabysDescription === undefined,
    retry: false,
    staleTime: Infinity,
  })
  const cabysDescription =
    form.cabysDescription ?? cabysLookup.data?.find((c) => c.code === form.cabysCode)?.description

  const dirty = JSON.stringify(form) !== JSON.stringify(initial)
  useEffect(() => {
    setDirty(dirty)
    return () => setDirty(false)
  }, [dirty, setDirty])

  const set = <K extends keyof Form>(k: K, v: Form[K]) => {
    setForm((f) => ({ ...f, [k]: v }))
    setErrors((e) => ({ ...e, [k === 'currency' ? 'price' : k]: undefined }))
  }

  // Opciones de impuesto: «sin impuesto», las del catálogo y, al editar, la actual si el catálogo no la conoce.
  const taxChoices = [
    { value: 'none', label: t('products.tax.none') },
    ...(taxOptions ?? []).map((o) => ({ value: o.key, label: o.label })),
    ...(product && initial.taxKey === 'current'
      ? [{ value: 'current', label: taxLabel(product.taxes, taxOptions) }]
      : []),
  ]
  const taxesFor = (key: string): ProductTax[] =>
    key === 'none'
      ? []
      : key === 'current'
        ? (product?.taxes ?? [])
        : (taxOptions?.find((o) => o.key === key)?.taxes ?? [])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const price = parseMoneyInput(form.price)
    const next: Errors = {}
    if (!form.code.trim()) next.code = t('product.codeRequired')
    if (!form.description.trim()) next.description = t('product.descriptionRequired')
    if (!form.cabysCode) next.cabysCode = t('product.cabysRequired')
    if (!form.unit.trim()) next.unit = t('product.unitCodeHelp')
    if (price === null || price === '0') next.price = t('product.priceInvalid')
    setErrors(next)
    setServerError(null)
    if (Object.keys(next).length > 0 || price === null) return

    const input: ProductInput = {
      code: form.code.trim(),
      description: form.description.trim(),
      cabysCode: form.cabysCode,
      unitOfMeasureCode: form.unit.trim(),
      unitPrice: price,
      currency: form.currency,
      isService: form.kind === 'service',
      taxes: taxesFor(form.taxKey),
    }
    setSaving(true)
    try {
      const saved = product
        ? await ds.products.update(product.id, patchOf(product, input))
        : await ds.products.create(input, keyFor(input))
      setDirty(false)
      await queryClient.invalidateQueries({ queryKey: ['products', activeOrg?.id] })
      toast({
        tone: 'success',
        title: t(product ? 'product.updated' : 'product.created'),
        body: saved.description,
      })
      navigate('/productos', { replace: true })
    } catch (err) {
      const api = err instanceof ApiError ? err : null
      if (api?.is('product-code-taken')) return setErrors({ code: t('product.codeTaken') })
      const fields: Errors = {}
      for (const fe of api?.errors ?? []) {
        const k = SERVER_FIELD[fe.field]
        if (k) fields[k] = fe.message
      }
      if (Object.keys(fields).length > 0) setErrors(fields)
      else
        setServerError(api ?? new ApiError({ status: 0, type: 'about:blank', title: '', correlationId: '' }))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className={styles.page} onSubmit={submit} noValidate>
      <PageHeader
        back={{ label: t('products.title'), to: '/productos' }}
        title={t(product ? 'product.edit.title' : 'product.new.title')}
      />
      {product && <InlineAlert tone="info">{t('product.edit.notice')}</InlineAlert>}
      {serverError && (
        <InlineAlert tone="danger" refCode={serverError.correlationId || undefined}>
          {t('product.serverError')}
        </InlineAlert>
      )}

      <div className={styles.card}>
        <TextField
          label={t('product.code')}
          required
          className={styles.mono}
          value={form.code}
          error={errors.code}
          onChange={(e) => set('code', e.target.value)}
        />
        <RadioChoice<Kind>
          label={t('product.kind')}
          value={form.kind}
          onChange={(v) => set('kind', v)}
          options={[
            { value: 'good', label: t('products.kind.good') },
            { value: 'service', label: t('products.kind.service') },
          ]}
        />
        <TextField
          className={styles.full}
          label={t('product.description')}
          required
          value={form.description}
          error={errors.description}
          onChange={(e) => set('description', e.target.value)}
        />

        <div className={styles.full}>
          <span className={styles.label}>
            {t('product.cabys')} <span className={styles.required}>*</span>
          </span>
          <div className={styles.cabysRow}>
            <div className={errors.cabysCode ? styles.cabysBoxInvalid : styles.cabysBox} aria-live="polite">
              <span className={styles.cabysCode}>{form.cabysCode || t('product.cabysNone')}</span>
              <span className={styles.cabysDesc}>{cabysDescription ?? t('product.cabysHint')}</span>
            </div>
            <Button type="button" variant="secondary" onClick={() => setCabysOpen(true)}>
              {t('product.cabysSearch')}
            </Button>
          </div>
          {errors.cabysCode && (
            <span className={styles.error} role="alert">
              {errors.cabysCode}
            </span>
          )}
          <span className={styles.warning}>
            <TriangleAlert size={12} aria-hidden /> {t('product.cabysIllustrative')}
          </span>
        </div>

        {units && units.length > 0 ? (
          <Select
            label={t('product.unit')}
            value={form.unit}
            options={withCurrent(units, form.unit).map((u) => ({ value: u.code, label: u.name }))}
            onChange={(e) => set('unit', e.target.value)}
          />
        ) : (
          <TextField
            label={t('product.unitCode')}
            required
            help={t('product.unitCodeHelp')}
            value={form.unit}
            error={errors.unit}
            onChange={(e) => set('unit', e.target.value)}
          />
        )}
        <MoneyField
          label={t('product.price')}
          required
          amount={form.price}
          currency={form.currency}
          onAmountChange={(v) => set('price', v)}
          onCurrencyChange={(c) => set('currency', c)}
          error={errors.price}
        />
        <div>
          <Select
            label={t('product.tax')}
            value={form.taxKey}
            options={taxChoices}
            help={taxOptions ? t('product.taxHelp') : undefined}
            onChange={(e) => set('taxKey', e.target.value)}
          />
          {!taxOptions && <p className={styles.taxUnavailable}>{t('product.taxUnavailable')}</p>}
        </div>
      </div>

      <div className={styles.actions}>
        <Button type="button" variant="secondary" onClick={() => navigate('/productos')}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" variant="primary" disabled={saving}>
          {t(saving ? 'product.saving' : 'product.save')}
        </Button>
      </div>

      <CabysDialog
        open={cabysOpen}
        onOpenChange={setCabysOpen}
        onPick={(c) => {
          setForm((f) => ({ ...f, cabysCode: c.code, cabysDescription: c.description }))
          setErrors((e) => ({ ...e, cabysCode: undefined }))
        }}
      />
    </form>
  )
}

/** Si el producto usa una unidad que el catálogo ya no trae, se conserva como opción (no se cambia sola). */
function withCurrent(units: { code: string; name: string }[], current: string) {
  return units.some((u) => u.code === current) || !current
    ? units
    : [...units, { code: current, name: current }]
}

/** Solo lo que cambió respecto del producto (PATCH: campo ausente = no cambia). */
function patchOf(before: Product, next: ProductInput): ProductPatch {
  const patch: ProductPatch = {}
  if (next.code !== before.code) patch.code = next.code
  if (next.description !== before.description) patch.description = next.description
  if (next.cabysCode !== before.cabysCode) patch.cabysCode = next.cabysCode
  if (next.unitOfMeasureCode !== before.unitOfMeasureCode) patch.unitOfMeasureCode = next.unitOfMeasureCode
  if (next.unitPrice !== before.unitPrice) patch.unitPrice = next.unitPrice
  if (next.currency !== before.currency) patch.currency = next.currency
  if (next.isService !== before.isService) patch.isService = next.isService
  if (JSON.stringify(next.taxes) !== JSON.stringify(before.taxes)) patch.taxes = next.taxes
  return patch
}
