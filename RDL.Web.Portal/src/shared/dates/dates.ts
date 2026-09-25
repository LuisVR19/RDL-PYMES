/**
 * Fechas en el portal (convenciones del repo de contratos §2):
 * - un **instante** (`2026-09-24T15:04:05Z`) se muestra en la zona horaria de la organización;
 * - una **fecha de negocio** (`2026-10-24`) se muestra tal cual, sin pasar por `new Date()`, que la correría un día.
 */
export const DEFAULT_TZ = 'America/Costa_Rica'

const BUSINESS_DATE = /^(\d{4})-(\d{2})-(\d{2})$/

function parts(iso: string, timeZone: string, withTime: boolean) {
  const fmt = new Intl.DateTimeFormat('en-GB', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    ...(withTime ? { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' as const } : {}),
  })
  const p = Object.fromEntries(fmt.formatToParts(new Date(iso)).map((x) => [x.type, x.value]))
  return p as Record<'day' | 'month' | 'year' | 'hour' | 'minute', string>
}

/** `24/09/2026 15:15` en la zona de la organización. */
export function formatInstant(iso: string, timeZone = DEFAULT_TZ): string {
  const p = parts(iso, timeZone, true)
  return `${p.day}/${p.month}/${p.year} ${p.hour}:${p.minute}`
}

/** Solo la fecha de un instante, `24/09/2026`, en la zona de la organización. */
export function formatInstantDate(iso: string, timeZone = DEFAULT_TZ): string {
  const p = parts(iso, timeZone, false)
  return `${p.day}/${p.month}/${p.year}`
}

/** Hora de un instante, `15:15`, en la zona de la organización. */
export function formatInstantTime(iso: string, timeZone = DEFAULT_TZ): string {
  const p = parts(iso, timeZone, true)
  return `${p.hour}:${p.minute}`
}

/** Fecha de negocio `2026-10-24` → `24/10/2026`, sin conversión de zona. */
export function formatBusinessDate(date: string): string {
  const m = BUSINESS_DATE.exec(date)
  if (!m) throw new Error(`Fecha de negocio inválida: ${date}`)
  return `${m[3]}/${m[2]}/${m[1]}`
}

/** Fecha de negocio de hoy en la zona de la organización. */
export function todayIn(timeZone = DEFAULT_TZ, now = new Date()): string {
  const p = parts(now.toISOString(), timeZone, false)
  return `${p.year}-${p.month}-${p.day}`
}

function toUtcDay(date: string): number {
  const m = BUSINESS_DATE.exec(date)
  if (!m) throw new Error(`Fecha de negocio inválida: ${date}`)
  return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3])) / 86_400_000
}

/** Días entre dos fechas de negocio (b − a). Positivo si `b` es posterior. */
export function daysBetween(a: string, b: string): number {
  return toUtcDay(b) - toUtcDay(a)
}

/** Días de atraso de un vencimiento a una fecha de corte; 0 si todavía no vence (vence hoy = al día). */
export function daysOverdue(dueDate: string, asOf: string): number {
  return Math.max(0, daysBetween(dueDate, asOf))
}

/**
 * Momento relativo para notificaciones (prototipo): «ahora», «hace 5 min», «hoy 09:12», «ayer 16:40» y, más atrás,
 * la fecha y hora completas. Se calcula en la zona de la organización.
 */
export function formatRelative(iso: string, now = new Date(), timeZone = DEFAULT_TZ): string {
  const minutes = Math.floor((now.getTime() - new Date(iso).getTime()) / 60_000)
  if (minutes < 1) return 'ahora'
  if (minutes < 60) return `hace ${minutes} min`
  const days = daysBetween(todayIn(timeZone, new Date(iso)), todayIn(timeZone, now))
  if (days === 0) return `hoy ${formatInstantTime(iso, timeZone)}`
  if (days === 1) return `ayer ${formatInstantTime(iso, timeZone)}`
  return formatInstant(iso, timeZone)
}
