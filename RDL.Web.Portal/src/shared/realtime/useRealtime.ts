import { useEffect, useState } from 'react'
import { setScenario, useScenario } from '@/shared/api/scenario'

const RETRY_EVERY_S = 8

/**
 * Estado de la conexión en tiempo real (notificaciones, pantalla 34). En esta etapa lo decide el escenario
 * simulado; en la de cableado vendrá del canal Realtime de la organización. Sin conexión, el portal sigue
 * funcionando: solo se avisa y se reintenta con cuenta regresiva.
 */
export function useRealtime() {
  const { realtime } = useScenario()
  const offline = realtime === 'offline'
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    if (!offline) return
    const startedAt = Date.now()
    const id = window.setInterval(() => setElapsed(Math.floor((Date.now() - startedAt) / 1000)), 1000)
    return () => {
      window.clearInterval(id)
      setElapsed(0)
    }
  }, [offline])

  return {
    offline,
    secondsToRetry: RETRY_EVERY_S - (elapsed % RETRY_EVERY_S),
    reconnect: () => setScenario({ realtime: 'ok' }),
  }
}
