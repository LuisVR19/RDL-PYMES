import { describe, expect, it } from 'vitest'
import {
  emptyForm,
  formIssues,
  issueList,
  parseQuantity,
  serverIssues,
  toDraftInput,
  toDraftPatch,
  type DraftForm,
  type DraftLine,
} from './model'

const line = (p: Partial<DraftLine> = {}): DraftLine => ({
  key: 'l1',
  productId: 'p1',
  description: 'Tornillo',
  cabys: '0000000000001',
  productCurrency: 'CRC',
  quantity: '2',
  unitPrice: '75',
  discount: '',
  discountReason: '',
  ...p,
})

const form = (p: Partial<DraftForm> = {}): DraftForm => ({
  ...emptyForm(),
  customer: { id: 'c1', legalName: 'Ferretería El Roble S.A.' },
  lines: [line()],
  ...p,
})

describe('modelo del borrador', () => {
  it('cantidad del contrato: coma decimal, hasta 3 decimales, distinta de cero', () => {
    expect(parseQuantity('2,5')).toBe('2.5')
    expect(parseQuantity('2.000')).toBe('2')
    expect(parseQuantity('0')).toBeNull()
    expect(parseQuantity('1,2345')).toBeNull()
    expect(parseQuantity('abc')).toBeNull()
  })

  it('arma el cuerpo del contrato; el portal no manda impuestos ni totales', () => {
    expect(toDraftInput(form({ lines: [line({ unitPrice: '18 500,50' })] }), 'invoice')).toEqual({
      documentType: 'invoice',
      customerId: 'c1',
      saleConditionCode: '01',
      currency: 'CRC',
      lines: [{ productId: 'p1', quantity: '2', unitPrice: '18500.5' }],
    })
  })

  it('crédito lleva plazo; otra moneda, tipo de cambio', () => {
    const input = toDraftInput(
      form({ saleCondition: '02', creditTermDays: '30', currency: 'USD', exchangeRate: '505,5' }),
      'invoice',
    )
    expect(input).toMatchObject({ saleConditionCode: '02', creditTermDays: 30, exchangeRate: '505.5' })
  })

  it('no se guarda sin cliente ni con un descuento sin motivo', () => {
    expect(toDraftInput(form({ customer: null }), 'invoice')).toBeNull()
    const f = form({ lines: [line({ discount: '10' })] })
    expect(toDraftInput(f, 'invoice')).toBeNull()
    expect(issueList(f, formIssues(f))).toEqual(['Línea 1: El motivo es obligatorio cuando hay descuento.'])
  })

  it('producto en otra moneda: el precio es obligatorio', () => {
    const f = form({ lines: [line({ productCurrency: 'USD', unitPrice: '' })] })
    expect(formIssues(f).lines.l1?.unitPrice).toMatch(/Indique el precio en CRC/)
  })

  it('un borrador sin líneas se guarda, pero no se emite', () => {
    const f = form({ lines: [] })
    expect(toDraftInput(f, 'invoice')).not.toBeNull()
    expect(issueList(f, formIssues(f))).toEqual(['Agregue al menos una línea.'])
  })

  it('el PATCH borra lo que se quitó (sucursal, plazo, notas)', () => {
    const input = toDraftInput(form(), 'invoice')
    expect(input && toDraftPatch(input)).toMatchObject({
      branchId: null,
      creditTermDays: null,
      notes: '',
      exchangeRate: '1',
    })
  })

  it('los errores por campo de Billing caen en su línea, por posición', () => {
    const f = form({ lines: [line(), line({ key: 'l2' })] })
    const out = serverIssues([{ field: 'lines[1].discountReason', message: 'Falta el motivo' }], f)
    expect(out.lines).toEqual({ l2: { discountReason: 'Falta el motivo' } })
  })
})
