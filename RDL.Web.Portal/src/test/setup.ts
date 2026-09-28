import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Sin `globals` de Vitest, Testing Library no desmonta solo entre pruebas.
afterEach(() => cleanup())

// jsdom no trae ResizeObserver y algunas primitivas de Radix (RadioGroup) lo usan para medir. En las pruebas no
// hay layout que observar: basta con un observador que no hace nada.
if (!('ResizeObserver' in globalThis)) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}

// El router carga los módulos en diferido. En las pruebas se precargan: la primera transformación de un módulo tarda
// más que la espera de `findBy…` y la prueba fallaría por tiempo, no por lo que verifica.
await Promise.all([
  import('@/features/billing/pages'),
  import('@/features/admin/pages'),
  import('@/features/receivables/pages'),
  import('@/features/home/pages/HomePage'),
])
