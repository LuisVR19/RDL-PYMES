/**
 * Zonas horarias que ofrece el portal (prototipo «28 Organización»). Platform acepta cualquier zona IANA; si la
 * organización ya tiene otra, la pantalla la muestra con su nombre IANA en vez de cambiarla sin avisar.
 */
export const TIMEZONES = [
  { value: 'America/Costa_Rica', label: 'Costa Rica (UTC−6)', place: 'Costa Rica' },
  { value: 'America/Panama', label: 'Panamá (UTC−5)', place: 'Panamá' },
] as const

/** «hora de Costa Rica»; para una zona que el portal no conoce, su nombre IANA. */
export function timezonePlace(tz: string): string {
  return TIMEZONES.find((z) => z.value === tz)?.place ?? tz
}
