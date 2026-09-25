import { describe, expect, it } from 'vitest'
import { can, ROLES, type Role } from './permissions'
import { STATUS } from '../status/status'

const who = (cap: Parameters<typeof can>[1]) => ROLES.filter((r: Role) => can(r, cap))

describe('matriz de permisos (design/pantallas.md)', () => {
  it('facturación: todos menos el cobrador la ven; editan propietario, administrador y facturador', () => {
    expect(who('billing.view')).toEqual(['owner', 'admin', 'biller', 'accountant', 'read_only'])
    expect(who('billing.edit')).toEqual(['owner', 'admin', 'biller'])
  })

  it('cobranza: todos menos el facturador la ven', () => {
    expect(who('receivables.view')).toEqual(['owner', 'admin', 'collector', 'accountant', 'read_only'])
  })

  it('configuración fiscal: el contador solo la lee', () => {
    expect(can('accountant', 'fiscal.config.view')).toBe(true)
    expect(can('accountant', 'fiscal.config.edit')).toBe(false)
  })

  it('solo un propietario gestiona propietarios', () => {
    expect(who('admin.owners')).toEqual(['owner'])
  })

  it('solo lectura no edita nada', () => {
    const edits = [
      'billing.edit',
      'fiscal.config.edit',
      'fiscal.retry',
      'receivables.edit',
      'receivables.void',
      'admin.view',
    ] as const
    for (const cap of edits) expect(can('read_only', cap)).toBe(false)
  })
})

describe('mapeo de estados', () => {
  it('cada estado tiene etiqueta, tono e icono', () => {
    for (const domain of Object.values(STATUS)) {
      for (const style of Object.values(domain)) {
        expect(style.label).not.toBe('')
        expect(style.icon).toBeTruthy()
        expect(['success', 'warning', 'danger', 'info', 'neutral']).toContain(style.tone)
      }
    }
  })

  it('cubre los estados de las máquinas de estado del repo de contratos', () => {
    expect(Object.keys(STATUS.invoice)).toEqual(expect.arrayContaining(['draft', 'issued', 'cancelled']))
    expect(Object.keys(STATUS.hacienda)).toEqual(
      expect.arrayContaining([
        'processing',
        'signed',
        'sent',
        'accepted',
        'rejected',
        'contingency',
        'error',
      ]),
    )
    expect(Object.keys(STATUS.receivable)).toEqual(
      expect.arrayContaining(['open', 'partially_paid', 'paid', 'cancelled']),
    )
    expect(Object.keys(STATUS.payment)).toEqual(['posted', 'voided'])
  })
})
