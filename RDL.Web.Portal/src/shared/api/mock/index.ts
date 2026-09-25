import { NOTIFICATIONS, ORGANIZATIONS, USER } from '@/mocks/session'
import type { DataSource } from '../ports'
import { simulate } from './simulate'

/** Implementación simulada de los puertos: datos en memoria del prototipo de diseño. */
export const mockDataSource: DataSource = {
  session: {
    currentUser: async () => USER,
    // La sesión no sigue el escenario de revisión: así una lista en «error» no tumba el armazón. El selector de
    // organización (pantalla 2) tendrá su propio método con estados cuando se construya.
    organizations: async () => ORGANIZATIONS,
  },
  notifications: {
    list: () => simulate(NOTIFICATIONS, []),
  },
  shell: {
    inboxAttentionCount: () => simulate(3, 0),
  },
}
