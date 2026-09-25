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
