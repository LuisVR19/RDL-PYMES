import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mockDataSource } from '@/shared/api/mock'
import { resetMockReceivables } from '@/shared/api/mock/receivables'
import type { DataSource } from '@/shared/api/ports'
import { formatMoney } from '@/shared/money/money'
import { renderApp } from '@/test/renderApp'
import { autoDistribute, checkApplications } from './model'

// El día de negocio del prototipo: 24/09/2026 en Costa Rica.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-24T18:00:00Z'))
})
afterEach(() => {
  vi.useRealTimers()
  resetMockReceivables()
})

// Como en home.test: el separador de miles (U+202F) se compara como espacio.
const money = (v: string, c: 'CRC' | 'USD') => formatMoney(v, c).replace(/\s/g, ' ')
const crc = (v: string) => money(v, 'CRC')
const field = (doc: string) => screen.getByRole('textbox', { name: `Monto a aplicar a ${doc}` })

describe('cobranza · reglas de presentación', () => {
  const accounts = [
    { id: 'viejo', balanceAmount: '85000' },
    { id: 'medio', balanceAmount: '40000' },
    { id: 'nuevo', balanceAmount: '63000' },
  ]

  it('repartir automáticamente: de la más antigua a la más nueva, cada una hasta su saldo', () => {
    expect(autoDistribute('150000', accounts)).toEqual({ viejo: '85000', medio: '40000', nuevo: '25000' })
    expect(autoDistribute('50000.5', accounts)).toEqual({ viejo: '50000.5', medio: '', nuevo: '' })
  })

  it('lo aplicado de más bloquea; más que el saldo de la cuenta es error de la fila; lo que sobra queda sin aplicar', () => {
    const over = checkApplications('150000', accounts, { viejo: '85000', medio: '40000', nuevo: '70 000' })
    expect(over.rowErrors).toEqual({ nuevo: 'overBalance' })
    expect(over.overPayment).toBe(true)
    expect(over.ok).toBe(false)
    const left = checkApplications('150000', accounts, { viejo: '85 000,00', medio: 'x' })
    expect(left.rowErrors).toEqual({ medio: 'invalid' })
    const fine = checkApplications('150000', accounts, { viejo: '85000', nuevo: '0' })
    expect(fine.ok).toBe(true)
    expect(fine.amounts).toEqual({ viejo: '85000' })
    expect(fine.remaining.toFixed()).toBe('65000')
  })
})

describe('pantalla 22 · cuentas por cobrar', () => {
  it('saldo, atraso y «Vencida» contra el día de negocio; abre la cuenta', async () => {
    const { router } = renderApp('/cobranza/cuentas')
    const table = await screen.findByRole('table', { name: 'Cuentas por cobrar' })
    const row = (await within(table).findByText('FAC-0000029')).closest('tr') as HTMLElement
    expect(within(row).getByText('Soluciones Ibis S.R.L.')).toBeInTheDocument()
    expect(within(row).getByText('! 20 días')).toBeInTheDocument()
    expect(within(row).getByText('Vencida')).toBeInTheDocument()
    const paid = (await within(table).findByText('FAC-0000035')).closest('tr') as HTMLElement
    expect(within(paid).getByText('Pagada')).toBeInTheDocument()
    expect(within(paid).getByText('—')).toBeInTheDocument()
    expect(screen.getByText('Saldo de esta página por moneda:')).toBeInTheDocument()
    await userEvent.setup({ advanceTimers: vi.advanceTimersByTime }).click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/cobranza/cuentas/r29'))
  })

  it('cada filtro es una sola consulta de Receivables y queda en la URL', async () => {
    const list = vi.fn(mockDataSource.receivables.list)
    const source: DataSource = { ...mockDataSource, receivables: { ...mockDataSource.receivables, list } }
    const { router } = renderApp('/cobranza/cuentas', { source })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await screen.findByText('FAC-0000040')
    await user.click(screen.getByRole('radio', { name: 'Vencidas' }))
    await waitFor(() => expect(router.state.location.search).toBe('?estado=overdue'))
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ overdue: true }))
    expect(await screen.findByText('FAC-0000029')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByText('FAC-0000040')).not.toBeInTheDocument())
  })

  it('solo lectura (contador): sin «Registrar pago»', async () => {
    renderApp('/cobranza/cuentas', { orgId: 'cm' })
    await screen.findByText('FAC-0000040')
    expect(screen.queryByRole('button', { name: 'Registrar pago' })).not.toBeInTheDocument()
  })
})

