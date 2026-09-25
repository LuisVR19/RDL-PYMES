/**
 * Aviso de «la sesión venció» (pantalla 35). Lo disparan el cliente HTTP (un 401 del gateway) y la barra de
 * revisión; lo escucha `AuthProvider`, que muestra el diálogo sin sacar al usuario de la página.
 */
type Listener = () => void

const listeners = new Set<Listener>()

export function expireSession(): void {
  for (const l of listeners) l()
}

export function onSessionExpired(listener: Listener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}
