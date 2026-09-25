import type { CurrentUser, Organization, PortalNotification } from '@/shared/api/types'

// Datos ficticios tomados del prototipo de diseño (design/referencias). Ninguna empresa ni persona es real.

export const USER: CurrentUser = {
  id: 'u1',
  fullName: 'María Rojas Vargas',
  email: 'maria.rojas@correo.example',
  initials: 'MR',
}

const base = { timezone: 'America/Costa_Rica', defaultCurrency: 'CRC' as const }

export const ORGANIZATIONS: Organization[] = [
  {
    id: 'ca',
    legalName: 'Comercial Los Almendros S.A.',
    initials: 'CA',
    identification: 'Jurídica 3-101-900001',
    role: 'admin',
    environment: 'test',
    ...base,
  },
  {
    id: 'cm',
    legalName: 'Café Monteazul S.A.',
    initials: 'CM',
    identification: 'Jurídica 3-101-900288',
    role: 'accountant',
    environment: 'prod',
    ...base,
  },
  {
    id: 'hb',
    legalName: 'Hotel Brisas del Pacífico S.A.',
    initials: 'HB',
    identification: 'Jurídica 3-101-900731',
    role: 'accountant',
    environment: 'prod',
    timezone: 'America/Costa_Rica',
    defaultCurrency: 'USD',
  },
  {
    id: 'fr',
    legalName: 'Ferretería El Roble S.A.',
    initials: 'FR',
    identification: 'Jurídica 3-101-900412',
    role: 'owner',
    environment: 'prod',
    ...base,
  },
  {
    id: 'si',
    legalName: 'Soluciones Ibis S.R.L.',
    initials: 'SI',
    identification: 'Jurídica 3-102-900219',
    role: 'biller',
    environment: 'test',
    ...base,
  },
  {
    id: 'tp',
    legalName: 'Taller Mecánico Los Pinos',
    initials: 'TP',
    identification: 'Física 1-0900-0415',
    role: 'read_only',
    environment: 'prod',
    ...base,
  },
]

export const NOTIFICATIONS: PortalNotification[] = [
  {
    id: 'n1',
    tone: 'success',
    title: 'FAC-0000040 aceptada por Hacienda',
    body: 'Café Monteazul S.A. · ₡452 500,00',
    occurredAt: '2026-09-24T20:55:00Z',
    unread: true,
  },
  {
    id: 'n2',
    tone: 'danger',
    title: 'FAC-0000038 rechazada por Hacienda',
    body: 'Laura Jiménez Solís. Revise el motivo y corrija con una nota de crédito.',
    occurredAt: '2026-09-24T20:38:00Z',
    unread: true,
    action: { label: 'Ver motivo', to: '/facturas/fac-38' },
  },
  {
    id: 'n3',
    tone: 'success',
    title: 'Pago registrado',
    body: 'PAG-0000018 · ₡50 000,00 · Soluciones Ibis S.R.L.',
    occurredAt: '2026-09-24T15:12:00Z',
    unread: false,
  },
  {
    id: 'n4',
    tone: 'success',
    title: 'Cuenta pagada',
    body: 'FAC-0000035 · Martín Ochoa Reyes',
    occurredAt: '2026-09-23T22:40:00Z',
    unread: false,
  },
]

/** La organización de la invitación simulada del prototipo (pantalla 4). */
export const INVITED_ORGANIZATION: Organization = {
  id: 'tc',
  legalName: 'Transportes Cerro Verde S.A.',
  initials: 'TC',
  identification: '3101900555',
  role: 'collector',
  environment: 'prod',
  ...base,
}
