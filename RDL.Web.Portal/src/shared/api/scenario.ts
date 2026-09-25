import { useSyncExternalStore } from 'react'

/**
 * Escenarios de revisión de la etapa de diseño: permiten ver cada pantalla en sus estados sin backend.
 * Solo existen con la fuente de datos simulada; la barra de desarrollo los cambia. No es parte del producto.
 */
export type Scenario = 'ok' | 'loading' | 'empty' | 'error' | 'partial'

export interface ScenarioState {
  scenario: Scenario
  latencyMs: number
  realtime: 'ok' | 'offline'
}

let state: ScenarioState = { scenario: 'ok', latencyMs: 350, realtime: 'ok' }
const listeners = new Set<() => void>()

export function getScenario(): ScenarioState {
  return state
}

export function setScenario(patch: Partial<ScenarioState>): void {
  state = { ...state, ...patch }
  listeners.forEach((l) => l())
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useScenario(): ScenarioState {
  return useSyncExternalStore(subscribe, getScenario, getScenario)
}
