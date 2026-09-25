import { describe, expect, it } from 'vitest'
import {
  daysBetween,
  daysOverdue,
  formatBusinessDate,
  formatRelative,
  formatInstant,
  formatInstantDate,
  todayIn,
} from './dates'

describe('instantes en la zona de la organización', () => {
  it('muestra la hora de Costa Rica', () => {
    expect(formatInstant('2026-09-24T21:15:00Z')).toBe('24/09/2026 15:15')
  })

  it('un instante cerca de medianoche UTC cae en el día correcto de Costa Rica', () => {
    // 05:30 UTC del 25 son las 23:30 del 24 en Costa Rica (UTC−6).
    expect(formatInstantDate('2026-09-25T05:30:00Z')).toBe('24/09/2026')
    expect(todayIn('America/Costa_Rica', new Date('2026-09-25T05:30:00Z'))).toBe('2026-09-24')
  })

  it('respeta otra zona si la organización la tiene', () => {
    expect(formatInstant('2026-09-24T21:15:00Z', 'America/Panama')).toBe('24/09/2026 16:15')
  })
})

describe('fechas de negocio', () => {
  it('no se corren un día', () => {
    expect(formatBusinessDate('2026-10-24')).toBe('24/10/2026')
    expect(formatBusinessDate('2028-02-29')).toBe('29/02/2028')
  })

  it('rechaza formatos inválidos', () => {
    expect(() => formatBusinessDate('24/10/2026')).toThrow()
  })

  it('cuenta días y atraso', () => {
    expect(daysBetween('2026-09-24', '2026-10-24')).toBe(30)
    expect(daysOverdue('2026-09-24', '2026-09-24')).toBe(0) // vence hoy: al día
    expect(daysOverdue('2026-09-24', '2026-09-30')).toBe(6)
    expect(daysOverdue('2026-10-24', '2026-09-30')).toBe(0)
  })
})

describe('momento relativo', () => {
  const now = new Date('2026-09-24T21:00:00Z') // 15:00 en Costa Rica
  it('usa minutos, hoy, ayer y fecha completa', () => {
    expect(formatRelative('2026-09-24T20:59:40Z', now)).toBe('ahora')
    expect(formatRelative('2026-09-24T20:55:00Z', now)).toBe('hace 5 min')
    expect(formatRelative('2026-09-24T15:12:00Z', now)).toBe('hoy 09:12')
    expect(formatRelative('2026-09-23T22:40:00Z', now)).toBe('ayer 16:40')
    expect(formatRelative('2026-09-20T22:40:00Z', now)).toBe('20/09/2026 16:40')
  })
})
