import { describe, expect, it } from 'vitest'
import { formatAmount, formatMoney, MINUS, parseMoneyInput, sumMoney, THIN_NBSP as S } from './money'

describe('formatMoney', () => {
  it('formatea colones y dólares como el diseño', () => {
    expect(formatMoney('113000', 'CRC')).toBe(`₡113${S}000,00`)
    expect(formatMoney('1250', 'USD')).toBe(`US$1${S}250,00`)
    expect(formatMoney('8420300.5', 'CRC')).toBe(`₡8${S}420${S}300,50`)
    expect(formatMoney('0', 'CRC')).toBe('₡0,00')
  })

  it('redondea a 2 decimales mitad hacia arriba solo para mostrar', () => {
    expect(formatMoney('129.9987', 'CRC')).toBe('₡130,00')
    expect(formatMoney('1039.985', 'CRC')).toBe(`₡1${S}039,99`)
    expect(formatMoney('0.005', 'CRC')).toBe('₡0,01')
  })

  it('usa el signo menos tipográfico', () => {
    expect(formatMoney('-5000', 'CRC')).toBe(`${MINUS}₡5${S}000,00`)
  })

  it('formatAmount omite la moneda', () => {
    expect(formatAmount('3582.4896')).toBe(`3${S}582,49`)
  })
})

describe('parseMoneyInput', () => {
  it.each([
    ['113 000,00', '113000'],
    ['113000', '113000'],
    ['₡113 000,00', '113000'],
    ['US$1 250,5', '1250.5'],
    ['113.000,50', '113000.5'],
    [`₡8${S}420${S}300,50`, '8420300.5'],
    ['0,00001', '0.00001'],
    ['1300.50', '1300.5'],
  ])('«%s» → %s', (input, expected) => {
    expect(parseMoneyInput(input)).toBe(expected)
  })

  it.each(['', 'abc', '-5', '1,2,3', '0,000001', '10000000000000'])('rechaza «%s»', (input) => {
    expect(parseMoneyInput(input)).toBeNull()
  })
})

describe('sumMoney', () => {
  it('suma sin perder decimales', () => {
    expect(sumMoney(['0.1', '0.2', '2542.5', '1039.9896'])).toBe('3582.7896')
  })
})
