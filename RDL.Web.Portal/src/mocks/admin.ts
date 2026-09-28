import type { Invitation, Member, OrganizationDetail } from '@/shared/api/admin-types'

// Administración del prototipo (ORGDATA0, MEM0, INV0). Ninguna empresa ni persona es real.

/** Datos de la organización activa simulada («Comercial Los Almendros S.A.», id `ca`). */
export const ORGANIZATION_DETAIL: OrganizationDetail = {
  id: 'ca',
  legalName: 'Comercial Los Almendros S.A.',
  tradeName: 'Los Almendros',
  identificationTypeCode: '02',
  identificationNumber: '3-101-900001',
  email: 'info@almendros.example',
  phone: '2222-4500',
  timezone: 'America/Costa_Rica',
  defaultCurrencyCode: 'CRC',
  status: 'active',
  createdAt: '2026-01-15T16:00:00Z',
  updatedAt: '2026-09-01T16:00:00Z',
}

/** `u1` es la usuaria de la sesión simulada (María Rojas, administradora en `ca`). */
export const MEMBERS: Member[] = [
  {
    userId: 'u0',
    fullName: 'Andrés Quesada Mora',
    email: 'andres.quesada@almendros.example',
    roles: ['owner'],
    status: 'active',
    joinedAt: '2026-01-15T16:00:00Z',
  },
  {
    userId: 'u1',
    fullName: 'María Rojas Vargas',
    email: 'maria.rojas@correo.example',
    roles: ['admin'],
    status: 'active',
    joinedAt: '2026-01-20T15:00:00Z',
  },
  {
    userId: 'u2',
    fullName: 'Sofía Chaves León',
    email: 'sofia.chaves@almendros.example',
    roles: ['biller'],
    status: 'active',
    joinedAt: '2026-02-03T15:00:00Z',
  },
  {
    userId: 'u3',
    fullName: 'Diego Arias Solano',
    email: 'diego.arias@almendros.example',
    roles: ['biller'],
    status: 'suspended',
    joinedAt: '2026-03-10T15:00:00Z',
  },
  {
    userId: 'u4',
    fullName: 'Carolina Brenes Ulate',
    email: 'carolina.brenes@contadores.example',
    roles: ['accountant'],
    status: 'active',
    joinedAt: '2026-09-03T15:00:00Z',
  },
]

export const INVITATIONS: Invitation[] = [
  {
    id: 'i1',
    email: 'laura.vindas@almendros.example',
    role: 'biller',
    status: 'pending',
    createdAt: '2026-09-22T15:00:00Z',
    expiresAt: '2026-09-29T15:00:00Z',
  },
  {
    id: 'i2',
    email: 'carolina.brenes@contadores.example',
    role: 'accountant',
    status: 'accepted',
    createdAt: '2026-09-02T15:00:00Z',
    expiresAt: '2026-09-09T15:00:00Z',
  },
  {
    id: 'i3',
    email: 'ana.mena@almendros.example',
    role: 'collector',
    status: 'revoked',
    createdAt: '2026-09-10T15:00:00Z',
    expiresAt: '2026-09-17T15:00:00Z',
  },
  {
    id: 'i4',
    email: 'pablo.solis@almendros.example',
    role: 'read_only',
    status: 'expired',
    createdAt: '2026-09-01T15:00:00Z',
    expiresAt: '2026-09-08T15:00:00Z',
  },
]
