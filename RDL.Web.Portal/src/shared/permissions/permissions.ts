/**
 * Permisos de la interfaz por rol. Es solo experiencia de usuario (ocultar lo que un rol no puede hacer): quien
 * decide es cada API. Refleja la tabla de roles de design/pantallas.md y la matriz de las APIs; es la única matriz
 * del portal, nada de `if (role === ...)` repartidos por las pantallas.
 */
export const ROLES = ['owner', 'admin', 'biller', 'collector', 'accountant', 'read_only'] as const
export type Role = (typeof ROLES)[number]

/** Un rol que llega de la API y el portal conoce (los de `core.roles`). */
export function isRole(value: string): value is Role {
  return (ROLES as readonly string[]).includes(value)
}

export type Capability =
  | 'home.view'
  | 'billing.view'
  | 'billing.edit' // clientes, productos, borradores, emitir, notas
  | 'billing.void' // anular una factura emitida (contrato: owner, admin)
  | 'fiscal.inbox' // bandeja y documento electrónico
  | 'fiscal.config.view'
  | 'fiscal.config.edit'
  | 'fiscal.retry'
  | 'receivables.view'
  | 'receivables.edit' // registrar pagos, aplicar, seguimientos, promesas
  | 'receivables.void' // revertir aplicaciones y anular pagos
  | 'admin.view'
  | 'admin.owners' // asignar o quitar el rol de propietario

const MATRIX: Record<Capability, readonly Role[]> = {
  'home.view': ROLES,
  'billing.view': ['owner', 'admin', 'biller', 'accountant', 'read_only'],
  'billing.edit': ['owner', 'admin', 'biller'],
  'billing.void': ['owner', 'admin'],
  'fiscal.inbox': ['owner', 'admin', 'biller', 'accountant', 'read_only'],
  'fiscal.config.view': ['owner', 'admin', 'accountant'],
  'fiscal.config.edit': ['owner', 'admin'],
  'fiscal.retry': ['owner', 'admin'],
  'receivables.view': ['owner', 'admin', 'collector', 'accountant', 'read_only'],
  'receivables.edit': ['owner', 'admin', 'collector'],
  'receivables.void': ['owner', 'admin'],
  'admin.view': ['owner', 'admin'],
  'admin.owners': ['owner'],
}

export function can(role: Role, capability: Capability): boolean {
  return MATRIX[capability].includes(role)
}

export const ROLE_LABEL: Record<Role, string> = {
  owner: 'Propietario',
  admin: 'Administrador',
  biller: 'Facturador',
  collector: 'Cobrador',
  accountant: 'Contador',
  read_only: 'Solo lectura',
}