describe('pantalla 23 · aging', () => {
  it('cinco tramos de la moneda elegida, con tabla y total; nunca mezcla colones con dólares', async () => {
    const { router } = renderApp('/cobranza/aging')
    const table = await screen.findByRole('table', { name: 'Saldo por tramo y moneda' })
    const row = (name: string) => within(table).getByRole('row', { name: new RegExp(`^${name}`) })
    expect(within(row('1–30 días')).getByText(crc('137000'))).toBeInTheDocument()
    expect(within(row('31–60 días')).getByText(crc('20000'))).toBeInTheDocument()
    expect(within(row('61–90 días')).getByText(crc('54000'))).toBeInTheDocument()
    expect(within(row('Más de 90')).getByText(crc('0'))).toBeInTheDocument()
    await userEvent
      .setup({ advanceTimers: vi.advanceTimersByTime })
      .click(screen.getByRole('radio', { name: 'Dólares' }))
    await waitFor(() => expect(router.state.location.search).toBe('?moneda=USD'))
    expect(await screen.findByText(money('2730', 'USD'), { selector: 'tfoot td' })).toBeInTheDocument()
  })
})

describe('pantalla 24 · detalle de la cuenta', () => {
  it('tres cifras, aplicación revertida con su motivo, seguimientos y promesa pendiente', async () => {
    renderApp('/cobranza/cuentas/r29')
    expect(await screen.findByRole('heading', { level: 1, name: 'FAC-0000029' })).toBeInTheDocument()
    expect(screen.getByText('20 días de atraso')).toBeInTheDocument()
    expect(screen.getByText('Revertida')).toBeInTheDocument()
    expect(screen.getByText('Motivo: Cheque devuelto por el banco')).toBeInTheDocument()
    expect(
      await screen.findByText('Confirma que paga la próxima semana por transferencia.'),
    ).toBeInTheDocument()
    expect(await screen.findByText('Para el 30/09/2026')).toBeInTheDocument()
  })

  it('registra un seguimiento con alta rápida y cierra una promesa', async () => {
    renderApp('/cobranza/cuentas/r29')
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await user.click(await screen.findByRole('button', { name: '+ Nuevo seguimiento' }))
    await user.click(screen.getByRole('button', { name: 'Guardar' }))
    expect(await screen.findByText('Escriba una nota breve.')).toBeInTheDocument()
    await user.type(screen.getByLabelText(/^Nota/), 'Dejó mensaje con la contadora.')
    await user.click(screen.getByRole('button', { name: 'Guardar' }))
    expect(
      await screen.findByText('Dejó mensaje con la contadora.', { selector: 'li span' }),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Cumplida' }))
    expect(await screen.findByText('Promesa actualizada')).toBeInTheDocument()
  })

  it('una nota de crédito aparece como ajuste', async () => {
    renderApp('/cobranza/cuentas/r27')
    expect(await screen.findByText(`Nota de crédito · −${crc('3500')}`)).toBeInTheDocument()
  })

  it('solo lectura: sin alta rápida ni revertir', async () => {
    renderApp('/cobranza/cuentas/r34', { orgId: 'cm' })
    await screen.findByRole('heading', { level: 1, name: 'FAC-0000034' })
    expect(screen.queryByRole('button', { name: '+ Nuevo seguimiento' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Revertir…' })).not.toBeInTheDocument()
  })
})

describe('pantallas 26 y 27 · registrar un pago, revertir y anular', () => {
  it('reparte de la más antigua a la más nueva, bloquea lo que no cabe y registra pago y aplicaciones juntos', async () => {
    const createPayment = vi.fn(mockDataSource.receivables.createPayment)
    const source: DataSource = {
      ...mockDataSource,
      receivables: { ...mockDataSource.receivables, createPayment },
    }
    const { router } = renderApp('/cobranza/pagos/nuevo', { source })
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await user.click(await screen.findByRole('button', { name: 'Siguiente: aplicar ›' }))
    expect(screen.getByText('Elija el cliente que pagó.')).toBeInTheDocument()
    expect(screen.getByText('Escriba un monto mayor que cero.')).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText(/^Cliente/), 'c9')
    await user.type(screen.getByLabelText(/^Monto/, { selector: 'input' }), '150 000')
    await user.click(screen.getByRole('button', { name: 'Siguiente: aplicar ›' }))

    await waitFor(() => expect(field('FAC-0000029')).toHaveValue('85000'))
    expect(field('FAC-0000031')).toHaveValue('40000')
    expect(field('FAC-0000034')).toHaveValue('25000')

    await user.clear(field('FAC-0000034'))
    await user.type(field('FAC-0000034'), '70000')
    expect(await screen.findByText(`Supera el saldo de esta cuenta (${crc('63000')}).`)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Siguiente: confirmar ›' })).toBeDisabled()
    await user.clear(field('FAC-0000034'))
    await user.type(field('FAC-0000034'), '15000')
    expect(screen.getByText(/Quedarán .* sin aplicar/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Siguiente: confirmar ›' }))
    expect(screen.getByText('Se aplicará a')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Registrar pago' }))

    await waitFor(() => expect(router.state.location.pathname).toMatch(/^\/cobranza\/pagos\/pg\d+$/))
    expect(createPayment).toHaveBeenCalledTimes(1)
    expect(createPayment.mock.calls[0]?.[0]).toMatchObject({
      customerId: 'c9',
      amount: '150000',
      currency: 'CRC',
      receivedOn: '2026-09-24',
      paymentMethodCode: '04',
      applications: [
        { receivableId: 'r29', amount: '85000' },
        { receivableId: 'r31', amount: '40000' },
        { receivableId: 'r34', amount: '15000' },
      ],
    })
    expect(
      await screen.findByText(
        `Quedan ${crc('10000')} sin aplicar. Podrá aplicarlos cuando el cliente tenga cuentas abiertas en la misma moneda.`,
      ),
    ).toBeInTheDocument()
  })

  it('revertir una aplicación y anular el pago, con motivo', async () => {
    renderApp('/cobranza/pagos/pg18')
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    expect(await screen.findByRole('heading', { level: 1, name: 'Pago del 24/09/2026' })).toBeInTheDocument()
    expect(await screen.findByText('FAC-0000034')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Revertir…' }))
    const reverse = await screen.findByRole('dialog', { name: 'Revertir aplicación' })
    await user.type(within(reverse).getByRole('textbox'), 'Se aplicó a la factura equivocada')
    await user.click(within(reverse).getByRole('button', { name: 'Revertir aplicación' }))
    expect(await screen.findByText('Aplicación revertida')).toBeInTheDocument()
    expect(await screen.findByText('Revertida')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Anular pago…' }))
    const dialog = await screen.findByRole('dialog', { name: 'Anular el pago del 24/09/2026' })
    await user.type(within(dialog).getByRole('textbox'), 'Transferencia devuelta por el banco')
    await user.click(within(dialog).getByRole('button', { name: 'Anular pago' }))
    expect(
      await screen.findByText(/Pago anulado\. Motivo: «Transferencia devuelta por el banco»/),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Anular pago…' })).not.toBeInTheDocument()
  })

  it('un contador ve el pago pero no lo anula ni revierte', async () => {
    renderApp('/cobranza/pagos/pg18', { orgId: 'cm' })
    await screen.findByRole('heading', { level: 1, name: 'Pago del 24/09/2026' })
    expect(screen.queryByRole('button', { name: 'Anular pago…' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Revertir…' })).not.toBeInTheDocument()
  })
})
