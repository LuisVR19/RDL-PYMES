import type { Role } from '@/shared/permissions/permissions'
import type { PageQuery } from './billing-types'

/**
 * Tipos de administración (pantallas 28–31), con la forma de Platform (`openapi/platform.yaml` del repo de
 * contratos): el gateway los reenvía sin cambios. Instantes en UTC; la pantalla los muestra en la zona de la
 * organización.
 */

/** Platform · `Organization` (`GET /v1/organizations/current`). */
export interface OrganizationDetail {
  id: string
  legalName: string
  tradeName?: string
  identificationTypeCode: string
  identificationNumber: string
  email: string
  phone?: string
  timezone: string
  defaultCurrencyCode: string
  status: 'active' | 'suspended' | 'closed'
  createdAt: string
  updatedAt: string
}

/**
 * `PATCH /v1/organizations/current`: campo ausente = sin cambio; "" en `tradeName` o `phone` lo borra. La razón
 * social, la identificación y la moneda por defecto no se editan por esta ruta.
 */
export interface OrganizationPatch {
  tradeName?: string
  email?: string
  phone?: string
  timezone?: string
}

/** Platform · `Member`. V1: un solo rol por membresía (la lista deja lista la API para varios). */
export interface Member {
  userId: string
  email: string
  fullName: string
  status: 'active' | 'suspended'
  roles: Role[]
  joinedAt: string
}

export interface MemberQuery extends PageQuery {
  status?: Member['status']
}

/** `PATCH /v1/organizations/current/users/{userId}`: asignar un rol reemplaza el anterior. */
export type MemberPatch = { role: Role } | { status: Member['status'] }

export type InvitationStatus = 'pending' | 'accepted' | 'revoked' | 'expired'

/** Platform · `Invitation`. `token` solo viene en la primera respuesta del alta (la base guarda su hash). */
export interface Invitation {
  id: string
  email: string
  role: Role
  status: InvitationStatus
  expiresAt: string
  createdAt: string
  token?: string
}

export interface InvitationQuery extends PageQuery {
  status?: InvitationStatus
}

export interface NewInvitation {
  email: string
  role: Role
}

/** `POST /v1/organizations/current/branches`. El código es único por organización e inmutable. */
export interface BranchInput {
  code: string
  name: string
  address?: string
  phone?: string
  email?: string
}

/** `PATCH .../branches/{id}`: el código no se edita; `isActive: false` es la baja lógica. */
export interface BranchPatch {
  name?: string
  address?: string
  phone?: string
  email?: string
  isActive?: boolean
}
